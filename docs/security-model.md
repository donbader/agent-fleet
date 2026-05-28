# Security Model

## Principles

1. **Default deny** — No network access unless an egress rule matches
2. **Secrets never in sandbox** — Credentials exist only in gateway auth providers
3. **Least privilege** — Agent runs as unprivileged user with minimal capabilities
4. **Defense in depth** — Multiple overlapping isolation layers
5. **Channel owns its own credentials** — Bot tokens are channel provider's responsibility, not gateway's
6. **Controlled escalation** — Docker access is opt-in and policy-enforced

## Isolation Layers (provided by OpenShell)

```
┌─────────────────────────────────────────────────────────┐
│  Container (Docker/Podman/VM)                            │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Supervisor (root, manages sandbox)              │    │
│  │                                                 │    │
│  │  ┌─────────────────────────────────────────┐    │    │
│  │  │  Agent + Channel (unprivileged)          │    │    │
│  │  │                                         │    │    │
│  │  │  • Landlock: filesystem whitelist       │    │    │
│  │  │  • Seccomp: syscall filter              │    │    │
│  │  │  • Network NS: forced through gateway   │    │    │
│  │  │  • Cap-drop ALL: no capabilities        │    │    │
│  │  │  • Non-root user                        │    │    │
│  │  └─────────────────────────────────────────┘    │    │
│  │                                                 │    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Egress Control

All outbound traffic from the sandbox goes through the gateway proxy:

```
Agent/Channel → Network Namespace → Gateway Proxy → Internet
                                      │
                                      ├── Evaluate egress rules (first match wins)
                                      ├── No match? → DENY (default deny)
                                      ├── Match without auth? → ALLOW (passthrough)
                                      └── Match with auth? → Inject credentials → ALLOW
```

### Egress Rule Evaluation

```yaml
gateways:
  gw-main:
    egress:
      # Rule 1: checked first
      - host: ["api.github.com", "github.com"]
        auth:
          provider: "github.com/donbader/agent-fleet/auth-providers/github-pat"
          options:
            token_env: GITHUB_PAT_TOKEN

      # Rule 2: checked second
      - endpoint: [https://mcp.notion.com/mcp]
        auth:
          provider: "github.com/donbader/agent-fleet/auth-providers/mcp-oauth"

      # Rule 3: catch-all (allow everything else without auth)
      - host: ["*"]
```

**Evaluation order:**
1. Request to `api.github.com` → matches Rule 1 → inject PAT → allow
2. Request to `mcp.notion.com/mcp` → matches Rule 2 → inject OAuth token → allow
3. Request to `registry.npmjs.org` → matches Rule 3 → allow (no auth)
4. If no `- host: ["*"]` at end → request to `registry.npmjs.org` → DENIED

### Strict Mode vs Permissive Mode

```yaml
# Strict: only listed hosts are reachable
gateways:
  strict-gw:
    egress:
      - host: ["api.github.com"]
        auth: ...
      # No catch-all → everything else is denied

# Permissive: auth injection where needed, allow everything else
gateways:
  permissive-gw:
    egress:
      - host: ["api.github.com"]
        auth: ...
      - host: ["*"]              # Allow all other traffic
```

## Credential Management

### Separation of Concerns

| Who | Manages What | How |
|-----|-------------|-----|
| **Channel provider** | Messaging platform credentials (bot token) | Reads from env, uses directly in API calls |
| **Gateway auth provider** | Third-party API credentials (PAT, OAuth tokens) | Injects at L7 proxy boundary |
| **Agent** | Nothing | Sees dummy tokens or no tokens at all |

### Why the Agent Never Sees Real Credentials

```
Agent env:
  GH_TOKEN=proxy_dummy_token     ← dummy (so gh CLI doesn't error)

Agent makes request:
  GET https://api.github.com/repos/...
  Authorization: token proxy_dummy_token

Gateway intercepts:
  - Strips: Authorization: token proxy_dummy_token
  - Injects: Authorization: token ghp_REAL_TOKEN_HERE

Forwarded to GitHub:
  GET https://api.github.com/repos/...
  Authorization: token ghp_REAL_TOKEN_HERE
```

### OAuth Flow (MCP Services)

```
1. User: /oauth notion
2. Channel: "Click to authorize: https://api.notion.com/v1/oauth/authorize?..."
3. User: [clicks, authorizes in browser]
4. User: /oauth callback https://redirect.example.com?code=abc123
5. Channel → Gateway: exchange code for tokens
6. Gateway mcp-oauth provider:
   - POST https://api.notion.com/v1/oauth/token (code → access_token)
   - Stores access_token + refresh_token
   - Schedules auto-refresh before expiry
7. Future requests to mcp.notion.com:
   - Gateway injects: Authorization: Bearer <fresh_token>
```

## Docker API Proxy Security

When `docker.enabled: true`, a Docker API Proxy runs alongside the sandbox:

### Threat Model

| Attack | Mitigation |
|--------|-----------|
| `docker run --privileged` | Proxy rejects privileged flag |
| `docker run --network host` | Proxy forces `--network internal` |
| Volume mount host paths | Proxy rejects host path binds |
| Recursive container spawning | Only sandbox agent has proxy auth token |
| Image supply chain attack | Image allowlist enforcement |
| Resource exhaustion | Per-agent container count + resource limits |
| Credential theft via Docker inspect | New containers don't get provider credentials |

### Why Recursive Spawning Can't Happen

```
Agent (in OpenShell sandbox)
  │ has auth token for Docker API Proxy
  │
  ├── spawns Container A
  │     - On internal network
  │     - NO auth token for Docker API Proxy
  │     - ❌ Cannot create new containers
  │
  └── spawns Container B
        - Same restrictions
        - ❌ Cannot create new containers
```

Only the original sandbox agent has the authentication token to talk to the Docker API Proxy.

## Network Topology

```
┌─────────────────────────────────────────────────────────────┐
│  Host                                                        │
│                                                             │
│  ┌─ OpenShell Gateway ────────────────────────────────────┐  │
│  │  - Sandbox lifecycle                                   │  │
│  │  - Policy delivery                                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─ Internal Network ─────────────────────────────────────┐  │
│  │                                                        │  │
│  │  ┌─ Sandbox (Agent + Channel) ───────────────────────┐ │  │
│  │  │  All egress → Gateway Proxy                       │ │  │
│  │  │  Channel talks to Telegram (bot token in request) │ │  │
│  │  └──────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Gateway Proxy (gw-main) ────────────────────────┐ │  │
│  │  │  Egress rules + auth injection                   │ │  │
│  │  │  OAuth token storage                             │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Docker API Proxy (optional) ────────────────────┐ │  │
│  │  │  Policy enforcement for container creation       │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Agent-spawned containers ───────────────────────┐ │  │
│  │  │  Also on internal network                        │ │  │
│  │  │  Egress also goes through gateway proxy          │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Trust Boundaries

| Boundary | What Crosses It | Control |
|----------|----------------|---------|
| Sandbox → Internet | HTTP requests | Gateway proxy (egress rules + auth injection) |
| Agent → Docker | Container API calls | Docker API Proxy (policy enforcement) |
| User → Agent | Chat messages | Channel provider (ACP protocol) |
| Agent → MCP servers | Tool calls | Gateway egress rules + mcp-oauth provider |
| .env → Gateway | Raw credentials | Read at startup, never forwarded to sandbox |
