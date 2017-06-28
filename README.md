# About Aggregator
[![CircleCI](https://circleci.com/gh/utilitywarehouse/uw-service-about-aggregator.svg?style=shield)](https://circleci.com/gh/utilitywarehouse/uw-service-about-aggregator)
[![Go Report Card](https://goreportcard.com/badge/github.com/utilitywarehouse/uw-service-about-aggregator)](https://goreportcard.com/report/github.com/utilitywarehouse/uw-service-about-aggregator)

Does service discovery via kubernetes api and calls `/__/about` for each service that exposes the endpoint. Service are filtered based on labels(`about=true`).   
For each service this information is pushed to several exporters:   

   * HTTP exporter - exposes list of services which expose /__/about   
   * Confluence exporter - pushes the list of services which expose /__/about to confluence

## Developing

Install dependencies

    $ make install

Run linting

    $ make lint

Build

    $ make build

Run tests

    $ make test

## Running

    export PORT="8080"
    export LABEL="about=true"
    export KUBERNETES_SERVICE_HOST="192.168.99.100"
    export KUBERNETES_SERVICE_PORT="8443"
    export KUBERNETES_TOKEN_PATH="/var/run/secrets/kubernetes.io/serviceaccount/token"
    export KUBERNETES_CERT_PATH="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
    export CONFLUENCE_HOST="https://confluence.example.com"
    export CONFLUENCE_CREDENTIALS="base 64 encoded <user:pass>" #Get the credentials from lastpass: Shared-Kubernetes/confluence/uw-service-about-aggregator 
    export CONFLUENCE_PAGE_ID="page id to update"

    $GOPATH/bin/uw-service-about-aggregator


## Endpoints   
Application specific endpoints:
   
   * `GET /__/about` - list of services which expose `/__/about`
   * `POST /reload`
   
