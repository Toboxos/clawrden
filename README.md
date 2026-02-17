# 🛡️ Clawrden

**The Hypervisor for Autonomous AI Agents**

Clawrden is a sidecar-based governance architecture that operationalizes "wild" autonomous AI agents (like AutoGPT, OpenDevin) within a Zero Trust environment. It intercepts agent actions at the binary level, routes them through a privileged supervisor for policy checks and human-in-the-loop (HITL) approval, then executes them safely.

## Core Concepts

- **Zero Trust**: The Agent (the "Prisoner") is treated as untrusted/compromised
- **Transparent Interception**: Agents believe they're running local commands; actually triggering RPCs
- **Hot-Pluggable Capabilities**: Tools injected dynamically via volume mounts without restarts
- **Universal Compatibility**: Static Go binaries work with any Linux container (Alpine, Ubuntu, Distroless)

## Quick Start

### Prerequisites

- **Nix** (recommended) or **Go 1.21+**
- **Docker** (optional, for full functionality)
- **Linux** or **WSL2**

### Build All Binaries

```bash
# Using Nix (recommended)
nix develop
make build

# Or with Go directly
go mod download
make build
```

This creates three binaries in `bin/`:
- `clawrden-shim` (2.4MB) - Universal command interceptor
- `clawrden-warden` (12MB) - Supervisor server
- `clawrden-cli` (8.4MB) - Control interface

### Run a Simple Example

**Terminal 1: Start the Warden**

```bash
./bin/clawrden-warden \
  --socket /tmp/warden.sock \
  --policy policy.yaml \
  --audit /tmp/audit.log \
  --api :8080
```

**Terminal 2: Open Web Dashboard**

```bash
# Open in your browser
open http://localhost:8080

# Or use the CLI
./bin/clawrden-cli --api http://localhost:8080 status
./bin/clawrden-cli history
./bin/clawrden-cli queue
```

The web dashboard provides:
- 📊 Real-time status monitoring
- ✅ One-click approve/deny for pending requests
- 📜 Command history with filtering
- 🔄 Auto-refresh (2s intervals)

**Terminal 3: Simulate Agent Commands**

```bash
# In a real deployment, the shim binary would be in the agent's PATH.
# For testing, we can manually test the socket communication.

# Test an allowed command (will execute immediately)
echo '{"command":"echo","args":["hello"],"cwd":"/tmp","env":[],"identity":{"uid":1000,"gid":1000}}' | \
  nc -U /tmp/warden.sock

# Check the audit log
./bin/clawrden-cli history
```

## Nix Flake Support

Clawrden provides a comprehensive Nix flake for reproducible builds and deployments.

### Available Packages

```bash
# Build specific packages
nix build .#shim          # Universal shim binary (2.4MB, static)
nix build .#warden        # Warden server
nix build .#cli           # CLI tool
nix build .#slack-bridge  # Slack notification bridge
nix build .#telegram-bridge  # Telegram notification bridge
nix build .#warden-docker # Docker image tarball

# Build all core binaries
nix build

# Run binaries directly without installing
nix run .#warden -- --help
nix run .#cli -- status
```

### Docker Image from Nix

```bash
# Build and load warden Docker image
nix build .#warden-docker
docker load < result
```

**See [NIX.md](./NIX.md) for complete Nix flake documentation including:**
- Development shell usage
- CI/CD integration examples
- Binary cache setup
- Comparison with Makefile targets
- Troubleshooting guide

## Architecture

```
┌─────────────────────────────────────────┐
│          Prisoner Container             │
│        (Untrusted AI Agent)            │
│                                         │
│  /clawrden/bin/  (shim binaries)       │
│    ├─ npm        → clawrden-shim       │
│    ├─ docker     → clawrden-shim       │
│    └─ kubectl    → clawrden-shim       │
│                                         │
│  Agent code runs normally...           │
│  When it calls 'npm install':          │
│    1. Shim intercepts the call         │
│    2. Sends request over Unix socket   │
│    3. Waits for response                │
└─────────────────────────────────────────┘
                    │
              Unix Socket
              /var/run/clawrden/warden.sock
                    │
                    ▼
┌─────────────────────────────────────────┐
│         Warden Container                │
│      (Privileged Supervisor)            │
│                                         │
│  ┌─────────────────────────────┐       │
│  │    Policy Engine             │       │
│  │  • Allow / Deny / Ask        │       │
│  │  • Pattern matching          │       │
│  └─────────────────────────────┘       │
│                │                        │
│                ▼                        │
│  ┌─────────────────────────────┐       │
│  │    HITL Queue                │       │
│  │  • Pending approvals         │       │
│  │  • Human oversight           │       │
│  └─────────────────────────────┘       │
│                │                        │
│                ▼                        │
│  ┌─────────────────────────────┐       │
│  │    Executor                  │       │
│  │  • Mirror: exec in prisoner  │       │
│  │  • Ghost: ephemeral container│       │
│  │  • Local: dev/test mode      │       │
│  └─────────────────────────────┘       │
│                                         │
│  HTTP API: :8080                        │
│    • /api/status                        │
│    • /api/queue                         │
│    • /api/history                       │
└─────────────────────────────────────────┘
                    ▲
                    │
                HTTP/JSON
                    │
┌─────────────────────────────────────────┐
│          Control Interfaces             │
│                                         │
│  • clawrden-cli (terminal)             │
│  • Web Dashboard (browser)              │
│  • API clients (programmatic)           │
└─────────────────────────────────────────┘
```

