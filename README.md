# About Aggregator

Does service discovery via kubernetes api and calls `/__/about` for each service that exposes the endpoint. Service are filtered based on labels(`about=true`).   
For each service this information is pushed to several exporters:   

   * HTTP exporter - exposes list of services which expose /__/about   
   * (not currently implemented) Confluence exporter

## Developing

Build with `go build .`  
Test with `go test .`

## Running

```
export PORT="8080"
export LABEL="about=true"
export KUBERNETES_SERVICE_HOST="192.168.99.100"
export KUBERNETES_SERVICE_PORT="8443"
export KUBERNETES_TOKEN_PATH="/var/run/secrets/kubernetes.io/serviceaccount/token"
export KUBERNETES_CERT_PATH="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

$GOPATH/bin/about-aggregator
```


## Endpoints   
Application specific endpoints:
   
   * `GET /__/about` - list of services which expose `/__/about`
   * `POST /reload`
   