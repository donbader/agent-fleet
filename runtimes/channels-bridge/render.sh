#!/bin/bash
# Render script for the channels-bridge runtime provider.
# Uses `agent-fleet ctx` to extract values from the render context.
# No external dependencies required.
#
# Supported options:
#   agent_provider       - runtime provider for the agent process (default: codex)
#   user_base_image_stage - path to user's Dockerfile for extra tools
#   channels             - array of channel configs (provider + options)

set -e

NAME=$(agent-fleet ctx .name)
GATEWAY_HOST=$(agent-fleet ctx .gateway_host)
GATEWAY_PORT=$(agent-fleet ctx .gateway_port)

# Agent command derived from agent_provider
AGENT_PROVIDER=$(agent-fleet ctx .options.agent_provider --default "codex")
AGENT_CMD=$(basename "$AGENT_PROVIDER")

# User base image stage (custom Dockerfile for extra tools)
USER_BASE=$(agent-fleet ctx .options.user_base_image_stage --default "")

# Extract telegram channel config using array indexing
ALLOWED_USERS=$(agent-fleet ctx .options.channels.0.options.allowed_users --default "[]")
if [ "$ALLOWED_USERS" != "[]" ] && [ -n "$ALLOWED_USERS" ]; then
    ALLOWED_USERS=$(echo "$ALLOWED_USERS" | tr -d '[]"' | tr ',' ',')
fi

# Build section — include additional_contexts for user base image
if [ -n "$USER_BASE" ]; then
    # user_base_image_stage points to a Dockerfile in the agent's directory
    # Docker Compose additional_contexts lets our Dockerfile reference it
    cat <<EOF
build:
  context: .
  dockerfile: Dockerfile
  additional_contexts:
    user-base: ../../agents/${NAME}
EOF
else
    cat <<EOF
build:
  context: .
  dockerfile: Dockerfile
  additional_contexts:
    user-base: docker-image://node:22-slim
EOF
fi

cat <<EOF
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
