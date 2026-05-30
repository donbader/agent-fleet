# agent-sandbox — Design Document

## Overview

An opinionated agent sandbox orchestrator that deploys AI coding agents inside secure Docker containers. Prioritizes minimal configuration while maintaining strong security boundaries.

**Philosophy:** One config file, one command. All infrastructure details hidden from the user.

---

## Requirements

1. User can choose supported runtime agent provider (codex, claude-code, pi, aider)
2. Agent sandbox enforced (transparent proxy, iptables — cannot be bypassed)
3. Minimize user configuration efforts
4. Allow user to customize packages and home directory
5. Allow agent to spin up Docker containers for development

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  agent-sandbox CLI (user's machine)                          │
│  - Reads agent.yaml                                         │
│  - Calls plugin.Contribute() for each enabled plugin        │
│  - Merges contributions → generates build artifacts         │
│  - Runs: docker compose up                                  │
└─────────────────────────────────────────────────────────────┘

┌─ Per-Agent Container ───────────────────────────────────────┐
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Gateway (universal Go binary, runs as `gateway` user)│    │
│  │  - iptables forces ALL TCP here (kernel enforced)    │    │
│  │  - MITM + credential injection for matched hosts     │    │
│  │  - Passthrough for everything else (allow-all)       │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Bridge (TypeScript, always the entrypoint)          │    │
│  │  - Spawns agent runtime as child process             │    │
│  │  - Loads channel plugins (telegram, slack, etc.)     │    │
│  │  - No channels → agent runs standalone               │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Agent Runtime (child of bridge)                      │    │
│  │  - codex | claude-code | pi | aider                  │    │
│  │  - Unaware of proxy, bridge, or channels             │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Plugin System

### Design

One plugin type. No categories. A plugin is a self-contained Go module that declares what it contributes to the container. The CLI merges all contributions and generates deployment artifacts.

### Interface

```go
package sdk

type Plugin interface {
    Name() string
    ConfigSchema() ConfigSchema
    Contribute(ctx ContributeContext) (*Contributions, error)
    NewInjector(cfg map[string]any) (Injector, error)  // nil if no runtime injection needed
}

type Injector interface {
    InjectCredentials(req *http.Request) error
}
```

`Contribute()` runs at build time (in the CLI). `Injector` runs at runtime (inside the gateway binary).

### Contributions (Grouped by Concern)

```go
type Contributions struct {
    Image      *ImageContribution
    Gateway    *GatewayContribution
    Bridge     *BridgeContribution
    Compose    *ComposeContribution
    Entrypoint *EntrypointContribution
}
```

Each sub-struct is nil if the plugin doesn't contribute to that concern.

```go
// What goes into the Dockerfile
type ImageContribution struct {
    BaseImage string       // only one plugin may set (conflict = error)
    Packages  Packages     // apt, npm, pip — merged across plugins
    Files     []File       // COPY into image (embed.FS source + dest path)
    Commands  []string     // RUN commands (no FROM/ENTRYPOINT allowed)
}

// What the gateway needs at runtime
type GatewayContribution struct {
    Rules    []EgressRule  // ordered within this plugin
    Priority int           // cross-plugin evaluation order (lower = first)
}

// Channel plugin for the bridge
type BridgeContribution struct {
    Name   string          // plugin name ("telegram", "slack")
    Source embed.FS        // TypeScript source to extract
    Config map[string]any  // runtime config passed to bridge
}

// Docker Compose service definition
type ComposeContribution struct {
    Services map[string]Service
    Volumes  []string
    Ports    []string
    Env      []EnvVar
}

type EnvVar struct {
    Key      string
    Value    string
    Strategy EnvStrategy  // Override | ErrorIfConflict | Append
}

// Scripts that run in entrypoint before agent starts
type EntrypointContribution struct {
    Hooks []Hook
}

type Hook struct {
    Name     string    // for logging: "[entrypoint] running: github-setup"
    Source   embed.FS  // script content
    Priority int       // execution order (lower = runs first)
}
```

### Why Grouped

- **Clear ownership**: each generator (Dockerfile, compose, gateway) only reads its own sub-struct
- **Explicit conflicts**: `EnvVar.Strategy` declares how to handle duplicates
- **Ordered execution**: `Hook.Priority` and `GatewayContribution.Priority` control cross-plugin ordering
- **Nil = not contributed**: plugin only fills what it needs, rest is nil

### Module Structure

```
plugins/<name>/
  go.mod              ← independent Go module
  plugin.go           ← implements sdk.Plugin
  plugin_test.go
  hooks/              ← entrypoint scripts (optional)
  bridge/             ← TypeScript channel code (optional)
```

### Registry (Compile-Time)

```go
// cmd/agent-sandbox/plugins.go
var Registry = []sdk.Plugin{
    codex.New(), claudecode.New(), pi.New(), aider.New(),  // runtimes
    github.New(), mcpoauth.New(), staticheader.New(),     // credentials
    docker.New(), telegram.New(), slack.New(),              // features/channels
}
```

All plugins compiled into one CLI binary and one gateway binary. No per-agent compilation. Runtime config determines which are active.

---

## Plugins

### Runtime Plugins

Runtime plugins set `BaseImage` and install the agent CLI. Auto-enabled by the `runtime:` field — user doesn't list them under `plugins:`.

```yaml
runtime: codex    # auto-enables codex plugin → sets BaseImage + installs @openai/codex
```

| Runtime | Base Image | Packages |
|---------|-----------|----------|
| `codex` | node:22-slim | git, curl, @openai/codex |
| `claude-code` | node:22-slim | git, curl, @anthropic-ai/claude-code |
| `pi` | node:22-slim | git, curl, pi-coding-agent |
| `aider` | python:3.12-slim | git, aider-chat |

For unsupported runtimes:
```yaml
plugins:
  custom-runtime:
    base_image: "python:3.12-slim"
    packages: ["git", "my-agent-cli"]
    cmd: "my-agent-cli"
```

### Credential Plugins

Declare egress rules + implement `Injector` for credential injection at the gateway.

| Plugin | Hosts | Injection |
|--------|-------|-----------|
| `github` | github.com, *.github.com | Header: `Authorization: token <PAT>` |
| `mcp-oauth` | user-defined MCP server URL | OAuth2 dynamic client registration + token refresh |
| `static-header` | user-defined endpoint | Static header injection (any API key) |

Note: LLM API credentials (OpenAI, Anthropic) are handled by the runtime itself (codex device flow, claude login). No dedicated plugins needed — the agent stores its own auth token in the home directory.

#### mcp-oauth plugin

Generic OAuth2 plugin for any MCP server. User provides the MCP URL, plugin handles:
1. Dynamic client registration (RFC 7591)
2. Authorization flow (redirect user to auth URL)
3. Token exchange (code → access_token + refresh_token)
4. Auto-refresh before expiry
5. Inject `Authorization: Bearer <token>` on matching requests

```yaml
plugins:
  mcp-oauth:
    servers:
      - url: "https://mcp.notion.com"
        name: "notion"
      - url: "https://mcp.linear.app"
        name: "linear"
```

The plugin auto-derives egress rules from the configured URLs. User triggers auth via channel command (`/oauth notion`).

### Channel Plugins

Contribute both egress rules (gateway side) AND bridge plugin code (channel side). One plugin, two contributions.

| Plugin | Egress | Bridge |
|--------|--------|--------|
| `telegram` | api.telegram.org → URL rewrite (bot token) | grammy bot, long-poll |
| `slack` | slack.com → OAuth token refresh | Slack socket mode |

### Feature Plugins

| Plugin | Contributes |
|--------|-------------|
| `docker` | DinD sidecar, docker CLI, DOCKER_HOST env, egress to DinD |

---

## User Configuration

### Single Agent

```
my-agent/
  agent.yaml          ← only config file
  home/               ← override home directory (optional)
  packages.sh         ← custom install script (optional)
  .env                ← secrets
```

```yaml
# agent.yaml
name: coder
runtime: codex

plugins:
  github:
    token: "${GITHUB_PAT}"
  docker: true
  telegram:
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    allowed_users: ["donbader"]

packages:
  - ripgrep
  - fd-find

home:
  persist: true
  override: ./home/
```

### Multi-Agent (Optional)

```yaml
# fleet.yaml
agents:
  - coder
  - reviewer

shared:
  plugins:
    github:
      token: "${GITHUB_PAT}"
```

Per-agent plugins **override** shared (same name → per-agent wins). Different plugins merge additively.

---

## Home Directory

| Mode | Config | Behavior |
|------|--------|----------|
| Ephemeral (default) | — | Home resets on restart. Auth token persists via small named volume. |
| Persistent | `home.persist: true` | Named volume at /home/agent. Runtime state survives. |
| Override | `home.override: ./home/` | Files staged to /opt/home-override/, cp'd on every start. |
| Combined | Both | Persistent + override always wins on start. |

Override mechanism uses `/opt/home-override/` staging (not in volume path). Entrypoint `cp -a` on every start ensures tracked configs always win over runtime state.

---

## Custom Packages

Declarative list (apt by default, specify type for others):
```yaml
packages:
  - ripgrep
  - name: typescript
    type: npm
```

For complex installs, add `packages.sh` (runs during docker build after declarative packages).

---

## Network & Security

### Egress Model

**Allow all by default.** MITM only for hosts where credential injection is needed. Everything else passes through with end-to-end TLS preserved.

Rationale: Dev agents need `npm install`, `pip install`, `curl` arbitrary URLs. Default deny creates too much friction.

### Transparent Proxy

```bash
# TCP: redirect to gateway (kernel enforced, agent cannot bypass)
iptables -t nat -A OUTPUT -p tcp -m owner ! --uid-owner gateway -j REDIRECT --to-port 8443
# UDP: only DNS allowed (to gateway's resolver), rest dropped
iptables -A OUTPUT -p udp --dport 53 -m owner ! --uid-owner gateway -j REDIRECT --to-port 8053
iptables -A OUTPUT -p udp -m owner ! --uid-owner gateway -j DROP
```

### Credential Flow

```
Agent → api.github.com:443
  → iptables redirects to gateway:8443
  → Gateway reads SNI: "api.github.com"
  → Matches github plugin rule → MITM mode
  → Terminates TLS (sandbox CA), reads HTTP request
  → Strips dummy token, injects real PAT
  → Opens TLS to real api.github.com, forwards
  → Agent receives response (thinks it talked directly)
```

Agent never sees real credentials. Bridge gets dummy tokens. Real creds exist only in gateway memory.

### Docker Access

When `docker: true`, the docker plugin contributes a DinD sidecar. Agent-spawned containers also route through the gateway:

1. Gateway listens on `0.0.0.0:8443` (network-accessible)
2. DinD injects iptables redirect wrapper into spawned containers
3. Spawned containers' traffic → agent container's IP:8443 → gateway
4. Spawned containers cannot spawn further containers (no Docker socket)

---

## Security Hardening

### Attack Surface

| Vector | Mitigation |
|--------|------------|
| Agent reads bridge env vars via /proc | Bridge only has dummy tokens. Real creds in gateway (different user, hidepid=2). |
| Agent kills gateway | Gateway runs as `gateway` user. Agent cannot signal it. |
| Agent modifies iptables | NET_ADMIN dropped after entrypoint. Agent has no capabilities. |
| Agent modifies gateway config | Config owned by root, mode 0444. |
| Agent ptraces bridge | hidepid=2 + different UIDs + no-new-privileges. |
| DNS tunneling | DNS redirected to gateway's resolver. No raw UDP allowed. |
| DinD direct access | DinD uses TLS client cert auth. Cert in gateway-owned path. |
| Resource exhaustion | mem_limit, cpus, pids_limit per container. |

### Container Security

```yaml
services:
  agent:
    cap_drop: [ALL]
    cap_add: [NET_ADMIN]        # entrypoint only, dropped after
    security_opt: [no-new-privileges:true]
    read_only: true
    tmpfs: [/tmp, /run]
    mem_limit: 4g
    cpus: 2
    pids_limit: 256
```

### File Permissions

```
/etc/gateway/config.yaml    root:root       0444
/etc/gateway/ca.key         gateway:gateway 0400
/usr/local/bin/gateway      root:root       0555
/home/agent/                agent:agent     0750 (writable, volume)
```

### Failure Modes

| Failure | Behavior |
|---------|----------|
| Gateway crashes | All TCP fails (safe default). Bridge detects, restarts or exits. |
| Bridge crashes | Agent dies (child). Docker restart policy recovers. |
| DinD crashes | Docker commands fail. Agent retries. |

---

## Build Flow

```
agent-sandbox up
  │
  ├── Detect mode: agent.yaml (single) or fleet.yaml (multi)
  ├── Auto-enable runtime plugin from runtime: field
  ├── Merge shared plugins (if fleet mode)
  ├── For each plugin: Contribute() → merge all Contributions
  │
  ├── Generate .build/:
  │     ├── gateway-src/        (go:embed → gateway Go source)
  │     ├── bridge/             (go:embed → bridge TypeScript)
  │     ├── bridge-plugins/     (from plugin embed.FS)
  │     ├── home-override/      (from user's home/ dir)
  │     ├── hooks/              (from plugins)
  │     ├── Dockerfile          (multi-stage: gateway compile + runtime)
  │     ├── gateway-config.yaml (merged egress rules)
  │     └── bridge-config.json  (channels + runtime to spawn)
  │
  ├── Generate docker-compose.yml + .env.example
  └── docker compose up -d --build
```

### Generated Dockerfile (Multi-Stage)

```dockerfile
# Stage 1: Compile gateway
FROM golang:1.24 AS gateway-builder
COPY gateway-src/ /src/
RUN cd /src && CGO_ENABLED=0 go build -o /gateway ./cmd/gateway

# Stage 2: Runtime
FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git curl ca-certificates iptables gosu docker.io ripgrep \
    && rm -rf /var/lib/apt/lists/*

RUN npm install -g @openai/codex

COPY bridge/ /opt/bridge/
RUN cd /opt/bridge && npm install --production
COPY bridge-plugins/telegram/ /opt/bridge/plugins/telegram/

COPY home-override/ /opt/home-override/
COPY packages.sh /tmp/packages.sh
RUN chmod +x /tmp/packages.sh && /tmp/packages.sh && rm /tmp/packages.sh

COPY --from=gateway-builder /gateway /usr/local/bin/gateway
COPY hooks/ /opt/entrypoint-hooks/

RUN useradd -m -s /bin/bash agent && useradd -r -s /usr/sbin/nologin gateway

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["node", "/opt/bridge/src/index.js"]
```

### Distribution

- **Gateway source**: embedded in CLI via `go:embed`. Extracted to `.build/gateway-src/`. Compiled in Docker multi-stage (user doesn't need Go).
- **Bridge source**: embedded in CLI via `go:embed`. Extracted to `.build/bridge/`.
- **Channel plugins**: embedded in each plugin's Go module via `go:embed`. Extracted to `.build/bridge-plugins/`.

---

## Multi-Agent Topology

```
┌─ Internal Network ──────────────────────────────────────────┐
│                                                              │
│  ┌─ coder ───────────────────────────────────────────────┐  │
│  │  Gateway (github + docker + telegram rules)            │  │
│  │  Bridge → codex                                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─ reviewer ────────────────────────────────────────────┐  │
│  │  Gateway (github + telegram rules)                     │  │
│  │  Bridge → claude-code                                  │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─ dind (shared) ───────────────────────────────────────┐  │
│  │  Docker daemon                                         │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

Each agent has its own gateway instance (same binary, different config). Agents share the network (can communicate). DinD shared if multiple agents need Docker.

---

## CLI

```bash
agent-sandbox init [--runtime codex]    # interactive scaffold
agent-sandbox up [agent-name]           # build + start
agent-sandbox down [agent-name]         # stop
agent-sandbox exec [agent-name] [cmd]   # shell (default: bash)
agent-sandbox logs [agent-name]         # stream logs
agent-sandbox status                    # health dashboard
agent-sandbox plugins                   # list available
agent-sandbox plugins info <name>       # plugin details
agent-sandbox validate                  # check config + suggest fixes
agent-sandbox upgrade                   # self-update
agent-sandbox up --dry-run              # preview without building
agent-sandbox up --debug                # verbose gateway + bridge logs
agent-sandbox generate                  # write .build/ without starting
agent-sandbox rebuild                   # rebuild image, keep container
agent-sandbox restart                   # restart container, keep image
```

---

## UX Design

### Progressive Disclosure

```yaml
# Minimal (works immediately):
name: coder
runtime: codex

# Add credentials:
plugins:
  github: { token: "${GITHUB_PAT}" }

# Add channels:
plugins:
  telegram: { bot_token: "${BOT_TOKEN}", allowed_users: ["me"] }

# Full power:
plugins:
  github: { token: "${GITHUB_PAT}" }
  docker: true
  telegram: { bot_token: "${BOT_TOKEN}", allowed_users: ["me"] }
packages: [ripgrep, fd-find]
home: { persist: true, override: ./home/ }
```

### Interactive Init

Auto-detects `gh auth token`, suggests plugins based on runtime, creates `.env` with detected credentials.

### Smart Validation

```bash
$ agent-sandbox validate
⚠ runtime 'codex' typically needs 'openai' plugin for API access.
✓ Config valid (1 warning)
```

### Helpful Errors

```bash
✗ Plugin 'github' failed: token is invalid or expired
  Fix: gh auth refresh && agent-sandbox up
```

---

## DX (Plugin Authors)

### Scaffold

```bash
$ agent-sandbox plugin new my-corp-api
Created plugins/my-corp-api/ (go.mod, plugin.go, plugin_test.go, README.md)
```

### Testing

```go
func TestContribute(t *testing.T) {
    p := New()
    contrib, err := p.Contribute(sdk.ContributeContext{
        AgentName: "test",
        Config:    map[string]any{"token": "ghp_test"},
    })
    require.NoError(t, err)
    assert.Equal(t, []string{"github.com", "*.github.com"}, contrib.EgressRules[0].Hosts)
}

func TestInjector(t *testing.T) {
    injector, _ := New().NewInjector(map[string]any{"token": "ghp_real"})
    req := httptest.NewRequest("GET", "https://api.github.com/repos", nil)
    injector.InjectCredentials(req)
    assert.Equal(t, "token ghp_real", req.Header.Get("Authorization"))
}
```

### Integration Test Helper

```go
sb := sdktest.NewTestSandbox(t, github.New(), telegram.New())
defer sb.Cleanup()
resp := sb.HTTPGet("https://api.github.com/user")
assert.Equal(t, 200, resp.StatusCode)
```

---

## Project Structure

```
agent-sandbox/
  go.work

  cmd/agent-sandbox/        ← CLI binary
    go.mod
    main.go
    cmd/                    ← cobra commands
    plugins.go              ← registry

  sdk/                      ← Plugin SDK
    go.mod
    plugin.go
    contributions.go

  gateway/                  ← Universal gateway binary
    go.mod
    cmd/gateway/main.go
    proxy.go, sni.go, mitm.go, injector_registry.go

  bridge/                   ← Bridge runtime (TypeScript)
    package.json
    src/index.ts, agent.ts, plugin-loader.ts, types.ts

  plugins/
    codex/      (go.mod, plugin.go)
    claude-code/(go.mod, plugin.go)
    github/     (go.mod, plugin.go, plugin_test.go)
    mcp-oauth/  (go.mod, plugin.go)
    static-header/ (go.mod, plugin.go)
    docker/     (go.mod, plugin.go)
    telegram/   (go.mod, plugin.go, bridge/src/telegram.ts)
    slack/      (go.mod, plugin.go, bridge/src/slack.ts)

  internal/
    compose/    ← docker-compose.yml generation
    dockerfile/ ← Dockerfile generation
    config/     ← agent.yaml + fleet.yaml parsing
    merge/      ← contribution merging + conflict detection

  templates/    ← entrypoint.sh, etc.
```

---

## Key Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Single plugin type | No artificial categorization. Plugin contributes what it needs. |
| 2 | Universal gateway binary | Build once, configure per-agent. No per-agent compilation. |
| 3 | Bridge always entrypoint | Agent is always a child process. No WrapCmd hack. |
| 4 | Runtime is a plugin (auto-enabled) | New runtime = new plugin. No CLI hardcoding. custom-runtime for unsupported. |
| 5 | Allow-all egress default | Dev agents need unrestricted installs. MITM only where needed. |
| 6 | Gateway inside each container | Self-contained. Per-agent config without routing complexity. |
| 7 | Compile-time plugin import | Single binary. Type-safe. No runtime discovery. |
| 8 | Home override via /opt staging | Volume hides /home/agent. Staging + entrypoint cp ensures configs win. |
| 9 | Channels are bridge sub-plugins | Messaging is bridge's concern. Plugin embeds TypeScript via go:embed. |
| 10 | All credentials through gateway | Even bridge gets dummy tokens. Real creds never in container env. |
| 11 | Gateway + bridge source via go:embed | CLI is self-contained. Docker multi-stage compiles gateway. No pre-built downloads. |
| 12 | Optional fleet.yaml | Single agent first-class. Multi-agent additive. |
| 13 | UDP restricted | DNS redirected to gateway resolver. All other UDP dropped. Prevents tunneling. |

---

## Comparison with agent-fleet

| Aspect | agent-fleet | agent-sandbox |
|--------|-------------|---------------|
| Config | fleet.yaml + agent.yaml | One agent.yaml (fleet optional) |
| Egress rules | User writes manually | Auto-derived from plugins |
| Runtime | Provider + render.sh | Plugin (auto-enabled by config) |
| Extensibility | Shell scripts | Go modules (declarative) |
| Home customization | user_base + init_scripts + Dockerfile | `home/` dir (auto-override) |
| Packages | Write Dockerfile template | YAML list or packages.sh |
| Docker access | Egress rule provider | `docker: true` |
| Deploy | generate → compose up | `up` |
| Egress default | Deny | Allow all |
| Gateway | Sidecar container | Inside agent container |

---

## Open Questions

1. **Plugin versioning** — SDK breaking changes? Version pinning?
2. **Custom egress restrictions** — Option for default-deny mode?
3. **External plugins** — Beyond built-in? Go module proxy?
4. **Resource limits** — CPU/memory per agent? Configurable?
5. **Health checks** — Detect agent crash vs idle?
6. **Auth flow** — Codex device flow, claude login — how to handle first-time auth?

---

## Architecture Review: Maintainability

### Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| CLI binary size (go:embed gateway + bridge) | ~50-80MB binary | Accept it. Single binary distribution is worth the size. Gateway source is small (~500KB). Bridge is ~2MB with node_modules excluded (npm install happens in Docker). |
| SDK interface changes break all plugins | All plugins need update | Semantic versioning on sdk module. Keep interface minimal. Add new capabilities via Contributions fields (additive), not interface methods. |
| Multi-stage build adds time | First build ~60s (Go compile) | Docker layer cache. Gateway stage rarely changes. Subsequent builds ~5s. |
| Two languages (Go + TypeScript) | Higher maintenance burden | Clear boundary: Go = build-time + network proxy. TypeScript = runtime messaging. No overlap. Each can be maintained independently. |
| Contributions struct grows (kitchen sink) | Complex merge logic | Keep fields flat and optional. Nil/empty = not contributed. Merge is simple: concatenate lists, error on conflicts (e.g., two BaseImages). |
| Gateway fix requires all containers rebuild | Slow rollout | `agent-sandbox rebuild` rebuilds image without config change. Docker cache means only gateway stage rebuilds (~10s). |
| Bridge overhead when no channels | Unnecessary process | Bridge in no-channel mode is trivial: spawn agent, wait. ~5MB RSS. Acceptable for architectural simplicity. |

### Interface Stability Strategy

```go
// sdk v1 — keep forever
type Plugin interface {
    Name() string
    ConfigSchema() ConfigSchema
    Contribute(ctx ContributeContext) (*Contributions, error)
    NewInjector(cfg map[string]any) (Injector, error)
}

// New capabilities → new Contributions fields (non-breaking)
type Contributions struct {
    BaseImage string  // v1.0
    Packages  Packages // v1.0
    // ...
    HealthCheck *HealthCheck // v1.3 — added later, nil = not used
}
```

Rule: never remove or rename fields. Only add. Plugins compiled against sdk v1.0 work with sdk v1.x.

### Upgrade Path

| Change | User action |
|--------|-------------|
| New plugin available | `agent-sandbox upgrade` (new CLI binary) |
| Plugin bug fix | `agent-sandbox upgrade` → `agent-sandbox rebuild` |
| Gateway security fix | `agent-sandbox upgrade` → `agent-sandbox rebuild` |
| Bridge fix | `agent-sandbox upgrade` → `agent-sandbox rebuild` |
| Config schema change | Edit agent.yaml → `agent-sandbox up` |
| New runtime | `agent-sandbox upgrade` (new plugin in registry) |

All upgrades are: upgrade CLI → rebuild containers. No migration scripts. No state to migrate (config is declarative, state is in volumes).

### Scaling Considerations

| Dimension | Current design | If it becomes a problem |
|-----------|---------------|-------------------------|
| Number of plugins | ~10-15 built-in | Fine. Go compile is fast. Binary stays <100MB. |
| Number of agents | 1-5 per host | Docker Compose handles this. Beyond 10 → consider k8s. |
| Plugin complexity | Simple Contribute() | If plugins need lifecycle hooks (start/stop/health), add optional interface. |
| External plugins | Not supported | Add go-plugin (HashiCorp) runtime discovery later. |
| Multi-host | Not supported | Out of scope. Use k8s + Helm chart if needed. |

### What NOT to Build (YAGNI)

- Plugin marketplace / registry (until >20 community plugins exist)
- Hot reload (rebuild is fast enough with Docker cache)
- Web UI dashboard (CLI + Telegram is sufficient)
- Multi-host orchestration (use k8s)
- Plugin dependency resolution (keep plugins independent)
- Config migration tool (config is simple YAML, manual edit is fine)
