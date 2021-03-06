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
	"github.com/golang/mock/gomock"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/takaishi/openstack-fip-controller/mock"
	"github.com/takaishi/openstack-fip-controller/pkg/openstack"
	"k8s.io/api/core/v1"
	"testing"
	"time"

	"github.com/onsi/gomega"
	openstackv1beta1 "github.com/takaishi/openstack-fip-controller/pkg/apis/openstack/v1beta1"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	FloatingIPAddress = "127.0.0.1"
	FloatingIPID      = "03bf39a9-e381-4141-9a1a-3c48aa75348c"
	PortID            = "683b5ff0-d930-4f26-b677-43d3eaaa6035"
	ServerID          = "e6871058-54ca-4d4d-aad7-be81fdeeffdc"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var depKey = types.NamespacedName{Name: "foo", Namespace: "default"}

const timeout = time.Second * 5

func newOpenStackClientMock(controller *gomock.Controller) openstack.OpenStackClientInterface {
	fip := floatingips.FloatingIP{ID: FloatingIPID, FloatingIP: FloatingIPAddress}
	server := servers.Server{ID: ServerID}
	port := ports.Port{ID: PortID}
	osClient := mock_openstack.NewMockOpenStackClientInterface(controller)

	osClient.EXPECT().GetServer(ServerID).Return(&server, nil).Times(2)
	osClient.EXPECT().FindPortByServer(server).Return(&port, nil).Times(2)
	osClient.EXPECT().AttachFIP(FloatingIPID, PortID).Return(nil)
	osClient.EXPECT().DetachFIP(FloatingIPID).Return(nil)
	osClient.EXPECT().GetFIP(FloatingIPID).Return(&fip, nil)

	return osClient
}

func newNodeParam() *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-foo",
		},
		Status: v1.NodeStatus{
			Addresses: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: FloatingIPAddress,
				},
			},
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: ServerID,
			},
		},
	}
}

func newFloatingIPParam() *openstackv1beta1.FloatingIP {
	return &openstackv1beta1.FloatingIP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "floatingip-foo",
			Namespace: "default",
		},
		Spec: openstackv1beta1.FloatingIPSpec{
			Network: "test-network",
		},
		Status: openstackv1beta1.FloatingIPStatus{
			ID: FloatingIPID,
		},
	}
}

func newFloatingIPAssociateParam() *openstackv1beta1.FloatingIPAssociate {
	return &openstackv1beta1.FloatingIPAssociate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
		Spec: openstackv1beta1.FloatingIPAssociateSpec{
			Node:       "node-foo",
			FloatingIP: "floatingip-foo",
		},
	}
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	nodeParam := newNodeParam()
	fipParam := newFloatingIPParam()
	fipAssociateParam := newFloatingIPAssociateParam()

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Setup the OpenStack client. Client is mock.
	osClient := newOpenStackClientMock(mockCtrl)

	recFn, requests := SetupTestReconcile(&ReconcileFloatingIPAssociate{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		osClient: osClient,
		recorder: mgr.GetRecorder("floatingipassociate-controller"),
	})
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Setup the object required by FloatingIPAssociate.
	err = c.Create(context.TODO(), nodeParam)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create objecti (Node), got an invalid object error: %v", err)
		return
	}

	// Setup the object required by FloatingIPAssociate.
	err = c.Create(context.TODO(), fipParam)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object (FloatingIP), got an invalid object error: %v", err)
		return
	}

	err = c.Status().Update(context.TODO(), fipParam)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to update object (FloatingIP), got an invalid object error: %v", err)
		return
	}

	// >> Start Test

	// Create the FloatingIPAssociate object and expect the Reconcile and to called OpenStack API.
	err = c.Create(context.TODO(), fipAssociateParam)
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object (FloatingIPAssociate), got an invalid object error: %v", err)
		return
	}

	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), fipAssociateParam)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// Check FloatingIPAssociate object was created.
	fipAssociate := &openstackv1beta1.FloatingIPAssociate{}
	g.Eventually(func() error { return c.Get(context.TODO(), depKey, fipAssociate) }, timeout).
		Should(gomega.Succeed())

	// Delete the FloatingIPAssociate and expect Reconcile to be called for FloatingIPAssociate deletion
	g.Expect(c.Delete(context.TODO(), fipAssociate)).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	// Manually delete Deployment since GC isn't enabled in the test control plane
	g.Eventually(func() error { return c.Delete(context.TODO(), fipAssociate) }, timeout).
		Should(gomega.MatchError("floatingipassociates.openstack.repl.info \"foo\" not found"))

}
