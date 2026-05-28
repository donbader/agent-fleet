# Agent Instructions

You are working on `agent-fleet` — an opinionated agent sandbox orchestrator written in Go.

## Project Overview

agent-fleet deploys AI coding agents (Codex, Claude Code, Pi) inside OpenShell sandboxes with:
- Default-deny egress control (all outbound traffic goes through a gateway proxy)
- Per-agent messaging channels (Telegram bots) speaking ACP protocol
- Fleet management (multiple agents from one config)
- Credential injection at the network boundary via auth providers
- Optional Docker API Proxy for controlled container spawning

## Repository Structure

```
agent-fleet/
├── cmd/agent-fleet/          # CLI entrypoint
├── pkg/
│   ├── config/               # fleet.yaml parsing and validation
│   ├── sandbox/              # OpenShell sandbox provisioning
│   ├── channel/              # Channel provider lifecycle and ACP protocol
│   ├── gateway/              # Gateway proxy + egress rule compilation
│   ├── auth-providers/       # Credential injection implementations
│   │   ├── github-pat/       # GitHub PAT injection
│   │   ├── mcp-oauth/       # MCP OAuth2 flow + token refresh
│   │   ├── mcp-token/       # MCP app credential injection
│   │   └── api-key/         # Generic API key injection
│   ├── docker-proxy/         # Docker API Proxy implementation
│   ├── fleet/                # Fleet orchestration (up/down/status)
│   └── adapters/             # Protocol adapters (pi-rpc-to-acp, etc.)
├── channel-providers/
│   └── telegram/             # Telegram channel provider
├── docs/                     # Architecture and design documents
├── examples/                 # Example fleet configurations
├── tests/                    # Integration tests
└── go.mod
```

## Key Design Decisions

1. **OpenShell is the sandbox layer** — We don't build our own sandbox. OpenShell handles isolation (Landlock, seccomp, network namespace). We orchestrate on top.

2. **Default deny egress** — No traffic leaves the sandbox unless an egress rule matches. Use `- host: ["*"]` as catch-all to allow all.

3. **Channel per agent** — Each agent has its own bot/channel instance. No shared bots with routing complexity.

4. **Auth at the gateway boundary** — Auth providers inject credentials into matching requests at the L7 proxy. Agent never sees real tokens.

5. **Provider pattern** — Channels and auth use Go module path identifiers (e.g., `github.com/donbader/agent-fleet/auth-providers/github-pat`). Built-in for now, extensible later.

6. **Gateways are shareable** — Multiple agents can reference the same gateway for shared egress rules and auth.

7. **OAuth via chat** — Users authorize OAuth services by sending `/oauth <provider>` in their chat with the bot.

## Configuration Format

See `docs/configuration.md` for full reference. Key concepts:

```yaml
fleet:
  name: my-fleet

agents:
  <name>:
    runtime: codex | claude-code | pi
    gateway: <gateway-name>
    channel:
      provider: "<provider-path>"
      options: { ... }
    docker: { enabled: true, ... }    # optional
    env: { ... }

gateways:
  <name>:
    egress:
      - host: [...]                   # domain match
        auth:                         # optional credential injection
          provider: "<provider-path>"
          options: { ... }
      - endpoint: [...]               # full URL match (for MCP)
        auth: { ... }
      - host: ["*"]                   # catch-all (allow remaining)
```

## Development Workflow

```bash
# Build
go build ./cmd/agent-fleet

# Test
go test ./...

# Integration tests (requires Docker + OpenShell)
go test ./tests/... -tags=integration

# Lint
golangci-lint run
```

## Conventions

- Go 1.22+
- Use `slog` for structured logging
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Table-driven tests
- Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`
- One concern per PR

## Key Dependencies

- [OpenShell](https://github.com/NVIDIA/OpenShell) — Sandbox runtime (CLI: `openshell`)
- [ACP SDK](https://github.com/anthropics/agent-client-protocol) — Agent Client Protocol
- [grammy](https://grammy.dev/) — Telegram bot framework (for channel provider, Node.js)
- Docker Engine — Container runtime

## What NOT to Do

- Don't bypass OpenShell — all sandbox operations go through `openshell` CLI or API
- Don't store secrets in fleet.yaml — use `.env` files referenced by `*_env` options
- Don't add features that only work with one agent runtime — keep it agent-agnostic
- Don't mix channel concerns with gateway concerns (channel = messaging, gateway = egress + auth)
- Don't inject Telegram bot tokens via gateway — channel provider manages its own platform credentials
