#!/bin/bash
# Render script for the channels-bridge runtime provider.
# Uses `agent-fleet ctx` to extract values from the render context.
# No external dependencies required.
#
# Supported options:
#   agent_provider  - runtime provider for the agent process (default: codex)
#   channels        - array of channel configs (provider + options)
#   channels[0].options.allowed_users - Telegram user filter

set -e

NAME=$(agent-fleet ctx .name)
GATEWAY_HOST=$(agent-fleet ctx .gateway_host)
GATEWAY_PORT=$(agent-fleet ctx .gateway_port)

# Agent command derived from agent_provider
AGENT_PROVIDER=$(agent-fleet ctx .options.agent_provider --default "codex")
# Extract just the last path segment as the command name
AGENT_CMD=$(basename "$AGENT_PROVIDER")

# Extract telegram channel config using array indexing
ALLOWED_USERS=$(agent-fleet ctx .options.channels.0.options.allowed_users --default "[]")
# Convert JSON array ["@user1","@user2"] to comma-separated string
if [ "$ALLOWED_USERS" != "[]" ] && [ -n "$ALLOWED_USERS" ]; then
    # Strip brackets and quotes, join with commas
    ALLOWED_USERS=$(echo "$ALLOWED_USERS" | tr -d '[]"' | tr ',' ',')
fi

cat <<EOF
build:
  context: .
  dockerfile: Dockerfile
cap_add:
  - NET_ADMIN
environment:
  AGENT_NAME: "${NAME}"
  GATEWAY_HOST: "${GATEWAY_HOST}"
  GATEWAY_PORT: "${GATEWAY_PORT}"
  AGENT_CMD: "${AGENT_CMD}"
  TELEGRAM_BOT_TOKEN: "000000000:DUMMY"
  TELEGRAM_ALLOWED_USERS: "${ALLOWED_USERS}"
restart: unless-stopped
EOF
