module github.com/takaishi/openstack-fip-controller

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/golang/mock v1.3.1
	github.com/gophercloud/gophercloud v0.1.0
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pkg/errors v0.8.1
	golang.org/x/tools v0.0.0-20200107184032-11e9d9cc0042 // indirect
	k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	sigs.k8s.io/controller-runtime v0.4.0
)
