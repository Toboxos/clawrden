#!/bin/bash
set -e

echo "🛡️  Clawrden Demo Script"
echo "======================="
echo ""

# Check prerequisites
if ! command -v docker &> /dev/null; then
    echo "❌ Error: docker is not installed"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ Error: docker-compose is not installed"
    exit 1
fi

# Check if binaries exist
if [ ! -f ./bin/clawrden-warden ]; then
    echo "❌ Error: clawrden-warden binary not found. Run 'make build' first."
    exit 1
fi

if [ ! -f ./bin/clawrden-shim ]; then
    echo "❌ Error: clawrden-shim binary not found. Run 'make build' first."
    exit 1
fi

# Check if hardened images exist
if ! docker image inspect clawrden-ubuntu &> /dev/null; then
    echo "📦 Building hardened Ubuntu image..."
    ./scripts/harden-container.sh --base-image ubuntu:22.04 --output-image clawrden-ubuntu
fi

echo "✅ Prerequisites checked"
echo ""

# Create logs directory
mkdir -p logs

# Start the warden and prisoner
echo "🚀 Starting Clawrden containers..."
docker-compose down -v 2>/dev/null || true
docker-compose up -d warden prisoner1

echo ""
echo "⏳ Waiting for warden to be healthy..."
sleep 5

# Check if containers are running
if ! docker-compose ps | grep -q "clawrden-warden.*Up"; then
    echo "❌ Warden container failed to start"
    docker-compose logs warden
    exit 1
fi

if ! docker-compose ps | grep -q "clawrden-prisoner1"; then
    echo "❌ Prisoner container failed to start"
    docker-compose logs prisoner1
    exit 1
fi

echo "✅ Containers started successfully"
echo ""

# Display status
echo "📊 Container Status:"
docker-compose ps
echo ""

# Show warden logs
echo "📝 Warden Logs (last 10 lines):"
docker-compose logs --tail=10 warden
echo ""

# Instructions
echo "🎯 Demo Instructions:"
echo ""
echo "1. Open Web Dashboard:"
echo "   http://localhost:8080"
echo ""
echo "2. Access prisoner container:"
echo "   docker exec -it clawrden-prisoner1 bash"
echo ""
echo "3. Inside prisoner, try commands:"
echo "   ls /app                    # Allowed - executes immediately"
echo "   echo 'Hello' > /app/test   # Requires approval"
echo "   cat /app/test              # Allowed - executes immediately"
echo ""
echo "4. Use CLI to manage approvals:"
echo "   ./bin/clawrden-cli status"
echo "   ./bin/clawrden-cli queue"
echo "   ./bin/clawrden-cli approve <request-id>"
echo "   ./bin/clawrden-cli history"
echo ""
echo "5. View audit log:"
echo "   tail -f logs/audit.log | jq ."
echo ""
echo "6. Stop demo:"
echo "   docker-compose down -v"
echo ""
echo "✨ Demo ready! Happy testing!"
