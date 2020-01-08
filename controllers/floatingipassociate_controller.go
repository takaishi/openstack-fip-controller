/*
Copyright 2020 Ryo TAKAISHI.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"github.com/gophercloud/gophercloud"
	"github.com/pkg/errors"
	"github.com/takaishi/openstack-fip-controller-v2/openstack"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"

	errors_ "k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackv1beta1 "github.com/takaishi/openstack-fip-controller-v2/api/v1beta1"
)

var floatingipassociateFinalizerName = "finalizer.floatingipassociate.openstack.repl.info"

// FloatingIPAssociateReconciler reconciles a FloatingIPAssociate object
type FloatingIPAssociateReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	osClient openstack.OpenStackClientInterface
	recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=openstack.repl.info.repl.info,resources=floatingipassociates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info.repl.info,resources=floatingipassociates/status,verbs=get;update;patch

func (r *FloatingIPAssociateReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("floatingipassociate", req.NamespacedName)

	// your logic here

	// Fetch the FloatingIPAssociate instance
	instance := openstackv1beta1.FloatingIPAssociate{}
	err := r.Get(context.TODO(), req.NamespacedName, &instance)
	if err != nil {
		if errors_.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := r.setFinalizer(&instance); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		return r.runFinalizer(&instance, req)
	}

	node := v1.Node{}
	nn := types.NamespacedName{Namespace: "", Name: instance.Spec.Node}
	if err := r.Get(ctx, nn, &node); err != nil {
		return reconcile.Result{}, err
	}

	var portID string
	for _, iface := range node.Status.Addresses {
		if iface.Type == v1.NodeInternalIP {
			if err != nil {
				return reconcile.Result{}, err
			}
			server, err := r.osClient.GetServer(strings.ToLower(node.Status.NodeInfo.SystemUUID))
			if err != nil {
				return reconcile.Result{}, err
			}
			port, err := r.osClient.FindPortByServer(*server)
			if err != nil {
				return reconcile.Result{}, err
			}
			portID = port.ID
		}
	}

	var fip openstackv1beta1.FloatingIP
	if instance.Status.FloatingIP == "" {
		nn := types.NamespacedName{Namespace: req.Namespace, Name: instance.Spec.FloatingIP}
		if err := r.Get(ctx, nn, &fip); err != nil {
			return reconcile.Result{}, err
		}
		if fip.Status.PortID != "" {
			if fip.Status.PortID != portID {
				log.Info("FloatingIP already attached another port", "floatingIP", fip.Status.FloatingIP, "portID", fip.Status.PortID)
				r.WarningEvent(&instance, "info", "FloatingIP %s already attached another port %s", fip.Status.FloatingIP, fip.Status.PortID)

				return reconcile.Result{}, errors.Errorf("FloatingIP %s already attached port %s", fip.Status.FloatingIP, fip.Status.PortID)
			}
		} else {
			r.NormalEvent(&instance, "info", "Attaching FloatingIP %s(%s) to port %s", fip.Status.FloatingIP, fip.Status.ID, portID)
			err := r.osClient.AttachFIP(fip.Status.ID, portID)
			if err != nil {
				log.Info("failed to attach Floating IP")
				r.WarningEvent(&instance, "info", "Faled to attach FloatingIP %s(%s) to port %s", fip.Status.FloatingIP, fip.Status.ID, portID)
				return reconcile.Result{}, err
			}
			r.NormalEvent(&instance, "info", "Attached FloatingIP %s(%s) to port %s", fip.Status.FloatingIP, fip.Status.ID, portID)
			fip.Status.PortID = portID
			if err := r.Status().Update(ctx, &fip); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *FloatingIPAssociateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1beta1.FloatingIPAssociate{}).
		Complete(r)
}

func (r *FloatingIPAssociateReconciler) deleteExternalDependency(instance *openstackv1beta1.FloatingIPAssociate, request reconcile.Request) error {
	ctx := context.Background()

	var fip openstackv1beta1.FloatingIP
	if instance.Status.FloatingIP == "" {
		nn := types.NamespacedName{Namespace: request.Namespace, Name: instance.Spec.FloatingIP}
		if err := r.Get(ctx, nn, &fip); err != nil {
			if errors_.IsNotFound(err) {
				return nil
			}
			return err
		}
	}

	_, err := r.osClient.GetFIP(fip.Status.ID)
	if err != nil {
		switch err.(type) {
		case gophercloud.ErrDefault404:
			return nil
		default:
			return err
		}
	}
	log.Info("Detatching Floating IP...", "ID", instance.Status.FloatingIP, "FloatingIP", instance.Status.FloatingIP)
	err = r.osClient.DetachFIP(fip.Status.ID)
	if err != nil {
		return err
	}
	fip.Status.PortID = ""
	if err := r.Status().Update(ctx, &fip); err != nil {
		return err
	}
	log.Info("Deleted Floating IP", "ID", instance.Status.FloatingIP, "FloatingIP", instance.Status.FloatingIP)

	return nil
}

func (r *FloatingIPAssociateReconciler) setFinalizer(associate *openstackv1beta1.FloatingIPAssociate) error {
	if !containsString(associate.ObjectMeta.Finalizers, floatingipassociateFinalizerName) {
		associate.ObjectMeta.Finalizers = append(associate.ObjectMeta.Finalizers, floatingipassociateFinalizerName)
		if err := r.Update(context.Background(), associate); err != nil {
			return err
		}
	}

	return nil
}

func (r *FloatingIPAssociateReconciler) runFinalizer(associate *openstackv1beta1.FloatingIPAssociate, request reconcile.Request) (reconcile.Result, error) {
	if containsString(associate.ObjectMeta.Finalizers, floatingipassociateFinalizerName) {
		if err := r.deleteExternalDependency(associate, request); err != nil {
			return reconcile.Result{}, err
		}

		associate.ObjectMeta.Finalizers = removeString(associate.ObjectMeta.Finalizers, floatingipassociateFinalizerName)
		if err := r.Update(context.Background(), associate); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *FloatingIPAssociateReconciler) NormalEvent(fip *openstackv1beta1.FloatingIPAssociate, reason string, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(fip, v1.EventTypeNormal, reason, messageFmt, args...)
}

func (r *FloatingIPAssociateReconciler) WarningEvent(fip *openstackv1beta1.FloatingIPAssociate, reason string, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(fip, v1.EventTypeWarning, reason, messageFmt, args...)
}
