#!/bin/bash
# Render script for the Codex runtime provider.
# Input: render context available via RENDER_CONTEXT env var
# Output: Docker Compose service fragment (YAML) to stdout
#
# Uses `agent-fleet ctx` to extract values from the render context.
# No external dependencies (jq, python, etc.) required.
#
# Note: user-defined env vars (from agent.yaml "env:" section) are
# automatically merged by the CLI after this script runs.

set -e

NAME=$(agent-fleet ctx .name)
GATEWAY_HOST=$(agent-fleet ctx .gateway_host)
GATEWAY_PORT=$(agent-fleet ctx .gateway_port)
AUTH_PORT=$(agent-fleet ctx .options.auth_port --default 1455)

cat <<EOF
build:
  context: .
  dockerfile: Dockerfile
cap_add:
  - NET_ADMIN
sysctls:
  - net.ipv4.conf.all.route_localnet=1
ports:
  - "${AUTH_PORT}:${AUTH_PORT}"
volumes:
  - ${NAME}-home:/home/agent
  - ${NAME}-codex-auth:/home/agent/.codex
environment:
  AGENT_NAME: "${NAME}"
  GATEWAY_HOST: "${GATEWAY_HOST}"
  GATEWAY_PORT: "${GATEWAY_PORT}"
  AUTH_PORT: "${AUTH_PORT}"
restart: unless-stopped
EOF
