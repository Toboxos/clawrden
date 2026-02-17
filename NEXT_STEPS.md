# Clawrden - Next Steps Quick Reference

**Last Updated:** 2026-02-17
**Current Status:** Phase 2 (4/6 complete) → Phase 3 (Production POC)

---

## 📚 Documentation Index

All planning documents created this session:

| Document | Purpose | When to Read |
|----------|---------|--------------|
| **`docs/production-roadmap.md`** | Refactored development plan | Start of each sprint |
| **`docs/chat-integration.md`** | Slack/Telegram HITL approvals | When implementing chat features |
| **`docs/docker-testing-guide.md`** | Complete Docker validation framework | Before Docker testing |
| **`docs/session-summary.md`** | Today's session overview | Review of recent work |
| **`NEXT_STEPS.md`** | This file - quick reference | Right now! |

---

## 🎯 Immediate Priorities (This Week)

### 1. Timeout Enforcement ⚡ **CRITICAL**

**Status:** Phase 2, Step 5 (last remaining item)
**Effort:** 1-2 days
**Priority:** 🔥 HIGH

**What to do:**
```bash
# Files to modify:
1. internal/warden/policy.go
   - Add `Timeout time.Duration` field to Rule struct
   - Parse timeout from policy.yaml

2. internal/executor/local.go, docker.go
   - Wrap all exec calls with context.WithTimeout
   - Return timeout error if exceeded

3. internal/warden/audit.go
   - Add "timeout_violation": true field
   - Track duration vs allowed timeout

4. Add tests for timeout behavior
```

**Policy YAML example:**
```yaml
rules:
  - command: npm
    action: ask
    timeout: 300s  # 5 minutes

default_timeout: 120s  # 2 minutes
```

**Success criteria:**
- [ ] Commands respect policy timeouts
- [ ] Timeout violations logged to audit
- [ ] Tests pass for timeout scenarios
- [ ] Agent receives proper error on timeout

---

### 2. Container Hardening Script 🛡️ **HIGH**

**Status:** Design complete (see `docs/docker-testing-guide.md`)
**Effort:** 1 day
**Priority:** 🔥 HIGH

**What to do:**
```bash
# 1. Create the script (already documented)
cp <script-from-guide> scripts/harden-container.sh
chmod +x scripts/harden-container.sh

# 2. Test on multiple base images
./scripts/harden-container.sh --base-image ubuntu:22.04 --output-image clawrden-ubuntu
./scripts/harden-container.sh --base-image alpine:latest --output-image clawrden-alpine
./scripts/harden-container.sh --base-image python:3.11-slim --output-image clawrden-python

# 3. Verify each image
docker run -it clawrden-ubuntu bash
# Inside: which npm docker git
# Should show /clawrden/bin/* paths

# 4. Validate binary locking
docker run -it clawrden-ubuntu bash
# Inside: npm.original --version  # Should exist if npm was in base image
```

**Success criteria:**
- [ ] Script works on Ubuntu, Alpine, Python base images
- [ ] Shim binaries correctly symlinked
- [ ] Original binaries locked (renamed)
- [ ] PATH precedence verified
- [ ] Directories created with correct permissions

**Script location:** `docs/docker-testing-guide.md` Part 1 (lines 60-200)

---

### 3. Docker Compose Validation 🐳 **HIGH**

**Status:** Configuration ready (see guide)
**Effort:** 2-3 days
**Priority:** 🔥 HIGH

**What to do:**

**Step 1: Update docker-compose.yml**
```bash
# Copy the complete docker-compose.yml from docs/docker-testing-guide.md
# It includes: warden + 3 prisoners + volumes + networks
```

**Step 2: Single prisoner test**
```bash
# Build hardened image first
./scripts/harden-container.sh --base-image ubuntu:22.04 --output-image clawrden-ubuntu

# Start warden + prisoner1
mkdir -p logs
docker-compose up -d warden prisoner1

# Exec into prisoner
docker exec -it clawrden-prisoner1 bash

# Run test commands
ls /app              # Should work (allowed)
touch /app/test.txt  # Should queue for approval
rm -rf /app          # Should deny immediately
```

**Step 3: Multi-prisoner test**
```bash
# Build all images
./scripts/harden-container.sh --base-image alpine:latest --output-image clawrden-alpine
./scripts/harden-container.sh --base-image python:3.11-slim --output-image clawrden-python

# Start all prisoners
docker-compose up -d

# Test concurrent commands
docker exec -it clawrden-prisoner1 bash &
docker exec -it clawrden-prisoner2 sh &
docker exec -it clawrden-prisoner3 bash &

# In each: mkdir /app/test-<prisoner-name>
# All should queue, approve all from dashboard
```

