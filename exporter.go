package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
)

const confluenceTemplatePath = "confluence.html"

type exporterService struct {
	exporters []exporter
}

type exporter interface {
	handle(about about) error
}

func (e *exporterService) export(about chan about, errors chan error) {
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
	return &httpExporter{mutex: sync.RWMutex{}, abouts: make(map[string]about)}
}

type httpExporter struct {
	mutex  sync.RWMutex //protects abouts
	abouts map[string]about
}

func (h *httpExporter) handle(about about) error {
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
	a := []about{}
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
	a := []about{}
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
	if err = mainTemplate.Execute(w, struct{ Abouts []about }{Abouts: a}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Couldn't render template file for html response"))
		return
	}
}

func newConfluenceExporter(confluenceHost string, confluenceCredentials string, confluencePageID string, client httpClient) (*confluenceExporter, error) {
	if confluenceHost == "" {
		return nil, fmt.Errorf("confluenceHost is required")
	}
	if confluencePageID == "" {
		return nil, fmt.Errorf("confluencePageID is required")
	}
	return &confluenceExporter{
		confluenceHost:        confluenceHost,
		confluenceCredentials: confluenceCredentials,
		confluencePageID:      confluencePageID,
		client:                client,
		mutex:                 sync.Mutex{},
		abouts:                make(map[string]about)}, nil
}

type confluenceExporter struct {
	confluenceHost        string
	confluenceCredentials string
	confluencePageID      string
	client                httpClient
	mutex                 sync.Mutex //protects abouts
	abouts                map[string]about
}

func (h *confluenceExporter) handle(ab about) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.abouts[ab.Service.Name] = ab
	a := []about{}
	for _, about := range h.abouts {
		a = append(a, about)
	}
	var b bytes.Buffer
	mainTemplate, err := template.ParseFiles(confluenceTemplatePath)
	if err != nil {
		return fmt.Errorf("Couldn't find template file for confluence page body: (%v)", err)
	}
	if err = mainTemplate.Execute(&b, struct{ Abouts []about }{Abouts: a}); err != nil {
		return fmt.Errorf("Couldn't render template file for confluence page body: (%v)", err)
	}

	page, err := h.getConfluencePage(h.confluencePageID)
	if err != nil {
		return fmt.Errorf("Could not get confluence page with ID %v: (%v)", h.confluencePageID, err)
	}
	page.Body = body{storage{Value: b.String(), Representation: "storage"}}
	page.Version.Number = page.Version.Number + 1

	if err = h.updateConfluencePage(page); err != nil {
		return fmt.Errorf("Could not update confluence page with ID %v: (%v)", h.confluencePageID, err)
	}
	return nil
}

func (h *confluenceExporter) getConfluencePage(pageID string) (confluencePage, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/wiki/rest/api/content/%s", h.confluenceHost, h.confluencePageID), nil)
	if err != nil {
		return confluencePage{}, fmt.Errorf("Could not create get page request for %v: (%v)", h.confluencePageID, err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", h.confluenceCredentials))
	resp, err := h.client.Do(req)
	if err != nil {
		return confluencePage{}, fmt.Errorf("Could not get response from %v: (%v)", req.URL.String(), err)
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return confluencePage{}, fmt.Errorf("Confluence api returned status %d", resp.StatusCode)
	}
	dec := json.NewDecoder(resp.Body)
	var page confluencePage
	if err := dec.Decode(&page); err != nil {
		return confluencePage{}, fmt.Errorf("Error decoding confluence response: (%v)", err)
	}
	return page, nil
}

func (h *confluenceExporter) updateConfluencePage(newPage confluencePage) error {
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(newPage)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/wiki/rest/api/content/%s", h.confluenceHost, h.confluencePageID), payload)
	if err != nil {
		return fmt.Errorf("Could not create update page request for %v: (%v)", h.confluencePageID, err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", h.confluenceCredentials))
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("Could not get response from %v: (%v)", req.URL.String(), err)
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Confluence api returned status %d", resp.StatusCode)
	}
	return nil
}

type confluencePage struct {
	Type    string  `json:"type"`
	Title   string  `json:"title"`
	Version version `json:"version"`
	Body    body    `json:"body"`
}

type version struct {
	Number int `json:"number"`
}

type body struct {
	Storage storage `json:"storage"`
}

type storage struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}
