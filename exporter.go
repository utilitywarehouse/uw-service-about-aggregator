package main

import "fmt"

type exporter struct {
	
}

func (e * exporter) export(about chan About) {
	for a := range about {
		fmt.Printf("Namespace: %s, Service: %s, About: %s\n", a.Service.Namespace, a.Service.Name, string(a.Doc))
	}
}
