#!/bin/bash
# Demo script for Clawrden
# Starts the warden server and opens the web dashboard

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🛡️  Clawrden Demo${NC}"
echo ""

# Check if binaries exist
if [ ! -f "./bin/clawrden-warden" ]; then
    echo -e "${YELLOW}Building binaries...${NC}"
    make build
fi

# Create temp directories
DEMO_DIR="/tmp/clawrden-demo"
mkdir -p "$DEMO_DIR"

echo -e "${GREEN}✓ Demo directory: $DEMO_DIR${NC}"

# Start warden
echo -e "${BLUE}Starting Warden...${NC}"
./bin/clawrden-warden \
    --socket "$DEMO_DIR/warden.sock" \
    --policy policy.yaml \
    --audit "$DEMO_DIR/audit.log" \
    --api :8080 &

WARDEN_PID=$!

echo -e "${GREEN}✓ Warden started (PID: $WARDEN_PID)${NC}"
echo ""

# Wait for warden to be ready
echo -e "${BLUE}Waiting for warden to be ready...${NC}"
sleep 2

# Check status
if curl -s http://localhost:8080/api/status > /dev/null; then
    echo -e "${GREEN}✓ Warden is ready!${NC}"
else
    echo -e "${YELLOW}Warning: Warden may not be fully ready${NC}"
fi

echo ""
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}  Clawrden is running!${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo -e "  📊 Web Dashboard: ${BLUE}http://localhost:8080${NC}"
echo -e "  📝 Audit Log:     ${BLUE}$DEMO_DIR/audit.log${NC}"
echo -e "  🔌 Socket:        ${BLUE}$DEMO_DIR/warden.sock${NC}"
echo ""
echo -e "${YELLOW}Usage:${NC}"
echo -e "  View status:   ${BLUE}./bin/clawrden-cli status${NC}"
echo -e "  View queue:    ${BLUE}./bin/clawrden-cli queue${NC}"
echo -e "  View history:  ${BLUE}./bin/clawrden-cli history${NC}"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
echo ""

# Cleanup on exit
cleanup() {
    echo ""
    echo -e "${YELLOW}Shutting down...${NC}"
    kill $WARDEN_PID 2>/dev/null || true
    wait $WARDEN_PID 2>/dev/null || true
    echo -e "${GREEN}✓ Warden stopped${NC}"
    echo -e "${BLUE}Audit log saved to: $DEMO_DIR/audit.log${NC}"
}

trap cleanup EXIT INT TERM

# Keep running
wait $WARDEN_PID
