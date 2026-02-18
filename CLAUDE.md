# Clawrden - Claude Developer Guide

**Version:** 0.1.0
**Last Updated:** 2026-02-17

This file provides context and guidelines for AI assistants (Claude) working on the Clawrden codebase.

## Project Overview

**Clawrden** is a Zero Trust hypervisor for autonomous AI agents. It operates as a sidecar-based governance architecture that intercepts agent commands at the binary level for policy enforcement and human oversight.

**Core Philosophy:**
- **Zero Trust**: Agents are assumed compromised and untrusted by default
- **Human-in-the-Loop (HITL)**: Critical decisions require human approval
- **Audit Everything**: Every command is logged with full context
- **Minimal Attack Surface**: Path boundaries, environment scrubbing, binary locking

## Architecture

```
Agent Container (Prisoner)
├── Clawrden Shim (2.4MB static binary)
│   └── Intercepts: npm, docker, pip, kubectl, terraform, etc.
└── Unix Socket → Warden

Warden (Supervisor)
├── Policy Engine (YAML-based allow/deny/ask rules)
├── HITL Queue (approval workflow)
├── Executor (Mirror/Local/Ghost modes)
├── Audit Log (JSON-lines)
├── HTTP API (RESTful control)
└── Chat Bridges (Slack, Telegram)
```

**Execution Modes:**
- **Mirror**: Exec command back in prisoner container (current default)
- **Local**: Exec on warden host (development/testing)
- **Ghost**: Ephemeral Docker container (future - Phase 4)

## Project Status

**Current Phase:** Phase 2 → Phase 3 Transition

**Completed:**
- ✅ Core interception (shim + warden)
- ✅ Policy engine (allow/deny/ask)
- ✅ HITL approval queue
- ✅ Audit logging (JSON-lines)
- ✅ HTTP API + CLI tool
- ✅ Chat bridges (Slack, Telegram)
- ✅ Jail management (explicit config, API, CLI)
- ✅ 22/22 tests passing

**Next Priorities (see ROADMAP.md):**
1. Timeout enforcement (Phase 2, Step 5)
2. Docker integration testing (Phase 3.1)
3. Multi-distro validation (Phase 3.2)
4. Metrics/monitoring (Phase 3.3)
5. Dashboard authentication (Phase 3.4)

## Key Files & Locations

### Entry Points
- `cmd/shim/main.go` - Command interceptor (static binary)
- `cmd/warden/main.go` - Supervisor server
- `cmd/cli/main.go` - Control CLI
- `cmd/slack-bridge/main.go` - Slack HITL integration
- `cmd/telegram-bridge/main.go` - Telegram HITL integration

### Core Logic
- `internal/shim/` - Shim logic (socket RPC, argument passing)
- `internal/warden/` - Policy engine, HITL queue, audit log, API
- `internal/executor/` - Execution strategies (mirror, local, ghost)
- `internal/jailhouse/` - Jail filesystem management (shim symlink trees)
- `pkg/protocol/` - Socket protocol (framed JSON-RPC)

### Configuration
- `policy.yaml` - Policy rules (example in repo root)
- `docker-compose.yml` - Multi-service deployment
- `flake.nix` - Nix builds (delegates to Makefile)
- `Makefile` - **SOURCE OF TRUTH** for build commands

### Documentation
- `README.md` - User-facing documentation
- `ROADMAP.md` - Production roadmap and priorities
- `NIX.md` - Nix flake usage guide
- `docs/architecture.md` - System design and data flow
- `docs/policy-configuration.md` - YAML policy reference
- `docs/chat-integration.md` - Slack/Telegram setup
- `docs/web-dashboard.md` - Dashboard features

### Tests
- `tests/integration/` - End-to-end integration tests
- `*_test.go` - Unit tests alongside implementation files

## Development Workflow

### Building

**Use Makefile for all builds** (Nix delegates to Make):

```bash
# Core binaries (shim + warden + cli)
make build

# All binaries (core + bridges)
make build-all

# Individual components
make build-shim
make build-warden
make build-cli
make build-slack-bridge
make build-telegram-bridge

# Using Nix (calls Make internally)
nix build              # All binaries
nix build .#shim       # Specific binary
nix build .#warden-docker  # Docker image
```

