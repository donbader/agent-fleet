#!/bin/bash
# Render script for the channels-bridge runtime provider.
# Uses `agent-fleet tools ctx` to extract values from the render context.
# No external dependencies required.
#
# Supported options:
#   agent_provider       - runtime provider for the agent process (default: codex)
#   user_base            - path to user's partial Dockerfile (template injection)
#   persist_auth_token   - persist agent auth token across restarts (default: true)
#   channels             - array of channel configs (provider + options)

set -e

NAME=$(agent-fleet tools ctx .name)
GATEWAY_HOST=$(agent-fleet tools ctx .gateway_host)
GATEWAY_PORT=$(agent-fleet tools ctx .gateway_port)

# Agent command derived from agent_provider
AGENT_PROVIDER=$(agent-fleet tools ctx .options.agent_provider --default "codex")
AGENT_CMD=$(basename "$AGENT_PROVIDER")

# User base template (optional)
USER_BASE=$(agent-fleet tools ctx .options.user_base --default "")

# Persist auth token (default: true)
PERSIST_AUTH=$(agent-fleet tools ctx .options.persist_auth_token --default "true")

# Extract telegram channel config using array indexing
ALLOWED_USERS=$(agent-fleet tools ctx .options.channels.0.options.allowed_users --default "[]")
if [ "$ALLOWED_USERS" != "[]" ] && [ -n "$ALLOWED_USERS" ]; then
    ALLOWED_USERS=$(echo "$ALLOWED_USERS" | tr -d '[]"' | tr ',' ',')
fi

# Generate Dockerfile with optional user_base injection
DOCKERFILE="Dockerfile"
if [ -n "$USER_BASE" ]; then
    AGENT_DIR=$(agent-fleet tools ctx .agent_dir)
    USER_TEMPLATE="${AGENT_DIR}/${USER_BASE}"

    # Process user template with magic variables
    USER_CONTENT=$(agent-fleet tools template inject \
        --source "$USER_TEMPLATE" \
        --var "AGENT_HOME=/home/agent" \
        --var "AGENT_USER=agent")

    # Generate combined Dockerfile: runtime base + user content + finalize
    GENERATED="/tmp/Dockerfile.${NAME}"
    sed '/^# === USER_BASE ===/,$d' "$(dirname "$0")/Dockerfile" > "$GENERATED"
    echo "# === User customization (from ${USER_BASE}) ===" >> "$GENERATED"
    echo "$USER_CONTENT" >> "$GENERATED"
    echo "" >> "$GENERATED"
    sed -n '/^# === FINALIZE ===/,$p' "$(dirname "$0")/Dockerfile" >> "$GENERATED"

    DOCKERFILE="$GENERATED"
fi

# Build volumes section
VOLUMES=""
if [ "$PERSIST_AUTH" = "true" ]; then
    VOLUMES="
volumes:
  - ${NAME}-codex-auth:/home/agent/.codex"
fi

cat <<EOF
build:
  context: .
  dockerfile: ${DOCKERFILE}
cap_add:
  - NET_ADMIN${VOLUMES}
environment:
  AGENT_NAME: "${NAME}"
  GATEWAY_HOST: "${GATEWAY_HOST}"
  GATEWAY_PORT: "${GATEWAY_PORT}"
  AGENT_CMD: "${AGENT_CMD}"
  TELEGRAM_BOT_TOKEN: "000000000:DUMMY"
  TELEGRAM_ALLOWED_USERS: "${ALLOWED_USERS}"
restart: unless-stopped
EOF
