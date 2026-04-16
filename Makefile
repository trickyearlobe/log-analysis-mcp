BINARY_NAME := log-analysis-mcp
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.DEFAULT_GOAL := help

.PHONY: help build test test-cover test-race install clean lint docker run allow-firewall version release-patch release-minor release-major

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the binary
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/log-analysis-mcp

test: ## Run tests
	go test ./...

test-cover: ## Run tests with coverage report
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-race: ## Run tests with race detector
	go test -race ./...

install: ## Install binary to GOPATH/bin
	go install $(LDFLAGS) ./cmd/log-analysis-mcp

clean: ## Remove build artifacts
	rm -rf bin/ coverage.out coverage.html

lint: ## Run go vet and staticcheck
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed"

docker: ## Build Docker image
	docker build -t $(BINARY_NAME):$(VERSION) .

run: ## Run the MCP server locally
	go run ./cmd/log-analysis-mcp

# macOS only: optional performance optimisation for remote SSH tools.
# The server auto-falls back to /usr/bin/ssh as a proxy when the firewall blocks
# net.Dial, so this is NOT required. It just avoids the ~20ms probe on first connect.
# Must be re-run after every rebuild since the firewall tracks binaries by content hash.
allow-firewall: ## macOS: add binary to firewall allowlist (optional)
	@if [ "$$(uname)" = "Darwin" ]; then \
		sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add "$$(pwd)/bin/$(BINARY_NAME)"; \
		sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp "$$(pwd)/bin/$(BINARY_NAME)"; \
	else \
		echo "Not macOS — firewall registration not needed"; \
	fi

version: ## Print current version from git tags
	@echo $(VERSION)

release-patch: ## Create next patch version tag (v1.0.0 -> v1.0.1)
	@latest=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
	minor=$$(echo $$latest | sed 's/^v//' | cut -d. -f2); \
	patch=$$(echo $$latest | sed 's/^v//' | cut -d. -f3); \
	next="v$$major.$$minor.$$((patch + 1))"; \
	echo "Tagging $$next (was $$latest)"; \
	git tag -a "$$next" -m "Release $$next"

release-minor: ## Create next minor version tag (v1.0.0 -> v1.1.0)
	@latest=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
	minor=$$(echo $$latest | sed 's/^v//' | cut -d. -f2); \
	next="v$$major.$$((minor + 1)).0"; \
	echo "Tagging $$next (was $$latest)"; \
	git tag -a "$$next" -m "Release $$next"

release-major: ## Create next major version tag (v1.0.0 -> v2.0.0)
	@latest=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
	next="v$$((major + 1)).0.0"; \
	echo "Tagging $$next (was $$latest)"; \
	git tag -a "$$next" -m "Release $$next"
