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
