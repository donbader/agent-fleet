# Agent Instructions

You are working on `agent-fleet` — an opinionated agent sandbox orchestrator written in Go.

## Project Overview

agent-fleet deploys AI coding agents (Codex, Claude Code, Pi) inside OpenShell sandboxes with:
- Mandatory egress control (all outbound traffic goes through a policy proxy)
- Messaging bridges (Telegram, Slack) speaking ACP protocol
- Fleet management (multiple agents from one config)
- Credential injection at the network boundary (secrets never enter the sandbox)
- Optional Docker API Proxy for controlled container spawning

## Repository Structure

```
agent-fleet/
├── cmd/agent-fleet/          # CLI entrypoint
├── pkg/
│   ├── config/               # fleet.yaml parsing and validation
│   ├── sandbox/              # OpenShell sandbox provisioning
│   ├── bridge/               # Bridge lifecycle and ACP protocol
│   ├── egress/               # Egress rule compilation and proxy config
│   ├── docker-proxy/         # Docker API Proxy implementation
│   ├── fleet/                # Fleet orchestration (up/down/status)
│   └── adapters/             # Protocol adapters (pi-rpc-to-acp, etc.)
├── bridges/
│   └── telegram/             # Telegram bridge Docker image
├── docker-proxy/             # Docker API Proxy Docker image
├── docs/                     # Architecture and design documents
├── examples/                 # Example fleet configurations
├── tests/                    # Integration tests
├── fleet.yaml                # Example fleet config (for development)
└── go.mod
```

## Key Design Decisions

1. **OpenShell is the sandbox layer** — We don't build our own sandbox. OpenShell handles isolation (Landlock, seccomp, network namespace). We orchestrate on top.

2. **ACP is the bridge protocol** — All bridges speak ACP (Agent Client Protocol). Agents that don't support ACP natively get a translation adapter.

3. **Secrets never enter the sandbox** — Credentials are stored in OpenShell providers and injected at the L7 proxy boundary. The agent process never sees raw tokens.

4. **Docker API Proxy is optional** — When enabled, agents can spin up containers through a policy-enforcing proxy. New containers join the same controlled network.

5. **Single config file** — Everything is defined in `fleet.yaml`. No scattered configs.

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
- [grammy](https://grammy.dev/) — Telegram bot framework (for bridge, Node.js sidecar)
- Docker Engine — Container runtime

## What NOT to Do

- Don't bypass OpenShell — all sandbox operations go through `openshell` CLI or API
- Don't store secrets in fleet.yaml — use `.env` files referenced by `env_file:`
- Don't add features that only work with one agent runtime — keep it agent-agnostic
- Don't mix bridge protocol concerns with agent protocol concerns
