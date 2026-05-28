# Architecture

## Overview

agent-fleet is an orchestrator that deploys AI coding agents inside secure sandboxes with messaging channels. It sits on top of [NVIDIA OpenShell](https://github.com/NVIDIA/OpenShell) for sandbox isolation and adds fleet management, channel wiring, and credential orchestration.

## System Layers

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 4: Fleet Orchestration (agent-fleet CLI)              │
│  - Reads fleet.yaml                                         │
│  - Provisions sandboxes via OpenShell                       │
│  - Starts channel providers                                 │
│  - Manages lifecycle (up/down/status/logs)                  │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: Channel (ACP ↔ Messaging Platform)                 │
│  - Speaks ACP to the agent                                  │
│  - Speaks platform API (Telegram, Slack, etc.)              │
│  - Runs inside the sandbox (egress controlled)              │
│  - Multi-session: one channel handles multiple chats        │
│  - Handles OAuth UX (/oauth <provider>)                     │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 2: Gateway (Egress Proxy + Auth Injection)            │
│  - Default deny — only matched rules pass                   │
│  - Auth providers inject credentials at L7 boundary         │
│  - Shared between agents or per-agent                       │
│  - Manages OAuth token storage and refresh                  │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: Sandbox (OpenShell)                                │
│  - Kernel isolation (Landlock, seccomp, namespaces)         │
│  - Network namespace forces all egress through gateway      │
│  - Process restriction (unprivileged agent)                 │
│  - Compute: Docker / Podman / VM                            │
└─────────────────────────────────────────────────────────────┘
```

## Component Breakdown

### 1. CLI (`cmd/agent-fleet/`)

The user-facing command-line tool. Commands:

| Command | Description |
|---------|-------------|
| `agent-fleet init <name>` | Scaffold a new fleet directory with example fleet.yaml |
| `agent-fleet up` | Provision all agents defined in fleet.yaml |
| `agent-fleet down` | Tear down all sandboxes and channels |
| `agent-fleet status` | Show running agents, channels, and health |
| `agent-fleet logs <agent>` | Stream logs from an agent's sandbox |
| `agent-fleet exec <agent>` | Open a shell in an agent's sandbox |

### 2. Config (`pkg/config/`)

Parses and validates `fleet.yaml`. Resolves:
- Agent runtime selection
- Channel provider wiring (which agent gets which bot)
- Gateway assignment (which agents share which gateway)
- Egress rule compilation
- Auth provider configuration

### 3. Sandbox (`pkg/sandbox/`)

Wraps OpenShell CLI/API to:
- Create sandboxes with the right image and policy
- Attach network policies (compiled from gateway egress rules)
- Manage sandbox lifecycle (create, connect, delete)
- Hot-reload network policies when config changes

### 4. Channel (`pkg/channel/`)

Manages channel provider processes:
- Starts channel provider alongside agent sandbox
- Configures ACP connection (Unix socket)
- Handles multi-session routing (multiple chats → one agent, different sessions)
- Delegates OAuth UX to channel provider

### 5. Gateway (`pkg/gateway/`)

Manages egress proxy instances:
- Compiles fleet.yaml egress rules into OpenShell network policy
- Loads and initializes auth providers
- Routes requests through auth providers for credential injection
- Manages OAuth token storage and refresh lifecycle
- Can be shared between multiple agents

### 6. Auth Providers (`pkg/auth-providers/`)

Each auth provider implements a specific credential injection interface:

| Provider | Interface | What It Does |
|----------|-----------|-------------|
| `github-pat` | HeaderInjector | Adds `Authorization: token <pat>` |
| `mcp-oauth` | OAuthProvider | Manages OAuth2 flow + token refresh + Bearer injection |
| `mcp-token` | HeaderInjector | Adds app-specific auth headers |
| `telegram` | URLRewriter | Injects bot token into Telegram API URL path |
| `api-key` | HeaderInjector | Adds custom header with API key |

### 7. Docker API Proxy (`pkg/docker-proxy/`)

Optional component. When enabled:
- Runs as a separate process accessible from the sandbox
- Validates all Docker API requests against policy
- Enforces: image allowlist, no privileged, resource limits, network inheritance
- Only accepts authenticated requests from known sandbox agents

### 8. Adapters (`pkg/adapters/`)

Protocol translation for agents that don't speak ACP natively:

| Adapter | From | To | Notes |
|---------|------|-----|-------|
| `pi-rpc-to-acp` | Pi RPC (stdin/stdout JSON) | ACP (ndJSON stdio) | Wraps Pi process |
| `claude-headless-to-acp` | Claude Code CLI (stream-json) | ACP (ndJSON stdio) | Wraps `claude -p` |

## Data Flow

### Message from Telegram to Agent

```
1. User sends message in Telegram
2. Telegram API → Channel provider (inside sandbox, bot token in request)
3. Channel creates/resumes ACP session for this chat
4. Channel sends ACP message to Agent process (via Unix socket)
5. Agent processes, may make tool calls
6. Agent responds via ACP
7. Channel sends response to Telegram API (bot token in request)
```

### Agent Makes External API Call (e.g., GitHub)

```
1. Agent code calls https://api.github.com/repos/...
2. Request hits sandbox network namespace → forced to gateway proxy
3. Gateway evaluates egress rules in order:
   - Rule: host ["api.github.com", "github.com"] → MATCH
   - Auth provider: github-pat → inject Authorization header
4. Request forwarded to api.github.com with real PAT
5. Response returned to agent
```

### OAuth Flow (e.g., Notion MCP)

```
1. User sends "/oauth notion" in Telegram chat
2. Channel provider handles command:
   - Generates OAuth authorization URL (using client_id from .env)
   - Sends URL to user in chat
3. User clicks URL, authorizes in browser
4. User pastes callback URL back in chat: "/oauth callback https://...?code=abc123"
5. Channel provider extracts code, sends to gateway
6. Gateway's mcp-oauth auth provider:
   - Exchanges code for access_token + refresh_token
   - Stores tokens
   - Auto-refreshes before expiry
7. Future requests to mcp.notion.com get Bearer token injected
```

### Agent Spins Up Docker Container (when docker enabled)

```
1. Agent calls Docker API proxy endpoint
2. Docker API Proxy validates:
   - Image in allowlist? ✅
   - No --privileged? ✅
   - Resource limits within budget? ✅
3. Proxy creates container on internal network
4. New container's egress also goes through gateway
5. Container ID returned to agent
```

## Key Invariants

1. **Default deny** — If no egress rule matches, the request is blocked
2. **Secrets never in sandbox** — Credentials exist only in gateway auth providers
3. **Channel manages its own platform credentials** — Bot tokens are channel provider's responsibility
4. **Gateway handles third-party auth** — GitHub, MCP, etc. credentials injected at proxy boundary
5. **Agent-agnostic** — Any agent that speaks ACP (or has an adapter) works
6. **Multi-session** — One channel instance handles multiple concurrent conversations
7. **Gateways are shareable** — Multiple agents can use the same gateway for egress

## Provider System

Providers are identified by Go module paths but are built-in to agent-fleet. The path is a naming convention for extensibility:

```
github.com/donbader/agent-fleet/channel-providers/telegram
github.com/donbader/agent-fleet/auth-providers/github-pat
github.com/donbader/agent-fleet/auth-providers/mcp-oauth
```

Different provider types implement different interfaces:
- **Channel providers** — messaging platform integration (receive/send messages, OAuth UX)
- **Auth providers** — credential injection at gateway boundary (header injection, OAuth flow, URL rewriting)

Future: support external providers via plugin mechanism.
