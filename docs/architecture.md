# Architecture

## Overview

agent-fleet is an orchestrator that deploys AI coding agents inside secure Docker containers with transparent egress proxying, messaging channels, and fleet management.

## System Layers

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 4: Fleet Orchestration (agent-fleet CLI)              │
│  - Reads fleet.yaml                                         │
│  - Generates Docker Compose                                 │
│  - Manages lifecycle (up/down/status/logs)                  │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: Channel (ACP ↔ Messaging Platform)                 │
│  - Speaks ACP to the agent                                  │
│  - Speaks platform API (Telegram, Slack, etc.)              │
│  - Runs inside the agent container                          │
│  - Multi-session: one channel handles multiple chats        │
│  - Handles OAuth UX (/oauth <provider>)                     │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 2: Gateway (Transparent Egress Proxy)                 │
│  - iptables redirects ALL outbound TCP to proxy             │
│  - Reads TLS SNI to identify destination                    │
│  - Evaluates egress rules (first match wins)                │
│  - Credential injection via MITM when needed                │
│  - Passthrough tunnel when no injection needed              │
│  - Default deny — no match = connection dropped             │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: Container Isolation (Docker)                        │
│  - Internal network (no direct internet access)             │
│  - Agent container can only reach proxy                     │
│  - Proxy container bridges internal ↔ external              │
└─────────────────────────────────────────────────────────────┘
```

## Component Breakdown

### 1. CLI (`cmd/agent-fleet/`)

The user-facing command-line tool. Commands:

| Command | Description |
|---------|-------------|
| `agent-fleet init <name>` | Scaffold a new fleet directory with example fleet.yaml |
| `agent-fleet up` | Generate Docker Compose and start all agents |
| `agent-fleet down` | Tear down all containers |
| `agent-fleet status` | Show running agents, channels, and health |
| `agent-fleet logs <agent>` | Stream logs from an agent |
| `agent-fleet exec <agent>` | Open a shell in an agent container |

### 2. Config (`pkg/config/`)

Parses and validates `fleet.yaml`. Resolves:
- Agent runtime selection
- Channel provider wiring
- Gateway assignment (shared or per-agent)
- Egress rule compilation
- Docker Compose generation

### 3. Gateway Proxy (`pkg/gateway/`)

A Go binary that runs as a transparent proxy inside the agent container:

- Listens on a local port (e.g., 8443)
- iptables redirects all outbound TCP to this port
- Reads TLS ClientHello SNI to determine destination host
- Evaluates egress rules in order (first match wins)
- For rules with providers: MITM TLS with sandbox CA, inject credentials
- For rules without providers: SNI-based passthrough tunnel
- No match: drop connection

### 4. Egress Rule Providers (`pkg/egress-rules/`)

Each provider implements credential injection or service exposure:

| Provider | Behavior |
|----------|----------|
| `egress-rules/github-pat` | MITM + inject `Authorization: token <pat>` |
| `egress-rules/mcp-oauth` | MITM + inject `Authorization: Bearer <token>` + auto-refresh |
| `egress-rules/mcp-token` | MITM + inject app credentials |
| `egress-rules/api-key` | MITM + inject custom header |
| `egress-rules/docker-api-proxy` | Expose Docker API endpoint with policy enforcement |

### 5. Channels Bridge (`pkg/bridge/`)

The `channels-bridge` runtime provider:
- Spawns the agent runtime as a child process
- Manages channel provider instances (Telegram, web-ui, etc.)
- Routes messages between channels and agent (ACP protocol)
- Handles command registration (channels register commands, bridge dispatches)
- Manages OAuth flows via registered commands
- Handles multi-session routing (multiple chats → one agent, different sessions)

### 6. Docker API Proxy (`pkg/docker-proxy/`)

Optional. When the `docker-api-proxy` egress rule is present:
- Runs a DinD (Docker-in-Docker) container on the internal network
- Docker API Proxy validates requests against policy
- Agent reaches proxy via the transparent proxy (allowed by egress rule)
- New containers also on internal network (egress through proxy)

### 7. Adapters (`pkg/adapters/`)

Protocol translation for agents that don't speak ACP natively:

| Adapter | From | To |
|---------|------|-----|
| `pi-rpc-to-acp` | Pi RPC (stdin/stdout JSON) | ACP (ndJSON stdio) |
| `claude-headless-to-acp` | Claude Code CLI (stream-json) | ACP (ndJSON stdio) |

## Transparent Proxy Flow

### How iptables Redirect Works

```
Agent process makes TCP connection to api.github.com:443
  │
  │ (kernel intercepts via iptables NAT REDIRECT)
  │ (original destination preserved via SO_ORIGINAL_DST)
  ▼
