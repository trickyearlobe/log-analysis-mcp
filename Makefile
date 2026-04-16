BINARY_NAME := log-analysis-mcp
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test test-cover test-race install clean lint docker run allow-firewall version release-patch release-minor release-major

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

# macOS only: optional performance optimisation for remote SSH tools.
# The server auto-falls back to /usr/bin/ssh as a proxy when the firewall blocks
# net.Dial, so this is NOT required. It just avoids the ~20ms probe on first connect.
# Must be re-run after every rebuild since the firewall tracks binaries by content hash.
allow-firewall:
	@if [ "$$(uname)" = "Darwin" ]; then \
		sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add "$$(pwd)/bin/$(BINARY_NAME)"; \
		sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp "$$(pwd)/bin/$(BINARY_NAME)"; \
	else \
		echo "Not macOS — firewall registration not needed"; \
	fi

version:
	@echo $(VERSION)

release-patch:
	@latest=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
	minor=$$(echo $$latest | sed 's/^v//' | cut -d. -f2); \
	patch=$$(echo $$latest | sed 's/^v//' | cut -d. -f3); \
	next="v$$major.$$minor.$$((patch + 1))"; \
	echo "Tagging $$next (was $$latest)"; \
	git tag -a "$$next" -m "Release $$next"

release-minor:
	@latest=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
	minor=$$(echo $$latest | sed 's/^v//' | cut -d. -f2); \
	next="v$$major.$$((minor + 1)).0"; \
	echo "Tagging $$next (was $$latest)"; \
	git tag -a "$$next" -m "Release $$next"

release-major:
	@latest=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
	next="v$$((major + 1)).0.0"; \
	echo "Tagging $$next (was $$latest)"; \
	git tag -a "$$next" -m "Release $$next"