### Testing

```bash
# All tests
make test              # go test -v ./...

# Integration tests only
make integration-test  # go test -v ./tests/integration/...

# Specific package
go test ./internal/warden -v
```

**Test Requirements:**
- All new features must include tests
- All tests must pass before commits
- Integration tests validate end-to-end flows
- Current status: 22/22 passing (DO NOT BREAK THIS)

### Docker Deployment

```bash
# Multi-container (production)
docker-compose up

# Build Docker image via Nix
nix build .#warden-docker
docker load < result

# All-in-one container (development)
docker run clawrden-warden:latest warden slack telegram
```

## Coding Conventions

### Go Style
- Follow standard Go conventions (`go fmt`, `go vet`)
- Use `gofmt -s` (simplified formatting)
- Avoid naked returns
- Handle all errors explicitly (no `_` without justification)
- Use context.Context for cancellation and timeouts
- Log structured data (JSON for production)

### Security-Critical Code
- **Never trust agent input**: Validate all paths, arguments, environment
- **Sanitize before execution**: Scrub dangerous env vars (LD_PRELOAD, DOCKER_HOST)
- **Preserve identity**: Pass through UID/GID from prisoner
- **Fail closed**: Default to deny if policy is ambiguous
- **Audit all actions**: Log before and after execution

### Error Handling
```go
// Prefer explicit error wrapping
if err != nil {
    return fmt.Errorf("failed to load policy: %w", err)
}

// Log errors with context
log.Printf("ERROR: failed to execute command: cmd=%s err=%v", cmd, err)
```

### Testing Patterns
```go
// Use table-driven tests
func TestPolicyEvaluate(t *testing.T) {
    tests := []struct {
        name     string
        command  string
        action   Action
    }{
        {"allow safe command", "ls", ActionAllow},
        {"deny sudo", "sudo", ActionDeny},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

## Build System: Make vs Nix

**Makefile is the source of truth** for build commands. Nix flake delegates to Make:

- **Why Make?** Simple, portable, no learning curve for contributors
- **Why Nix?** Reproducible builds, dependency management, Docker images
- **Integration:** `flake.nix` uses `buildGoModule` but overrides `buildPhase` to call `make build-all`

**When adding new binaries:**
1. Add target to `Makefile` (e.g., `build-mybinary`)
2. Add to `build-all` target
3. Update `flake.nix` to extract binary from main build
4. Update `docker-compose.yml` if it's a service

## Security Guidelines

### Path Validation
- All paths must be validated against `allowed_paths` in policy
- Reject `..` traversal attempts
- Reject absolute paths outside `/app` or `/tmp`

### Environment Scrubbing
- Strip dangerous variables: `LD_PRELOAD`, `LD_LIBRARY_PATH`, `DOCKER_HOST`
- Preserve necessary variables: `PATH`, `HOME`, `USER`
- Pass through identity: `UID`, `GID`

### Binary Locking
- Original tools renamed to `.original` (e.g., `/usr/bin/npm.original`)
- Shim symlinks take precedence via PATH ordering
- Prevents agent from bypassing interception

### Audit Trail
- Every command logged before execution
- Include: timestamp, command, args, user, decision, exit code
- JSON-lines format for machine parsing
- Never log secrets (sanitize before logging)

## Policy Configuration

**Location:** `policy.yaml` (mount into warden)

**Structure:**
```yaml
default_action: deny  # fail closed

allowed_paths:
  - "/app/*"          # workspace only
  - "/tmp/*"          # temporary files

rules:
  - command: ls       # safe read operations
    action: allow

  - command: npm      # package installation
    action: ask       # requires approval
    patterns:
      - "install*"
      - "ci*"

  - command: sudo     # privileged operations
    action: deny      # never allowed
```

**Actions:**
- `allow` - Execute immediately (safe commands)
- `deny` - Block immediately (dangerous commands)
- `ask` - Queue for human approval (risky commands)

**Best Practices:**
- Default to `deny` (fail closed)
- Use `allow` only for read-only operations
- Use `ask` for state-changing operations
- Use `deny` for privileged operations

## HTTP API

**Base URL:** `http://localhost:8080/api`

