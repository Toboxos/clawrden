#!/bin/bash
# install-clawrden.sh
# Installs the Clawrden shim binary and configures PATH interception.
# Usage: install-clawrden.sh --user UID [--lock-binaries "npm,docker,kubectl"]
#
# This script is designed to run inside a Dockerfile's RUN directive.
# It works on Alpine (musl), Ubuntu/Debian (glibc), and minimal images.

set -euo pipefail

# ─── Defaults ────────────────────────────────────────────────────────────────
CLAWRDEN_BIN_DIR="/clawrden/bin"
CLAWRDEN_SOCKET_DIR="/var/run/clawrden"
SHIM_SOURCE="/tmp/clawrden-shim"
TARGET_USER=""
LOCK_BINARIES=""

# ─── Parse Arguments ─────────────────────────────────────────────────────────
usage() {
    echo "Usage: $0 --user UID [--lock-binaries \"tool1,tool2,...\"]"
    echo ""
    echo "Options:"
    echo "  --user UID              UID for the agent user (required)"
    echo "  --lock-binaries LIST    Comma-separated list of binaries to intercept"
    echo "  --shim-source PATH      Path to the compiled shim binary (default: /tmp/clawrden-shim)"
    echo "  --help                  Show this help message"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --user)
            TARGET_USER="$2"
            shift 2
            ;;
        --lock-binaries)
            LOCK_BINARIES="$2"
            shift 2
            ;;
        --shim-source)
            SHIM_SOURCE="$2"
            shift 2
            ;;
        --help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

if [[ -z "$TARGET_USER" ]]; then
    echo "Error: --user is required"
    usage
fi

echo "🛡️  Installing Clawrden..."
echo "   Target user UID: $TARGET_USER"
echo "   Lock binaries: ${LOCK_BINARIES:-none}"

# ─── Step 1: Create Directory Structure ──────────────────────────────────────
echo "📁 Creating directory structure..."
mkdir -p "$CLAWRDEN_BIN_DIR"
mkdir -p "$CLAWRDEN_SOCKET_DIR"

# Set permissions
chmod 755 "$CLAWRDEN_BIN_DIR"
chmod 777 "$CLAWRDEN_SOCKET_DIR"

# ─── Step 2: Install the Shim Binary ─────────────────────────────────────────
echo "📦 Installing shim binary..."
if [[ ! -f "$SHIM_SOURCE" ]]; then
    echo "Error: Shim binary not found at $SHIM_SOURCE"
    echo "Build it first: CGO_ENABLED=0 go build -ldflags='-s -w' -o $SHIM_SOURCE ./cmd/shim"
    exit 1
fi

cp "$SHIM_SOURCE" "$CLAWRDEN_BIN_DIR/clawrden-shim"
chmod 755 "$CLAWRDEN_BIN_DIR/clawrden-shim"

# ─── Step 3: Create Symlinks for Locked Binaries ─────────────────────────────
if [[ -n "$LOCK_BINARIES" ]]; then
    echo "🔗 Creating tool symlinks..."
    IFS=',' read -ra TOOLS <<< "$LOCK_BINARIES"
    for tool in "${TOOLS[@]}"; do
        tool=$(echo "$tool" | xargs) # Trim whitespace

        # Create symlink: /clawrden/bin/<tool> -> clawrden-shim
        ln -sf "$CLAWRDEN_BIN_DIR/clawrden-shim" "$CLAWRDEN_BIN_DIR/$tool"
        echo "   ✓ $tool -> clawrden-shim"

        # Lock the original binary if it exists
        for search_dir in /usr/local/bin /usr/bin /bin /usr/local/sbin /usr/sbin /sbin; do
            original="$search_dir/$tool"
            if [[ -f "$original" && ! -f "${original}.original" ]]; then
                echo "   🔒 Locking $original -> ${original}.original"
                mv "$original" "${original}.original"
            fi
        done
    done
fi

# ─── Step 4: Configure PATH Precedence ───────────────────────────────────────
echo "🔧 Configuring PATH precedence..."

PATH_LINE="export PATH=\"$CLAWRDEN_BIN_DIR:\$PATH\""

# Add to /etc/profile (system-wide for login shells)
if [[ -f /etc/profile ]]; then
    if ! grep -q "clawrden" /etc/profile 2>/dev/null; then
        echo "" >> /etc/profile
        echo "# Clawrden PATH interception" >> /etc/profile
        echo "$PATH_LINE" >> /etc/profile
    fi
fi

# Add to /etc/bash.bashrc or /etc/bashrc (system-wide for interactive shells)
for bashrc in /etc/bash.bashrc /etc/bashrc; do
    if [[ -f "$bashrc" ]]; then
        if ! grep -q "clawrden" "$bashrc" 2>/dev/null; then
            echo "" >> "$bashrc"
            echo "# Clawrden PATH interception" >> "$bashrc"
            echo "$PATH_LINE" >> "$bashrc"
        fi
    fi
done

# Add environment file for non-interactive shells
mkdir -p /etc/profile.d
cat > /etc/profile.d/clawrden.sh << EOF
# Clawrden PATH interception
$PATH_LINE
EOF
chmod 644 /etc/profile.d/clawrden.sh

# ─── Step 5: Set ownership ──────────────────────────────────────────────────
echo "👤 Setting ownership..."
chown -R "$TARGET_USER" "$CLAWRDEN_BIN_DIR" 2>/dev/null || true
chown -R "$TARGET_USER" "$CLAWRDEN_SOCKET_DIR" 2>/dev/null || true

# ─── Done ────────────────────────────────────────────────────────────────────
echo ""
echo "✅ Clawrden installed successfully!"
echo "   Shim binary: $CLAWRDEN_BIN_DIR/clawrden-shim"
echo "   Socket dir:  $CLAWRDEN_SOCKET_DIR"
echo "   PATH will be: $CLAWRDEN_BIN_DIR:\$PATH"
if [[ -n "$LOCK_BINARIES" ]]; then
    echo "   Locked tools: $LOCK_BINARIES"
fi
