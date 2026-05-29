#!/bin/bash
# Render script for the channels-bridge runtime provider.
# Uses `agent-fleet ctx` to extract values from the render context.
# No external dependencies required.

set -e

NAME=$(agent-fleet ctx .name)
GATEWAY_HOST=$(agent-fleet ctx .gateway_host)
GATEWAY_PORT=$(agent-fleet ctx .gateway_port)
AGENT_CMD=$(agent-fleet ctx .options.agent_cmd --default "codex")
ALLOWED_USERS=$(agent-fleet ctx .options.allowed_users --default "")

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
