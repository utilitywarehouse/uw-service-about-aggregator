package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const htmlResponse = "<!DOCTYPE html>\n<head>\n    <title>UW Documentation</title>\n</head>\n<body>\n<h1>UW Documented services</h1>\n<table style='font-size: 10pt; font-family: MONOSPACE;'>\n    \n    \n    <tr>\n        <td><a href=\"/../../../billing/services/uw-service-refdata:80/__/about\">billing.uw-service-refdata</a></td>\n    </tr>\n    \n    \n</table>\n</body>\n</html>"
const jsonResponse = "[{\"Service\":{\"Name\":\"uw-service-refdata\",\"Namespace\":\"billing\",\"BaseURL\":\"\"},\"Doc\":{\"name\":\"uw-service-refdata\",\"description\":\"uw-service-refdata\",\"owners\":[{\"name\":\"Billing\",\"slack\":\"#billing\"}],\"links\":[{\"url\":\"http://readme\",\"description\":\"readme\"}],\"build-info\":{\"revision\":\"revision\"}}}]"

func TestExporterService(t *testing.T) {
	errors := make(chan error, 10)
	ab := make(chan about, 10)
	exporters := []exporter{newHTTPExporter(), newHTTPExporter()}
	e := exporterService{exporters: exporters}
	ab <- about{}
	close(ab)
	close(errors)
	e.export(ab, errors)
	//give the exporters a chance to process as they run in different go routines
	time.Sleep(1 * time.Second)

	for i, ex := range e.exporters {
		fmt.Println(i)
		assert.Equal(t, 1, func() int {
			ex.(*httpExporter).mutex.RLock()
			l := len(ex.(*httpExporter).abouts)
			ex.(*httpExporter).mutex.RUnlock()
			return l
		}())
	}

}

func TestHTTPExporterHandler(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		name        string
		req         *http.Request
		exporter    *httpExporter
		statusCode  int
		contentType string // Contents of the Content-Type header
		body        string
	}{
		{"Success html", newRequest("GET", "/__/about", "text/html", nil), createHTTPExporterAndHandle(about{Service: service{Name: "uw-service-refdata", Namespace: "billing"}}), http.StatusOK, "text/html", htmlResponse},
		{"Success json", newRequest("GET", "/__/about", "application/json", nil), createHTTPExporterAndHandle(about{
			Service: service{Name: "uw-service-refdata", Namespace: "billing"},
			Doc: doc{
				Name:        "uw-service-refdata",
				Description: "uw-service-refdata",
				Owners:      []owner{owner{Name: "Billing", Slack: "#billing"}},
				Links:       []link{link{URL: "http://readme", Description: "readme"}},
				BuildInfo:   buildInfo{Revision: "revision"},
			}}),
			http.StatusOK, "application/json", jsonResponse},
	}

	for _, test := range tests {
		rec := httptest.NewRecorder()
		router(test.exporter).ServeHTTP(rec, test.req)
		assert.True(test.statusCode == rec.Code, fmt.Sprintf("%s: Wrong response code, was %d, should be %d", test.name, rec.Code, test.statusCode))
		assert.Equal(strings.TrimSpace(test.body), strings.TrimSpace(rec.Body.String()), fmt.Sprintf("%s: Wrong body", test.name))
	}
}

func createHTTPExporterAndHandle(about about) *httpExporter {
	httpExporter := newHTTPExporter()
	httpExporter.handle(about)
	return httpExporter
}

func newRequest(method, url string, accept string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	req.Header.Add("Accept", accept)
	if err != nil {
		panic(err)
	}
	return req
}

func router(e *httpExporter) *mux.Router {
	m := mux.NewRouter()
	m.HandleFunc("/__/about", e.handleHTTP).Methods("GET")
	return m
}

const confluenceURL = "https://utilitywarehouse.atlassian.net"
const confluencePageID = "1234"
const confluenceGetPageResponse = "{\"type\":\"page\",\"title\":\"some page\",\"version\":{\"number\":%d}}"
const confluenceCredentials = "credentials"

