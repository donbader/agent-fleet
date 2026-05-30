# codex Runtime

Runs OpenAI Codex CLI agent inside a sandboxed container with transparent egress proxy.

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `auth_port` | number | `1455` | Host port for codex auth device flow |
| `user_base` | string | `""` | Path to partial Dockerfile template (relative to agent dir) |

## user_base

When set, the provider injects your partial Dockerfile into the runtime's build. Your template can use these magic variables:

| Variable | Value |
|----------|-------|
| `${AGENT_HOME}` | `/home/agent` |
| `${AGENT_USER}` | `agent` |

Example:
```dockerfile
# agents/coder/Dockerfile
RUN apt-get update && apt-get install -y ripgrep fd-find jq
COPY --chown=${AGENT_USER}:${AGENT_USER} home-override/ ${AGENT_HOME}/
```

## Volumes

The runtime outputs two named volumes:
- `{name}-home:/home/agent` — persistent home directory
- `{name}-codex-auth:/home/agent/.codex` — codex auth token (persists across home resets)

## Environment Variables

Set automatically:
- `AGENT_NAME` — agent name from fleet config
- `GATEWAY_HOST` / `GATEWAY_PORT` — proxy gateway address
- `AUTH_PORT` — port for codex device flow auth
