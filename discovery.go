package main

import (
	"fmt"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

type serviceDiscovery struct {
	client kubernetesClient
	label  string
	res    chan<- Service
	errors chan<- error
}

type kubernetesClient interface {
	Core() v1core.CoreInterface
}

func newServiceDiscovery(label string, res chan<- Service, errors chan<- error) (*serviceDiscovery, error) {

	config, err := rest.InClusterConfig()
	if err != nil {
		return &serviceDiscovery{}, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return &serviceDiscovery{}, err
	}
	return &serviceDiscovery{client: clientset, label: label, res: res, errors: errors}, nil
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
			d.res <- Service{
				Name:      s.Name,
				Namespace: n.Name,
				BaseURL:   fmt.Sprintf("http://%s.%s/", s.Name, n.Name),
			}
		}
	}
}

type Service struct {
	Name      string
	Namespace string
	BaseURL   string
}
