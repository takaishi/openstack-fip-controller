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

package floatingipset

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"strings"

	openstackv1beta1 "github.com/takaishi/openstack-fip-controller/pkg/apis/openstack/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new FloatingIPSet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileFloatingIPSet{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("floatingipset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to FloatingIPSet
	err = c.Watch(&source.Kind{Type: &openstackv1beta1.FloatingIPSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by FloatingIPSet - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &openstackv1beta1.FloatingIPSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileFloatingIPSet{}

// ReconcileFloatingIPSet reconciles a FloatingIPSet object
type ReconcileFloatingIPSet struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a FloatingIPSet object and makes changes based on the state read
// and what is in the FloatingIPSet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingipsets/status,verbs=get;update;patch
func (r *ReconcileFloatingIPSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the FloatingIPSet instance
	instance := &openstackv1beta1.FloatingIPSet{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	clientset, err := kubeClient()
	if err != nil {
		log.Info("Error", "Failed to create kubeClient", err.Error())
		return reconcile.Result{}, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: labelSelector(instance)})
	if err != nil {
		log.Info("Error", "Failed to NodeList", err.Error())
		return reconcile.Result{}, err
	}

	for _, node := range nodes.Items {
		fmt.Println(node.Name)
	}

	return reconcile.Result{}, nil
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

func labelSelector(set *openstackv1beta1.FloatingIPSet) string {
	labelSelector := []string{}
	for k, v := range set.Spec.NodeSelector {
		if k == "role" {
			labelSelector = append(labelSelector, fmt.Sprintf("node-role.kubernetes.io/%s", v))
		} else {
			labelSelector = append(labelSelector, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return strings.Join(labelSelector, ",")
}
