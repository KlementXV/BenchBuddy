.PHONY: build test lint clean runner-image e2e

GOBIN ?= $(shell go env GOPATH)/bin
VERSION ?= dev
LDFLAGS = -ldflags "-X github.com/clementlevoux/benchbuddy/internal/cli.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o benchbuddy ./cmd/benchbuddy
	go build $(LDFLAGS) -o runner ./cmd/runner

test:
	go test ./... -race -count=1

test-cover:
	go test ./... -race -count=1 -coverprofile=coverage.txt

lint:
	golangci-lint run

runner-image:
	docker build -t benchbuddy/runner:$(VERSION) -f images/runner/Dockerfile .

e2e:
	./scripts/e2e.sh

clean:
	rm -f benchbuddy runner coverage.txt
	rm -rf dist/ tmp/
