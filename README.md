# agent-fleet

Opinionated agent sandbox orchestrator. Deploy AI coding agents with enforced security boundaries, messaging bridges, and fleet management.

## What It Does

- **Sandbox isolation** — Every agent runs inside an [OpenShell](https://github.com/NVIDIA/OpenShell) sandbox with mandatory egress control
- **Bridge abstraction** — Connect agents to Telegram, Slack, or any messaging platform via ACP (Agent Client Protocol)
- **Fleet management** — Deploy and manage multiple agents from a single configuration
- **Credential injection** — Secrets never enter the sandbox; they're injected at the network boundary
- **Docker API Proxy** — Optionally allow agents to spin up containers in a controlled, policy-enforced way

## Quick Start

```bash
# Install
curl -sSL https://raw.githubusercontent.com/donbader/agent-fleet/main/install.sh | sh

# Initialize a fleet
agent-fleet init my-fleet
cd my-fleet

# Configure your agent (edit fleet.yaml)
vim fleet.yaml

# Deploy
agent-fleet up
```

## Configuration

A fleet is defined by a single `fleet.yaml`:

```yaml
fleet:
  name: my-agents

agents:
  coder:
    runtime: codex
    bridge: telegram
    sandbox:
      egress:
        - host: api.github.com
          port: 443
          access: full
        - host: api.openai.com
          port: 443
          access: full
      secrets:
        env_file: .env
      docker:
        enabled: true
        allowed_images: ["node:*", "python:*"]
        max_containers: 5

bridges:
  telegram:
    type: telegram
    token_env: TELEGRAM_BOT_TOKEN
    allowed_chats: [123456789]

secrets:
  providers:
    - name: openai
      env: OPENAI_API_KEY
    - name: github
      type: github-pat
      env: GITHUB_TOKEN
    - name: notion
      type: oauth
      provider: notion
      client_id_env: NOTION_CLIENT_ID
      client_secret_env: NOTION_CLIENT_SECRET
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  agent-fleet CLI                                                 │
│  - Reads fleet.yaml                                             │
│  - Provisions OpenShell sandboxes                               │
│  - Wires bridges, proxies, and credentials                      │
└──────────────────────────────┬──────────────────────────────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
   ┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐
   │  Agent: coder│     │ Agent: ops  │     │ Agent: ...  │
   │             │     │             │     │             │
   │ ┌─────────┐│     │ ┌─────────┐│     │             │
   │ │ Bridge  ││     │ │ Bridge  ││     │             │
   │ │(ACP↔TG) ││     │ │(ACP↔SK) ││     │             │
   │ └─────────┘│     │ └─────────┘│     │             │
   │             │     │             │     │             │
   │ OpenShell  │     │ OpenShell  │     │ OpenShell  │
   │ Sandbox    │     │ Sandbox    │     │ Sandbox    │
   └─────────────┘     └─────────────┘     └─────────────┘
          │                    │                    │
          └────────────────────┼────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │  Egress Proxy Layer  │
                    │  (credential inject) │
                    └─────────────────────┘
```

## Supported Agents

| Runtime | Protocol | Status |
|---------|----------|--------|
| Codex | ACP (native) | ✅ Primary |
| Claude Code | ACP (via adapter) | 🔜 Planned |
| Pi | Pi RPC (via adapter) | 🔜 Planned |

## Supported Bridges

| Platform | Status |
|----------|--------|
| Telegram | ✅ Primary |
| Slack | 🔜 Planned |
| Discord | 🔜 Planned |

## Documentation

- [Architecture](docs/architecture.md) — System design and component overview
- [Configuration](docs/configuration.md) — Full configuration reference
- [Security Model](docs/security-model.md) — Sandbox isolation and egress control
- [Bridge Protocol](docs/bridge-protocol.md) — ACP bridge and adapter design
- [Docker API Proxy](docs/docker-api-proxy.md) — Controlled container spawning

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
