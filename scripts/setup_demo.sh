set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color


DEMO_DIR="/tmp/clawrden-demo-$$"
SHIM_DIR="$DEMO_DIR/bin"

mkdir -p "$DEMO_DIR"
mkdir -p "$SHIM_DIR"

echo -e "${GREEN}✓ Demo directory: $DEMO_DIR${NC}"

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

export CLAWRDEN_SOCKET="/tmp/clawrden-demo/warden.sock"
echo -e "${GREEN}✓ Set Clawrden socket to :${CLAWRDEN_SOCKET}"

cd $DEMO_DIR

echo -e "${BLUE}Use following for executing shims: ${YELLOW}PATH=${SHIM_DIR}:\$PATH <command>"
