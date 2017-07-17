REPO?=persistence-operator
TAG?=$(shell git rev-parse --short HEAD)
NAMESPACE?=po-e2e-$(shell LC_CTYPE=C tr -dc a-z0-9 < /dev/urandom | head -c 13 ; echo '')
COMMONENVVAR = GOOS=linux GOARCH=amd64
BUILDENVVAR = CGO_ENABLED=0
PREFIX ?= $(shell pwd)

CLUSTER_IP?=$(shell kubectl config view --minify | grep server: | cut -f 3 -d ":" | tr -d "//")

pkgs = $(shell go list ./... | grep -v /vendor/ | grep -v /test/)

all: check-license format build test

build:
	$(COMMONENVVAR) $(BUILDENVVAR) go build -o persistence-operator ./cmd/operator

test:
	@go test -short $(pkgs)

format:
	go fmt $(pkgs)

check-license:
	./scripts/check_license.sh

container:
	docker build -t $(REPO):$(TAG) .

embedmd:
	@go get github.com/campoy/embedmd

apidocgen:
	@go install github.com/coreos/prometheus-operator/cmd/apidocgen

docs: embedmd apidocgen
	embedmd -w `find Documentation -name "*.md"`
	apidocgen pkg/client/monitoring/v1alpha1/types.go > Documentation/api.md

generate:
	hack/generate.sh
	@$(MAKE) docs

.PHONY: all build test format check-license container embedmd apidocgen docs