**Endpoints:**
- `GET /status` - Warden health check
- `GET /queue` - List pending approvals
- `POST /queue/:id/approve` - Approve request
- `POST /queue/:id/deny` - Deny request
- `GET /history` - View audit log
- `POST /kill` - Emergency stop
- `GET /jails` - List all jails
- `POST /jails` - Create a jail
- `GET /jails/:id` - Get jail details
- `DELETE /jails/:id` - Delete a jail

**Authentication:** None (Phase 3.4 will add Basic Auth)

## Common Tasks

### Adding a New Command to Intercept
1. No code changes needed - shim is universal
2. Add symlink in prisoner: `ln -s /clawrden/bin/clawrden-shim /clawrden/bin/mycommand`
3. Update PATH to prioritize `/clawrden/bin`
4. Add policy rule in `policy.yaml`

### Adding a Policy Action
1. Define action in `internal/warden/policy.go` (Action enum)
2. Add evaluation logic in `EvaluateCommand()`
3. Update audit log to record new action
4. Add tests in `policy_test.go`
5. Document in `docs/policy-configuration.md`

### Adding a New Executor Mode
1. Implement `Executor` interface in `internal/executor/`
2. Register in `internal/warden/server.go`
3. Add configuration to `policy.yaml` (if per-command)
4. Add tests in `tests/integration/`
5. Document in `docs/architecture.md`

### Adding a Chat Integration
1. Create new cmd (e.g., `cmd/discord-bridge/`)
2. Implement HTTP client to warden API
3. Poll `/api/queue` and post to chat
4. Accept approval/deny from chat and POST to warden
5. Add to `docker-compose.yml`
6. Document in `docs/chat-integration.md`

## Debugging Tips

### Test Shim Locally
```bash
# Build shim
make build-shim

# Start warden in one terminal
./bin/clawrden-warden --socket /tmp/warden.sock --policy policy.yaml

# Symlink shim
ln -s $(pwd)/bin/clawrden-shim /tmp/myls

# Use symlink (intercepts ls)
/tmp/myls /
```

### View Audit Log
```bash
# Tail JSON-lines log
tail -f /var/log/clawrden/audit.log | jq .

# Filter by command
jq 'select(.command == "npm")' /var/log/clawrden/audit.log
```

### Docker Socket Debugging
```bash
# Check socket mount
docker inspect <container> | jq '.[].Mounts'

# Test from prisoner
ls -la /var/run/clawrden/warden.sock

# Test from warden
ss -lxp | grep warden
```

## Pitfalls to Avoid

### DO NOT:
- **Bypass policy checks** - Every command must go through policy engine
- **Log secrets** - Sanitize environment variables and arguments
- **Use naked `os.Exec`** - Always use executor interface
- **Assume shim is installed** - Validate socket connectivity
- **Mutate global state** - Use context and dependency injection
- **Commit without tests** - All tests must pass
- **Change Makefile without Nix** - Keep `flake.nix` in sync
- **Add dependencies without updating vendorHash** - Nix will fail

### DO:
- **Validate all paths** - Use `isPathAllowed()` helper
- **Scrub environment** - Use `scrubEnvironment()` helper
- **Log structured data** - Use JSON for machine parsing
- **Handle cancellation** - Respect context.Context
- **Write table-driven tests** - Cover edge cases
- **Document public APIs** - Godoc comments for exported symbols
- **Check ROADMAP.md** - Align with current priorities

## External Dependencies

- **Go 1.21+** - Language runtime
- **Docker** - Container runtime (for Ghost mode)
- **Unix Sockets** - IPC between shim and warden
- **Make** - Build orchestration
- **Nix** - Reproducible builds (optional but recommended)

## Performance Considerations

- **Shim overhead:** Target < 10ms per command interception
- **Socket I/O:** Use framed protocol to avoid buffering issues
- **Policy evaluation:** Keep rules simple (< 100 rules)
- **Audit logging:** Async writes to avoid blocking execution
- **HITL queue:** Bounded queue size (default 1000)

## Future Considerations (Phase 4+)

