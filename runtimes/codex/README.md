# codex Runtime

Runs OpenAI Codex CLI agent inside a sandboxed container with transparent egress proxy.

## Daily Workflow

Once the fleet is up (`agent-fleet generate && agent-fleet compose up -d --build`), working with the codex agent is the same as running codex locally — just sandboxed.

`agent-fleet compose` passes through to `docker compose` with the correct project name and compose file, so you never need to remember container names.

### First-time login

Codex requires a one-time device flow login. Exec into the container and run codex:

```bash
agent-fleet compose exec coder codex
```

The codex TUI will prompt you to visit a URL and enter a code. After login, the auth token is stored in `/home/agent/.codex` (persisted via a named volume — survives container restarts and even home directory resets).

### Daily use

After login, just exec in and use codex normally:

```bash
agent-fleet compose exec coder codex "fix the failing test in src/utils.ts"
```

Or start an interactive session:

```bash
agent-fleet compose exec coder codex
```

It's the same codex CLI experience, but all network traffic goes through the transparent proxy (egress rules enforced, credentials injected automatically).

Other useful commands:

```bash
agent-fleet compose logs coder -f     # tail logs
agent-fleet compose restart coder     # restart the agent
agent-fleet compose exec coder bash   # shell into container
```

### Exploring files with VSCode

Attach VSCode to the running container for full IDE access:

1. Install the [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension
2. Open Command Palette → **Dev Containers: Attach to Running Container...**
3. Select the agent container (e.g. `myfleet-coder-1`)
4. VSCode opens a new window inside the container — browse files, edit, use terminal

Alternatively, use the Docker extension's right-click → **Attach Visual Studio Code** on the container.

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
