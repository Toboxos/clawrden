# Docker Testing Guide - Complete Setup and Validation

**Last Updated:** 2026-02-17
**Audience:** Developers, QA Engineers, DevOps

## Overview

This guide provides a **complete, step-by-step process** to test Clawrden with real Docker containers:

1. **Container Hardening** - Prepare a prisoner container with shim binaries
2. **Warden Deployment** - Spin up the supervisor with proper configuration
3. **Multi-Prisoner Setup** - Test with multiple agents simultaneously
4. **Validation Tests** - Verify all functionality works end-to-end

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Part 1: Container Hardening Script](#part-1-container-hardening-script)
3. [Part 2: Warden Setup](#part-2-warden-setup)
4. [Part 3: Single Prisoner Test](#part-3-single-prisoner-test)
5. [Part 4: Multi-Prisoner Test](#part-4-multi-prisoner-test)
6. [Part 5: Validation Checklist](#part-5-validation-checklist)
7. [Part 6: Troubleshooting](#part-6-troubleshooting)
8. [Part 7: Cleanup](#part-7-cleanup)

---

## Prerequisites

**System Requirements:**
- Docker 20.10+ installed
- Docker Compose v2.0+
- Linux or WSL2 (required for Unix sockets)
- 4GB+ RAM available
- 10GB+ disk space

**Clawrden Artifacts:**
```bash
# Build all binaries first
make build

# Verify binaries exist
ls -lh bin/
# Should see:
#   clawrden-shim    (2.4MB)
#   clawrden-warden  (12MB)
#   clawrden-cli     (8.4MB)
```

---

## Part 1: Container Hardening Script

This script **transforms any existing Docker image** into a Clawrden-compatible prisoner container.

### Create `scripts/harden-container.sh`

```bash
#!/bin/bash
set -e

###############################################################################
# Clawrden Container Hardening Script
#
# Usage: ./harden-container.sh [OPTIONS]
#
# This script installs the Clawrden shim binary into a Docker container and
# locks the original binaries to enforce interception.
#
# Options:
#   --base-image IMAGE    Base Docker image to harden (default: ubuntu:22.04)
#   --user UID:GID        User to run as (default: 1000:1000)
#   --lock-binaries LIST  Comma-separated list of binaries to intercept
#                         (default: npm,docker,pip,kubectl,git)
#   --output-image IMAGE  Name for output image (default: clawrden-prisoner)
#   --shim-path PATH      Path to clawrden-shim binary (default: ./bin/clawrden-shim)
#
# Examples:
#   ./harden-container.sh --base-image python:3.11-slim
#   ./harden-container.sh --lock-binaries "npm,yarn,git" --output-image clawrden-node
###############################################################################

# Default values
BASE_IMAGE="ubuntu:22.04"
USER_UID=1000
USER_GID=1000
LOCK_BINARIES="npm,docker,pip,kubectl,git"
OUTPUT_IMAGE="clawrden-prisoner"
SHIM_PATH="./bin/clawrden-shim"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --base-image)
      BASE_IMAGE="$2"
      shift 2
      ;;
    --user)
      IFS=':' read -r USER_UID USER_GID <<< "$2"
      shift 2
      ;;
    --lock-binaries)
      LOCK_BINARIES="$2"
      shift 2
      ;;
    --output-image)
      OUTPUT_IMAGE="$2"
      shift 2
      ;;
    --shim-path)
      SHIM_PATH="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Verify shim binary exists
if [ ! -f "$SHIM_PATH" ]; then
  echo "ERROR: Shim binary not found at $SHIM_PATH"
  echo "Run 'make build-shim' first"
  exit 1
fi

echo "🛡️  Clawrden Container Hardening"
echo "================================"
echo "Base image:      $BASE_IMAGE"
echo "Output image:    $OUTPUT_IMAGE"
echo "User:            $USER_UID:$USER_GID"
echo "Lock binaries:   $LOCK_BINARIES"
echo ""

# Create temporary directory for build context
BUILD_DIR=$(mktemp -d)
trap "rm -rf $BUILD_DIR" EXIT

# Copy shim binary
cp "$SHIM_PATH" "$BUILD_DIR/clawrden-shim"
chmod +x "$BUILD_DIR/clawrden-shim"

# Generate Dockerfile
cat > "$BUILD_DIR/Dockerfile" <<EOF
FROM $BASE_IMAGE

# Install dependencies (if needed)
RUN if command -v apt-get >/dev/null 2>&1; then \\
      apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*; \\
    elif command -v apk >/dev/null 2>&1; then \\
      apk add --no-cache ca-certificates; \\
    fi

# Create clawrden directories
RUN mkdir -p /clawrden/bin /var/run/clawrden /app && \\
    chmod 755 /clawrden/bin /var/run/clawrden

# Copy the universal shim binary
COPY clawrden-shim /clawrden/bin/clawrden-shim
RUN chmod +x /clawrden/bin/clawrden-shim

# Create symlinks for each intercepted tool
EOF

# Add symlink commands for each binary
IFS=',' read -ra BINARIES <<< "$LOCK_BINARIES"
for binary in "${BINARIES[@]}"; do
  cat >> "$BUILD_DIR/Dockerfile" <<EOF
RUN ln -sf /clawrden/bin/clawrden-shim /clawrden/bin/$binary && \\
    if command -v $binary >/dev/null 2>&1; then \\
      ORIG=\$(command -v $binary) && \\
      mv "\$ORIG" "\$ORIG.original" 2>/dev/null || true; \\
    fi
EOF
done

# Continue Dockerfile
cat >> "$BUILD_DIR/Dockerfile" <<EOF

# Update PATH to prioritize Clawrden binaries
ENV PATH="/clawrden/bin:\$PATH"

# Add PATH to shell profiles (for interactive shells)
RUN echo 'export PATH="/clawrden/bin:\$PATH"' >> /etc/profile && \\
    echo 'export PATH="/clawrden/bin:\$PATH"' >> /etc/bash.bashrc 2>/dev/null || true && \\
    echo 'export PATH="/clawrden/bin:\$PATH"' >> /etc/zsh/zshenv 2>/dev/null || true

# Create non-root user if specified
RUN if [ $USER_UID -ne 0 ]; then \\
      groupadd -g $USER_GID clawrden 2>/dev/null || true && \\
      useradd -u $USER_UID -g $USER_GID -s /bin/bash -m clawrden 2>/dev/null || true && \\
      chown -R $USER_UID:$USER_GID /app /var/run/clawrden; \\
    fi

# Set working directory
WORKDIR /app

# Set user
USER $USER_UID:$USER_GID

# Default command (override in docker-compose)
CMD ["/bin/bash"]
EOF

echo "📝 Generated Dockerfile:"
cat "$BUILD_DIR/Dockerfile"
echo ""

# Build the image
echo "🔨 Building hardened image: $OUTPUT_IMAGE"
docker build -t "$OUTPUT_IMAGE" "$BUILD_DIR"

echo ""
echo "✅ Success! Hardened image created: $OUTPUT_IMAGE"
echo ""
echo "📋 Verification:"
docker run --rm "$OUTPUT_IMAGE" sh -c "ls -la /clawrden/bin/ && which npm docker pip git 2>/dev/null || true"

echo ""
echo "🚀 Next steps:"
echo "   1. docker run -it $OUTPUT_IMAGE /bin/bash"
echo "   2. Inside container: npm --version  (will use shim)"
echo "   3. Set up Warden to handle shim requests"
```

Make it executable:
```bash
chmod +x scripts/harden-container.sh
```

### Test the Hardening Script

```bash
# Test with Ubuntu
./scripts/harden-container.sh \
  --base-image ubuntu:22.04 \
  --output-image clawrden-ubuntu

# Test with Alpine
./scripts/harden-container.sh \
  --base-image alpine:latest \
  --output-image clawrden-alpine

# Test with Python
./scripts/harden-container.sh \
  --base-image python:3.11-slim \
  --lock-binaries "pip,python,git" \
  --output-image clawrden-python

# Test with Node.js
./scripts/harden-container.sh \
  --base-image node:18-alpine \
  --lock-binaries "npm,yarn,node,git" \
  --output-image clawrden-node
```

### Verify Hardening

```bash
# Run the hardened image
docker run -it --rm clawrden-ubuntu bash

# Inside container, test:
which npm         # Should show: /clawrden/bin/npm
ls -la /clawrden/bin/
file /clawrden/bin/npm  # Should show it's the shim binary
echo $PATH        # Should start with /clawrden/bin

# Try running a locked binary (will fail without warden)
npm --version     # Will hang or error (no warden listening)
```

---

## Part 2: Warden Setup

### Create `docker-compose.yml` for Testing

```yaml
version: '3.8'

services:
  # The Warden (Supervisor)
  warden:
    image: alpine:latest
    container_name: clawrden-warden
    hostname: warden

    # Mount the warden binary
    volumes:
      - ./bin/clawrden-warden:/usr/local/bin/clawrden-warden:ro
      - ./policy.yaml:/etc/clawrden/policy.yaml:ro

      # Shared socket directory (all prisoners will mount this)
      - socket-dir:/var/run/clawrden

      # Docker socket (for Mirror/Ghost execution)
      - /var/run/docker.sock:/var/run/docker.sock

      # Audit logs
      - ./logs:/var/log/clawrden

      # Shared workspace
      - workspace:/app

    # Warden needs full privileges for Docker operations
    privileged: true

    # Environment
    environment:
      - CLAWRDEN_PRISONER_ID=prisoner1  # Will be updated dynamically

    # Command
    command: >
      /usr/local/bin/clawrden-warden
      --socket /var/run/clawrden/warden.sock
      --policy /etc/clawrden/policy.yaml
      --audit /var/log/clawrden/audit.log
      --api :8080

    # Expose API
    ports:
      - "8080:8080"

    # Network
    networks:
      - clawrden

    # Health check
    healthcheck:
      test: ["CMD", "wget", "-q", "-O-", "http://localhost:8080/api/status"]
      interval: 10s
      timeout: 5s
      retries: 3

  # Prisoner 1 (Ubuntu-based agent)
  prisoner1:
    image: clawrden-ubuntu  # Built by harden-container.sh
    container_name: clawrden-prisoner1
    hostname: prisoner1

    # Mount shared volumes
    volumes:
      - socket-dir:/var/run/clawrden   # Warden socket
      - workspace:/app                  # Shared workspace

    # Environment
    environment:
      - CLAWRDEN_SOCKET=/var/run/clawrden/warden.sock

    # No network access (firewalled)
    network_mode: none

    # Keep container running
    command: tail -f /dev/null

    depends_on:
      warden:
        condition: service_healthy

  # Prisoner 2 (Alpine-based agent) - Optional
  prisoner2:
    image: clawrden-alpine
    container_name: clawrden-prisoner2
    hostname: prisoner2

    volumes:
      - socket-dir:/var/run/clawrden
      - workspace:/app

    environment:
      - CLAWRDEN_SOCKET=/var/run/clawrden/warden.sock

    network_mode: none

    command: tail -f /dev/null

    depends_on:
      warden:
        condition: service_healthy

  # Prisoner 3 (Python-based agent) - Optional
  prisoner3:
    image: clawrden-python
    container_name: clawrden-prisoner3
    hostname: prisoner3

    volumes:
      - socket-dir:/var/run/clawrden
      - workspace:/app

    environment:
      - CLAWRDEN_SOCKET=/var/run/clawrden/warden.sock

    network_mode: none

    command: tail -f /dev/null

    depends_on:
      warden:
        condition: service_healthy

# Named volumes
volumes:
  socket-dir:
    name: clawrden-socket
  workspace:
    name: clawrden-workspace

# Networks
networks:
  clawrden:
    name: clawrden-net
    driver: bridge
```

### Create Test Policy

Create `policy.yaml`:

```yaml
# Clawrden Test Policy
default_action: deny

# Path restrictions
allowed_paths:
  - "/app/*"
  - "/tmp/*"

rules:
  # Safe read-only commands - auto-allow
  - command: ls
    action: allow

  - command: pwd
    action: allow

  - command: whoami
    action: allow

  - command: cat
    action: allow

  - command: echo
    action: allow

  - command: date
    action: allow

  # Write operations - require approval
  - command: touch
    action: ask

  - command: mkdir
    action: ask

  - command: cp
    action: ask

  - command: mv
    action: ask

  # Package managers - require approval
  - command: npm
    action: ask

  - command: pip
    action: ask

  - command: apt-get
    action: ask

  # Version control - require approval
  - command: git
    action: ask
    patterns:
      - "clone"
      - "pull"
      - "push"
      - "commit"

  # Dangerous commands - always deny
  - command: rm
    action: deny
    patterns:
      - "-rf"
      - "-fr"

  - command: sudo
    action: deny

  - command: su
    action: deny

  - command: chmod
    action: deny
    patterns:
      - "777"
      - "666"

  # Docker commands - deny (agents shouldn't use Docker directly)
  - command: docker
    action: deny

  - command: kubectl
    action: deny
```

---

## Part 3: Single Prisoner Test

### Step 1: Build Hardened Images

```bash
# Build the prisoner images
./scripts/harden-container.sh --base-image ubuntu:22.04 --output-image clawrden-ubuntu
./scripts/harden-container.sh --base-image alpine:latest --output-image clawrden-alpine
./scripts/harden-container.sh --base-image python:3.11-slim --output-image clawrden-python
```

### Step 2: Start Warden and Single Prisoner

```bash
# Create logs directory
mkdir -p logs

# Start only warden and prisoner1
docker-compose up -d warden prisoner1

# Verify containers are running
docker-compose ps

# Check warden logs
docker-compose logs -f warden

# Should see:
#   Starting socket server on /var/run/clawrden/warden.sock
#   Starting HTTP API server on :8080
#   Loaded policy from /etc/clawrden/policy.yaml
```

### Step 3: Get Prisoner Container ID

```bash
# Get prisoner1 container ID
PRISONER_ID=$(docker ps -qf "name=clawrden-prisoner1")
echo "Prisoner 1 ID: $PRISONER_ID"

# Update warden environment (if needed)
docker-compose exec warden sh -c "export CLAWRDEN_PRISONER_ID=$PRISONER_ID"
```

### Step 4: Run Commands in Prisoner

Open a new terminal and exec into the prisoner:

```bash
# Enter prisoner container
docker exec -it clawrden-prisoner1 bash

# Inside prisoner, run test commands:

# Test 1: Allowed command (should execute immediately)
ls /app

# Test 2: Echo (allowed)
echo "Hello from prisoner"

# Test 3: Create file (requires approval - will hang waiting)
touch /app/test.txt
```

### Step 5: Approve Command via Dashboard

In another terminal:

```bash
# Open dashboard in browser
open http://localhost:8080

# Or use CLI to see pending requests
./bin/clawrden-cli --api http://localhost:8080 queue

# You should see the 'touch' command pending
# Approve it:
./bin/clawrden-cli --api http://localhost:8080 approve <request-id>
```

Back in the prisoner terminal, the `touch` command should now complete.

### Step 6: Test Denied Command

In prisoner terminal:

```bash
# Try a denied command
rm -rf /app

# Should immediately fail with exit code 1
# Check warden logs to see it was denied
```

### Step 7: View Audit Log

```bash
# View audit log
cat logs/audit.log | jq .

# Or use CLI
./bin/clawrden-cli --api http://localhost:8080 history

# Should see all executed commands with timestamps, decisions, exit codes
```

---

## Part 4: Multi-Prisoner Test

Test multiple agents running simultaneously.

### Step 1: Start All Prisoners

```bash
# Start all containers
docker-compose up -d

# Verify all running
docker-compose ps

# Should see:
#   clawrden-warden
#   clawrden-prisoner1
#   clawrden-prisoner2
#   clawrden-prisoner3
```

### Step 2: Concurrent Command Execution

Open **3 separate terminals**, one for each prisoner:

**Terminal 1 (Prisoner 1 - Ubuntu):**
```bash
docker exec -it clawrden-prisoner1 bash

# Run commands
ls /app
echo "Prisoner 1: Ubuntu" > /app/prisoner1.txt
cat /app/prisoner1.txt
```

**Terminal 2 (Prisoner 2 - Alpine):**
```bash
docker exec -it clawrden-prisoner2 sh

# Run commands
ls /app
echo "Prisoner 2: Alpine" > /app/prisoner2.txt
cat /app/prisoner2.txt
```

**Terminal 3 (Prisoner 3 - Python):**
```bash
docker exec -it clawrden-prisoner3 bash

# Run commands
ls /app
python -c "print('Prisoner 3: Python')" > /app/prisoner3.txt
cat /app/prisoner3.txt
```

### Step 3: Test HITL Queue with Multiple Requests

In each prisoner terminal, run a command that requires approval **at the same time**:

**Prisoner 1:**
```bash
mkdir /app/prisoner1-dir
```

**Prisoner 2:**
```bash
mkdir /app/prisoner2-dir
```

**Prisoner 3:**
```bash
mkdir /app/prisoner3-dir
```

All three will hang waiting for approval.

**In dashboard or CLI:**
```bash
# View queue (should show 3 pending requests)
./bin/clawrden-cli queue

# Approve all
./bin/clawrden-cli approve <id1>
./bin/clawrden-cli approve <id2>
./bin/clawrden-cli approve <id3>
```

All three commands should complete simultaneously.

### Step 4: Test Workspace Sharing

All prisoners share the same `/app` volume:

**Prisoner 1:**
```bash
echo "Shared data" > /app/shared.txt
```

**Prisoner 2:**
```bash
cat /app/shared.txt  # Should see "Shared data"
```

**Prisoner 3:**
```bash
cat /app/shared.txt  # Should see "Shared data"
```

---

## Part 5: Validation Checklist

### Functional Tests

- [ ] **Command Interception**
  - [ ] Shim binary executes when calling intercepted commands
  - [ ] Original binaries are locked (renamed to `.original`)
  - [ ] PATH prioritizes `/clawrden/bin`

- [ ] **Policy Enforcement**
  - [ ] Allowed commands execute immediately
  - [ ] Denied commands fail with exit code 1
  - [ ] Ask commands queue for HITL approval

- [ ] **HITL Workflow**
  - [ ] Pending requests appear in queue
  - [ ] Web dashboard shows pending requests
  - [ ] CLI can approve/deny requests
  - [ ] Approved commands execute correctly
  - [ ] Denied commands return error to agent

- [ ] **Audit Logging**
  - [ ] All commands logged to audit file
  - [ ] Timestamps in RFC3339 format
  - [ ] Exit codes captured
  - [ ] Duration measured in milliseconds
  - [ ] Policy decisions recorded

- [ ] **Multi-Prisoner**
  - [ ] Multiple prisoners can connect simultaneously
  - [ ] Concurrent requests handled without conflicts
  - [ ] Workspace sharing works correctly
  - [ ] Each prisoner properly isolated (no network access)

- [ ] **API & Dashboard**
  - [ ] `/api/status` returns warden health
  - [ ] `/api/queue` lists pending requests
  - [ ] `/api/history` returns audit log
  - [ ] Dashboard auto-refreshes
  - [ ] Approve/deny buttons work

### Security Tests

- [ ] **Path Validation**
  - [ ] Commands outside `/app` are denied
  - [ ] Path traversal attacks blocked (`../../etc/passwd`)

- [ ] **Environment Scrubbing**
  - [ ] `LD_PRELOAD` filtered out
  - [ ] `DOCKER_HOST` not passed through
  - [ ] Safe variables (PATH, LANG) preserved

- [ ] **Network Isolation**
  - [ ] Prisoners have no internet access
  - [ ] Cannot reach Docker socket directly
  - [ ] Can only communicate via Unix socket to warden

- [ ] **Binary Locking**
  - [ ] Cannot execute `.original` binaries
  - [ ] Cannot modify PATH to bypass shim
  - [ ] Cannot delete shim binary

### Performance Tests

- [ ] **Latency**
  - [ ] Shim adds < 10ms overhead for allowed commands
  - [ ] Policy evaluation < 1ms

- [ ] **Concurrency**
  - [ ] Handle 10+ concurrent requests without deadlock
  - [ ] Audit log writes are thread-safe

- [ ] **Resource Usage**
  - [ ] Warden uses < 100MB RAM under normal load
  - [ ] Socket buffer doesn't overflow

---

## Part 6: Troubleshooting

### Issue: Prisoner commands hang indefinitely

**Symptoms:**
- Commands never return
- No output from shim

**Diagnosis:**
```bash
# Check if warden is running
docker-compose ps warden

# Check socket exists
docker exec clawrden-warden ls -la /var/run/clawrden/

# Check prisoner can access socket
docker exec clawrden-prisoner1 ls -la /var/run/clawrden/
```

**Solutions:**
- Ensure socket volume is mounted in both warden and prisoner
- Check warden logs for errors: `docker-compose logs warden`
- Verify socket permissions: Should be `0666` or `0777`

---

### Issue: Shim not found

**Symptoms:**
```
bash: npm: command not found
```

**Diagnosis:**
```bash
docker exec clawrden-prisoner1 which npm
docker exec clawrden-prisoner1 echo $PATH
docker exec clawrden-prisoner1 ls -la /clawrden/bin/
```

**Solutions:**
- Rebuild prisoner image: `./scripts/harden-container.sh ...`
- Verify PATH: Should start with `/clawrden/bin`
- Check symlinks: `ls -la /clawrden/bin/npm`

---

### Issue: Warden can't exec in prisoner (Mirror mode)

**Symptoms:**
```
Error: container not found
```

**Diagnosis:**
```bash
# Check CLAWRDEN_PRISONER_ID is set
docker exec clawrden-warden env | grep PRISONER

# Check Docker socket mounted
docker exec clawrden-warden ls -la /var/run/docker.sock

# Test Docker connectivity
docker exec clawrden-warden docker ps
```

**Solutions:**
- Set `CLAWRDEN_PRISONER_ID` environment variable in warden
- Ensure `/var/run/docker.sock` is mounted
- Check warden has Docker client installed

---

### Issue: Permission denied writing to /app

**Symptoms:**
```
touch: cannot touch '/app/test.txt': Permission denied
```

**Diagnosis:**
```bash
# Check ownership of /app in prisoner
docker exec clawrden-prisoner1 ls -ld /app

# Check UID/GID of prisoner user
docker exec clawrden-prisoner1 id
```

**Solutions:**
- Ensure prisoner runs as correct UID/GID (1000:1000)
- Check `/app` volume ownership matches prisoner user
- Fix with: `docker exec clawrden-warden chown -R 1000:1000 /app`

---

### Issue: Audit log not writing

**Symptoms:**
- `logs/audit.log` is empty
- No entries in history

**Diagnosis:**
```bash
# Check log directory exists
ls -la logs/

# Check warden logs for errors
docker-compose logs warden | grep audit
```

**Solutions:**
- Create logs directory: `mkdir -p logs`
- Check warden has write permissions to volume
- Verify `--audit` flag points to mounted volume path

---

## Part 7: Cleanup

### Stop All Containers

```bash
# Stop all containers
docker-compose down

# Remove volumes (WARNING: deletes workspace data)
docker-compose down -v

# Remove all containers, networks, volumes
docker-compose down -v --remove-orphans
```

### Remove Hardened Images

```bash
# List clawrden images
docker images | grep clawrden

# Remove specific image
docker rmi clawrden-ubuntu
docker rmi clawrden-alpine
docker rmi clawrden-python

# Remove all clawrden images
docker images | grep clawrden | awk '{print $3}' | xargs docker rmi -f
```

### Clear Logs

```bash
rm -rf logs/
```

### Full Reset

```bash
# Stop everything
docker-compose down -v --remove-orphans

# Remove images
docker rmi clawrden-ubuntu clawrden-alpine clawrden-python

# Clean Docker system
docker system prune -af --volumes
```

---

## Advanced Testing Scenarios

### Scenario 1: Agent Installing Packages

**Setup:** Python prisoner tries to install packages

```bash
docker exec -it clawrden-prisoner3 bash

# Try to install package (will require approval)
pip install requests
```

**Expected:**
- Command queues for approval
- Appears in dashboard
- After approval, executes via Ghost mode (if implemented) or Local mode

---

### Scenario 2: Multi-Step Workflow

**Setup:** Agent performs git workflow

```bash
docker exec -it clawrden-prisoner1 bash

# Initialize repo
cd /app
git init  # Requires approval
git config user.name "Test"
git config user.email "test@example.com"

# Create file and commit
echo "Test" > README.md
git add README.md  # Requires approval
git commit -m "Initial commit"  # Requires approval
```

**Expected:**
- Each git command queues separately
- Can approve in batch from dashboard
- Audit log shows full workflow

---

### Scenario 3: Stress Test - 100 Concurrent Requests

```bash
# Script to generate concurrent load
for i in {1..100}; do
  docker exec clawrden-prisoner1 sh -c "echo test$i > /app/file$i.txt" &
done

wait
```

**Expected:**
- All requests queue properly
- No deadlocks or race conditions
- Audit log captures all 100 commands
- Performance remains acceptable

---

## Summary

**This guide covers:**

✅ **Container Hardening** - Automated script to prepare any image
✅ **Warden Deployment** - Docker Compose orchestration
✅ **Single Prisoner Testing** - Basic validation
✅ **Multi-Prisoner Testing** - Concurrent agents
✅ **Validation Checklist** - Complete test matrix
✅ **Troubleshooting** - Common issues and fixes
✅ **Cleanup** - Proper teardown

**Total Testing Time:** ~2-3 hours for comprehensive validation

**Next Steps:**
1. Run through checklist systematically
2. Document any issues encountered
3. Update policy.yaml based on real-world needs
4. Scale to more prisoners if needed
5. Implement monitoring/metrics for production

---

**Ready for Production POC when all checklist items pass!** 🛡️
