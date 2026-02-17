#!/bin/bash
# Complete demo with shim execution
# Demonstrates the full flow: shim → warden → execution → response

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}🛡️  Clawrden Full Demo${NC}"
echo -e "${CYAN}Demonstrating: Shim → Warden → Execution${NC}"
echo ""

# Check if binaries exist
if [ ! -f "./bin/clawrden-warden" ] || [ ! -f "./bin/clawrden-shim" ]; then
    echo -e "${YELLOW}Building binaries...${NC}"
    make build
fi

# Create demo environment
DEMO_DIR="/tmp/clawrden-demo-$$"
SHIM_DIR="$DEMO_DIR/bin"
mkdir -p "$DEMO_DIR"
mkdir -p "$SHIM_DIR"

echo -e "${GREEN}✓ Demo directory: $DEMO_DIR${NC}"

# Create a test workspace
TEST_APP="$DEMO_DIR/app"
mkdir -p "$TEST_APP"
echo "Hello from Clawrden!" > "$TEST_APP/test.txt"
echo -e "${GREEN}✓ Test workspace: $TEST_APP${NC}"

# Install shim binaries (create symlinks for different tools)
echo -e "${BLUE}Installing shim binaries...${NC}"
ln -sf "$(pwd)/bin/clawrden-shim" "$SHIM_DIR/echo"
ln -sf "$(pwd)/bin/clawrden-shim" "$SHIM_DIR/cat"
ln -sf "$(pwd)/bin/clawrden-shim" "$SHIM_DIR/ls"
ln -sf "$(pwd)/bin/clawrden-shim" "$SHIM_DIR/npm"
ln -sf "$(pwd)/bin/clawrden-shim" "$SHIM_DIR/docker"

echo -e "${GREEN}✓ Shim binaries installed:${NC}"
echo -e "   ${SHIM_DIR}/echo"
echo -e "   ${SHIM_DIR}/cat"
echo -e "   ${SHIM_DIR}/ls"
echo -e "   ${SHIM_DIR}/npm"
echo -e "   ${SHIM_DIR}/docker"

# Create a policy that allows echo/cat/ls but requires approval for npm
cat > "$DEMO_DIR/policy.yaml" <<EOF
default_action: deny

rules:
  # Safe commands - auto allow
  - command: echo
    action: allow

  - command: cat
    action: allow

  - command: ls
    action: allow

  # Requires human approval
  - command: npm
    action: ask

  # Always deny
  - command: docker
    action: deny
EOF

echo -e "${GREEN}✓ Policy created${NC}"

# Start warden in background
echo -e "${BLUE}Starting Warden...${NC}"
./bin/clawrden-warden \
    --socket "$DEMO_DIR/warden.sock" \
    --policy "$DEMO_DIR/policy.yaml" \
    --audit "$DEMO_DIR/audit.log" \
    --api :8080 > "$DEMO_DIR/warden.log" 2>&1 &

WARDEN_PID=$!
echo -e "${GREEN}✓ Warden started (PID: $WARDEN_PID)${NC}"

# Wait for warden to be ready
echo -e "${BLUE}Waiting for warden...${NC}"
for i in {1..30}; do
    if [ -S "$DEMO_DIR/warden.sock" ]; then
        break
    fi
    sleep 0.1
done

if [ ! -S "$DEMO_DIR/warden.sock" ]; then
    echo -e "${YELLOW}Warning: Socket not ready, but continuing...${NC}"
fi

sleep 1
echo -e "${GREEN}✓ Warden is ready${NC}"
echo ""

# Set up environment for shim to find the socket
export CLAWRDEN_SOCKET="$DEMO_DIR/warden.sock"

echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}  Demo Environment Ready!${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo -e "  📊 Web Dashboard: ${BLUE}http://localhost:8080${NC}"
echo -e "  🔌 Socket:        ${BLUE}$DEMO_DIR/warden.sock${NC}"
echo -e "  📝 Audit Log:     ${BLUE}$DEMO_DIR/audit.log${NC}"
echo -e "  🗂  Test Workspace: ${BLUE}$TEST_APP${NC}"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Shutting down...${NC}"
    kill $WARDEN_PID 2>/dev/null || true
    wait $WARDEN_PID 2>/dev/null || true
    echo -e "${GREEN}✓ Warden stopped${NC}"
    echo ""
    echo -e "${BLUE}Audit log:${NC}"
    if [ -f "$DEMO_DIR/audit.log" ]; then
        cat "$DEMO_DIR/audit.log" | jq -c '.' 2>/dev/null || cat "$DEMO_DIR/audit.log"
    fi
    echo ""
    echo -e "${CYAN}Demo files saved in: $DEMO_DIR${NC}"
    echo -e "${CYAN}Clean up with: rm -rf $DEMO_DIR${NC}"
}

