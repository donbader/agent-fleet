#!/bin/bash
# Login to Codex inside the agent container.
#
# Usage:
#   ./login.sh
#
# This script handles the OAuth device flow for Codex CLI running
# inside a Docker container where localhost callbacks don't work.
#
# Flow:
#   1. Starts `codex auth` inside the container
#   2. Shows you the auth URL to open in your browser
#   3. After you authorize, your browser redirects to localhost (which fails)
#   4. You paste the failed callback URL here
#   5. Script curls it inside the container where the callback server is running

set -e

# Auto-detect the coder container
CONTAINER=$(docker ps --format '{{.Names}}' | grep -E 'coder' | head -1)

if [ -z "$CONTAINER" ]; then
    echo "Error: no running container with 'coder' in the name found."
    echo "Run 'agent-fleet up' first."
    exit 1
fi

echo "Found container: $CONTAINER"

echo "Starting Codex auth inside container..."
echo ""

# Run codex auth in background, capture output
docker exec -d "$CONTAINER" bash -c "codex auth > /tmp/codex-auth.log 2>&1"
sleep 2

# Show the auth URL
AUTH_URL=$(docker exec "$CONTAINER" cat /tmp/codex-auth.log 2>/dev/null | grep -oE 'https?://[^ ]+' | head -1)

if [ -z "$AUTH_URL" ]; then
    echo "Could not detect auth URL. Running interactively instead:"
    echo ""
    echo "  docker exec -it $CONTAINER codex auth"
    echo ""
    echo "After authorizing, copy the failed callback URL and run:"
    echo "  docker exec $CONTAINER curl -s '<callback-url>'"
    exit 1
fi

echo "Open this URL in your browser:"
echo ""
echo "  $AUTH_URL"
echo ""
echo "After authorizing, your browser will try to redirect to"
echo "http://localhost:... which will fail. That's expected."
echo ""
read -p "Paste the full callback URL from your browser's address bar: " CALLBACK_URL

if [ -z "$CALLBACK_URL" ]; then
    echo "No URL provided. Aborting."
    exit 1
fi

echo ""
echo "Completing auth..."
docker exec "$CONTAINER" curl -s "$CALLBACK_URL" > /dev/null 2>&1

sleep 1
echo "✓ Done! Codex should now be authenticated."
echo ""
echo "Test with: docker exec -it $CONTAINER codex --help"
