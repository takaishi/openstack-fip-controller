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
	"fmt"
	v1 "k8s.io/api/core/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	errors_ "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"math/rand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackv1beta1 "github.com/takaishi/openstack-fip-controller/api/v1beta1"
)

var floatingipsetFinalizerName = "finalizer.floatingipset.openstack.repl.info"

const (
	randomLength = 5
)

// FloatingIPSetReconciler reconciles a FloatingIPSet object
type FloatingIPSetReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=openstack.repl.info.repl.info,resources=floatingipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=openstack.repl.info.repl.info,resources=floatingipsets/status,verbs=get;update;patch

func (r *FloatingIPSetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("floatingipset", req.NamespacedName)

	// Fetch the FloatingIPSet instance
	instance := openstackv1beta1.FloatingIPSet{}
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

	nodes := v1.NodeList{}
	ls, err := convertLabelSelectorToLabelsSelector(labelSelector(&instance))
	if err != nil {
		return reconcile.Result{}, err
	}

	listOpts := client.ListOptions{
		LabelSelector: ls,
	}
	err = r.List(ctx, &nodes, &listOpts)
	if err != nil {
		log.Info("Error", "Failed to NodeList", err.Error())
		return reconcile.Result{}, err
	}

	for _, node := range nodes.Items {
		if !containsString(instance.Status.Nodes, node.Name) {
			if err := r.AttachFIPToNode(&instance, node, req); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	for _, name := range instance.Status.Nodes {
		exist := false
		for _, node := range nodes.Items {
			if name == node.Name {
				exist = true
			}
		}
		if !exist {
			var associate openstackv1beta1.FloatingIPAssociate

			nn := types.NamespacedName{Namespace: req.Namespace, Name: name}
			if err := r.Get(ctx, nn, &associate); err != nil {
				if errors_.IsNotFound(err) {
					instance.Status.Nodes = removeString(instance.Status.Nodes, name)
					break
				}
				return reconcile.Result{}, err
			}
			r.NormalEvent(&instance, "info", "Deleting FloatingIPAssociate %s", name)
			log.Info("Deleting FloatingIPAssociate...", "Name", name)
			if err := r.Delete(ctx, &associate); err != nil {
				return reconcile.Result{}, err
			}
			log.Info("Deleted FloatingIPAssociate", "Name", name)
			r.NormalEvent(&instance, "info", "Deleted FloatingIPAssociate %s", name)

			var fip openstackv1beta1.FloatingIP
			nn = types.NamespacedName{Namespace: req.Namespace, Name: associate.Spec.FloatingIP}
			if err := r.Get(ctx, nn, &fip); err != nil {
				return reconcile.Result{}, err
			}
			r.NormalEvent(&instance, "info", "Deleting FloatingIP %s", name)
			log.Info("Deleting FloatingIP...", "Name", name)
			if err := r.Delete(ctx, &fip); err != nil {
				return reconcile.Result{}, err
			}
			r.NormalEvent(&instance, "info", "Deleted FloatingIP %s", name)
			log.Info("Deleted FloatingIP", "Name", name)

			instance.Status.Nodes = removeString(instance.Status.Nodes, name)
		}
	}

	if err := r.Status().Update(ctx, &instance); err != nil {
		log.Error(err, "Unable to update FloatingIPSet status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *FloatingIPSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openstackv1beta1.FloatingIPSet{}).
		Complete(r)
}

func (r *FloatingIPSetReconciler) deleteExternalDependency(instance *openstackv1beta1.FloatingIPSet, req reconcile.Request) error {
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

func (r *FloatingIPSetReconciler) setFinalizer(fip *openstackv1beta1.FloatingIPSet) error {
	if !containsString(fip.ObjectMeta.Finalizers, finalizerName) {
		fip.ObjectMeta.Finalizers = append(fip.ObjectMeta.Finalizers, finalizerName)
		if err := r.Update(context.Background(), fip); err != nil {
			return err
		}
	}

	return nil
}

func (r *FloatingIPSetReconciler) runFinalizer(fip *openstackv1beta1.FloatingIPSet, req reconcile.Request) (reconcile.Result, error) {
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

func (r *FloatingIPSetReconciler) AttachFIPToNode(instance *openstackv1beta1.FloatingIPSet, node v1.Node, request reconcile.Request) error {
	ctx := context.Background()
	var associate openstackv1beta1.FloatingIPAssociate

	nn := types.NamespacedName{Namespace: request.Namespace, Name: node.Name}
	if err := r.Get(ctx, nn, &associate); err != nil {
		if errors_.IsNotFound(err) {
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
				return err
			}
			r.NormalEvent(instance, "info", "Created FloatingIP %s", name)

			associate.ObjectMeta = metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        node.Name,
				Namespace:   request.Namespace,
			}
			associate.Spec.Node = node.Name
			associate.Spec.FloatingIP = fip.Name
			if err := r.Create(ctx, &associate); err != nil {
				return err
			}
			r.NormalEvent(instance, "info", "Created FloatingIPAssociate %s", node.Name)
		} else {
			return err
		}
	}
	instance.Status.Nodes = append(instance.Status.Nodes, node.Name)

	return nil
}

func (r *FloatingIPSetReconciler) NormalEvent(fip *openstackv1beta1.FloatingIPSet, reason string, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(fip, v1.EventTypeNormal, reason, messageFmt, args...)
}

func (r *FloatingIPSetReconciler) WarningEvent(fip *openstackv1beta1.FloatingIPSet, reason string, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(fip, v1.EventTypeWarning, reason, messageFmt, args...)
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

func convertLabelSelectorToLabelsSelector(selector string) (labels.Selector, error) {
	s, err := metav1.ParseToLabelSelector(selector)
	if err != nil {
		return nil, err
	}

	return metav1.LabelSelectorAsSelector(s)
}
