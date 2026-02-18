# Clawrden Architecture

## Overview

Clawrden is a sidecar-based governance architecture that intercepts autonomous agent actions, routing them through a privileged supervisor for policy evaluation and safe execution.

## Components

```
┌──────────────────────┐     Unix Socket     ┌──────────────────────┐
│   The Prisoner       │ ◄──────────────────► │   The Warden         │
│   (Agent Container)  │   JSON + Frames     │   (Supervisor)       │
│                      │                     │                      │
│  ┌──────────────┐    │                     │  ┌──────────────┐    │
│  │ Clawrden     │    │                     │  │ Socket       │    │
│  │ Shim Binary  │────┼─────────────────────┼──│ Server       │    │
│  │ (Go static)  │    │                     │  └──────┬───────┘    │
│  └──────────────┘    │                     │         │            │
│                      │                     │  ┌──────▼───────┐    │
│  /clawrden/bin/      │                     │  │ Policy       │    │
│   ├── npm → shim     │                     │  │ Engine       │    │
│   ├── docker → shim  │                     │  └──────┬───────┘    │
│   └── pip → shim     │                     │         │            │
│                      │                     │  ┌──────▼───────┐    │
│  No internet access  │                     │  │ Executor     │    │
│  No Docker socket    │                     │  │ ├── Mirror   │    │
│                      │                     │  │ └── Ghost    │    │
└──────────────────────┘                     │  └──────────────┘    │
                                             │                      │
                                             │  Docker Socket ✓     │
                                             │  Internet Access ✓   │
                                             └──────────────────────┘
```

## Data Flow

1. Agent invokes `npm install express`
2. Shell resolves `npm` → `/clawrden/bin/npm` (symlink to shim)
3. Shim captures: command, args, cwd, env, uid/gid
4. Shim serializes to JSON, sends to Warden via Unix socket
5. Warden evaluates policy: allow / deny / ask (HITL)
6. If allowed, Warden chooses execution strategy:
   - **Mirror**: exec back in prisoner container (safe commands)
   - **Ghost**: ephemeral container with the real tool (heavy ops)
7. Output is streamed back to shim via framing protocol
8. Shim writes to stdout/stderr, exits with Warden's exit code

## Wire Protocol

```
Request:  [4-byte length][JSON payload]
Ack:      [1-byte: 0=allowed, 1=denied, 2=pending]
Frame:    [1-byte type][4-byte length][payload]

Stream types: 1=stdout, 2=stderr, 3=exit, 4=cancel
```

## Security Model

- **Zero Trust**: Agent is treated as compromised
- **Path Boundary**: All operations must be within `/app`
- **Environment Scrubbing**: Allowlist/blocklist for env vars
- **Identity Preservation**: UID/GID passed through for permission enforcement
- **Binary Locking**: Original tools renamed to prevent PATH bypass

## Directory Structure

```
/clawrden
├── cmd/
│   ├── shim/             # Shim binary entry point
│   │   └── main.go
│   └── warden/           # Warden binary entry point
│       └── main.go
├── internal/
│   ├── shim/             # Shim logic (socket dial, signal handling)
│   ├── warden/           # Server, policy, HITL queue, env scrubber
│   ├── executor/         # Docker SDK wrappers (Mirror, Ghost, Local)
│   └── jailhouse/        # Jail filesystem management (shim symlink trees)
├── pkg/
│   └── protocol/         # Shared types and framing protocol
├── scripts/
│   └── install-clawrden.sh
├── docker/
│   ├── Dockerfile.prisoner
│   └── Dockerfile.warden
├── docs/
│   └── architecture.md
├── policy.yaml
├── docker-compose.yml
├── flake.nix
└── go.mod
```
