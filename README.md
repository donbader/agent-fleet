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

# Initialize a fleet
agent-fleet init my-fleet
cd my-fleet

# Configure (edit fleet.yaml + .env)
vim fleet.yaml

# Deploy
agent-fleet up
```

## Configuration

A fleet is defined by a single `fleet.yaml`:

```yaml
fleet:
  name: my-agent

agents:
  coder:
    runtime: codex
    gateway: gw-main
    channel:
      provider: "github.com/donbader/agent-fleet/channel-providers/telegram"
      options:
        bot_token_env: TELEGRAM_BOT_TOKEN
        allowed_users: ["@myusername"]
    env:
      GH_TOKEN: proxy_dummy_token

gateways:
  gw-main:
    egress:
      # GitHub with PAT injection
      - host: ["api.github.com", "github.com"]
        provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
        options:
          token_env: GITHUB_PAT_TOKEN

      # MCP services with OAuth (managed via /oauth command in chat)
      - endpoint: [https://mcp.notion.com/mcp]
        provider: "github.com/donbader/agent-fleet/egress-rules/mcp-oauth"

      # Docker API Proxy (exposes controlled Docker access to sandbox)
      - provider: "github.com/donbader/agent-fleet/egress-rules/docker-api-proxy"
        options:
          max_containers: 5
          disk_quota: "10Gi"
          resources:
            limits:
              memory: "2Gi"
              cpu: "2"

      # Allow all other traffic (no provider = passthrough)
      - host: ["*"]
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  agent-fleet CLI                                                 │
│  - Reads fleet.yaml                                             │
│  - Generates Docker Compose                                     │
│  - Wires channels and gateways                                  │
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
                   │ (shared gateway)
          ┌────────▼────────┐
          │  Gateway gw-main │
          │  (egress rules)  │
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
