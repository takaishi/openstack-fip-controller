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
	"fmt"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	openstackv1beta1 "github.com/takaishi/openstack-fip-controller/pkg/apis/openstack/v1beta1"
	"github.com/takaishi/openstack-fip-controller/pkg/openstack"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
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
var finalizerName = "finalizer.securitygroups.openstack.repl.info"

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
	log.Info("Info", "deleting the external dependencies", instance.Status.ID)

	osClient, err := openstack.NewClient()
	if err != nil {
		return err
	}

	err = osClient.DeleteFIP(instance.Status.ID)
	if err != nil {
		return err
	}

	return nil
}

// Reconcile reads that state of the cluster for a FloatingIP object and makes changes based on the state read
// and what is in the FloatingIP.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingips,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info,resources=floatingips/status,verbs=get;update;patch
func (r *ReconcileFloatingIP) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the FloatingIP instance
	instance := &openstackv1beta1.FloatingIP{}
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

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		log.Info("Debug: deletion timestamp is zero")
		err := r.setFinalizer(instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else {
		err := r.runFinalizer(instance)
		if err != nil {
			return reconcile.Result{}, err

		}
		return reconcile.Result{}, nil
	}

	r.osClient, err = openstack.NewClient()
	if err != nil {
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
		r.setFIPtoNode(node, instance)
	}

	if err := r.Update(context.Background(), instance); err != nil {
		log.Info("Debug", "err", err.Error())
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileFloatingIP) setFIPtoNode(node v1.Node, instance *openstackv1beta1.FloatingIP) (reconcile.Result, error) {
	id := strings.ToLower(node.Status.NodeInfo.SystemUUID)
	server, err := r.osClient.GetServer(id)
	if err != nil {
		return reconcile.Result{}, err
	}

	fixedIPs := getFixedIPByServer(server)
	for _, fixedIP := range fixedIPs {
		fip, err := r.osClient.FindFIP(instance.Spec.Network, fixedIP)
		if err != nil {
			switch err := err.(type) {
			case *openstack.ErrFloatingIPNotFound:
				log.Info("Info: Creating Floating IP...", "network", instance.Spec.Network, "fixed_ip", fixedIP)
				fip, err2 := r.osClient.CreateFIP(instance.Spec.Network, *server)
				if err2 != nil {
					return reconcile.Result{}, err2
				}
				log.Info("Info: Success to create Floating IP", "network", instance.Spec.Network, "fixed_ip", fixedIP, "floating_ip", fip.FloatingIP)
				instance.Status.ID = fip.ID
				if err := r.Update(context.Background(), instance); err != nil {
					log.Info("Debug", "err", err.Error())
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			default:
				log.Info("Debug", "err", err.Error())
				return reconcile.Result{}, err
			}
		}
		log.Info("Info: FloatingIP exists", "network", instance.Spec.Network, "fixed_ip", fixedIP, "floating_ip", fip.FloatingIP)
	}

	return reconcile.Result{}, nil
}

func getFixedIPByServer(server *servers.Server) []string {
	fixedIPs := []string{}

	for _, v := range server.Addresses {
		for _, addr := range v.([]interface{}) {
			if addr.(map[string]interface{})["OS-EXT-IPS:type"].(string) == "fixed" {
				fixedIPs = append(fixedIPs, addr.(map[string]interface{})["addr"].(string))
			}
		}
	}

	return fixedIPs
}

func (r *ReconcileFloatingIP) setFinalizer(fip *openstackv1beta1.FloatingIP) error {
	if !containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		fip.ObjectMeta.Finalizers = append(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			log.Info("Debug", "err", err.Error())
			return err
		}
	}

	return nil
}

func (r *ReconcileFloatingIP) runFinalizer(fip *openstackv1beta1.FloatingIP) error {
	if containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		if err := r.deleteExternalDependency(fip); err != nil {
			return err
		}

		fip.ObjectMeta.Finalizers = removeString(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			return err
		}
	}

	return nil
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

func labelSelector(fip *openstackv1beta1.FloatingIP) string {
	labelSelector := []string{}
	for k, v := range fip.Spec.NodeSelector {
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
func hasKey(dict map[string]string, key string) bool {
	_, ok := dict[key]

	return ok
}
