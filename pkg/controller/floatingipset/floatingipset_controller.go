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
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"strings"
	"time"

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
var finalizerName = "finalizer.floatingipset.openstack.repl.info"

const (
	randomLength = 5
)

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

func (r *ReconcileFloatingIPSet) deleteExternalDependency(instance *openstackv1beta1.FloatingIPSet, req reconcile.Request) error {
	ctx := context.Background()
	log.Info("Deleting External Dependency...")

	for _, node := range instance.Status.Nodes {
		var associate openstackv1beta1.FloatingIPAssociate

		nn := types.NamespacedName{Namespace: req.Namespace, Name: node}
		if err := r.Get(ctx, nn, &associate); err != nil {
			return err
		}

		log.Info("Deleting FloatingIPAssociate...", "Name", node)
		if err := r.Delete(ctx, &associate); err != nil {
			return err
		}
		log.Info("Deleted FloatingIPAssociate", "Name", node)

		var fip openstackv1beta1.FloatingIP
		nn = types.NamespacedName{Namespace: req.Namespace, Name: associate.Spec.FloatingIP}
		if err := r.Get(ctx, nn, &fip); err != nil {
			return err

		}
		log.Info("Deleting FloatingIP...", "Name", node)
		if err := r.Delete(ctx, &fip); err != nil {
			return err
		}
		log.Info("Deleted FloatingIP", "Name", node)

	}
	log.Info("Deleted External Dependency...")

	return nil
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
	ctx := context.Background()

	// Fetch the FloatingIPSet instance
	instance := openstackv1beta1.FloatingIPSet{}
	err := r.Get(context.TODO(), request.NamespacedName, &instance)
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
		return r.runFinalizer(&instance, request)
	}

	clientset, err := kubeClient()
	if err != nil {
		log.Info("Error", "Failed to create kubeClient", err.Error())
		return reconcile.Result{}, err
	}

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: labelSelector(&instance)})
	if err != nil {
		log.Info("Error", "Failed to NodeList", err.Error())
		return reconcile.Result{}, err
	}

	for _, node := range nodes.Items {
		if !containsString(instance.Status.Nodes, node.Name) {
			var associate openstackv1beta1.FloatingIPAssociate

			nn := types.NamespacedName{Namespace: request.Namespace, Name: node.Name}
			if err := r.Get(ctx, nn, &associate); err != nil {
				if errors.IsNotFound(err) {
					rand.Seed(time.Now().Unix())

					name := fmt.Sprintf("floatingip-%s", utilrand.String(randomLength))

					fip := openstackv1beta1.FloatingIP{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      make(map[string]string),
							Annotations: make(map[string]string),
							Name:        name,
							Namespace:   request.Namespace,
						},
						Spec: openstackv1beta1.FloatingIPSpec{
							Network: instance.Spec.Network,
						},
					}
					if err := r.Create(ctx, &fip); err != nil {
						return reconcile.Result{}, err
					}

					associate.ObjectMeta = metav1.ObjectMeta{
						Labels:      make(map[string]string),
						Annotations: make(map[string]string),
						Name:        node.Name,
						Namespace:   request.Namespace,
					}
					associate.Spec.Node = node.Name
					associate.Spec.FloatingIP = fip.Name
					if err := r.Create(ctx, &associate); err != nil {
						return reconcile.Result{}, err
					}
				} else {
					return reconcile.Result{}, err
				}
			}
			instance.Status.Nodes = append(instance.Status.Nodes, node.Name)
		}
	}

	if err := r.Status().Update(ctx, &instance); err != nil {
		log.Error(err, "Unable to update FloatingIPSet status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileFloatingIPSet) setFinalizer(fip *openstackv1beta1.FloatingIPSet) error {
	if !containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		fip.ObjectMeta.Finalizers = append(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileFloatingIPSet) runFinalizer(fip *openstackv1beta1.FloatingIPSet, req reconcile.Request) (reconcile.Result, error) {
	if containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		if err := r.deleteExternalDependency(fip, req); err != nil {
			return reconcile.Result{}, err
		}

		fip.ObjectMeta.Finalizers = removeString(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			return reconcile.Result{}, err
		}
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
