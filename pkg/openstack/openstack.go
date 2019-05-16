package openstack

import (
	"crypto/tls"
	"fmt"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gophercloud/gophercloud"
	_openstack "github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/identity/v2/tenants"
	"github.com/gophercloud/gophercloud/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/pkg/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("controller")

type OpenStackClient struct {
	providerClient *gophercloud.ProviderClient
	regionName     string
}

func NewClient() (*OpenStackClient, error) {
	client := OpenStackClient{}
	client.regionName = os.Getenv("OS_REGION_NAME")
	cert := os.Getenv("OS_CERT")
	key := os.Getenv("OS_KEY")

	authOpts, err := _openstack.AuthOptionsFromEnv()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create AuthOptions from env")
	}

	client.providerClient, err = _openstack.NewClient(authOpts.IdentityEndpoint)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{}
	if cert != "" && key != "" {
		clientCert, err := ioutil.ReadFile(cert)
		if err != nil {
			return nil, err
		}
		clientKey, err := ioutil.ReadFile(key)
		if err != nil {
			return nil, err
		}
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.BuildNameToCertificate()
		transport := &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: tlsConfig}

		client.providerClient.HTTPClient.Transport = transport
	}

	err = _openstack.Authenticate(client.providerClient, authOpts)
	if err != nil {
		return nil, err
	}

	return &client, nil
}

func (client *OpenStackClient) GetTenant(id string) (tenants.Tenant, error) {
	identityClient, err := _openstack.NewIdentityV3(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return tenants.Tenant{}, err
	}

	res := tenants.Get(identityClient, id)
	if res.Err != nil {
		return tenants.Tenant{}, res.Err
	}

	t, err := res.Extract()
	if err != nil {
		return tenants.Tenant{}, err
	}

	return *t, nil
}

func (client *OpenStackClient) GetTenantByName(name string) (projects.Project, error) {
	identityClient, err := _openstack.NewIdentityV3(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return projects.Project{}, err
	}

	listOpts := projects.ListOpts{}
	resp := []projects.Project{}
	projects.List(identityClient, &listOpts).EachPage(func(page pagination.Page) (bool, error) {
		extracted, err := projects.ExtractProjects(page)
		if err != nil {
			return false, err
		}

		for _, item := range extracted {
			if item.Name == name {
				resp = append(resp, item)
			}
		}

		return true, nil
	})

	if len(resp) == 0 {
		return projects.Project{}, errors.New(fmt.Sprintf("Cound not found tenant '%s'", name))
	}

	if len(resp) > 1 {
		return projects.Project{}, errors.New(fmt.Sprintf("Found some tenant has same name '%s'", name))
	}
	return resp[0], nil
}

func (client *OpenStackClient) GetServer(id string) (*servers.Server, error) {
	computeClient, err := _openstack.NewComputeV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return nil, err
	}

	return servers.Get(computeClient, id).Extract()
}

func (client *OpenStackClient) GetNetworkByName(name string) (networks.Network, error) {
	networkClient, err := _openstack.NewNetworkV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return networks.Network{}, err
	}

	listOpts := networks.ListOpts{Name: name}
	resp := []networks.Network{}
	networks.List(networkClient, &listOpts).EachPage(func(page pagination.Page) (bool, error) {
		extracted, err := networks.ExtractNetworks(page)
		if err != nil {
			return false, err
		}

		for _, item := range extracted {
			if item.Name == name {
				resp = append(resp, item)
			}
		}

		return true, nil
	})

	if len(resp) == 0 {
		return networks.Network{}, errors.New(fmt.Sprintf("Cound not found network '%s'", name))
	}

	if len(resp) > 1 {
		return networks.Network{}, errors.New(fmt.Sprintf("Found some network has same name '%s'", name))
	}
	return resp[0], nil
}

func (client *OpenStackClient) FindFIP(networkName string, fixedIP string) (floatingips.FloatingIP, error) {
	networkClient, err := _openstack.NewNetworkV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return floatingips.FloatingIP{}, err
	}

	network, err := client.GetNetworkByName(networkName)
	if err != nil {
		return floatingips.FloatingIP{}, err
	}
	listOpts := floatingips.ListOpts{
		FloatingNetworkID: network.ID,
		FixedIP:           fixedIP,
	}
	resp := []floatingips.FloatingIP{}
	floatingips.List(networkClient, &listOpts).EachPage(func(page pagination.Page) (bool, error) {
		extracted, err := floatingips.ExtractFloatingIPs(page)
		if err != nil {
			return false, err
		}

		for _, item := range extracted {
			resp = append(resp, item)
		}

		return true, nil
	})

	if len(resp) == 0 {
		return floatingips.FloatingIP{}, &ErrFloatingIPNotFound{FixedIP: fixedIP, NetworkName: networkName}
	}

	if len(resp) > 1 {
		return floatingips.FloatingIP{}, errors.New(fmt.Sprintf("Found some floatingIP has same name attached %s in %s", fixedIP, networkName))
	}
	return resp[0], nil
}

type ErrFloatingIPNotFound struct {
	FixedIP     string
	NetworkName string
}

func (e ErrFloatingIPNotFound) Error() string {
	return fmt.Sprintf("Unable to find FloatingIP with FixedIP %s and NetworkName %s", e.FixedIP, e.NetworkName)
}

func (client *OpenStackClient) CreateFIP(networkName string) (*floatingips.FloatingIP, error) {
	networkClient, err := _openstack.NewNetworkV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return nil, err
	}
	network, err := client.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	createOpts := floatingips.CreateOpts{FloatingNetworkID: network.ID}
	return floatingips.Create(networkClient, createOpts).Extract()
}

func (client *OpenStackClient) AttachFIP(id string, portID string) error {
	networkClient, err := _openstack.NewNetworkV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return err
	}

	updateOpts := floatingips.UpdateOpts{PortID: &portID}
	return floatingips.Update(networkClient, id, updateOpts).Err
}

func (client *OpenStackClient) DetachFIP(id string) error {
	portID := ""
	networkClient, err := _openstack.NewNetworkV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return err
	}

	updateOpts := floatingips.UpdateOpts{PortID: &portID}
	return floatingips.Update(networkClient, id, updateOpts).Err
}

func (client *OpenStackClient) FindPortByServer(server servers.Server) (*ports.Port, error) {
	networkClient, err := _openstack.NewNetworkV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return nil, err
	}

	listOpts := ports.ListOpts{
		DeviceID: server.ID,
	}
	resp := []ports.Port{}
	ports.List(networkClient, &listOpts).EachPage(func(page pagination.Page) (bool, error) {
		extracted, err := ports.ExtractPorts(page)
		if err != nil {
			return false, err
		}

		for _, item := range extracted {
			resp = append(resp, item)
		}

		return true, nil
	})

	if len(resp) == 0 {
		return nil, errors.New(fmt.Sprintf("Unable to find Port with DeviceID %s", server.ID))
	}

	if len(resp) > 1 {
		return nil, errors.New(fmt.Sprintf("Found some Port with DeviceID %s", server.ID))
	}

	return &resp[0], nil
}

func (client *OpenStackClient) DeleteFIP(id string) error {
	networkClient, err := _openstack.NewNetworkV2(client.providerClient, gophercloud.EndpointOpts{Region: client.regionName})
	if err != nil {
		return err
	}

	res := floatingips.Delete(networkClient, id)
	if res.Err != nil {
		return res.Err
	}

	return nil
}
