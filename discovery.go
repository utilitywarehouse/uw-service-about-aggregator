package main

import (
	"fmt"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"net"
)

type serviceDiscovery struct {
	client kubernetesClient
	label  string
	res    chan<- service
	errors chan<- error
}

type kubernetesClient interface {
	Core() v1core.CoreInterface
}

func newServiceDiscovery(host string, port string, tokenPath string, certPath string, label string, res chan<- service, errors chan<- error) (*serviceDiscovery, error) {

	config, err := clusterConfig(host, port, tokenPath, certPath)
	if err != nil {
		return &serviceDiscovery{}, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return &serviceDiscovery{}, err
	}
	return &serviceDiscovery{client: clientset, label: label, res: res, errors: errors}, nil
}

func clusterConfig(host string, port string, tokenPath string, certPath string) (*rest.Config, error) {
	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}
	tlsClientConfig := rest.TLSClientConfig{}
	tlsClientConfig.CAFile = certPath

	return &rest.Config{
		Host:            "https://" + net.JoinHostPort(host, port),
		BearerToken:     string(token),
		TLSClientConfig: tlsClientConfig,
	}, nil
}

func (d *serviceDiscovery) getServices() {
	namespaces, err := d.client.Core().Namespaces().List(v1.ListOptions{})
	if err != nil {
		select {
		case d.errors <- fmt.Errorf("Could not get namespaces via kubernetes api: (%v)", err):
		default:
		}
		return
	}
	for _, n := range namespaces.Items {
		services, err := d.client.Core().Services(n.Name).List(v1.ListOptions{LabelSelector: d.label})
		if err != nil {
			select {
			case d.errors <- fmt.Errorf("Could not get services via kubernetes api: (%v)", err):
			default:
			}
			return
		}

		for _, s := range services.Items {
			d.res <- service{
				Name:      s.Name,
				Namespace: n.Name,
				BaseURL:   fmt.Sprintf("http://%s.%s/", s.Name, n.Name),
			}
		}
	}
}

type service struct {
	Name      string
	Namespace string
	BaseURL   string
}
