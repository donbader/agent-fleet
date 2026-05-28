# Agent Instructions

You are working on `agent-fleet` — an opinionated agent sandbox orchestrator written in Go.

## Project Overview

agent-fleet deploys AI coding agents (Codex, Claude Code, Pi) inside Docker containers with:
- Transparent egress proxy (iptables forces ALL TCP through our Go proxy)
- Default-deny egress with first-match-wins rules
- Per-agent messaging channels (Telegram bots) speaking ACP protocol
- Credential injection at L7 via MITM (agent never sees real tokens)
- Fleet management (multiple agents from one config)
- Optional Docker API Proxy for controlled container spawning

## Repository Structure

```
agent-fleet/
├── cmd/agent-fleet/          # CLI entrypoint
├── pkg/
│   ├── config/               # fleet.yaml + agent.yaml parsing and validation
│   ├── compose/              # Docker Compose generation
│   ├── gateway/              # Transparent proxy (Go, iptables + TLS MITM)
│   ├── bridge/               # channels-bridge runtime (spawns agent + manages channels)
│   ├── egress-rules/         # Egress rule provider implementations
│   │   ├── github-pat/       # GitHub PAT injection
│   │   ├── mcp-oauth/       # MCP OAuth2 flow + token refresh
│   │   ├── mcp-token/       # MCP app credential injection
│   │   ├── docker-api-proxy/ # Docker API Proxy + DinD
│   │   └── api-key/         # Generic API key injection
│   ├── fleet/                # Fleet orchestration (up/down/status)
│   └── adapters/             # Protocol adapters (pi-rpc-to-acp, etc.)
├── runtimes/
│   ├── codex/                # Codex runtime provider (+ schema.json)
│   ├── claude-code/          # Claude Code runtime provider (+ schema.json)
│   ├── pi/                   # Pi runtime provider (+ schema.json)
│   └── channels-bridge/      # channels-bridge runtime (+ schema.json)
├── channel-providers/
│   └── telegram/             # Telegram channel provider (+ schema.json)
├── schemas/
│   ├── fleet.schema.json     # Top-level fleet.yaml validation
│   └── agent.schema.json     # Top-level agent.yaml validation
├── images/
│   ├── sandbox/              # Base sandbox Dockerfile (iptables + CA + proxy)
│   └── docker-proxy/        # Docker API Proxy Dockerfile
├── docs/                     # Architecture and design documents
├── examples/                 # Example fleet configurations
├── tests/                    # Integration tests
└── go.mod
```

## Key Design Decisions

1. **No OpenShell** — We use Docker + our own transparent proxy. OpenShell doesn't support allow-all traffic (`host: ["*"]`), which is essential for dev agents.

2. **Transparent proxy** — iptables redirects ALL outbound TCP to our Go proxy. Agent cannot bypass it. No HTTP_PROXY env vars needed.

3. **Default deny egress** — No traffic leaves unless an egress rule matches. `host: ["*"]` as catch-all to allow all.

4. **Channel per agent** — Each agent has its own bot/channel instance. No shared bots.

5. **Unified egress-rules** — Everything is an egress rule with a provider. GitHub PAT, MCP OAuth, Docker API Proxy — all the same abstraction.

6. **MITM only when needed** — Rules with credential injection use MITM. Rules without use passthrough (zero overhead, end-to-end TLS preserved).

7. **Egress presets are composable** — Agents select multiple presets. Rules evaluated in order across presets (first match wins).

## Configuration Format

See `docs/configuration.md` for full reference. Key concepts:

```
my-fleet/
  fleet.yaml              # shared egress-presets + agent list
  .env                    # secrets
  agents/
    <name>/
      agent.yaml          # per-agent config
```

**fleet.yaml:**
```yaml
fleet:
  name: my-fleet

agents:
  - <name>

egress-presets:
  <name>:
    - host: [...]                   # domain match
      provider: "<provider-path>"   # optional: handles injection
      options: { ... }
    - endpoint: [...]               # full URL match (for MCP)
      provider: "<provider-path>"
    - host: ["*"]                   # catch-all (allow remaining)
```

## Development Workflow

```bash
# Build
go build ./cmd/agent-fleet

# Test
go test ./...

# Integration tests (requires Docker)
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

- Go standard library `crypto/tls` — TLS MITM with generated CA
- `github.com/elazarl/goproxy` or custom — transparent proxy
- Docker Engine — container runtime
- Docker Compose — orchestration
- [grammy](https://grammy.dev/) — Telegram bot framework (channel provider, Node.js)

## What NOT to Do

- Don't use OpenShell — we manage our own proxy and isolation
- Don't store secrets in fleet.yaml — use `.env` + `${VAR}` interpolation in options
- Don't add features that only work with one agent runtime — keep it agent-agnostic
- Don't mix channel concerns with gateway concerns
- Don't set HTTP_PROXY env vars — our proxy is transparent (iptables)
- Don't bypass iptables — the transparent proxy is the security boundary
