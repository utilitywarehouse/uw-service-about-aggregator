package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/watch"
	"k8s.io/client-go/rest"
	"testing"
)

func TestDiscoveryServicesAddedToChannel(t *testing.T) {
	errors := make(chan error, 10)
	services := make(chan Service, 10)
	d := serviceDiscovery{client: &mockK8Client{}, label: "about=true", res: services, errors: errors}

	go func() {
		d.getServices()
		close(services)
		close(errors)
	}()

	select {
	case <-errors:
		t.Errorf("Should not get an error")

	case s := <-services:
		assert.Equal(t, "someService", s.Name)
		assert.Equal(t, "billing", s.Namespace)
		assert.Equal(t, "http://someService.billing/", s.BaseURL)
	}
}

func TestDiscoveryErrorAddedToChannel(t *testing.T) {
	errors := make(chan error, 10)
	services := make(chan Service, 10)
	d := serviceDiscovery{client: &mockK8Client{}, label: "", res: services, errors: errors}

	go func() {
		d.getServices()
		close(services)
		close(errors)
	}()

	select {
	case err := <-errors:
		assert.EqualError(t, err, "Could not get services via kubernetes api: (No service matching label)")
	case <-services:
		t.Errorf("Should not get any services")
	}
}

type mockK8Client struct {
}

func (m *mockK8Client) Core() v1core.CoreInterface {
	return &mockCoreClient{}
}

type mockCoreClient struct {
}

func (c *mockCoreClient) Namespaces() v1core.NamespaceInterface {
	return &mockNamespaceClient{}
}

func (c *mockCoreClient) Services(namespace string) v1core.ServiceInterface {
	return &mockServiceClient{services: &v1.ServiceList{Items: []v1.Service{v1.Service{ObjectMeta: v1.ObjectMeta{Name: "someService"}}}}}

}

func (c *mockCoreClient) GetRESTClient() *rest.RESTClient {
	return &rest.RESTClient{}
}

func (c *mockCoreClient) ComponentStatuses() v1core.ComponentStatusInterface {
	return nil
}

func (c *mockCoreClient) ConfigMaps(namespace string) v1core.ConfigMapInterface {
	return nil
}

func (c *mockCoreClient) Endpoints(namespace string) v1core.EndpointsInterface {
	return nil
}

func (c *mockCoreClient) Events(namespace string) v1core.EventInterface {
	return nil
}

func (c *mockCoreClient) LimitRanges(namespace string) v1core.LimitRangeInterface {
	return nil
}

func (c *mockCoreClient) Nodes() v1core.NodeInterface {
	return nil
}

func (c *mockCoreClient) PersistentVolumes() v1core.PersistentVolumeInterface {
	return nil
}

func (c *mockCoreClient) PersistentVolumeClaims(namespace string) v1core.PersistentVolumeClaimInterface {
	return nil
}

func (c *mockCoreClient) Pods(namespace string) v1core.PodInterface {
	return nil
}

func (c *mockCoreClient) PodTemplates(namespace string) v1core.PodTemplateInterface {
	return nil
}

func (c *mockCoreClient) ReplicationControllers(namespace string) v1core.ReplicationControllerInterface {
	return nil
}

func (c *mockCoreClient) ResourceQuotas(namespace string) v1core.ResourceQuotaInterface {
	return nil
}

func (c *mockCoreClient) Secrets(namespace string) v1core.SecretInterface {
	return nil
}

func (c *mockCoreClient) ServiceAccounts(namespace string) v1core.ServiceAccountInterface {
	return nil
}

type mockNamespaceClient struct {
}

func (n *mockNamespaceClient) List(opts v1.ListOptions) (*v1.NamespaceList, error) {
	return &v1.NamespaceList{Items: []v1.Namespace{
		v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "billing"}}}}, nil
}

func (n *mockNamespaceClient) Create(*v1.Namespace) (*v1.Namespace, error) {
	return &v1.Namespace{}, nil
}
func (n *mockNamespaceClient) Update(*v1.Namespace) (*v1.Namespace, error) {
	return &v1.Namespace{}, nil
}
func (n *mockNamespaceClient) UpdateStatus(*v1.Namespace) (*v1.Namespace, error) {
	return &v1.Namespace{}, nil
}
func (n *mockNamespaceClient) Delete(name string, options *v1.DeleteOptions) error {
	return nil
}
func (n *mockNamespaceClient) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return nil
}
func (n *mockNamespaceClient) Get(name string) (*v1.Namespace, error) {
	return &v1.Namespace{}, nil
}
func (n *mockNamespaceClient) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return nil, nil
}
func (n *mockNamespaceClient) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Namespace, err error) {
	return &v1.Namespace{}, nil
}

func (n *mockNamespaceClient) Finalize(item *v1.Namespace) (*v1.Namespace, error) {
	return &v1.Namespace{}, nil
}

type mockServiceClient struct {
	services *v1.ServiceList
}

func (s *mockServiceClient) List(opts v1.ListOptions) (*v1.ServiceList, error) {
	expectedOpts := v1.ListOptions{LabelSelector: "about=true"}
	if opts == expectedOpts {
		return s.services, nil
	}
	return &v1.ServiceList{}, fmt.Errorf("No service matching label")
}

func (c *mockServiceClient) Create(service *v1.Service) (result *v1.Service, err error) {
	return &v1.Service{}, nil
}

func (c *mockServiceClient) Update(service *v1.Service) (result *v1.Service, err error) {
	return &v1.Service{}, nil
}

func (c *mockServiceClient) UpdateStatus(service *v1.Service) (result *v1.Service, err error) {
	return &v1.Service{}, nil
}

func (c *mockServiceClient) Delete(name string, options *v1.DeleteOptions) error {
	return nil
}

func (c *mockServiceClient) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return nil
}

func (c *mockServiceClient) Get(name string) (result *v1.Service, err error) {
	return &v1.Service{}, nil
}

func (c *mockServiceClient) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return nil, nil
}

func (c *mockServiceClient) Patch(name string, pt api.PatchType, data []byte, subresources ...string) (result *v1.Service, err error) {
	return &v1.Service{}, nil
}

func (c *mockServiceClient) ProxyGet(scheme, name, port, path string, params map[string]string) rest.ResponseWrapper {
	return nil
}
