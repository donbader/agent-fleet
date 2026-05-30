# agent-fleet

Opinionated agent sandbox orchestrator. Deploy AI coding agents with enforced security boundaries, messaging channels, and fleet management.

## Features

- **Sandbox isolation** — Docker containers with transparent egress proxy and default-deny rules
- **Credential injection** — Secrets never enter the sandbox; injected at the network boundary via MITM
- **Channel abstraction** — Connect agents to Telegram (or other platforms) via ACP protocol
- **Fleet management** — Multiple agents from a single configuration
- **Composable egress** — Mix and match egress presets per agent

## Install

```bash
curl -sSL https://raw.githubusercontent.com/donbader/agent-fleet/main/install.sh | sh
```

## Quick Start

```bash
agent-fleet init my-fleet && cd my-fleet
# Edit fleet.yaml + agents/coder/agent.yaml + .env
agent-fleet generate
agent-fleet compose up -d --build
```

See [Getting Started](docs/getting-started.md) for the full walkthrough.

## Documentation

| Doc | Description |
|-----|-------------|
| [Getting Started](docs/getting-started.md) | Full setup and daily workflow guide |
| [Configuration](docs/configuration.md) | fleet.yaml and agent.yaml reference |
| [Customization](docs/customization.md) | Home directory strategies and Dockerfile templates |
| [Architecture](docs/architecture.md) | System design and component overview |
| [Security Model](docs/security-model.md) | Egress rules, MITM, credential injection |
| [Bridge Protocol](docs/bridge-protocol.md) | ACP protocol and channel adapters |
| [Docker API Proxy](docs/docker-api-proxy.md) | Controlled container spawning [PLANNED] |
| [ADRs](docs/adr/) | Architecture decision records |

## Supported Runtimes

| Runtime | Status |
|---------|--------|
| [Codex](runtimes/codex/) | ✅ Primary |
| Claude Code | 🔜 Planned |

## Supported Channels

| Platform | Status |
|----------|--------|
| [Telegram](runtimes/channels-bridge/) | ✅ Primary |
| Slack | 🔜 Planned |

## Development

```bash
go build ./cmd/agent-fleet
go test ./...
golangci-lint run
```

## License

MIT
