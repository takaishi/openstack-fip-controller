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
	"github.com/takaishi/openstack-fip-controller-v2/openstack"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	openstackv1beta1 "github.com/takaishi/openstack-fip-controller-v2/api/v1beta1"
)

var log = logf.Log.WithName("controller")
var finalizerName = "finalizer.floatingip.openstack.repl.info"

// FloatingIPReconciler reconciles a FloatingIP object
type FloatingIPReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	osClient openstack.OpenStackClientInterface
	recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=openstack.repl.info.repl.info,resources=floatingips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info.repl.info,resources=floatingips/status,verbs=get;update;patch

func (r *FloatingIPReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("floatingip", req.NamespacedName)

	// your logic here

	// Fetch the FloatingIP instance
	var instance openstackv1beta1.FloatingIP
	err := r.Get(ctx, req.NamespacedName, &instance)
	if err != nil {
		if errors.IsNotFound(err) {
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
		return r.runFinalizer(&instance)
	}

	if instance.Status.ID == "" {
		log.Info("Creating Floating IP...", "network", instance.Spec.Network)
		r.NormalEvent(&instance, "info", "Creating FloatingIP to network %s ...", instance.Spec.Network)
		fip, err := r.osClient.CreateFIP(instance.Spec.Network)
		if err != nil {
			return reconcile.Result{}, err
		}
		log.Info("Created Floating IP", "ID", fip.ID, "FloatingIP", fip.FloatingIP)
		r.NormalEvent(&instance, "info", "Created FloatingIP %s (%s)", fip.FloatingIP, fip.ID)
		instance.Status.ID = fip.ID
		instance.Status.FloatingIP = fip.FloatingIP
	}

	if err := r.Status().Update(ctx, &instance); err != nil {
		log.Error(err, "Unable to update FloatingIP status")
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *FloatingIPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1beta1.FloatingIP{}).
		Complete(r)
}

func (r *FloatingIPReconciler) deleteExternalDependency(instance *openstackv1beta1.FloatingIP) error {
	_, err := r.osClient.GetFIP(instance.Status.ID)
	if err != nil {
		switch err.(type) {
		case gophercloud.ErrDefault404:
			return nil
		default:
			log.Error(err, "Unable to get FloatingIP")
			return err
		}
	}
	log.Info("Deleting Floating IP...", "ID", instance.Status.ID, "FloatingIP", instance.Status.FloatingIP)
	err = r.osClient.DeleteFIP(instance.Status.ID)
	if err != nil {
		return err
	}
	log.Info("Deleted Floating IP", "ID", instance.Status.ID, "FloatingIP", instance.Status.FloatingIP)

	return nil
}

func (r *FloatingIPReconciler) setFinalizer(fip *openstackv1beta1.FloatingIP) error {
	if !containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		fip.ObjectMeta.Finalizers = append(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			return err
		}
	}

	return nil
}

func (r *FloatingIPReconciler) runFinalizer(fip *openstackv1beta1.FloatingIP) (reconcile.Result, error) {
	if containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		if err := r.deleteExternalDependency(fip); err != nil {
			return reconcile.Result{}, err
		}

		fip.ObjectMeta.Finalizers = removeString(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *FloatingIPReconciler) NormalEvent(fip *openstackv1beta1.FloatingIP, reason string, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(fip, v1.EventTypeNormal, reason, messageFmt, args...)
}

func (r *FloatingIPReconciler) WarningEvent(fip *openstackv1beta1.FloatingIP, reason string, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(fip, v1.EventTypeWarning, reason, messageFmt, args...)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
