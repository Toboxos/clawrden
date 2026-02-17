.PHONY: build build-shim build-warden build-cli build-bridges build-slack-bridge build-telegram-bridge test lint clean integration-test

# Build all binaries (core + chat bridges)
build: build-shim build-warden build-cli

# Build all binaries including chat bridges
build-all: build build-bridges

# Build the statically-linked shim binary
build-shim:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="-s -w" -o bin/clawrden-shim ./cmd/shim

# Build the warden binary
build-warden:
	go build -o bin/clawrden-warden ./cmd/warden

# Build the CLI binary
build-cli:
	go build -o bin/clawrden-cli ./cmd/cli

# Build all chat bridges
build-bridges: build-slack-bridge build-telegram-bridge

# Build Slack bridge
build-slack-bridge:
	go build -o bin/slack-bridge ./cmd/slack-bridge

# Build Telegram bridge
build-telegram-bridge:
	go build -o bin/telegram-bridge ./cmd/telegram-bridge

# Run all tests
test:
	go test -v ./...

# Run integration tests only
integration-test:
	go test -v ./tests/integration/...

# Run linter
lint:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf bin/
