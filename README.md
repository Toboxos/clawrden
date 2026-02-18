# Clawrden

**The Hypervisor for Autonomous Agents**

A sidecar-based governance architecture that intercepts AI agent actions at the binary level for policy enforcement and human oversight.

[![Tests](https://img.shields.io/badge/tests-22%2F22%20passing-brightgreen)]()
[![Go](https://img.shields.io/badge/go-1.21+-blue)]()
[![License](https://img.shields.io/badge/license-MIT-blue)]()

## Quick Start

```bash
# Build all binaries
make build

# Or using Nix
nix build

# Start the warden
./bin/clawrden-warden \
  --socket /var/run/clawrden/warden.sock \
  --policy policy.yaml \
  --audit /var/log/clawrden/audit.log \
  --api :8080

# In another terminal, check status
./bin/clawrden-cli status

# View pending approvals
./bin/clawrden-cli queue
```

## What is Clawrden?

Clawrden operates on a **Zero Trust** model: autonomous agents are treated as untrusted entities. Every command they attempt is intercepted, evaluated against policy, and optionally queued for human approval before execution.

### How It Works

```
Agent runs: npm install express
      ↓
Shim intercepts (symlink: /clawrden/bin/npm → shim)
      ↓
Socket RPC to Warden
      ↓
Policy Engine evaluates (allow / deny / ask)
      ↓
If approved: Execute & stream output back
      ↓
Agent receives stdout/stderr/exit code
```

## Architecture

```
┌──────────────────────┐     Unix Socket     ┌──────────────────────┐
│   The Prisoner       │ ◄──────────────────► │   The Warden         │
│   (Agent Container)  │   JSON + Frames     │   (Supervisor)       │
│                      │                     │                      │
│  ┌──────────────┐    │                     │  ┌──────────────┐    │
│  │ Clawrden     │    │                     │  │ Socket       │    │
│  │ Shim Binary  │────┼─────────────────────┼──│ Server       │    │
│  │ (2.4MB)      │    │                     │  └──────┬───────┘    │
│  └──────────────┘    │                     │         │            │
│                      │                     │  ┌──────▼───────┐    │
│  /clawrden/bin/      │                     │  │ Policy       │    │
│   ├── npm → shim     │                     │  │ Engine       │    │
│   ├── docker → shim  │                     │  └──────┬───────┘    │
│   └── pip → shim     │                     │         │            │
│                      │                     │  ┌──────▼───────┐    │
│  No internet ✗       │                     │  │ Executor     │    │
│  No Docker socket ✗  │                     │  │ & Audit Log  │    │
└──────────────────────┘                     │  └──────────────┘    │
                                             │                      │
                                             │  Docker Socket ✓     │
                                             │  Internet Access ✓   │
                                             └──────────────────────┘
```

## Key Features

- **Binary Interception**: Static Go shim (2.4MB) replaces standard tools
- **Policy Engine**: YAML-based rules (allow/deny/ask)
- **Human-in-the-Loop**: Queue commands for approval before execution
- **Audit Trail**: JSON-lines log of every command with full metadata
- **HTTP API**: RESTful interface for automation (port 8080)
- **CLI Tool**: Command-line control interface
- **Zero Trust**: Agent treated as compromised by default
- **Path Validation**: Commands restricted to `/app` workspace
- **Jail Management**: Define jails in `policy.yaml`, manage via API/CLI

## Jail Management

Jails define which commands are intercepted for each agent container. Define them in `policy.yaml`, and the warden creates shim directories on startup.

### 1. Define Jails in Policy

```yaml
# policy.yaml
jails:
  my-agent:
    commands: [ls, cat, npm, docker, kubectl]
    hardened: true
  python-agent:
    commands: [ls, cat, pip, python]
    hardened: false
```

### 2. Mount Jails in Docker Compose

```yaml
# docker-compose.yml
prisoner1:
  volumes:
    - jailhouse-data:/var/lib/clawrden:ro  # Jail shim directory
    - socket-dir:/var/run/clawrden         # Warden socket
  environment:
    # PATH includes the jail's bin dir so shim symlinks take precedence
    - PATH=/var/lib/clawrden/jailhouse/my-agent/bin:/usr/local/bin:/usr/bin:/bin
    - CLAWRDEN_SOCKET=/var/run/clawrden/warden.sock
```

### 3. Manage via CLI

```bash
# List all jails
clawrden-cli jails

# Create a new jail
clawrden-cli jails create my-jail --commands=ls,npm,docker --hardened

# View jail details
clawrden-cli jails get my-jail

# Delete a jail
clawrden-cli jails delete my-jail
```

### 4. Manage via API

```bash
# List jails
curl http://localhost:8080/api/jails

# Create a jail
curl -X POST http://localhost:8080/api/jails \
  -H 'Content-Type: application/json' \
  -d '{"jail_id":"my-jail","commands":["ls","npm"],"hardened":false}'

# Get jail details
curl http://localhost:8080/api/jails/my-jail

# Delete a jail
curl -X DELETE http://localhost:8080/api/jails/my-jail
```

## Components

| Component | Description | Size |
|-----------|-------------|------|
| **clawrden-shim** | Universal command interceptor | 2.4MB |
| **clawrden-warden** | Policy engine & supervisor | 12MB |
| **clawrden-cli** | Control interface | 8.4MB |
| **slack-bridge** | Slack HITL integration | 8MB |
| **telegram-bridge** | Telegram HITL integration | 8MB |

## Policy Configuration

Create a `policy.yaml` file:

```yaml
default_action: deny

allowed_paths:
  - "/app/*"
  - "/tmp/*"

jails:
  my-agent:
    commands: [ls, cat, npm, docker, kubectl]
    hardened: true

rules:
  - command: ls
    action: allow

  - command: npm
    action: ask
    patterns:
      - "install*"

  - command: sudo
    action: deny
```

**Actions:**
- `allow` - Execute immediately (safe commands)
- `deny` - Block immediately (dangerous commands)
- `ask` - Queue for human approval (risky commands)

See [docs/policy-configuration.md](docs/policy-configuration.md) for details.

## Docker Deployment

### Option 1: Separate Containers (Production)

```bash
docker-compose up
```

Runs warden, slack-bridge, and telegram-bridge as separate containers.

### Option 2: All-in-One (Development)

```bash
docker run -v /var/run/clawrden:/var/run/clawrden \
  clawrden-warden:latest warden slack telegram
```

Single container runs all services.

### Build Docker Image

```bash
# Using Nix
nix build .#warden-docker
docker load < result

# Or using Docker Compose
docker-compose build
```

## CLI Commands

```bash
# View warden status
clawrden-cli status

# List pending approvals
clawrden-cli queue

# Approve a command
clawrden-cli approve <request-id>

# Deny a command
clawrden-cli deny <request-id>

# View command history
clawrden-cli history

# Emergency stop
clawrden-cli kill

# Jail management
clawrden-cli jails                  # List all jails
clawrden-cli jails create <id>     # Create a jail
clawrden-cli jails get <id>        # Show jail details
clawrden-cli jails delete <id>     # Delete a jail
```

## API Endpoints

```
GET    /api/status         - Warden health check
GET    /api/queue          - List pending approvals
POST   /api/queue/:id/:action - Approve/deny a request
GET    /api/history        - View audit log
POST   /api/kill           - Emergency stop
GET    /api/jails          - List all jails
POST   /api/jails          - Create a jail
GET    /api/jails/:id      - Get jail details
DELETE /api/jails/:id      - Delete a jail
```

## Chat Integrations

Approve commands from Slack or Telegram:

```bash
# Start chat bridges
./bin/slack-bridge --warden-url http://localhost:8080
./bin/telegram-bridge --warden-url http://localhost:8080
```

See [docs/chat-integration.md](docs/chat-integration.md) for setup instructions.

## Development

### Prerequisites

- Go 1.21+
- Make
- Docker (optional)
- Nix (optional but recommended)

### Build

```bash
# Using Make (Makefile is source of truth)
make build           # Core binaries
make build-all       # Core + bridges
make test            # Run tests
make integration-test # Integration tests only

# Using Nix (delegates to Make)
nix develop          # Enter dev shell
nix build            # Build all
nix build .#warden   # Build specific package
```

### Test

```bash
# All tests
go test ./...

# Integration tests only
go test ./tests/integration -v

# Specific package
go test ./internal/warden -v
```

### Project Structure

```
clawrden/
├── cmd/                    # Entry points
│   ├── shim/              # Command interceptor
│   ├── warden/            # Supervisor server
│   ├── cli/               # Control CLI
│   ├── slack-bridge/      # Slack integration
│   └── telegram-bridge/   # Telegram integration
├── internal/
│   ├── shim/              # Shim logic
│   ├── warden/            # Policy, HITL, audit, API
│   ├── executor/          # Execution strategies
│   └── jailhouse/         # Jail filesystem management
├── pkg/
│   └── protocol/          # Socket protocol
├── tests/
│   └── integration/       # E2E tests
├── docs/                  # Documentation
├── Makefile              # Build system (source of truth)
├── flake.nix             # Nix builds (delegates to Make)
├── policy.yaml           # Example policy
└── docker-compose.yml    # Multi-service deployment
```

## Security Model

- **Zero Trust**: Agent assumed compromised
- **Path Boundary**: All operations restricted to `/app`
- **Environment Scrubbing**: Dangerous variables filtered (LD_PRELOAD, DOCKER_HOST)
- **Identity Preservation**: UID/GID passed through
- **Binary Locking**: Original tools renamed to prevent bypass
- **Audit Trail**: Every command logged with full context

## Status

**Phase 2 Complete**:
- Core infrastructure (shim, warden, policy, HITL)
- Integration tests
- Audit logging
- HTTP API + CLI tool
- Chat bridges (Slack, Telegram)
- Jail management (explicit config, API, CLI)

**Next:** Production hardening, metrics, Docker validation

See [ROADMAP.md](ROADMAP.md) for detailed roadmap.

## Documentation

- [Architecture](docs/architecture.md) - System design and data flow
- [Policy Configuration](docs/policy-configuration.md) - YAML policy guide
- [Chat Integration](docs/chat-integration.md) - Slack/Telegram setup
- [Nix Usage](NIX.md) - Nix flake reference

## Contributing

Contributions welcome! Please ensure:
- All tests pass (`make test`)
- Code is formatted (`go fmt`)
- Commit messages are descriptive

## License

MIT License - See LICENSE file for details.

## Related Work

- [gVisor](https://gvisor.dev/) - Application kernel for containers
- [Falco](https://falco.org/) - Cloud-native runtime security
- [OPA](https://www.openpolicyagent.org/) - Policy-based control
- [Teleport](https://goteleport.com/) - Access plane for infrastructure

---

**Built for the age of autonomous agents. Stay in control.**
