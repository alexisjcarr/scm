GO ?= go
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/alexisjcarr/scm/internal/version.Version=$(VERSION) -X github.com/alexisjcarr/scm/internal/version.Commit=$(COMMIT) -X github.com/alexisjcarr/scm/internal/version.BuildDate=$(DATE)

.PHONY: all build test cover generate clean release

all: build

build:
	$(GO) build -ldflags "$(LDFLAGS)" ./cmd/...

test:
	./scripts/test.sh

cover:
	./scripts/cover.sh

generate:
	./scripts/generate.sh

clean:
	rm -rf dist

release:
	./scripts/release.sh $(VERSION)
