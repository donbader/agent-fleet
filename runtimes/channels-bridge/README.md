# channels-bridge Runtime

Connects messaging channels (Telegram) to an agent process via ACP protocol.

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `agent_provider` | string | `"codex"` | Which agent CLI to spawn |
| `user_base` | string | `""` | Path to partial Dockerfile template (relative to agent dir) |
| `channels` | array | `[]` | Channel configurations |

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

## Channels

```yaml
channels:
  - provider: "telegram"
    options:
      allowed_users: ["username1", "username2"]
```

## Environment Variables

The following env vars are set automatically:
- `AGENT_NAME` — agent name from fleet config
- `GATEWAY_HOST` / `GATEWAY_PORT` — proxy gateway address
- `AGENT_CMD` — agent CLI command (derived from agent_provider)
- `TELEGRAM_BOT_TOKEN` — injected by gateway proxy (dummy value in compose)
- `TELEGRAM_ALLOWED_USERS` — comma-separated allowed usernames
