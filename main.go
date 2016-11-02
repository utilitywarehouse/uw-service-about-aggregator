package main

import (
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

func main() {
	app := cli.App("about-aggregator", "Calls /__/about for services that expose the endpoint")
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
	app.Action = func() {
		errors := make(chan error, 10)
		services := make(chan Service, 10)
		about := make(chan About, 10)
		d, err := NewServiceDiscovery(*label, services, errors)
		if err != nil {
			panic(fmt.Sprintf("Could not create service discovery: error=(%v)", err))
		}
		f := NewAboutFetcher()
		exporters := []exporter{}
		httpExporter := NewHTTPExporter()
		exporters = append(exporters, httpExporter)
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

		log.Printf("Listening on [%v].\n", port)
		err = http.ListenAndServe(":"+*port, nil)
		if err != nil {
			panic(fmt.Sprintf("Web server failed: error=(%v).\n", err))
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
	fmt.Fprintf(w, "{\"ok\":true}")
}

type httpClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type aboutFetcher struct {
	client httpClient
}

func NewAboutFetcher() aboutFetcher {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 128,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
		},
	}
	return aboutFetcher{client: httpClient}
}

func (a *aboutFetcher) readAbouts(services chan Service, about chan About, errors chan error) {
	readers := 5
	for i := 0; i < readers; i++ {
		go func(services chan Service, about chan About) {
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
				bytes, _ := ioutil.ReadAll(resp.Body)
				resp.Body.Close()
				about <- About{Service: s, Doc: bytes}
			}
		}(services, about)
	}

}

type About struct {
	Service Service
	Doc     []byte
}
