# Work In Progress

## Completed (Phase 1) ✅
- Nix flake, Go module, project structure, git repo
- Shared protocol package (`pkg/protocol`) with framing
- Universal shim binary (`cmd/shim`) — 2.4MB static binary
- Warden server (`cmd/warden`) with policy engine, HITL queue, env scrubbing  
- Docker executor (Mirror + Ghost) and Local executor fallback
- Installer script, Docker Compose, Dockerfiles
- 19/19 unit tests passing, `go vet` clean

## Current Focus: Phase 2 — Integration & CLI

### Step 1: Integration Testing
- End-to-end test: shim ↔ warden over real Unix socket
- Test full flow: shim connects → sends request → warden evaluates policy → executes locally → streams back
- Test denied command flow
- Test HITL pending flow (approve/deny from another goroutine)

### Step 2: Command Audit Log
- Structured audit log (`internal/warden/audit.go`)
- Log every request with timestamp, command, args, policy decision, exit code
- JSON-lines format for easy parsing
- Configurable log file path

### Step 3: CLI Tool (`cmd/cli`)
- `clawrden-cli status` — Show warden state
- `clawrden-cli queue` — List pending HITL requests  
- `clawrden-cli approve <id>` — Approve pending request
- `clawrden-cli deny <id>` — Deny pending request
- `clawrden-cli history` — View command audit log
- `clawrden-cli kill` — Kill switch (pause/kill prisoner)
- Communicates with Warden via a secondary control socket or HTTP API

### Step 4: Warden HTTP API
- `/api/status` — Warden health and stats
- `/api/queue` — List pending HITL requests
- `/api/queue/:id/approve` — Approve request
- `/api/queue/:id/deny` — Deny request  
- `/api/history` — Command audit log
- `/api/kill` — Kill switch endpoint

### Step 5: Timeout Enforcement
- `context.WithTimeout` on all executions
- Configurable per-command timeouts in policy.yaml
- Auto-deny on timeout

### Step 6: Ghost Image Configuration
- Move hardcoded image map to policy.yaml
- Support custom tool → image mappings

## Future (Phase 3)
- Web Dashboard UI
- Docker Compose smoke test on real Docker
- Multi-image installer verification (Alpine, Ubuntu, python:slim)
- Chat integration (Slack/Telegram bots)
