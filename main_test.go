package main

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestFetcherAboutAddedToChannel(t *testing.T) {
	a := assert.New(t)
	errors := make(chan error, 10)
	services := make(chan service, 10)
	about := make(chan about, 10)

	expectedService := service{Name: "someService", Namespace: "billing", BaseURL: "http://someService.billing/"}
	services <- expectedService
	close(services)

	fetcher := aboutFetcher{client: &dummyClient{assert: a, URL: "http://someService.billing/__/about", resp: http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(strings.NewReader("about endpoint response"))}, err: nil}}
	fetcher.readAbouts(services, about, errors)

	select {
	case <-errors:
		t.Errorf("Should not get an error")

	case about := <-about:
		assert.Equal(t, expectedService, about.Service)
		assert.Equal(t, "about endpoint response", string(about.Doc))
	}
	close(about)
	close(errors)
}

func TestFetcherErrorAddedToChannel(t *testing.T) {
	a := assert.New(t)
	errors := make(chan error, 10)
	services := make(chan service, 10)
	about := make(chan about, 10)

	expectedService := service{Name: "someService", Namespace: "billing", BaseURL: "http://someService.billing/"}
	services <- expectedService
	close(services)

	fetcher := aboutFetcher{client: &dummyClient{assert: a, URL: "http://someService.billing/__/about", resp: http.Response{}, err: fmt.Errorf("error calling __about")}}
	fetcher.readAbouts(services, about, errors)

	select {
	case err := <-errors:
		assert.EqualError(t, err, "Could not get response from http://someService.billing/: (error calling __about)")
	case <-about:
		t.Errorf("Should not get any about")
	}
	close(about)
	close(errors)
}

func TestFetcherErrorAddedToChannelForNon200(t *testing.T) {
	a := assert.New(t)
	errors := make(chan error, 10)
	services := make(chan service, 10)
	about := make(chan about, 10)

	expectedService := service{Name: "someService", Namespace: "billing", BaseURL: "http://someService.billing/"}
	services <- expectedService
	close(services)

	fetcher := aboutFetcher{client: &dummyClient{assert: a, URL: "http://someService.billing/__/about", resp: http.Response{StatusCode: http.StatusNotFound, Body: ioutil.NopCloser(strings.NewReader("about not found"))}, err: nil}}
	fetcher.readAbouts(services, about, errors)

	select {
	case err := <-errors:
		assert.EqualError(t, err, "__/about returned 404 for http://someService.billing/")
	case <-about:
		t.Errorf("Should not get any about")
	}
	close(about)
	close(errors)
}

type dummyClient struct {
	assert *assert.Assertions
	URL    string
	resp   http.Response
	err    error
}

func (d *dummyClient) Do(req *http.Request) (resp *http.Response, err error) {
	d.assert.Equal(d.URL, req.URL.String())
	return &d.resp, d.err
}
