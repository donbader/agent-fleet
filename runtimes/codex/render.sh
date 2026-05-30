#!/bin/bash
# Render script for the Codex runtime provider.
# Input: render context available via RENDER_CONTEXT env var
# Output: Docker Compose service fragment (YAML) to stdout
#
# Uses `agent-fleet tools ctx` to extract values from the render context.
# No external dependencies (jq, python, etc.) required.
#
# Note: user-defined env vars (from agent.yaml "env:" section) are
# automatically merged by the CLI after this script runs.

set -e

NAME=$(agent-fleet tools ctx .name)
GATEWAY_HOST=$(agent-fleet tools ctx .gateway_host)
GATEWAY_PORT=$(agent-fleet tools ctx .gateway_port)
AUTH_PORT=$(agent-fleet tools ctx .options.auth_port --default 1455)
PERSIST_AUTH=$(agent-fleet tools ctx .options.persist_auth_token --default "true")
INIT_SCRIPTS=$(agent-fleet tools ctx .options.init_scripts --default "")
USER_BASE=$(agent-fleet tools ctx .options.user_base --default "")
AGENT_DIR=$(agent-fleet tools ctx .agent_dir)
PROVIDER_DIR="$(dirname "$0")"

# Determine build context and Dockerfile
BUILD_CONTEXT="$PROVIDER_DIR"
DOCKERFILE="Dockerfile"

# If init_scripts or user_base is set, generate a combined Dockerfile
# with agent dir as build context
if [ -n "$INIT_SCRIPTS" ] || [ -n "$USER_BASE" ]; then
    BUILD_CONTEXT="$AGENT_DIR"
    GENERATED="/tmp/Dockerfile.${NAME}"

    # Start with provider's base Dockerfile
    cp "$PROVIDER_DIR/Dockerfile" "$GENERATED"

    # Remove the final CMD/ENTRYPOINT lines (we'll re-add after injections)
    sed -i '/^ENTRYPOINT/d; /^CMD/d' "$GENERATED"

    # Inject user_base content if set
    if [ -n "$USER_BASE" ]; then
        USER_TEMPLATE="${AGENT_DIR}/${USER_BASE}"
        USER_CONTENT=$(agent-fleet tools template inject \
            --source "$USER_TEMPLATE" \
            --var "AGENT_HOME=/home/agent" \
            --var "AGENT_USER=agent")
        echo "" >> "$GENERATED"
        echo "# === User customization (from ${USER_BASE}) ===" >> "$GENERATED"
        echo "$USER_CONTENT" >> "$GENERATED"
    fi

    # Auto-COPY init_scripts
    if [ -n "$INIT_SCRIPTS" ]; then
        echo "" >> "$GENERATED"
        echo "# === Init scripts (auto-copied by provider) ===" >> "$GENERATED"
        echo "COPY $(echo "$INIT_SCRIPTS" | tr ',' ' ') /etc/agent-fleet/init.d/" >> "$GENERATED"
        echo "RUN chmod +x /etc/agent-fleet/init.d/*" >> "$GENERATED"
    fi

    # Re-add entrypoint (copy from provider dir)
    echo "" >> "$GENERATED"
    echo "COPY --from=provider /usr/local/bin/entrypoint.sh /usr/local/bin/entrypoint.sh" >> "$GENERATED"
    echo "ENTRYPOINT [\"/usr/local/bin/entrypoint.sh\"]" >> "$GENERATED"
    echo "CMD [\"sleep\", \"infinity\"]" >> "$GENERATED"

    # Add provider as a build stage so we can COPY entrypoint from it
    FINAL="/tmp/Dockerfile.${NAME}.final"
    echo "# Provider stage (for entrypoint)" > "$FINAL"
    echo "FROM node:22-slim AS provider" >> "$FINAL"
    echo "COPY --from=context ${PROVIDER_DIR}/entrypoint.sh /usr/local/bin/entrypoint.sh" >> "$FINAL"
    # Actually, simpler: just inline the entrypoint COPY differently
    # Let's use a different approach — copy entrypoint into agent dir temporarily

    # Simpler approach: copy provider's entrypoint to a temp location in build context
    cp "$PROVIDER_DIR/entrypoint.sh" "$AGENT_DIR/.entrypoint.sh"

    # Rewrite the generated Dockerfile to use local entrypoint
    sed -i "s|COPY --from=provider /usr/local/bin/entrypoint.sh /usr/local/bin/entrypoint.sh|COPY .entrypoint.sh /usr/local/bin/entrypoint.sh\nRUN chmod +x /usr/local/bin/entrypoint.sh|" "$GENERATED"

    # Clean up the failed multi-stage attempt
    rm -f "$FINAL"

    DOCKERFILE="$GENERATED"
fi

# Build volumes list (no home volume — ephemeral by default)
VOLUMES=""
if [ "$PERSIST_AUTH" = "true" ]; then
    VOLUMES="
volumes:
  - ${NAME}-codex-auth:/home/agent/.codex"
fi

# Build INIT_SCRIPTS env var — now points to /etc/agent-fleet/init.d/ scripts
INIT_SCRIPTS_ENV=""
if [ -n "$INIT_SCRIPTS" ]; then
    # Convert relative paths to /etc/agent-fleet/init.d/ basenames
    SCRIPT_NAMES=""
    IFS=',' read -r -a SCRIPTS <<< "$INIT_SCRIPTS"
    for s in "${SCRIPTS[@]}"; do
        base=$(basename "$s")
        if [ -n "$SCRIPT_NAMES" ]; then
            SCRIPT_NAMES="${SCRIPT_NAMES},/etc/agent-fleet/init.d/${base}"
        else
            SCRIPT_NAMES="/etc/agent-fleet/init.d/${base}"
        fi
    done
    INIT_SCRIPTS_ENV="
  INIT_SCRIPTS: \"${SCRIPT_NAMES}\""
fi

cat <<EOF
build:
  context: ${BUILD_CONTEXT}
  dockerfile: ${DOCKERFILE}
cap_add:
  - NET_ADMIN
sysctls:
  - net.ipv4.conf.all.route_localnet=1
ports:
  - "${AUTH_PORT}:${AUTH_PORT}"${VOLUMES}
environment:
  AGENT_NAME: "${NAME}"
  GATEWAY_HOST: "${GATEWAY_HOST}"
  GATEWAY_PORT: "${GATEWAY_PORT}"
  AUTH_PORT: "${AUTH_PORT}"${INIT_SCRIPTS_ENV}
restart: unless-stopped
EOF
