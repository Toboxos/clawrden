.PHONY: build build-shim build-warden test lint clean

# Build both binaries
build: build-shim build-warden

# Build the statically-linked shim binary
build-shim:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -ldflags="-s -w" -o bin/clawrden-shim ./cmd/shim

# Build the warden binary
build-warden:
	go build -o bin/clawrden-warden ./cmd/warden

# Run all tests
test:
	go test -v ./...

# Run linter
lint:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf bin/
