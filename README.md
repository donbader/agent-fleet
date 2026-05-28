# agent-fleet

Opinionated agent sandbox orchestrator. Deploy AI coding agents with enforced security boundaries, messaging channels, and fleet management.

## What It Does

- **Sandbox isolation** — Every agent runs inside a Docker container with transparent egress proxy and default-deny rules
- **Channel abstraction** — Connect agents to Telegram (or other platforms) via ACP (Agent Client Protocol)
- **Fleet management** — Deploy and manage multiple agents from a single configuration
- **Credential injection** — Secrets never enter the sandbox; egress rule providers inject them at the network boundary
- **Docker API Proxy** — Optionally allow agents to spin up containers via a policy-enforced egress rule

## Quick Start

```bash
# Install
curl -sSL https://raw.githubusercontent.com/donbader/agent-fleet/main/install.sh | sh

# Initialize a fleet (scaffolds fleet.yaml + agents/ folder)
agent-fleet init my-fleet
cd my-fleet

# Configure
# 1. Edit fleet.yaml — set egress-presets
# 2. Edit agents/coder/agent.yaml — set runtime, channel, egress refs
# 3. Create .env with your secrets

# Deploy
agent-fleet up
```

## Configuration

A fleet is a directory with `fleet.yaml` + per-agent folders:

```
my-fleet/
  fleet.yaml              # shared egress-presets + agent list
  .env                    # secrets (never committed)
  agents/
    coder/
      agent.yaml          # runtime, egress refs, channel
```

**fleet.yaml** — shared config:

```yaml
fleet:
  name: my-agent

agents:
  - coder

egress-presets:
  telegram-bot-1:
    - host: ["api.telegram.org"]
      provider: "github.com/donbader/agent-fleet/egress-rules/telegram-bot"
      options:
        token: "${TELEGRAM_BOT_TOKEN}"

  main:
    - host: ["api.github.com", "github.com"]
      provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
      options:
        token: "${GITHUB_PAT_TOKEN}"
    - host: ["*"]
```

**agents/coder/agent.yaml** — per-agent config:

```yaml
egress: [telegram-bot-1, main]

runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/channels-bridge"
  options:
    agent_provider: "github.com/donbader/agent-fleet/runtimes/codex"
    channels:
      - provider: "github.com/donbader/agent-fleet/channel-providers/telegram"
        options:
          allowed_users: ["@myusername"]

env:
  GH_TOKEN: proxy_dummy_token
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  agent-fleet CLI                                                 │
│  - Reads fleet.yaml                                             │
│  - Generates Docker Compose                                     │
│  - Wires channels and egress presets                             │
└──────────────────────────────┬──────────────────────────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
   ┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐
   │ Agent: coder│     │ Agent: ops  │     │ Agent: ...  │
   │             │     │             │     │             │
   │ ┌─────────┐│     │ ┌─────────┐│     │             │
   │ │ Channel ││     │ │ Channel ││     │             │
   │ │(ACP↔TG) ││     │ │(ACP↔TG) ││     │             │
   │ └─────────┘│     │ └─────────┘│     │             │
   │             │     │             │     │             │
   │ Docker     │     │ Docker     │     │ Docker     │
   │ + Proxy    │     │ + Proxy    │     │ + Proxy    │
   └──────┬──────┘     └──────┬──────┘     └─────────────┘
          │                    │
          └────────┬───────────┘
                   │ (shared egress presets)
          ┌────────▼────────┐
          │  Egress Proxy    │
          │  (transparent)   │
          └─────────────────┘
```

## Supported Agents

| Runtime | Protocol | Status |
|---------|----------|--------|
| Codex | ACP (native) | ✅ Primary |
| Claude Code | ACP (via adapter) | 🔜 Planned |
| Pi | Pi RPC (via adapter) | 🔜 Planned |

## Supported Channels

| Platform | Status |
|----------|--------|
| Telegram | ✅ Primary |
| Slack | 🔜 Planned |
| Discord | 🔜 Planned |

## Documentation

- [Architecture](docs/architecture.md) — System design and component overview
- [Configuration](docs/configuration.md) — Full configuration reference
- [Security Model](docs/security-model.md) — Sandbox isolation and egress control
- [Bridge Protocol](docs/bridge-protocol.md) — ACP protocol and adapter design
- [Docker API Proxy](docs/docker-api-proxy.md) — Controlled container spawning
- [Roadmap](docs/roadmap.md) — Phase plan

## Development

```bash
# Build
go build ./cmd/agent-fleet

# Test
go test ./...

# Lint
golangci-lint run
```

## License

MIT