Gateway Proxy (listening on localhost:8443)
  │
  ├── Reads TLS ClientHello → SNI: "api.github.com"
  ├── Looks up original destination (SO_ORIGINAL_DST)
  ├── Evaluates egress rules in order:
  │     Rule 1: host ["api.github.com"] + github-pat provider → MATCH
  │
  ├── Provider needs credential injection → MITM mode:
  │     1. Complete TLS handshake with agent (using sandbox CA)
  │     2. Read HTTP request from agent
  │     3. Inject: Authorization: token ghp_REAL_TOKEN
  │     4. Open TLS connection to real api.github.com
  │     5. Forward modified request
  │     6. Relay response back to agent
  │
  └── Agent receives response (thinks it talked directly to GitHub)
```

### Passthrough (No Credential Injection)

```
Agent connects to registry.npmjs.org:443
  │
  ▼
Gateway Proxy
  ├── SNI: "registry.npmjs.org"
  ├── Rule match: host ["*"] (catch-all, no provider)
  ├── No credential injection needed → passthrough mode:
  │     1. Open TCP connection to registry.npmjs.org:443
  │     2. Splice/relay raw bytes (no TLS termination)
  │     3. Agent's TLS goes end-to-end to destination
  └── Zero overhead, no MITM
```

## Network Topology

```
┌─────────────────────────────────────────────────────────────┐
│  Docker Host                                                 │
│                                                             │
│  ┌─ Internal Network (no internet access) ────────────────┐  │
│  │                                                        │  │
│  │  ┌─ Agent Container ─────────────────────────────────┐ │  │
│  │  │                                                   │ │  │
│  │  │  ┌─────────────────────────────────────────────┐  │ │  │
│  │  │  │  Agent (codex) + Channel (telegram bot)     │  │ │  │
│  │  │  │  All TCP → iptables REDIRECT → proxy        │  │ │  │
│  │  │  └─────────────────────────────────────────────┘  │ │  │
│  │  │                                                   │ │  │
│  │  │  ┌─────────────────────────────────────────────┐  │ │  │
│  │  │  │  Gateway Proxy (Go binary, transparent)     │  │ │  │
│  │  │  │  Egress rules + credential injection        │  │ │  │
│  │  │  └─────────────────────────────────────────────┘  │ │  │
│  │  │                                                   │ │  │
│  │  └───────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Docker API Proxy (optional) ────────────────────┐ │  │
│  │  │  Policy enforcement → DinD container             │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Agent-spawned containers (optional) ────────────┐ │  │
│  │  │  Also on internal network, egress via proxy      │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─ External Network ─────────────────────────────────────┐  │
│  │  Gateway Proxy has access here (bridges internal↔ext)  │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Key Invariants

1. **All egress is proxied** — iptables forces ALL TCP through the gateway proxy. No bypass possible.
2. **Default deny** — If no egress rule matches, connection is dropped
3. **Transparent** — Agent doesn't know about the proxy. No HTTP_PROXY env vars needed.
4. **MITM only when needed** — Passthrough for rules without credential injection (zero overhead)
5. **Secrets never in container** — Credentials exist only in proxy memory, injected at L7
6. **Channel manages its own credentials** — Bot token is channel's responsibility
7. **Agent-agnostic** — Any agent that speaks ACP (or has an adapter) works
8. **Multi-session** — One channel instance handles multiple concurrent conversations

## Technology Choices

| Component | Technology | Why |
|-----------|-----------|-----|
| CLI | Go | Single binary, good CLI libraries (cobra) |
| Gateway Proxy | Go (`goproxy` or custom) | Same language as CLI, can embed in same binary |
| Channel Provider | Node.js (grammy) | Best Telegram bot library ecosystem |
| Docker API Proxy | Go | Same binary, HTTP reverse proxy |
| Container Orchestration | Docker Compose | Simple, no k8s needed |
| TLS MITM | Go `crypto/tls` + generated CA | Standard library, no external deps |

## Comparison with Alternatives

| | agent-fleet | Plain Docker Compose |
|---|---|---|
| Transparent proxy | ✅ iptables redirect | ❌ Manual HTTP_PROXY |
| Allow-all traffic | ✅ `host: ["*"]` | ✅ No restrictions |
| Credential injection | ✅ MITM at proxy | ❌ Env vars exposed |
| Docker for agents | ✅ Policy-enforced proxy | ✅ But no policy |
| Kernel isolation | ❌ Docker only | ❌ Docker only |
| Deployment | Docker Compose | Docker Compose |
| Complexity | Medium | Low |
