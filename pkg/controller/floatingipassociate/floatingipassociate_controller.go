/*
Copyright 2019 TAKAISHI Ryo.

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

package floatingipassociate

import (
	"context"
	"github.com/gophercloud/gophercloud"
	"github.com/pkg/errors"
	openstackv1beta1 "github.com/takaishi/openstack-fip-controller/pkg/apis/openstack/v1beta1"
	"github.com/takaishi/openstack-fip-controller/pkg/openstack"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	errors_ "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
)

var log = logf.Log.WithName("controller")
var finalizerName = "finalizer.floatingipassociate.openstack.repl.info"

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new FloatingIPAssociate Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	osClient, err := openstack.NewClient()
	if err != nil {
		return err
	}

	return add(mgr, newReconciler(mgr, osClient))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, osClient openstack.OpenStackClientInterface) reconcile.Reconciler {
	return &ReconcileFloatingIPAssociate{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		osClient: osClient,
		recorder: mgr.GetRecorder("floatingipassociate-controller"),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("floatingipassociate-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to FloatingIPAssociate
	err = c.Watch(&source.Kind{Type: &openstackv1beta1.FloatingIPAssociate{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by FloatingIPAssociate - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &openstackv1beta1.FloatingIPAssociate{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileFloatingIPAssociate{}

// ReconcileFloatingIPAssociate reconciles a FloatingIPAssociate object
type ReconcileFloatingIPAssociate struct {
	client.Client
	scheme   *runtime.Scheme
	osClient openstack.OpenStackClientInterface
	recorder record.EventRecorder
}

func (r *ReconcileFloatingIPAssociate) deleteExternalDependency(instance *openstackv1beta1.FloatingIPAssociate, request reconcile.Request) error {
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

// Reconcile reads that state of the cluster for a FloatingIPAssociate object and makes changes based on the state read
// and what is in the FloatingIPAssociate.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingipassociates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingipassociates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=event,verbs=create
func (r *ReconcileFloatingIPAssociate) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	// Fetch the FloatingIPAssociate instance
	instance := openstackv1beta1.FloatingIPAssociate{}
	err := r.Get(context.TODO(), request.NamespacedName, &instance)
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
		return r.runFinalizer(&instance, request)
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
		nn := types.NamespacedName{Namespace: request.Namespace, Name: instance.Spec.FloatingIP}
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

	return reconcile.Result{}, nil
}

func (r *ReconcileFloatingIPAssociate) setFinalizer(associate *openstackv1beta1.FloatingIPAssociate) error {
	if !containsString(associate.ObjectMeta.Finalizers, finalizerName) {
		associate.ObjectMeta.Finalizers = append(associate.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), associate); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileFloatingIPAssociate) runFinalizer(associate *openstackv1beta1.FloatingIPAssociate, request reconcile.Request) (reconcile.Result, error) {
	if containsString(associate.ObjectMeta.Finalizers, finalizerName) {
		if err := r.deleteExternalDependency(associate, request); err != nil {
			return reconcile.Result{}, err
		}

		associate.ObjectMeta.Finalizers = removeString(associate.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), associate); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileFloatingIPAssociate) NormalEvent(fip *openstackv1beta1.FloatingIPAssociate, reason string, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(fip, v1.EventTypeNormal, reason, messageFmt, args...)
}

func (r *ReconcileFloatingIPAssociate) WarningEvent(fip *openstackv1beta1.FloatingIPAssociate, reason string, messageFmt string, args ...interface{}) {
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

func kubeClient() (*kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Info("Error", "Failed to get config", err.Error())
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Info("Error", "Failed to NewForConfig", err.Error())
		return nil, err
	}

	return clientset, nil
}
