VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v0.1.0-dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

BINS := secrets-engine-daemon docker-pass

.PHONY: all $(BINS) clean test

all: $(BINS)

secrets-engine-daemon:
	go build $(LDFLAGS) -o $@ ./cmd/$@

docker-pass:
	go build $(LDFLAGS) -o $@ ./cmd/$@

clean:
	rm -f $(BINS)

test:
	go test ./...
