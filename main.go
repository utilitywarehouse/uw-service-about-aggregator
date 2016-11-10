package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

var client = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 128,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
	},
}

func main() {
	app := cli.App("uw-service-about-aggregator", "Calls /__/about for services that expose the endpoint")
	port := app.String(cli.StringOpt{
		Name:   "port",
		Value:  "8080",
		Desc:   "Port to listen on",
		EnvVar: "PORT",
	})
	label := app.String(cli.StringOpt{
		Name:   "label",
		Value:  "about=true",
		Desc:   "Label to filter services via kubernetes api",
		EnvVar: "LABEL",
	})
	kubernetesHost := app.String(cli.StringOpt{
		Name:   "kubernetes-service-host",
		Value:  "",
		Desc:   "Kubernetes service host",
		EnvVar: "KUBERNETES_SERVICE_HOST",
	})
	kubernetesPort := app.String(cli.StringOpt{
		Name:   "kubernetes-service-port",
		Value:  "",
		Desc:   "Kubernetes service port",
		EnvVar: "KUBERNETES_SERVICE_PORT",
	})
	kubernetesTokenPath := app.String(cli.StringOpt{
		Name:   "kubernetes-token-path",
		Value:  "/var/run/secrets/kubernetes.io/serviceaccount/token",
		Desc:   "Path to the kubernetes api token",
		EnvVar: "KUBERNETES_TOKEN_PATH",
	})
	kubernetesCertPath := app.String(cli.StringOpt{
		Name:   "kubernetes-cert-path",
		Value:  "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		Desc:   "Path to the kubernetes cert",
		EnvVar: "KUBERNETES_CERT_PATH",
	})
	confluenceHost := app.String(cli.StringOpt{
		Name:   "confluence-host",
		Value:  "",
		Desc:   "Confluence host",
		EnvVar: "CONFLUENCE_HOST",
	})
	confluenceCredentials := app.String(cli.StringOpt{
		Name:   "confluence-credentials",
		Value:  "",
		Desc:   "Base 64 encoded <user:pass> used in Basic authentication",
		EnvVar: "CONFLUENCE_CREDENTIALS",
	})
	confluencePageID := app.String(cli.StringOpt{
		Name:   "confluence-page-id",
		Value:  "",
		Desc:   "Confluence page id",
		EnvVar: "CONFLUENCE_PAGE_ID",
	})

	app.Action = func() {
		errors := make(chan error, 10)
		services := make(chan service, 10)
		about := make(chan about, 10)
		d, err := newServiceDiscovery(*kubernetesHost, *kubernetesPort, *kubernetesTokenPath, *kubernetesCertPath, *label, services, errors)
		if err != nil {
			log.Fatalf("ERROR: Could not create service discovery: error=(%v)", err)
		}
		f := newAboutFetcher()
		exporters := []exporter{}
		httpExporter := newHTTPExporter()
		confluenceExporter, err := newConfluenceExporter(*confluenceHost, *confluenceCredentials, *confluencePageID, client)
		if err != nil {
			log.Fatalf("ERROR: Could not create confluence exporter: error=(%v)", err)
		}
		exporters = append(exporters, httpExporter, confluenceExporter)
		e := exporterService{exporters: exporters}
		h := handler{discovery: d}

		go d.getServices()
		go f.readAbouts(services, about, errors)
		go e.export(about, errors)
		go func() {
			for e := range errors {
				log.Printf("ERROR: %v", e)
			}
		}()

		m := mux.NewRouter()
		http.Handle("/", handlers.CombinedLoggingHandler(os.Stdout, m))
		m.HandleFunc("/reload", h.reload).Methods("POST")
		m.HandleFunc("/__/about", httpExporter.handleHTTP).Methods("GET")

		log.Printf("Listening on [%v].\n", *port)
		err = http.ListenAndServe(":"+*port, nil)
		if err != nil {
			log.Fatalf("ERROR: Web server failed: error=(%v).\n", err)
		}
	}
	app.Run(os.Args)
}

type handler struct {
	discovery *serviceDiscovery
}

func (h *handler) reload(w http.ResponseWriter, r *http.Request) {
	go h.discovery.getServices()
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprint(w, "{\"ok\":true}")
}

type httpClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type aboutFetcher struct {
	client httpClient
}

func newAboutFetcher() aboutFetcher {
	return aboutFetcher{client: client}
}

func (a *aboutFetcher) readAbouts(services chan service, ab chan about, errors chan error) {
	readers := 5
	for i := 0; i < readers; i++ {
		go func(services chan service, ab chan about) {
			for s := range services {
				req, err := http.NewRequest("GET", s.BaseURL+"__/about", nil)
				if err != nil {
					select {
					case errors <- fmt.Errorf("Could not get response from %v: (%v)", s.BaseURL, err):
					default:
					}
					continue
				}
				resp, err := a.client.Do(req)
				if err != nil {
					select {
					case errors <- fmt.Errorf("Could not get response from %v: (%v)", s.BaseURL, err):
					default:
					}
					continue
				}

				if resp.StatusCode != http.StatusOK {
					select {
					case errors <- fmt.Errorf("__/about returned %d for %s", resp.StatusCode, s.BaseURL):
					default:
					}
					if resp != nil && resp.Body != nil {
						io.Copy(ioutil.Discard, resp.Body)
						resp.Body.Close()
					}
					continue
				}
				dec := json.NewDecoder(resp.Body)
				var doc doc

				if err := dec.Decode(&doc); err != nil {
					select {
					case errors <- fmt.Errorf("Could not json decode __/about response for %s", s.BaseURL):
					default:
					}
					continue
				}
				resp.Body.Close()
				ab <- about{Service: s, Doc: doc}
			}
		}(services, ab)
	}

}

type about struct {
	Service service
	Doc     doc
}

type doc struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Owners      []owner   `json:"owners"`
	Links       []link    `json:"links"`
	BuildInfo   buildInfo `json:"build-info"`
}

type owner struct {
	Name  string `json:"name"`
	Slack string `json:"slack"`
}

type link struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type buildInfo struct {
	Revision string `json:"revision"`
}
