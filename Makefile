.PHONY: build build-shim build-warden build-cli test lint clean integration-test

# Build all binaries
build: build-shim build-warden build-cli

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
