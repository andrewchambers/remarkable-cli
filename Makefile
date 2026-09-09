.PHONY: build test clean
build:
	mkdir -p bin
	go build -trimpath -o bin/remarkablectl ./cmd/remarkablectl
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o bin/remarkable-agent-linux-arm64 ./cmd/remarkable-agent

test:
	go test ./...
	go vet ./...

clean:
	rm -rf bin
