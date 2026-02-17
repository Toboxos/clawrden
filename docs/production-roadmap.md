# Production Roadmap - Refactored Plan

**Last Updated:** 2026-02-17
**Status:** Phase 2 → Phase 3 Transition

## Overview

This refactored roadmap prioritizes getting Clawrden to a **production-ready POC** before implementing advanced features like Ghost mode Docker execution. The focus is on:

1. **Core Stability**: Hardening existing features
2. **Real-World Testing**: Validating with actual Docker containers
3. **Operational Readiness**: Monitoring, logging, and control interfaces
4. **Deferred Complexity**: Ghost mode moves to post-POC phase

---

## Phase 2: Completion Tasks (2 Steps Remaining)

### ✅ Completed (4/6)
- [x] Integration testing framework
- [x] Command audit logging (JSON-lines)
- [x] HTTP API server
- [x] CLI control tool

### Step 5: Timeout Enforcement ⏳ **NEXT**

**Priority:** High
**Effort:** 1-2 days

**Scope:**
- Add `context.WithTimeout` to all executor calls
- Support per-command timeout configuration in `policy.yaml`
- Track timeout violations in audit log
- Return proper error to agent on timeout

**Implementation:**
```yaml
# policy.yaml
rules:
  - command: npm
    action: ask
    timeout: 300s  # 5 minutes max

  - command: terraform
    action: allow
    timeout: 1800s  # 30 minutes max

default_timeout: 120s  # 2 minutes default
```

**Changes Required:**
- `internal/warden/policy.go`: Add `Timeout` field to Rule struct
- `internal/executor/*.go`: Wrap all exec calls with context timeout
- `internal/warden/audit.go`: Add `timeout_violation` field
- Tests: Add timeout test cases

**Success Criteria:**
- Commands respect policy-defined timeouts
- Audit log captures timeout events
- Agent receives proper error message

---

### Step 6: Ghost Image Configuration ⏳ **DEFERRED**

**Priority:** Low (moved to post-POC)
**Reason:** Ghost mode (ephemeral Docker containers) adds complexity. For POC, we'll focus on:
- **Mirror mode only** (exec back in prisoner container)
- **Local mode** (for development/testing)

**When to implement:**
- After production POC is validated
- When users request specific tool isolation
- Phase 4: Advanced Features

---

## Phase 3: Production POC (Prioritized)

### Step 1: Docker Integration Testing ⚡ **HIGH PRIORITY**

**Goal:** Validate Clawrden works with real Docker containers, not just local testing.

**Components:**
1. **Prisoner Container Hardening Script**
   - Install shim binary
   - Lock original binaries (rename to `.original`)
   - Configure PATH precedence
   - Test on multiple base images (Alpine, Ubuntu, Python)

2. **Docker Compose Validation**
   - Multi-container setup (warden + prisoner)
   - Volume mounts for `/app` workspace
   - Unix socket sharing
   - Network isolation for prisoner

3. **Smoke Tests**
   - Agent runs commands → shim intercepts → warden enforces policy
   - HITL approval workflow end-to-end
   - Audit log persistence across restarts
   - Kill switch functionality

**Deliverables:**
- `scripts/harden-container.sh` - Automated container preparation
- `docker-compose.yml` - Updated with real integration test
- `docs/docker-testing-guide.md` - Step-by-step testing instructions
- `tests/docker-smoke/` - Automated Docker test suite

---

### Step 2: Installer Multi-Distro Validation

**Goal:** Ensure `install-clawrden.sh` works across popular base images.

**Test Matrix:**
| Base Image | Shell | Package Manager | Status |
|------------|-------|----------------|---------|
| `alpine:latest` | ash | apk | ⏳ Test |
| `ubuntu:22.04` | bash | apt | ⏳ Test |
| `debian:bullseye` | bash | apt | ⏳ Test |
| `python:3.11-slim` | bash | apt | ⏳ Test |
| `node:18-alpine` | ash | apk | ⏳ Test |

**Validation Checklist:**
- [ ] Shim binary copied to `/clawrden/bin/`
- [ ] Symlinks created for each intercepted tool
- [ ] PATH modified in `/etc/profile` and `~/.bashrc`
- [ ] Original binaries locked (renamed)
- [ ] Socket directory created with correct permissions
- [ ] Test command execution through shim

---

### Step 3: Metrics & Monitoring

**Goal:** Production observability for Clawrden operations.

**Features:**
- **Prometheus Metrics Endpoint**: `/metrics`
  - `clawrden_requests_total` - Counter by command/decision
  - `clawrden_queue_size` - Pending HITL approvals
  - `clawrden_execution_duration_seconds` - Histogram
  - `clawrden_errors_total` - Error counter

- **Structured Logging**
  - JSON format for machine parsing
  - Log levels (DEBUG, INFO, WARN, ERROR)
  - Correlation IDs for request tracing

- **Health Checks**
  - `/healthz` - Liveness probe
  - `/readyz` - Readiness probe

**Implementation:**
- Use `prometheus/client_golang`
- Add metrics middleware to warden server
- Update Docker Compose with Prometheus/Grafana stack (optional)

---

### Step 4: Web Dashboard Enhancements

**Current:** Basic HITL approval interface
**Needed for Production:**

1. **Authentication/Authorization**
   - Basic Auth (simple)
   - OAuth2/OIDC (production-grade)
   - API token support

2. **Real-Time Updates**
   - Replace polling with WebSocket
   - Server-sent events (SSE) as fallback
   - Instant notification on new requests

3. **Advanced Filtering**
   - Search audit log by command/user/date
   - Filter queue by command type
   - Export audit log (CSV/JSON)