**Step 4: Run validation checklist**
```bash
# See docs/docker-testing-guide.md Part 5
# Complete all 33 checklist items
```

**Success criteria:**
- [ ] Warden starts and serves API on :8080
- [ ] All prisoners can connect via socket
- [ ] Commands execute through shim
- [ ] HITL approval workflow works
- [ ] Multi-prisoner concurrency validated
- [ ] All validation checklist items pass

---

### 4. Chat Integration (Optional Quick Win) 💬 **MEDIUM**

**Status:** Design complete + code provided
**Effort:** 2-4 hours
**Priority:** ⚠️ MEDIUM (nice to have)

**What to do:**

**Option A: Slack**
```bash
# 1. Create Slack app at https://api.slack.com/apps
# 2. Get bot token (starts with xoxb-)
# 3. Copy code from docs/chat-integration.md
# 4. Build and run:

go get github.com/slack-go/slack
go build -o bin/slack-bridge cmd/slack-bridge/main.go

export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_CHANNEL="#clawrden-approvals"
export WARDEN_API_URL="http://localhost:8080"

./bin/slack-bridge
```

**Option B: Telegram**
```bash
# 1. Talk to @BotFather, create bot
# 2. Get bot token
# 3. Copy code from docs/chat-integration.md
# 4. Build and run:

go get github.com/go-telegram-bot-api/telegram-bot-api/v5
go build -o bin/telegram-bridge cmd/telegram-bridge/main.go

export TELEGRAM_BOT_TOKEN="..."
export TELEGRAM_CHAT_ID="..."
export WARDEN_API_URL="http://localhost:8080"

./bin/telegram-bridge
```

**Success criteria:**
- [ ] Bridge service connects to warden API
- [ ] Notifications posted to chat
- [ ] Approve/deny buttons work
- [ ] Commands execute after approval
- [ ] Audit log shows chat-based approvals

---

## 📅 Weekly Sprint Plan

### Week 1 (Current)
- [x] Plan refactoring (Ghost mode deferred)
- [x] Chat integration design
- [x] Docker testing guide
- [ ] Timeout enforcement implementation
- [ ] Hardening script creation
- [ ] Single prisoner Docker test

### Week 2
- [ ] Multi-prisoner Docker validation
- [ ] Multi-distro testing (Alpine, Ubuntu, Python)
- [ ] Complete validation checklist
- [ ] Chat integration POC (Slack or Telegram)
- [ ] Metrics/Prometheus integration start

### Week 3
- [ ] Dashboard authentication (Basic Auth)
- [ ] Kill switch Docker SDK integration
- [ ] Performance benchmarking
- [ ] Security audit
- [ ] Bug fixes and polish

### Week 4
- [ ] Production POC validation
- [ ] External beta testing
- [ ] Documentation finalization
- [ ] Demo preparation

**Total time to Production POC:** ~4 weeks

---

## 🚦 Definition of "Production Ready POC"

Before declaring POC complete, verify:

### Core Functionality ✅
- [x] Command interception via shim
- [x] Policy enforcement (allow/deny/ask)
- [x] HITL approval workflow
- [x] Audit logging
- [x] Mirror execution
- [ ] Timeout enforcement

### Operational Readiness ⏳
- [x] Web dashboard
- [x] CLI tool
- [ ] Metrics/monitoring
- [ ] Docker integration validated
- [ ] Multi-distro installer tested

### Security ⏳
- [x] Path validation
- [x] Environment scrubbing
- [x] Identity preservation
- [ ] Dashboard authentication
- [x] Audit trail

### Testing ⏳
- [x] Unit tests (22/22 passing)
- [x] Integration tests (5/5 passing)
- [ ] Docker smoke tests
- [ ] Multi-distro tests

### Documentation ✅
- [x] Architecture docs
- [x] API reference
- [x] Docker deployment guide
- [x] Policy configuration guide
- [ ] Production operations guide

**Progress: 15/22 items complete (68%)**

---

## 🛠️ Quick Commands Reference

### Build Everything
```bash
make build              # All binaries
make test               # All tests
make integration-test   # Integration only
```