func TestConfluenceExporterHandler(t *testing.T) {
	assert := assert.New(t)
	tests := []struct {
		name   string
		ab     about
		client mockedClient
		err    error
	}{
		{"Success - incrementing page version on update",
			about{
				Service: service{Name: "uw-service-refdata", Namespace: "billing"},
				Doc: doc{
					Name:        "uw-service-refdata",
					Description: "uw-service-refdata",
					Owners:      []owner{owner{Name: "Billing", Slack: "#billing"}},
					Links:       []link{link{URL: "http://readme", Description: "readme"}},
					BuildInfo:   buildInfo{Revision: "revision"},
				}},
			mockedClient{assert, map[string]httpCall{
				"GET": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 1}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(confluencePageResponse(1)))}, err: nil},
				"PUT": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 2}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("ok"))}, err: nil},
			}}, nil,
		},
		{"Failure - confluence get page request error - confluence unreachable",
			about{
				Service: service{Name: "uw-service-refdata", Namespace: "billing"},
				Doc: doc{
					Name:        "uw-service-refdata",
					Description: "uw-service-refdata",
					Owners:      []owner{owner{Name: "Billing", Slack: "#billing"}},
					Links:       []link{link{URL: "http://readme", Description: "readme"}},
					BuildInfo:   buildInfo{Revision: "revision"},
				}},
			mockedClient{assert, map[string]httpCall{
				"GET": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 1}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(confluencePageResponse(1)))}, err: fmt.Errorf("host unreachable")},
				"PUT": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 2}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("ok"))}, err: nil},
			}}, fmt.Errorf("Could not get confluence page with ID 1234: (Could not get response from https://utilitywarehouse.atlassian.net/wiki/rest/api/content/1234: (host unreachable))"),
		},
		{"Failure - confluence get page request error - bad json format",
			about{
				Service: service{Name: "uw-service-refdata", Namespace: "billing"},
				Doc: doc{
					Name:        "uw-service-refdata",
					Description: "uw-service-refdata",
					Owners:      []owner{owner{Name: "Billing", Slack: "#billing"}},
					Links:       []link{link{URL: "http://readme", Description: "readme"}},
					BuildInfo:   buildInfo{Revision: "revision"},
				}},
			mockedClient{assert, map[string]httpCall{
				"GET": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 1}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("not json"))}, err: nil},
				"PUT": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 2}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("ok"))}, err: nil},
			}}, fmt.Errorf("Could not get confluence page with ID 1234: (Error decoding confluence response: (invalid character 'o' in literal null (expecting 'u')))"),
		},
		{"Failure - confluence get page request error - non 200",
			about{
				Service: service{Name: "uw-service-refdata", Namespace: "billing"},
				Doc: doc{
					Name:        "uw-service-refdata",
					Description: "uw-service-refdata",
					Owners:      []owner{owner{Name: "Billing", Slack: "#billing"}},
					Links:       []link{link{URL: "http://readme", Description: "readme"}},
					BuildInfo:   buildInfo{Revision: "revision"},
				}},
			mockedClient{assert, map[string]httpCall{
				"GET": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 1}, resp: http.Response{StatusCode: 500, Body: ioutil.NopCloser(strings.NewReader("not json"))}, err: nil},
				"PUT": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 2}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("ok"))}, err: nil},
			}}, fmt.Errorf("Could not get confluence page with ID 1234: (Confluence api returned status 500)"),
		},
		{"Failure - confluence page update request error - confluence unreachable",
			about{
				Service: service{Name: "uw-service-refdata", Namespace: "billing"},
				Doc: doc{
					Name:        "uw-service-refdata",
					Description: "uw-service-refdata",
					Owners:      []owner{owner{Name: "Billing", Slack: "#billing"}},
					Links:       []link{link{URL: "http://readme", Description: "readme"}},
					BuildInfo:   buildInfo{Revision: "revision"},
				}},
			mockedClient{assert, map[string]httpCall{
				"GET": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 1}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(confluencePageResponse(1)))}, err: nil},
				"PUT": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 2}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader("ok"))}, err: fmt.Errorf("host unreachable")},
			}}, fmt.Errorf("Could not update confluence page with ID 1234: (Could not get response from https://utilitywarehouse.atlassian.net/wiki/rest/api/content/1234: (host unreachable))"),
		},
		{"Failure - confluence page update request error - non 200",
			about{
				Service: service{Name: "uw-service-refdata", Namespace: "billing"},
				Doc: doc{
					Name:        "uw-service-refdata",
					Description: "uw-service-refdata",
					Owners:      []owner{owner{Name: "Billing", Slack: "#billing"}},
					Links:       []link{link{URL: "http://readme", Description: "readme"}},
					BuildInfo:   buildInfo{Revision: "revision"},
				}},
			mockedClient{assert, map[string]httpCall{
				"GET": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 1}, resp: http.Response{StatusCode: 200, Body: ioutil.NopCloser(strings.NewReader(confluencePageResponse(1)))}, err: nil},
				"PUT": httpCall{req: confluencePageRequest{confluenceURL + "/wiki/rest/api/content/" + confluencePageID, 2}, resp: http.Response{StatusCode: 500, Body: ioutil.NopCloser(strings.NewReader("internal server error"))}, err: nil},
			}}, fmt.Errorf("Could not update confluence page with ID 1234: (Confluence api returned status 500)"),
		},
	}

	for _, test := range tests {
		confluenceExporter, _ := newConfluenceExporter(confluenceURL, confluenceCredentials, confluencePageID, &test.client)
		err := confluenceExporter.handle(test.ab)
		assert.Equal(test.err, err)
	}

}

func confluencePageResponse(pageVersion int) string {
	return fmt.Sprintf(confluenceGetPageResponse, pageVersion)
}

type mockedClient struct {
	assert *assert.Assertions
	m      map[string]httpCall
}

type httpCall struct {
	req  confluencePageRequest
	resp http.Response
	err  error
}

type confluencePageRequest struct {
	url         string
	pageVersion int
}

func (d *mockedClient) Do(req *http.Request) (resp *http.Response, err error) {
	call := d.m[req.Method]
	if call.err != nil {
		return nil, call.err
	}
	d.assert.Equal(call.req.url, req.URL.String())
	d.assert.Equal("Basic "+confluenceCredentials, req.Header["Authorization"][0])
	if req.Method == "PUT" {
		body, _ := ioutil.ReadAll(req.Body)
		d.assert.Contains(string(body), fmt.Sprintf("\"version\":{\"number\":%d", call.req.pageVersion))
	}
	return &call.resp, nil
}