## How It Works

### 1. The Universal Shim

A single static Go binary that impersonates multiple tools:

```bash
# These are all the SAME binary, just with different names:
/clawrden/bin/npm      -> clawrden-shim
/clawrden/bin/docker   -> clawrden-shim
/clawrden/bin/kubectl  -> clawrden-shim
```

When executed, the shim:
1. Detects which tool it's pretending to be (from `os.Args[0]`)
2. Captures the command, args, cwd, environment, UID/GID
3. Serializes to JSON and sends over Unix socket to warden
4. Streams back stdout/stderr/exit code

### 2. The Warden

Receives requests and enforces policy:

**Policy Example** (`policy.yaml`):
```yaml
default_action: deny

rules:
  # Safe read-only commands - auto-allow
  - command: ls
    action: allow
  - command: cat
    action: allow
  - command: grep
    action: allow

  # Potentially dangerous - require human approval
  - command: npm
    action: ask

  # Destructive - always deny
  - command: rm
    action: deny
    patterns:
      - "-rf"
```

**Actions:**
- `allow`: Execute immediately
- `deny`: Reject immediately
- `ask`: Queue for human approval (HITL)

### 3. Execution Strategies

#### Mirror Mode (Safe Commands)
Executes command **back inside the prisoner container**:
```bash
Agent calls: ls /app
  → Warden receives request
  → Warden validates: "ls" is allowed
  → Warden runs: docker exec <prisoner> ls /app
  → Streams output back to agent
```

#### Ghost Mode (Heavy Tools)
Spins up **temporary container** with the required tool:
```bash
Agent calls: npm install express
  → Warden receives request
  → Human approves via HITL
  → Warden runs: docker run --rm -v app:/app node:18 npm install express
  → Fixes file ownership (chown to agent's UID/GID)
  → Streams output back to agent
```

## Configuration

### Policy File (`policy.yaml`)

```yaml
# Default action for unknown commands
default_action: deny

# Path restrictions - glob patterns for allowed directories
allowed_paths:
  - "/app/*"                  # Allow anything under /app
  - "/tmp/*"                  # Allow /tmp for testing
  - "/home/*/workspace/*"     # Allow user workspaces

# Command rules
rules:
  # Pattern 1: Simple allow/deny
  - command: echo
    action: allow

  # Pattern 2: Ask for approval
  - command: npm
    action: ask

  # Pattern 3: Deny specific patterns
  - command: rm
    action: deny
    patterns:
      - "-rf"
      - "-fr"

  # Pattern 4: Allow with specific args
  - command: apt-get
    action: ask
    patterns:
      - "install*"
```

### Warden Command-Line Flags

```bash
./bin/clawrden-warden --help
  --socket string      Unix socket path (default: /var/run/clawrden/warden.sock)
  --policy string      Policy file path (default: policy.yaml)
  --prisoner-id string Docker container ID of prisoner
  --audit string       Audit log path (default: /var/log/clawrden/audit.log)
  --api string         HTTP API address (default: :8080)
```

### CLI Command-Line Flags

```bash
./bin/clawrden-cli --help
  --api string  Warden API URL (default: http://localhost:8080)

Commands:
  status           Show warden status
  queue            List pending HITL requests
  approve <id>     Approve pending request
  deny <id>        Deny pending request
  history          View command audit log
  kill             Trigger kill switch
```

## Development

### Project Structure

```
clawrden/
├── cmd/
│   ├── shim/           # Universal command interceptor
│   ├── warden/         # Supervisor server
│   └── cli/            # Control CLI
├── internal/
│   ├── shim/           # Shim logic & signal handling
│   ├── warden/         # Server, policy, HITL, audit, API
│   └── executor/       # Docker & local execution strategies
├── pkg/
│   └── protocol/       # Shared socket protocol (JSON framing)
├── tests/
│   └── integration/    # End-to-end tests
├── docker/             # Container definitions
├── scripts/            # Installation scripts
└── docs/               # Additional documentation
```

### Run Tests