### Start Development Environment
```bash
# Terminal 1: Warden
./bin/clawrden-warden \
  --socket /tmp/warden.sock \
  --policy policy.yaml \
  --audit /tmp/audit.log \
  --api :8080

# Terminal 2: Dashboard
open http://localhost:8080

# Terminal 3: CLI
./bin/clawrden-cli --api http://localhost:8080 status
./bin/clawrden-cli queue
./bin/clawrden-cli history
```

### Docker Testing
```bash
# Build prisoner images
./scripts/harden-container.sh --base-image ubuntu:22.04 --output-image clawrden-ubuntu

# Start containers
docker-compose up -d

# Exec into prisoner
docker exec -it clawrden-prisoner1 bash

# View logs
docker-compose logs -f warden
docker-compose logs -f prisoner1

# Cleanup
docker-compose down -v
```

---

## 📊 Current Metrics

**Codebase:**
- Go modules: `cmd/`, `internal/`, `pkg/`
- Total tests: 22/22 passing ✅
- Binary sizes:
  - Shim: 2.4MB (static)
  - Warden: 12MB
  - CLI: 8.4MB

**Documentation:**
- Architecture: 93 lines
- README: 542 lines
- Testing guide: 800+ lines
- Chat integration: 500+ lines
- Production roadmap: 400+ lines

**Features:**
- ✅ Command interception
- ✅ Policy engine (YAML)
- ✅ HITL queue
- ✅ HTTP API
- ✅ CLI tool
- ✅ Web dashboard
- ✅ Audit logging
- ⏳ Timeout enforcement
- ⏳ Docker integration
- ❌ Ghost mode (deferred)

---

## 🔍 Where to Find Things

**Need to...**
| Task | Document | Location |
|------|----------|----------|
| Understand architecture | Architecture docs | `docs/architecture.md` |
| Check project status | Current status | `docs/current-status.md` |
| See what's next | This file | `NEXT_STEPS.md` |
| Test with Docker | Docker guide | `docs/docker-testing-guide.md` |
| Add chat integration | Chat guide | `docs/chat-integration.md` |
| Review roadmap | Production roadmap | `docs/production-roadmap.md` |
| Configure policy | Policy docs | `docs/policy-configuration.md` |
| Use web dashboard | Dashboard docs | `docs/web-dashboard.md` |
| See API reference | README | `README.md` (HTTP API section) |
| Track progress | WIP file | `wip.md` |

**Code locations:**
| Component | Path |
|-----------|------|
| Shim | `cmd/shim/main.go` |
| Warden | `cmd/warden/main.go` |
| CLI | `cmd/cli/main.go` |
| Policy engine | `internal/warden/policy.go` |
| HITL queue | `internal/warden/hitl.go` |
| Executors | `internal/executor/*.go` |
| Protocol | `pkg/protocol/*.go` |
| Tests | `tests/integration/*.go` |

---

## ❓ Decision Log

**Key decisions made this session:**

1. **Ghost Mode Deferred** ✅
   - Reason: Too complex for POC
   - Mirror mode sufficient for validation
   - Move to Phase 4 after 1000+ successful deployments

2. **Chat Integration is Feasible** ✅
   - Uses existing HTTP API
   - No warden changes needed
   - 2-4 hours per platform
   - Slack and Telegram guides provided

3. **Docker Testing Priority** ✅
   - Critical for production readiness
   - Complete guide created
   - Multi-distro validation required
   - 33-item validation checklist

4. **Timeline Estimate** ✅
   - Phase 2 completion: 1-2 days
   - Phase 3 (POC): 10-15 days
   - Total to production: ~4 weeks

---

## 🎬 Next Session Checklist

Before starting next coding session:

- [ ] Read `docs/production-roadmap.md` (5 min)
- [ ] Review `docs/session-summary.md` (3 min)
- [ ] Check this file for immediate priorities (2 min)
- [ ] Pick one task from "Immediate Priorities" section
- [ ] Update `wip.md` with current task
- [ ] Start coding!

**Recommended first task:** Timeout enforcement (highest priority, clearest scope)

---

## 🚀 Ready to Code!

**Status:** All planning complete, implementation can begin.

**Three main tracks available:**
1. **Core features** → Timeout enforcement
2. **Docker testing** → Hardening script + validation
3. **Quick wins** → Chat integration (2-4 hours)

**Choose based on:**
- Available time (short session → chat, long session → Docker)
- Skills (Go dev → timeout, DevOps → Docker)
- Impact (highest → timeout + Docker testing)

---

**All documentation is ready. Time to build!** 🛡️