4. **Mobile Responsive**
   - Touch-friendly approve/deny buttons
   - Mobile-optimized layout

**Priority:** Medium (authentication is critical for multi-user)

---

### Step 5: Container Kill Switch Implementation

**Current:** HTTP endpoint exists but not wired to Docker
**Needed:**

1. **Docker SDK Integration**
   - Store prisoner container ID on warden startup
   - Implement `docker pause <container>`
   - Implement `docker kill <container>`

2. **Safety Mechanisms**
   - Confirmation dialog in UI
   - Audit log entry for kill events
   - Optional auto-resume after timeout

3. **API Endpoints**
   - `POST /api/kill` - Immediate stop
   - `POST /api/pause` - Temporary pause
   - `POST /api/resume` - Resume paused container

**Implementation:**
```go
// internal/executor/docker.go
func (e *DockerExecutor) PauseContainer(ctx context.Context, containerID string) error {
    return e.client.ContainerPause(ctx, containerID)
}

func (e *DockerExecutor) KillContainer(ctx context.Context, containerID string) error {
    return e.client.ContainerKill(ctx, containerID, "SIGTERM")
}
```

---

## Phase 4: Advanced Features (Post-POC)

These features are **deferred** until after production POC validation.

### Ghost Mode (Ephemeral Containers)

**Why Defer:**
- Adds significant complexity (container lifecycle management)
- Requires image configuration (`policy.yaml` mappings)
- File ownership issues (chown after execution)
- Resource management (container cleanup, limits)

**When to Implement:**
- After 1000+ successful POC deployments
- User demand for specific tool isolation (e.g., terraform, kubectl)
- When Mirror mode limitations are validated

**Design:**
```yaml
# policy.yaml (future)
ghost_images:
  npm: node:18-alpine
  pip: python:3.11-slim
  terraform: hashicorp/terraform:latest

rules:
  - command: npm
    action: ask
    executor: ghost  # Use ephemeral container
    timeout: 600s
```

---

### Chat Integrations (Slack/Telegram)

**Priority:** Medium
**Status:** Design ready, implementation in Phase 4

**Use Case:**
- Approve HITL requests from Slack/Telegram
- Receive notifications for new pending requests
- View audit logs via chat commands

**See:** `docs/chat-integration.md` (created separately)

---

## Updated Timeline

| Phase | Tasks | Duration | Priority |
|-------|-------|----------|----------|
| **Phase 2 Completion** | Timeout enforcement | 1-2 days | 🔥 High |
| **Phase 3.1** | Docker integration testing | 3-5 days | 🔥 High |
| **Phase 3.2** | Multi-distro validation | 2-3 days | 🔥 High |
| **Phase 3.3** | Metrics/monitoring | 2-3 days | ⚠️ Medium |
| **Phase 3.4** | Dashboard auth | 2-4 days | ⚠️ Medium |
| **Phase 3.5** | Kill switch wiring | 1 day | ⚠️ Medium |
| **Phase 4** | Ghost mode + Chat | TBD | 💡 Future |

**Total to Production POC:** ~10-15 days of focused development

---

## Definition of "Production Ready POC"

A production-ready POC must demonstrate:

1. **✅ Core Functionality**
   - [x] Command interception via shim
   - [x] Policy enforcement (allow/deny/ask)
   - [x] HITL approval workflow
   - [x] Audit logging
   - [x] Mirror execution (exec in prisoner)
   - [ ] Timeout enforcement

2. **✅ Operational Readiness**
   - [x] Web dashboard for HITL
   - [x] CLI control tool
   - [ ] Metrics/monitoring
   - [ ] Docker integration validated
   - [ ] Multi-distro installer tested

3. **✅ Security**
   - [x] Path validation
   - [x] Environment scrubbing
   - [x] Identity preservation
   - [ ] Dashboard authentication
   - [x] Audit trail

4. **✅ Documentation**
   - [x] Architecture docs
   - [x] API reference
   - [ ] Docker deployment guide
   - [ ] Production operations guide
   - [x] Policy configuration guide

5. **✅ Testing**
   - [x] Unit tests (22/22 passing)
   - [x] Integration tests (5/5 passing)
   - [ ] Docker smoke tests
   - [ ] Multi-distro tests

---

## Success Metrics

**Before declaring POC ready:**
- [ ] 3+ different autonomous agents successfully governed
- [ ] 1000+ commands intercepted and logged
- [ ] 100+ HITL approvals processed
- [ ] 0 critical security issues
- [ ] < 10ms median interception latency
- [ ] Docker Compose deployment validated on 3+ distros

---

## Next Immediate Actions

**This Week:**
1. ✅ Complete timeout enforcement (Step 5)
2. ✅ Create Docker testing guide
3. ✅ Implement chat integration (quick version)
4. ✅ Build container hardening script

**Next Week:**
1. Docker integration smoke tests
2. Multi-distro installer validation
3. Metrics/Prometheus integration
4. Dashboard authentication (basic auth)

**Month 1:**
- Production POC validation
- External beta testing
- Security audit
- Performance benchmarking

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Docker socket security | High | Document principle of least privilege |
| Agent bypass of shim | High | Binary locking + PATH validation |
| Dashboard auth complexity | Medium | Start with Basic Auth, evolve to OAuth |
| Performance overhead | Medium | Benchmark and optimize socket I/O |
| Multi-distro edge cases | Low | Comprehensive test matrix |

---

**Summary:** Focus on production-ready core functionality first. Ghost mode and advanced features come after POC validation. Prioritize Docker integration testing and operational tooling.