- **Ghost Mode:** Ephemeral Docker containers for isolation
- **Advanced Auth:** OAuth2/OIDC for dashboard
- **WebSocket API:** Real-time updates for dashboard
- **Metrics:** Prometheus integration
- **Distributed Warden:** Multi-agent governance
- **Policy Language:** More expressive rules (e.g., OPA/Rego)

## Questions or Issues?

- Check `ROADMAP.md` for current priorities
- Read `docs/architecture.md` for design rationale
- Review existing tests for usage examples
- Consult `policy.yaml` for configuration reference

## Task Management Workflow

**All development tasks are tracked in the `tasks/` directory (gitignored).**

### Task Lifecycle

1. **Create Task**: Add new file in `tasks/` directory
   ```bash
   # Format: tasks/<task-id>-<short-name>.md
   tasks/001-timeout-enforcement.md
   tasks/002-docker-integration-tests.md
   ```

2. **Document Progress**: Update the task file as you work
   - Add implementation notes
   - Record decisions made
   - Document blockers or issues
   - Update status section

3. **Mark Complete**: When finished, mark as DONE but keep the file
   - Do NOT delete completed tasks
   - They serve as historical record and context

### Task File Structure

```markdown
# Task: <Title>

**Status:** IN_PROGRESS | DONE | BLOCKED
**Priority:** HIGH | MEDIUM | LOW
**Assignee:** <name or "claude">
**Created:** YYYY-MM-DD
**Completed:** YYYY-MM-DD (if done)

## Objective

[What needs to be accomplished]

## Context

[Why this is needed, links to ROADMAP.md, related issues]

## Progress Log

### YYYY-MM-DD - Session 1
- [x] Did this thing
- [ ] Still need to do this
- Notes: Added timeout field to policy struct

### YYYY-MM-DD - Session 2
- [x] Completed that thing
- Blocker: Need to clarify timeout behavior for long-running commands

## Implementation Details

[Technical notes, code patterns used, files modified]

## Testing

- [ ] Unit tests added
- [ ] Integration tests pass
- [ ] Manual testing completed

## Related Files

- `internal/warden/policy.go:45`
- `tests/integration/timeout_test.go`
```

### Example Task

See `tasks/000-example-task.md` (template) for reference structure.

### Why This Approach?

- **Gitignored**: Tasks contain work-in-progress notes, not production docs
- **Persistent History**: Completed tasks provide context for future work
- **Living Documents**: Tasks evolve as implementation progresses
- **Searchable**: Easy to grep for past decisions or patterns

### Task Directory Rules

1. **One task per file** - Keep tasks focused and atomic
2. **Descriptive names** - Use `<id>-<kebab-case-description>.md`
3. **Update frequently** - Document progress during work, not after
4. **Mark done, don't delete** - History matters for context
5. **Reference commits** - Link to git commits when task is complete

### Working with Tasks

```bash
# List all tasks
ls -la tasks/

# Find in-progress tasks
grep -l "Status: IN_PROGRESS" tasks/*.md

# Find tasks by keyword
grep -r "timeout" tasks/

# Create new task from template
cp tasks/000-example-task.md tasks/006-new-feature.md
```

## Working with Claude

When working on this codebase:
1. **Check tasks/ directory first** - See what's in progress or recently completed
2. **Create task file for new work** - Document as you go
3. **Update task file frequently** - Record progress, decisions, blockers
4. **Always read relevant files before modifying** - Understand context
5. **Run tests after changes** - Ensure nothing breaks
6. **Follow existing patterns** - Consistency matters
7. **Update documentation** - Keep docs in sync with code
8. **Ask before major refactors** - Discuss architectural changes
9. **Prioritize security** - This is Zero Trust infrastructure
10. **Check ROADMAP.md** - Align with project priorities
11. **Mark tasks DONE when finished** - Keep file for historical context

### Claude's Task Workflow

When starting work:
```
1. Read ROADMAP.md → understand priorities
2. Check tasks/ directory → see what's active
3. Create new task file → document objective
4. Work on implementation → update task file
5. Run tests → ensure quality
6. Mark task DONE → leave file in place
7. Reference task in commit message
```

Remember: Clawrden is security-critical infrastructure. Err on the side of caution.
