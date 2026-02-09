# cm Makefile
#
# Build the clockmail coordination CLI

.PHONY: build install clean test vet

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o cm ./cmd/cm

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/cm

clean:
	rm -f cm
	go clean

test:
	go test ./...

vet:
	go vet ./...