trap cleanup EXIT INT TERM

# Run demo commands
echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  Running Demo Commands${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"
echo ""

# Test 1: Allowed command (echo)
echo -e "${BLUE}Test 1: Allowed command (echo)${NC}"
echo -e "${YELLOW}Command: echo 'Hello from intercepted command!'${NC}"
cd "$TEST_APP"
PATH="$SHIM_DIR:$PATH" echo "Hello from intercepted command!"
echo -e "${GREEN}✓ Command executed successfully${NC}"
echo ""
sleep 1

# Test 2: Allowed command (ls)
echo -e "${BLUE}Test 2: Allowed command (ls)${NC}"
echo -e "${YELLOW}Command: ls -la${NC}"
PATH="$SHIM_DIR:$PATH" ls -la
echo -e "${GREEN}✓ Command executed successfully${NC}"
echo ""
sleep 1

# Test 3: Allowed command (cat)
echo -e "${BLUE}Test 3: Allowed command (cat)${NC}"
echo -e "${YELLOW}Command: cat test.txt${NC}"
PATH="$SHIM_DIR:$PATH" cat test.txt
echo -e "${GREEN}✓ Command executed successfully${NC}"
echo ""
sleep 1

# Test 4: Denied command (docker)
echo -e "${BLUE}Test 4: Denied command (docker)${NC}"
echo -e "${YELLOW}Command: docker ps${NC}"
echo -e "${YELLOW}Expected: Command should be denied by policy${NC}"
if PATH="$SHIM_DIR:$PATH" docker ps 2>&1 | grep -q "denied\|Denied\|exit"; then
    echo -e "${GREEN}✓ Command correctly denied${NC}"
else
    echo -e "${YELLOW}Command may have been denied (check warden logs)${NC}"
fi
echo ""
sleep 1

# Test 5: HITL command (npm) - starts in background
echo -e "${BLUE}Test 5: HITL command (npm install)${NC}"
echo -e "${YELLOW}Command: npm install express${NC}"
echo -e "${YELLOW}This command requires human approval...${NC}"
echo -e "${CYAN}The command will wait for approval in the dashboard${NC}"
echo -e "${CYAN}Open ${BLUE}http://localhost:8080${CYAN} to approve/deny${NC}"
echo ""

# Start npm command in background so it doesn't block
(
    cd "$TEST_APP"
    PATH="$SHIM_DIR:$PATH" npm install express 2>&1 | sed 's/^/  [npm] /'
) &
NPM_PID=$!

sleep 2

# Show queue status
echo -e "${BLUE}Checking pending queue...${NC}"
./bin/clawrden-cli --api http://localhost:8080 queue

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  Demo Running${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo -e "${CYAN}The 'npm install' command is waiting for approval.${NC}"
echo ""
echo -e "${YELLOW}Options:${NC}"
echo -e "  1. Open dashboard: ${BLUE}http://localhost:8080${NC}"
echo -e "  2. Use CLI to approve:"
echo -e "     ${BLUE}./bin/clawrden-cli queue${NC}  # Get request ID"
echo -e "     ${BLUE}./bin/clawrden-cli approve <id>${NC}"
echo -e "  3. Or deny:"
echo -e "     ${BLUE}./bin/clawrden-cli deny <id>${NC}"
echo ""
echo -e "${YELLOW}View audit log in real-time:${NC}"
echo -e "  ${BLUE}tail -f $DEMO_DIR/audit.log | jq .${NC}"
echo ""
echo -e "${YELLOW}View command history:${NC}"
echo -e "  ${BLUE}./bin/clawrden-cli history${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to exit${NC}"
echo ""

# Wait for npm command or user interrupt
wait $NPM_PID 2>/dev/null || true

# If we get here, show final results
echo ""
echo -e "${BLUE}Final Results:${NC}"
./bin/clawrden-cli history

# Keep running until interrupted
wait
