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

package floatingip

import (
	"context"
	openstackv1beta1 "github.com/takaishi/openstack-fip-controller/pkg/apis/openstack/v1beta1"
	"github.com/takaishi/openstack-fip-controller/pkg/openstack"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller")
var finalizerName = "finalizer.floatingip.openstack.repl.info"

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new FloatingIP Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileFloatingIP{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("floatingip-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to FloatingIP
	err = c.Watch(&source.Kind{Type: &openstackv1beta1.FloatingIP{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by FloatingIP - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &openstackv1beta1.FloatingIP{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileFloatingIP{}

// ReconcileFloatingIP reconciles a FloatingIP object
type ReconcileFloatingIP struct {
	client.Client
	scheme   *runtime.Scheme
	osClient *openstack.OpenStackClient
}

func (r *ReconcileFloatingIP) deleteExternalDependency(instance *openstackv1beta1.FloatingIP) error {
	osClient, err := openstack.NewClient()
	if err != nil {
		return err
	}

	log.Info("Deleting Floating IP...", "ID", instance.Status.ID, "FloatingIP", instance.Status.FloatingIP)
	err = osClient.DeleteFIP(instance.Status.ID)
	if err != nil {
		return err
	}
	log.Info("Deleted Floating IP", "ID", instance.Status.ID, "FloatingIP", instance.Status.FloatingIP)

	return nil
}

// Reconcile reads that state of the cluster for a FloatingIP object and makes changes based on the state read
// and what is in the FloatingIP.Spec
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingips/status,verbs=get;update;patch
func (r *ReconcileFloatingIP) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	// Fetch the FloatingIP instance
	var instance openstackv1beta1.FloatingIP
	err := r.Get(ctx, request.NamespacedName, &instance)
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

	r.osClient, err = openstack.NewClient()
	if err != nil {
		return reconcile.Result{}, err
	}

	if instance.Status.ID == "" {
		log.Info("Creating Floating IP...", "network", instance.Spec.Network)
		fip, err := r.osClient.CreateFIP(instance.Spec.Network)
		if err != nil {
			return reconcile.Result{}, err
		}
		log.Info("Created Floating IP", "ID", fip.ID, "FloatingIP", fip.FloatingIP)
		instance.Status.ID = fip.ID
		instance.Status.FloatingIP = fip.FloatingIP
	}

	if err := r.Status().Update(ctx, &instance); err != nil {
		log.Error(err, "Unable to update FloatingIP status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
func (r *ReconcileFloatingIP) setFinalizer(fip *openstackv1beta1.FloatingIP) error {
	if !containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		fip.ObjectMeta.Finalizers = append(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileFloatingIP) runFinalizer(fip *openstackv1beta1.FloatingIP) (reconcile.Result, error) {
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
