SHELL:=/bin/bash

.DEFAULT_GOAL: test

install:
	go get -t -d -v ./...
	go get -u github.com/golang/lint/golint

lint:
	go vet .
	[[ -z "$(shell gofmt -e -l .)" ]] || exit 1 # gofmt
	golint .

build:
	go build

test: lint
	go test ./...
