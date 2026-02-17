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

## Completed (Phase 3 - Partial) ✅

### Web Dashboard UI ✅
- Real-time status monitoring (auto-refresh every 2s)
- One-click approve/deny for HITL requests
- Command history viewer (last 20 commands)
- Dark theme optimized for extended viewing
- Pure HTML/CSS/JS (no build dependencies)
- Embedded in warden binary via go:embed
- Served at http://localhost:8080/

## Current Focus: Production POC Preparation

### Just Completed ✅
- **Production Roadmap** - Refactored plan prioritizing POC over Ghost mode
- **Chat Integration Guide** - Quick Slack/Telegram implementation (2-4 hours per platform)
- **Docker Testing Guide** - Complete step-by-step validation framework

### Next Immediate Tasks (This Week)

1. **Timeout Enforcement** (Phase 2, Step 5) ⏳
   - Add context.WithTimeout to executors
   - Support per-command timeouts in policy.yaml
   - Track violations in audit log

2. **Container Hardening Script** (Phase 3.1) ⏳
   - Implement `scripts/harden-container.sh`
   - Test on Alpine, Ubuntu, Python base images
   - Validate binary locking mechanism

3. **Docker Compose Validation** (Phase 3.1) ⏳
   - Update docker-compose.yml with multi-prisoner setup
   - Test socket sharing between warden and prisoners
   - Validate network isolation

4. **Chat Integration POC** (Phase 3 - Optional Quick Win)
   - Implement Slack bridge service
   - Test HITL approval via chat
   - 2-4 hours implementation time

### Deferred to Phase 4 (Post-POC)
- Ghost mode (ephemeral Docker containers)
- Ghost image configuration in policy.yaml
- WebSocket real-time updates
- Multi-user authentication
