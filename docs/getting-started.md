# Getting Started

This guide walks you through setting up and running your first agent fleet.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/donbader/agent-fleet/main/install.sh | sh
```

For private repo access (requires `gh` CLI or `GITHUB_TOKEN`):
```bash
curl -sSL https://raw.githubusercontent.com/donbader/agent-fleet/main/install.sh | GITHUB_TOKEN=$GITHUB_TOKEN sh
```

## Initialize a Fleet

```bash
mkdir my-fleet && cd my-fleet
agent-fleet init
```

This creates:
```
my-fleet/
  fleet.yaml
  agents/
    coder/
      agent.yaml
  .env.example
  .gitignore
```

## Configure

### fleet.yaml

Defines shared egress presets and lists your agents:

```yaml
fleet:
  name: my-fleet

agents:
  - coder

egress-presets:
  default:
    - host: ["api.openai.com"]
    - host: ["github.com", "*.github.com"]
      provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
      options:
        token: "${GITHUB_PAT}"
    - host: ["*"]
```

### agents/coder/agent.yaml

Per-agent configuration — runtime, egress, channels:

```yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
  options:
    auth_port: 1455

egress:
  - default
```

### .env

Copy `.env.example` to `.env` and fill in your secrets:

```bash
cp .env.example .env
# Edit .env with your actual values
```

## Generate & Deploy

```bash
# Generate deployment artifacts (docker-compose.yml, gateway rules, etc.)
agent-fleet generate

# Build images and start containers
agent-fleet compose up -d --build
```

## Daily Workflow

### Run codex

```bash
agent-fleet compose exec coder codex "fix the failing test"
```

First time, codex will prompt you to login via device flow. Auth is persisted across restarts.

### View logs

```bash
agent-fleet compose logs coder -f
```

### Shell into container

```bash
agent-fleet compose exec coder bash
```

### Restart an agent

```bash
agent-fleet compose restart coder
```

### Stop the fleet

```bash
agent-fleet compose down
```

## Update Configuration

After editing fleet.yaml or agent.yaml, regenerate and restart:

```bash
agent-fleet generate
agent-fleet compose up -d --build
```

## Explore Files with VSCode

1. Install the [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension
2. Command Palette → **Dev Containers: Attach to Running Container...**
3. Select your agent container
4. Browse files, edit, use terminal inside the container

## Upgrade

```bash
agent-fleet upgrade
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `agent-fleet init` | Scaffold a new fleet directory |
| `agent-fleet generate` | Generate docker-compose.yml, gateway rules, .env.example, .gitignore |
| `agent-fleet validate` | Validate config without generating |
| `agent-fleet compose [args...]` | Passthrough to docker compose with fleet context |
| `agent-fleet tools ctx <path>` | JSON path extractor (for render scripts) |
| `agent-fleet tools template inject` | Variable substitution (for render scripts) |
| `agent-fleet upgrade` | Self-update to latest release |

## Next Steps

- [Configuration Reference](configuration.md) — full fleet.yaml and agent.yaml options
- [Home Directory Strategies](user-base.md) — persist and customize agent home
- [Architecture](architecture.md) — how the proxy, gateway, and bridge work
- [Security Model](security-model.md) — egress rules, MITM, credential injection