```bash
# All tests
make test

# Integration tests only
make integration-test

# Specific package
go test ./internal/warden -v
```

**Current Test Coverage:** 22/22 tests passing ✅

### Build System

```bash
make build           # Build all binaries
make build-shim      # Build shim only (static)
make build-warden    # Build warden only
make build-cli       # Build CLI only
make test            # Run all tests
make integration-test # Run integration tests
make clean           # Remove build artifacts
```

### Development Environment

**Using Nix (recommended):**
```bash
nix develop
# Provides: go, docker, docker-compose, make, etc.
```

**Using Go directly:**
```bash
go version  # Requires 1.21+
go mod download
make build
```

## HTTP API Reference

All endpoints return JSON.

### `GET /api/status`
Returns warden health and statistics.

**Response:**
```json
{
  "status": "running",
  "pending_count": 2,
  "uptime": 3600.5
}
```

### `GET /api/queue`
Lists pending HITL approval requests.

**Response:**
```json
[
  {
    "id": "abc123",
    "command": "npm",
    "args": ["install", "express"],
    "cwd": "/app",
    "identity": {"uid": 1000, "gid": 1000}
  }
]
```

### `POST /api/queue/{id}/approve`
Approves a pending request.

**Response:**
```json
{"status": "approved"}
```

### `POST /api/queue/{id}/deny`
Denies a pending request.

**Response:**
```json
{"status": "denied"}
```

### `GET /api/history`
Returns the complete audit log.

**Response:**
```json
[
  {
    "timestamp": "2026-02-17T01:23:45.123456Z",
    "command": "npm",
    "args": ["install", "express"],
    "cwd": "/app",
    "identity": {"uid": 1000, "gid": 1000},
    "decision": "allow (after HITL)",
    "exit_code": 0,
    "duration_ms": 5432.1
  }
]
```

### `POST /api/kill`
Triggers the kill switch (pauses/kills prisoner).

**Response:**
```json
{
  "status": "acknowledged",
  "message": "Kill switch activated"
}
```

## Audit Log

All commands are logged in JSON-lines format to the audit file:

```bash
tail -f /tmp/audit.log | jq .
```

**Example Entry:**
```json
{
  "timestamp": "2026-02-17T01:23:45.123456789Z",
  "command": "npm",
  "args": ["install", "express"],
  "cwd": "/app",
  "identity": {"uid": 1000, "gid": 1000},
  "decision": "allow (after HITL)",
  "exit_code": 0,
  "duration_ms": 5432.1
}
```

## Docker Compose Example

```bash
docker-compose up
```

This spins up:
- **Warden**: Privileged supervisor with Docker socket access
- **Prisoner**: Minimal container with shim binaries pre-installed

**Note:** Docker Compose integration is currently in development.

## Security Considerations

### Path Validation
Commands are restricted to `/app` directory (configurable):
```go
if !strings.HasPrefix(req.Cwd, "/app") {
    return "denied: outside security boundary"
}
```

### Environment Scrubbing
Dangerous environment variables are filtered:
- `LD_PRELOAD` (library injection)
- `DOCKER_HOST` (Docker hijacking)
- `KUBECONFIG` (Kubernetes access)

### Identity Preservation
Commands execute with the **original UID/GID** from the prisoner to prevent privilege escalation.

### Audit Trail
Every command is logged with full context for security analysis.

## Use Cases

### 1. AI Agent Sandboxing
Run AutoGPT/OpenDevin safely with human oversight on dangerous operations.

### 2. CI/CD Pipeline Safety
Require approval before deployment commands execute.

### 3. Multi-Tenant Environments
Isolate tenant operations with policy enforcement.

### 4. Development Environment Control
Allow devs to work freely but require approval for production access.

## Roadmap

- [x] Phase 1: Core Infrastructure
  - [x] Universal shim binary
  - [x] Warden server with policy engine
  - [x] Mirror/Ghost execution strategies

- [x] Phase 2: Integration & Control (4/6)
  - [x] Integration testing
  - [x] Audit logging
  - [x] HTTP API
  - [x] CLI tool
  - [ ] Timeout enforcement
  - [ ] Ghost image configuration

- [ ] Phase 3: Production Readiness
  - [x] Web dashboard
  - [ ] Docker Compose validation
  - [ ] Multi-distro testing
  - [ ] Metrics/monitoring
  - [ ] Chat integrations (Slack/Telegram)

## Contributing

Contributions welcome! Please:
1. Read `docs/architecture.md`
2. Run tests: `make test`
3. Follow existing code style
4. Add tests for new features

## License

MIT License - See LICENSE file for details

## Acknowledgments

Inspired by the need to safely operationalize autonomous AI agents in production environments.

---

**Built with Go, Docker, and Zero Trust principles** 🛡️
