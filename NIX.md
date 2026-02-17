# Nix Flake Usage

This repository provides a comprehensive Nix flake for building and deploying all Clawrden components.

## Available Packages

The flake provides the following packages corresponding to Makefile targets:

- `shim` - Statically-linked universal shim binary (CGO_ENABLED=0)
- `warden` - Warden server with policy engine and HITL queue
- `cli` - CLI tool for managing the warden
- `slack-bridge` - Slack notification bridge for HITL approvals
- `telegram-bridge` - Telegram notification bridge for HITL approvals
- `warden-docker` - Docker image for the warden service
- `default` - All core binaries (shim + warden + cli)

## Quick Start

### Build All Packages

```bash
# Build core binaries (shim, warden, cli)
nix build

# Build specific package
nix build .#shim
nix build .#warden
nix build .#cli
nix build .#slack-bridge
nix build .#telegram-bridge

# Build Docker image
nix build .#warden-docker
```

### Run Binaries Directly

```bash
# Run warden
nix run .#warden -- --socket /var/run/clawrden/warden.sock --policy ./policy.yaml

# Run CLI
nix run .#cli -- status

# Run bridges
nix run .#slack-bridge
nix run .#telegram-bridge
```

### Install to User Profile

```bash
# Install all core binaries
nix profile install .#default

# Install specific binary
nix profile install .#warden
nix profile install .#cli
```

### Development Shell

```bash
# Enter development environment
nix develop

# Inside the shell, you have access to:
# - Go toolchain (go, gopls, golangci-lint, delve)
# - Docker and docker-compose
# - Make and git
```

## Docker Image Usage

The `warden-docker` package builds a Docker image compatible with the docker-compose.yml configuration.

### Build and Load Docker Image

```bash
# Build the Docker image
nix build .#warden-docker

# Load into Docker
docker load < result

# Tag the image
docker tag clawrden-warden:latest clawrden-warden:latest
```

### Using with Docker Compose

The Nix-built Docker image can replace the Alpine-based warden in docker-compose.yml:

```yaml
services:
  warden:
    image: clawrden-warden:latest  # Use Nix-built image
    # ... rest of configuration remains the same
```

## Package Details

### Shim Package

- **Binary name**: `shim`
- **Static linking**: Yes (CGO_ENABLED=0, ldflags: -s -w)
- **Size**: ~2.4 MB
- **Purpose**: Universal command interception binary

### Warden Package

- **Binary name**: `warden`
- **Dependencies**: Docker client libraries, YAML parser
- **Purpose**: Policy engine and HITL queue manager

### CLI Package

- **Binary name**: `cli`
- **Purpose**: Command-line interface for warden management

### Docker Image Package

- **Base**: Minimal (busybox-based)
- **Includes**: warden binary, wget (for healthcheck)
- **Exposed ports**: 8080/tcp
- **Default command**: Runs warden with standard paths

## Comparison with Makefile

| Makefile Target | Nix Package | Binary Output |
|-----------------|-------------|---------------|
| `make build-shim` | `nix build .#shim` | `bin/shim` |
| `make build-warden` | `nix build .#warden` | `bin/warden` |
| `make build-cli` | `nix build .#cli` | `bin/cli` |
| `make build-slack-bridge` | `nix build .#slack-bridge` | `bin/slack-bridge` |
| `make build-telegram-bridge` | `nix build .#telegram-bridge` | `bin/telegram-bridge` |
| `make build` | `nix build` | All core binaries |
| N/A | `nix build .#warden-docker` | Docker image tarball |

## Advantages of Nix

1. **Reproducible builds**: Exact same binaries every time
2. **Declarative dependencies**: All dependencies specified in flake.nix
3. **Isolated builds**: No system pollution
4. **Caching**: Binary cache for faster rebuilds
5. **Cross-platform**: Build for multiple architectures
6. **Docker integration**: Build Docker images without Dockerfile

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build with Nix
on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cachix/install-nix-action@v24
      - name: Build all packages
        run: nix build .#default
      - name: Build Docker image
        run: nix build .#warden-docker
      - name: Run tests
        run: nix develop --command make test
```

### Binary Cache

Consider setting up a binary cache (e.g., Cachix) to speed up builds:

```bash
# Push to binary cache
cachix push my-cache $(nix build .#default --print-out-paths --no-link)
```

## Updating the Flake

### Update vendorHash

If Go dependencies change, update the `vendorHash` in flake.nix:

```bash
# Build will fail with correct hash
nix build .#shim

# Copy the correct hash from error message
# Update vendorHash in flake.nix
```

### Update nixpkgs

```bash
nix flake update
```

## Troubleshooting

### "dirty Git tree" Warning

This is normal when you have uncommitted changes. It doesn't prevent builds.

### vendorHash Mismatch

If you modify go.mod or go.sum:

1. Build will fail with the correct hash
2. Copy the hash from error message
3. Update `vendorHash` in flake.nix
4. Rebuild

### Docker Image Not Loading

Ensure you're using the correct command:

```bash
docker load < result
```

Not `docker import` - that's for different format.

## Further Reading

- [Nix Flakes Documentation](https://nixos.wiki/wiki/Flakes)
- [buildGoModule Reference](https://nixos.org/manual/nixpkgs/stable/#sec-language-go)
- [dockerTools Reference](https://nixos.org/manual/nixpkgs/stable/#sec-pkgs-dockerTools)
