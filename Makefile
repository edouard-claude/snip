.PHONY: build test test-race lint install clean

BINARY=snip
BUILD_DIR=cmd/snip
LDFLAGS=-ldflags="-s -w"

build:
	CGO_ENABLED=0 go build -o $(BINARY) $(LDFLAGS) ./$(BUILD_DIR)

test:
	go test -cover ./...

test-race:
	go test -race ./...

lint:
	go vet ./...
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run || echo "golangci-lint not installed, skipping"

install: build
	cp $(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || cp $(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY)
	go clean -testcache
