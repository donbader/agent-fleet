#!/bin/bash
# Render script for the Codex runtime provider.
# Input: JSON context via stdin
# Output: Docker Compose service fragment (YAML) to stdout
#
# Context fields:
#   name         - agent name
#   fleet_name   - fleet name
#   options      - runtime options from agent.yaml
#   env          - user-defined environment variables
#   gateway_host - gateway service name
#   gateway_port - gateway port

set -e

# Read context from stdin
CTX=$(cat)

NAME=$(echo "$CTX" | jq -r '.name')
GATEWAY_HOST=$(echo "$CTX" | jq -r '.gateway_host')
GATEWAY_PORT=$(echo "$CTX" | jq -r '.gateway_port')
AUTH_PORT=$(echo "$CTX" | jq -r '.options.auth_port // "1455"')

# Build environment from context
ENV_VARS="  AGENT_NAME: \"$NAME\""
ENV_VARS="$ENV_VARS
  GATEWAY_HOST: \"$GATEWAY_HOST\""
ENV_VARS="$ENV_VARS
  GATEWAY_PORT: \"$GATEWAY_PORT\""

# Add user-defined env vars
USER_ENVS=$(echo "$CTX" | jq -r '.env // {} | to_entries[] | "  \(.key): \"\(.value)\""')
if [ -n "$USER_ENVS" ]; then
    ENV_VARS="$ENV_VARS
$USER_ENVS"
fi

# Output compose service fragment
cat <<EOF
build:
  context: .
  dockerfile: Dockerfile
cap_add:
  - NET_ADMIN
ports:
  - "${AUTH_PORT}:${AUTH_PORT}"
volumes:
  - ${NAME}-codex-auth:/home/agent/.codex
environment:
$ENV_VARS
restart: unless-stopped
EOF
