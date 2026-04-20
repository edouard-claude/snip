.PHONY: build build-lite build-windows test test-race lint install install-lite clean

BINARY=snip
BUILD_DIR=cmd/snip
VERSION=$(shell git describe --tags --always 2>/dev/null | sed 's/^v//' || echo dev)
LDFLAGS=-ldflags="-s -w -X 'github.com/edouard-claude/snip/internal/cli.version=$(VERSION)'"

build:
	CGO_ENABLED=0 go build -o $(BINARY) $(LDFLAGS) ./$(BUILD_DIR)

build-lite:
	CGO_ENABLED=0 go build -tags lite -o $(BINARY) $(LDFLAGS) ./$(BUILD_DIR)

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o $(BINARY).exe $(LDFLAGS) ./$(BUILD_DIR)

test:
	go test -cover ./...

test-race:
	go test -race ./...

lint:
	go vet ./...
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

install: build
	cp $(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || cp $(BINARY) /usr/local/bin/$(BINARY)

install-lite: build-lite
	cp $(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || cp $(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY) $(BINARY).exe
	go clean -testcache
