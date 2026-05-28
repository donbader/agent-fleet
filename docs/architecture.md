# Architecture

## Overview

agent-fleet is an orchestrator that deploys AI coding agents inside secure sandboxes with messaging bridges. It sits on top of [NVIDIA OpenShell](https://github.com/NVIDIA/OpenShell) for sandbox isolation and adds fleet management, bridge wiring, and credential orchestration.

## System Layers

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 4: Fleet Orchestration (agent-fleet CLI)              │
│  - Reads fleet.yaml                                         │
│  - Provisions sandboxes via OpenShell                       │
│  - Starts bridges                                           │
│  - Manages lifecycle (up/down/status/logs)                  │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: Bridge (ACP ↔ Messaging Platform)                  │
│  - Speaks ACP to the agent                                  │
│  - Speaks platform API (Telegram, Slack, etc.)              │
│  - Runs inside the sandbox (egress controlled)              │
│  - Multi-session: one bridge handles multiple chats         │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 2: Sandbox (OpenShell)                                │
│  - Kernel isolation (Landlock, seccomp, namespaces)         │
│  - Network policy (L7 proxy, default-deny)                  │
│  - Credential injection at proxy boundary                   │
│  - Process restriction (unprivileged agent)                 │
└─────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: Compute (Docker / Podman / VM)                     │
│  - Container or VM per sandbox                              │
│  - Managed by OpenShell gateway                             │
└─────────────────────────────────────────────────────────────┘
```

## Component Breakdown

### 1. CLI (`cmd/agent-fleet/`)

The user-facing command-line tool. Commands:

| Command | Description |
|---------|-------------|
| `agent-fleet init <name>` | Scaffold a new fleet directory with example fleet.yaml |
| `agent-fleet up` | Provision all agents defined in fleet.yaml |
| `agent-fleet down` | Tear down all sandboxes and bridges |
| `agent-fleet status` | Show running agents, bridges, and health |
| `agent-fleet logs <agent>` | Stream logs from an agent's sandbox |
| `agent-fleet exec <agent>` | Open a shell in an agent's sandbox |
| `agent-fleet provider sync` | Sync .env secrets to OpenShell providers |

### 2. Config (`pkg/config/`)

Parses and validates `fleet.yaml`. Resolves:
- Agent runtime selection
- Bridge wiring (which agent connects to which bridge)
- Egress rules compilation
- Secret provider mapping
- Docker proxy policy

### 3. Sandbox (`pkg/sandbox/`)

Wraps OpenShell CLI/API to:
- Create sandboxes with the right image, providers, and policy
- Attach network policies (compiled from fleet.yaml egress rules)
- Manage sandbox lifecycle (create, connect, delete)
- Hot-reload network policies when config changes

### 4. Bridge (`pkg/bridge/`)

Manages bridge processes:
- Starts bridge containers/processes alongside agent sandboxes
- Configures ACP connection (Unix socket or TCP)
- Handles multi-session routing (multiple chats → one agent, different sessions)

### 5. Egress (`pkg/egress/`)

Compiles fleet.yaml egress rules into OpenShell network policy YAML:

```yaml
# fleet.yaml (user writes)
egress:
  - host: api.github.com
    port: 443
    access: full

# Compiled to OpenShell policy (agent-fleet generates)
network_policies:
  github_api:
    name: github-api
    endpoints:
      - host: api.github.com
        port: 443
        protocol: rest
        enforcement: enforce
        access: full
    binaries:
      - path: "*"
```

### 6. Docker API Proxy (`pkg/docker-proxy/`)

Optional component. When enabled:
- Runs as a separate process accessible from the sandbox
- Validates all Docker API requests against policy
- Enforces: image allowlist, no privileged, resource limits, network inheritance
- Only accepts authenticated requests from known sandbox agents

### 7. Adapters (`pkg/adapters/`)

Protocol translation for agents that don't speak ACP natively:

| Adapter | From | To | Notes |
|---------|------|-----|-------|
| `pi-rpc-to-acp` | Pi RPC (stdin/stdout JSON) | ACP (ndJSON stdio) | Wraps Pi process |
| `claude-headless-to-acp` | Claude Code CLI (stream-json) | ACP (ndJSON stdio) | Wraps `claude -p` |

## Data Flow

### Message from Telegram to Agent

```
1. User sends message in Telegram
2. Telegram API → Bridge (inside sandbox, via egress policy allowing api.telegram.org)
3. Bridge creates/resumes ACP session for this chat
4. Bridge sends ACP message to Agent process (via Unix socket)
5. Agent processes, may make tool calls
6. Agent responds via ACP
7. Bridge sends response to Telegram API
```

### Agent Makes External API Call

```
1. Agent code calls https://api.github.com/...
2. Request hits sandbox network namespace → forced to local proxy
3. OpenShell proxy checks policy: api.github.com:443 allowed? ✅
4. Proxy injects credentials (Authorization: token ghp_xxx)
5. Request forwarded to api.github.com
6. Response returned to agent
```

### Agent Spins Up Docker Container (when Docker proxy enabled)

```
1. Agent calls Docker API proxy endpoint
2. Docker API Proxy validates:
   - Image in allowlist? ✅
   - No --privileged? ✅
   - Resource limits within budget? ✅
3. Proxy creates container on internal network
4. New container's egress also goes through sandbox proxy
5. Container ID returned to agent
```

## Key Invariants

1. **All egress is proxied** — No direct internet access from any agent or agent-spawned container
2. **Secrets never in sandbox** — Credentials exist only in OpenShell provider store and proxy memory
3. **Bridge is inside sandbox** — Bridge's Telegram API access is also egress-controlled
4. **Default deny** — If no egress rule matches, the request is blocked
5. **Agent-agnostic** — Any agent that speaks ACP (or has an adapter) works
6. **Multi-session** — One bridge instance handles multiple concurrent conversations

## Comparison with Alternatives

| | agent-fleet | OpenShell alone | Docker Compose (manual) |
|---|---|---|---|
| Fleet management | ✅ Single config | ❌ Per-sandbox CLI | ❌ Manual compose files |
| Bridge abstraction | ✅ ACP protocol | ❌ None | ❌ DIY |
| Sandbox security | ✅ OpenShell | ✅ OpenShell | ❌ Basic container isolation |
| Credential injection | ✅ Automatic | ✅ Manual provider setup | ❌ Env vars in compose |
| Docker for agents | ✅ Policy-enforced proxy | ❌ Not supported | ✅ But no policy |
| Multi-agent | ✅ Fleet-native | ⚠️ Manual per-sandbox | ⚠️ Manual compose services |
