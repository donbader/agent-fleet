#!/bin/bash
# Login to Codex inside the agent container.
# Port 1455 is exposed by the runtime's render script, so the OAuth
# callback from your browser will reach the container directly.
#
# Usage: ./login.sh

set -e

CONTAINER=$(docker ps --format '{{.Names}}' | grep -E 'coder' | head -1)

if [ -z "$CONTAINER" ]; then
    echo "Error: no running container with 'coder' in the name found."
    echo "Run 'agent-fleet up' first."
    exit 1
fi

echo "Found container: $CONTAINER"
echo "Starting Codex login (port 1455 is exposed for OAuth callback)..."
echo ""

docker exec -it "$CONTAINER" codex
