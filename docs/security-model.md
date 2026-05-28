# Security Model

## Principles

1. **Default deny** — No network access unless an egress rule matches
2. **Transparent interception** — iptables forces ALL TCP through proxy. Agent cannot bypass.
3. **Secrets never in container** — Credentials exist only in proxy memory
4. **MITM only when needed** — Passthrough for non-injection rules (zero overhead)
5. **All credentials through proxy** — Bot tokens, PATs, OAuth tokens all injected by proxy. Agent/channel never sees real credentials.
6. **Controlled escalation** — Docker access is opt-in via egress rule provider

## Isolation Layers

```
┌─────────────────────────────────────────────────────────┐
│  Docker Container                                        │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Network Isolation                               │    │
│  │  • Internal-only Docker network                 │    │
│  │  • iptables REDIRECT all TCP → proxy            │    │
│  │  • No direct internet access                    │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Gateway Proxy (transparent, Go)                 │    │
│  │  • Evaluates egress rules (first match wins)    │    │
│  │  • MITM for credential injection                │    │
│  │  • Passthrough for allow-only rules             │    │
│  │  • DROP for unmatched traffic                   │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Agent + Channel (unprivileged process)          │    │
│  │  • Cannot bypass proxy (iptables enforced)      │    │
│  │  • Cannot see real credentials                  │    │
│  │  • Thinks it's talking directly to the internet │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Transparent Proxy Security

### Why Transparent (not Explicit)?

| | Explicit (HTTP_PROXY) | Transparent (iptables) |
|---|---|---|
| Agent can bypass? | ✅ Yes (ignore env var) | ❌ No (kernel enforced) |
| Works with all tools? | ⚠️ Only proxy-aware tools | ✅ All TCP traffic |
| Agent awareness | Knows about proxy | Completely unaware |
| Setup complexity | Low (env var) | Medium (iptables + NET_ADMIN) |

### iptables Rules (inside agent container)

```bash
# Redirect all outbound TCP to proxy
iptables -t nat -A OUTPUT -p tcp -m owner ! --uid-owner proxy-uid -j REDIRECT --to-port 8443

# The proxy process itself (running as proxy-uid) is NOT redirected
# This prevents infinite loops
```

The proxy runs as a dedicated user (`proxy-uid`). Its own outbound connections are exempt from redirection — this is how it reaches the actual internet.

### TLS Handling

| Scenario | Proxy Behavior | Agent Sees |
|----------|---------------|-----------|
| Rule match + credential injection | MITM: terminate TLS, inject header, re-encrypt | Valid TLS cert (from sandbox CA) |
| Rule match + passthrough | SNI tunnel: relay raw TCP bytes | Original server cert (end-to-end TLS) |
| No rule match | DROP connection | Connection timeout/reset |

For MITM to work, the agent container trusts a sandbox CA certificate (injected at container build time). The proxy generates per-destination certificates signed by this CA.

### Sandbox CA Trust

```
Container build:
  1. Generate ephemeral CA key + cert
  2. Install CA cert in container trust store
  3. Proxy holds CA private key

Runtime:
  Agent connects to api.github.com:443
  → Proxy intercepts (iptables)
  → Proxy generates cert for "api.github.com" signed by sandbox CA
  → Agent validates cert against trust store → ✅ trusted
  → Proxy reads HTTP request, injects credentials
  → Proxy connects to real api.github.com with real cert
```

## Credential Management

### Separation of Concerns

| Who | Manages What | How |
|-----|-------------|-----|
| **Channel provider** | Messaging platform credentials (bot token) | Reads from env, uses directly in API calls |
| **Gateway proxy** | Third-party API credentials (PAT, OAuth tokens) | Injects at L7 via MITM |
| **Agent** | Nothing | Sees dummy tokens or no tokens at all |

### Why the Agent Never Sees Real Credentials

```
Agent env:
  GH_TOKEN=proxy_dummy_token     ← dummy (so gh CLI doesn't error)

Agent makes request:
  GET https://api.github.com/repos/...
  Authorization: token proxy_dummy_token

Proxy intercepts (transparent, iptables):
  - Terminates TLS (MITM with sandbox CA)
  - Reads HTTP request
  - Strips: Authorization: token proxy_dummy_token
  - Injects: Authorization: token ghp_REAL_TOKEN_HERE
  - Opens new TLS to real api.github.com
  - Forwards modified request

GitHub receives:
  GET /repos/...
  Authorization: token ghp_REAL_TOKEN_HERE
```

### OAuth Flow (MCP Services)

```
1. User: /oauth notion
2. Channel: "Click to authorize: https://api.notion.com/v1/oauth/authorize?..."
3. User: [clicks, authorizes in browser]
4. User: /oauth callback https://redirect.example.com?code=abc123
5. Channel → Proxy: exchange code for tokens
6. Proxy's mcp-oauth provider:
   - POST https://api.notion.com/v1/oauth/token (code → access_token)
   - Stores access_token + refresh_token in memory
   - Schedules auto-refresh before expiry
7. Future requests to mcp.notion.com:
   - Proxy MITMs, injects: Authorization: Bearer <fresh_token>
```

## Docker API Proxy Security

When the `docker-api-proxy` egress rule is present:

### Architecture

```
Agent container
  │ (transparent proxy allows connection to docker-proxy)
  ▼
Docker API Proxy (separate container on internal network)
  │ validates: no privileged, resource limits, etc.
  ▼
DinD container (actual Docker daemon)
  │ creates containers on internal network
  ▼
Agent-spawned containers (also on internal network, egress via proxy)
```

### Threat Model

| Attack | Mitigation |
|--------|-----------|
| `docker run --privileged` | Proxy rejects privileged flag |
| `docker run --network host` | Proxy forces internal network |
| Volume mount host paths | Proxy rejects host path binds |
| Recursive container spawning | Only agent container has proxy auth token |
| Resource exhaustion | Per-agent container count + resource limits |
| Escape via Docker socket | Agent never has Docker socket — goes through API proxy |

### Why Recursive Spawning Can't Happen

Agent-spawned containers are on the internal network. Their traffic goes through the transparent proxy too. But they don't have the auth token needed to talk to the Docker API Proxy — only the original agent container does.

## Network Topology

```
┌─────────────────────────────────────────────────────────────┐
│  Docker Host                                                 │
│                                                             │
│  ┌─ Internal Network (Docker internal: true) ─────────────┐  │
│  │  No direct internet access                             │  │
│  │                                                        │  │
│  │  ┌─ Agent Container ─────────────────────────────────┐ │  │
│  │  │  Agent + Channel + Gateway Proxy                  │ │  │
│  │  │  iptables: all TCP → proxy (transparent)          │ │  │
│  │  │  Proxy bridges to external network                │ │  │
│  │  └──────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Docker API Proxy + DinD (optional) ─────────────┐ │  │
│  │  │  Policy enforcement for container creation       │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Agent-spawned containers (optional) ────────────┐ │  │
│  │  │  Also on internal network                        │ │  │
│  │  │  Egress also through agent's proxy               │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Trust Boundaries

| Boundary | What Crosses It | Control |
|----------|----------------|---------|
| Container → Internet | All TCP traffic | Transparent proxy (iptables enforced) |
| Agent → Docker | Container API calls | Docker API Proxy (policy enforcement) |
| User → Agent | Chat messages | Channel provider (ACP protocol) |
| Agent → MCP servers | Tool calls | Egress rules + credential injection |
| .env → Proxy | Raw credentials | Read at startup, held in proxy memory only |
