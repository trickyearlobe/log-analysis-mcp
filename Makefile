BINARY_NAME := log-analysis-mcp
VERSION := 1.0.0
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test test-cover test-race install clean lint docker run

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/log-analysis-mcp

test:
	go test ./...

test-cover:
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-race:
	go test -race ./...

install:
	go install $(LDFLAGS) ./cmd/log-analysis-mcp

clean:
	rm -rf bin/ coverage.out coverage.html

lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed"

docker:
	docker build -t $(BINARY_NAME):$(VERSION) .

run:
	go run ./cmd/log-analysis-mcp
