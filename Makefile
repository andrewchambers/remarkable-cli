VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
VERSION_FLAGS = -X remarkable-cli/internal/buildinfo.Version=$(VERSION) -X remarkable-cli/internal/buildinfo.Commit=$(COMMIT)

.PHONY: build test clean
build:
	mkdir -p bin
	go build -trimpath -ldflags='$(VERSION_FLAGS)' -o bin/remarkablectl ./cmd/remarkablectl
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w $(VERSION_FLAGS)' -o bin/remarkable-agent-linux-arm64 ./cmd/remarkable-agent

test:
	go test ./...
	go vet ./...

clean:
	rm -rf bin
