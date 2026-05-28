# Security Model

## Principles

1. **Default deny** — No network access unless explicitly allowed
2. **Secrets never in sandbox** — Credentials exist only at the proxy boundary
3. **Least privilege** — Agent runs as unprivileged user with minimal capabilities
4. **Defense in depth** — Multiple overlapping isolation layers
5. **Controlled escalation** — Docker access is opt-in and policy-enforced

## Isolation Layers (provided by OpenShell)

```
┌─────────────────────────────────────────────────────────┐
│  Container (Docker/Podman/VM)                            │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │  Supervisor (root, manages sandbox)              │    │
│  │                                                 │    │
│  │  ┌─────────────────────────────────────────┐    │    │
│  │  │  Agent (unprivileged, restricted)        │    │    │
│  │  │                                         │    │    │
│  │  │  • Landlock: filesystem whitelist       │    │    │
│  │  │  • Seccomp: syscall filter              │    │    │
│  │  │  • Network NS: forced through proxy     │    │    │
│  │  │  • Cap-drop ALL: no capabilities        │    │    │
│  │  │  • Non-root user                        │    │    │
│  │  └─────────────────────────────────────────┘    │    │
│  │                                                 │    │
│  │  Policy Proxy (L7 inspection + credential inject)│    │
│  └─────────────────────────────────────────────────┘    │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## Egress Control

All outbound traffic from the agent goes through OpenShell's policy proxy:

```
Agent → Network Namespace → Policy Proxy → Internet
                              │
                              ├── Check: destination allowed by policy?
                              ├── Check: calling binary trusted?
                              ├── Check: L7 rules (method, path)?
                              ├── Inject: credentials from provider store
                              └── Block: if no rule matches (default deny)
```

### Egress Rule Format

Users define egress rules in `fleet.yaml`:

```yaml
egress:
  - host: api.github.com
    port: 443
    access: full              # GET, POST, PUT, DELETE, PATCH

  - host: api.openai.com
    port: 443
    access: full

  - host: "*.npmjs.org"
    port: 443
    access: read-only         # GET, HEAD, OPTIONS only

  - host: api.telegram.org
    port: 443
    access: full
    # Bridge needs this — auto-added when bridge type is telegram
```

These compile to OpenShell network policy YAML at deploy time.

### Access Levels

| Level | HTTP Methods Allowed |
|-------|---------------------|
| `full` | All methods |
| `read-only` | GET, HEAD, OPTIONS |
| `write-only` | POST, PUT, PATCH, DELETE |
| `custom` | Specify `methods: [GET, POST]` explicitly |

## Credential Management

### Flow

```
1. User puts secrets in .env file (local, never committed)
2. `agent-fleet up` reads .env and creates OpenShell providers
3. OpenShell stores credentials in gateway credential store
4. At runtime, proxy injects credentials into matching requests
5. Agent never sees raw credential values
```

### Provider Types

| Type | What It Does | Injection Method |
|------|-------------|-----------------|
| `generic` | Injects env var or header | Header rewrite on matching endpoint |
| `github-pat` | GitHub Personal Access Token | `Authorization: token <pat>` on github.com |
| `oauth` | OAuth2 with auto-refresh | `Authorization: Bearer <token>` with refresh |
| `api-key` | API key injection | Custom header (e.g., `X-API-Key`, `Authorization: Bearer`) |

### OAuth Flow (for MCP services like Notion, Jira)

```
1. User runs `agent-fleet auth notion`
2. CLI opens browser for OAuth consent
3. Callback receives tokens
4. Tokens stored as OpenShell provider with refresh config
5. Provider auto-refreshes tokens before expiry
6. Agent's MCP calls to Notion API get Bearer token injected
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

### Policy Enforcement

```yaml
docker:
  enabled: true
  allowed_images:
    - "node:20-*"
    - "python:3.12-*"
    - "golang:1.22-*"
    - "ubuntu:24.04"
  denied_options:
    - privileged
    - network=host
    - cap-add
    - pid=host
    - ipc=host
  resource_limits:
    memory: "2g"
    cpus: "2"
    pids: 256
  max_containers: 5
  network: inherit    # New containers join sandbox's internal network
```

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

Only the original sandbox agent has the authentication token to talk to the Docker API Proxy. Agent-spawned containers are plain Docker containers with no proxy access.

## Network Topology

```
┌─────────────────────────────────────────────────────────────┐
│  Host                                                        │
│                                                             │
│  ┌─ OpenShell Gateway ────────────────────────────────────┐  │
│  │  - Credential store                                    │  │
│  │  - Policy delivery                                     │  │
│  │  - Sandbox lifecycle                                   │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─ Internal Network (openshell bridge) ──────────────────┐  │
│  │                                                        │  │
│  │  ┌─ Sandbox ─────────────────────────────────────────┐ │  │
│  │  │  Agent + Bridge                                   │ │  │
│  │  │  All egress → Policy Proxy → Internet             │ │  │
│  │  └──────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Docker API Proxy (optional) ────────────────────┐ │  │
│  │  │  Listens on internal network                     │ │  │
│  │  │  Forwards to Docker daemon (host)                │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  │  ┌─ Agent-spawned containers ───────────────────────┐ │  │
│  │  │  Also on internal network                        │ │  │
│  │  │  Egress also goes through policy proxy           │ │  │
│  │  └─────────────────────────────────────────────────┘ │  │
│  │                                                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Trust Boundaries

| Boundary | What Crosses It | Control |
|----------|----------------|---------|
| Sandbox → Internet | HTTP requests | Policy proxy (L7 inspection) |
| Agent → Docker | Container API calls | Docker API Proxy (policy enforcement) |
| User → Agent | Chat messages | Bridge (ACP protocol) |
| Agent → MCP servers | Tool calls | Egress policy + credential injection |
| Host → Sandbox | Credentials | OpenShell provider injection (never raw) |
