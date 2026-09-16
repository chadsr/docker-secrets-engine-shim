VERSION  ?= $(shell git describe --tags --match 'v[0-9]*' --dirty 2>/dev/null || echo v0.0.0-dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GOFLAGS  := -buildmode=pie -trimpath -mod=readonly -modcacherw
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

BINS := docker-secrets-engine-shim

.PHONY: all $(BINS) clean test

all: $(BINS)

docker-secrets-engine-shim:
	CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o dist/$@ ./cmd/secrets-engine-shim

clean:
	rm -rf dist

test:
	go test ./...
