package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
)

type exporterService struct {
	exporters []exporter
}

type exporter interface {
	handle(about About) error
}

func (e *exporterService) export(about chan About, errors chan error) {
	for a := range about {
		for _, ex := range e.exporters {
			go func(exporter exporter) {
				err := exporter.handle(a)
				if err != nil {
					select {
					case errors <- fmt.Errorf("Error while exporting: (%v)", err):
					default:
						return
					}
				}
			}(ex)
		}
	}
}

func newHTTPExporter() *httpExporter {
	return &httpExporter{mutex: sync.RWMutex{}, abouts: make(map[string]About)}
}

type httpExporter struct {
	mutex  sync.RWMutex //protects abouts
	abouts map[string]About
}

func (h *httpExporter) handle(about About) error {
	h.mutex.Lock()
	h.abouts[about.Service.Name] = about
	h.mutex.Unlock()
	return nil
}

func (h *httpExporter) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") == "application/json" {
		h.jsonHandler(w, r)
	} else {
		h.htmlHandler(w, r)
	}
}

func (h *httpExporter) jsonHandler(w http.ResponseWriter, r *http.Request) {
	a := []About{}
	h.mutex.RLock()
	for _, about := range h.abouts {
		a = append(a, about)
	}
	h.mutex.RUnlock()
	enc := json.NewEncoder(w)
	err := enc.Encode(a)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error during json encoding"))
		return
	}
	w.Header().Add("Content-Type", "application/json")
}

func (h *httpExporter) htmlHandler(w http.ResponseWriter, r *http.Request) {
	a := []About{}
	h.mutex.RLock()
	for _, about := range h.abouts {
		a = append(a, about)
	}
	h.mutex.RUnlock()
	w.Header().Add("Content-Type", "text/html")
	mainTemplate, err := template.ParseFiles("main.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Couldn't open template file for html response"))
		return
	}
	if err = mainTemplate.Execute(w, struct{ Abouts []About }{Abouts: a}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Couldn't render template file for html response"))
		return
	}
}
