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
│  - Merges contributions                                     │
│  - Generates: Dockerfile, docker-compose.yml, gateway config│
│  - Runs: docker compose up                                  │
└─────────────────────────────────────────────────────────────┘

┌─ Per-Agent Container ───────────────────────────────────────┐
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Gateway (universal Go binary)                       │    │
│  │  - Transparent proxy (iptables forces all TCP here)  │    │
│  │  - Evaluates egress rules from enabled plugins       │    │
│  │  - MITM + credential injection (Injector interface)  │    │
│  │  - Passthrough for non-injection traffic             │    │
│  │  - Default: allow all (not default deny)             │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Bridge (TypeScript, always the entrypoint)          │    │
│  │  - Spawns agent runtime as child process             │    │
│  │  - Loads channel plugins (telegram, slack, etc.)     │    │
│  │  - Routes messages between channels and agent        │    │
│  │  - No channels configured → agent runs standalone    │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Agent Runtime (child process of bridge)              │    │
│  │  - codex | claude-code | pi | aider                  │    │
│  │  - Unaware of proxy, bridge, or channels             │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
└──────────────────────────────────────────────────��──────────┘
```

---

## Plugin System

### Single Plugin Type

No categories (runtime/credential/feature). A plugin is just a Go module that returns what it contributes. The CLI merges all contributions and resolves conflicts.

### Plugin Interface

```go
// sdk/plugin.go
package sdk

type Plugin interface {
    // Identity
    Name() string
    ConfigSchema() ConfigSchema

    // Build-time: what does this plugin contribute to the container?
    Contribute(ctx ContributeContext) (*Contributions, error)

    // Runtime: credential injection logic (runs inside gateway binary)
    // Return nil if this plugin doesn't need runtime injection.
    NewInjector(cfg map[string]any) (Injector, error)
}

type Injector interface {
    // Called by gateway for each intercepted request matching this plugin's egress rules
    InjectCredentials(req *http.Request) error
}
```

### Contributions Struct

```go
type Contributions struct {
    // Container image
    BaseImage      string              // only one plugin may set this
    Packages       Packages            // apt, npm, pip packages to install
    DockerfilePre  []string            // Dockerfile lines before user customization
    DockerfilePost []string            // Dockerfile lines after user customization

    // Egress / Gateway
    EgressRules    []EgressRule        // hosts to match + injection behavior

    // Bridge / Channels
    BridgePlugin   *BridgePlugin       // TypeScript channel code to load in bridge

    // Compose topology
    Services       map[string]Service  // sidecar containers (e.g., DinD)
    Volumes        []string            // named volumes
    Ports          []string            // exposed ports
    Env            map[string]string   // environment variables

    // Entrypoint
    Hooks          []embed.FS          // scripts to run before agent starts

    // Embedded files
    EmbeddedFiles  []EmbeddedDir       // files to COPY into container
}
```

### Plugin Module Structure

Each plugin is a self-contained Go module:

```
plugins/<name>/
  go.mod                    ← independent Go module
  plugin.go                 ← implements sdk.Plugin
  plugin_test.go            ← unit tests
  hooks/                    ← entrypoint hook scripts (optional)
  bridge/                   ← TypeScript channel code (optional, for channel plugins)
    src/
    package.json
```

### Plugin Connection: Compile-Time Import

All plugins are imported in the CLI binary at compile time (Go workspace):

```go
// cmd/agent-sandbox/plugins.go
var Registry = []sdk.Plugin{
    github.New(),
    openai.New(),
    anthropic.New(),
    docker.New(),
    telegram.New(),
    slack.New(),
}
```

### Gateway: Universal Binary

All plugins' Injector implementations are compiled into a single gateway binary. At runtime, the gateway reads its config to determine which injectors to activate.

No per-agent compilation needed. Same binary, different config.

---

## Planned Plugins

| Plugin | Contributes | Runtime Logic |
|--------|-------------|---------------|
| `github` | Egress rules for github.com, env GH_TOKEN=dummy | Static header injection (PAT) |
| `openai` | Egress rules for api.openai.com | Static header injection (API key) |
| `anthropic` | Egress rules for api.anthropic.com | Static header injection (API key) |
| `docker` | DinD sidecar service, docker CLI package, egress to DinD | None (passthrough) |
| `telegram` | Egress for api.telegram.org, bridge channel plugin | URL rewrite (bot token) |
| `slack` | Egress for slack.com, bridge channel plugin | OAuth token refresh |
| `notion` | Egress for api.notion.com | OAuth token exchange + refresh |
| `custom-api` | User-defined host + header injection | Static header injection |

---

## Runtime Selection

Runtime is a config value that tells the bridge which agent CLI to spawn. But the runtime's container requirements (base image, packages) are defined by a **runtime plugin** — a regular plugin that sets `BaseImage`.

```yaml
runtime: codex          # bridge spawns: codex --args
runtime: claude-code    # bridge spawns: claude --args
runtime: pi             # bridge spawns: pi --args
```

Each runtime has a corresponding plugin that's **auto-enabled** based on the `runtime:` field:

```go
// plugins/codex/plugin.go — auto-enabled when runtime: codex
func (p *Plugin) Contribute(ctx sdk.ContributeContext) (*sdk.Contributions, error) {
    return &sdk.Contributions{
        BaseImage: "node:22-slim",
        Packages:  sdk.Packages{Apt: []string{"git", "curl"}, Npm: []string{"@openai/codex"}},
    }, nil
}
```

This means adding a new runtime = adding a new plugin. No CLI release needed.

| Runtime | Plugin | Base Image | Packages |
|---------|--------|-----------|----------|
| `codex` | `plugins/codex/` | node:22-slim | git, curl, @openai/codex |
| `claude-code` | `plugins/claude-code/` | node:22-slim | git, curl, @anthropic-ai/claude-code |
| `pi` | `plugins/pi/` | node:22-slim | git, curl, pi-coding-agent |
| `aider` | `plugins/aider/` | python:3.12-slim | git, aider-chat |

Runtime plugins are special only in that:
1. They set `BaseImage` (only one plugin can do this)
2. They're auto-enabled by the `runtime:` field (user doesn't list them under `plugins:`)

---

## User Configuration

### Single Agent

```
my-agent/
  agent.yaml          ← only config file
  home/               ← files to override in home directory (optional)
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
  - jq

home:
  persist: true         # named volume for home directory
  override: ./home/     # override on every start from this dir
```

### Multi-Agent (Optional Fleet File)

```
my-fleet/
  fleet.yaml
  agents/
    coder/
      agent.yaml
      home/
    reviewer/
      agent.yaml
  .env
```

```yaml
# fleet.yaml
agents:
  - coder
  - reviewer

shared:
  plugins:
    github:
      token: "${GITHUB_PAT}"
    openai:
      api_key: "${OPENAI_API_KEY}"
```

```yaml
# agents/coder/agent.yaml
runtime: codex
plugins:
  telegram:
    bot_token: "${CODER_BOT_TOKEN}"
    allowed_users: ["donbader"]
  docker: true
```

```yaml
# agents/reviewer/agent.yaml
runtime: claude-code
plugins:
  telegram:
    bot_token: "${REVIEWER_BOT_TOKEN}"
    allowed_users: ["donbader"]
```

### Plugin Config Merge Rules

- Per-agent plugins **override** shared plugins (same plugin name → per-agent wins)
- Different plugins → merged additively
- No complex merge logic needed

---

## Home Directory

### Default: Ephemeral

Container home resets on every restart. Only auth tokens persist (via small named volume for token path only).

### Persist: Named Volume

```yaml
home:
  persist: true
```

Adds a named volume at `/home/agent`. Runtime state survives restarts.

### Override: Tracked Configs

```yaml
home:
  override: ./home/
```

Files in `./home/` are:
1. Build time: COPYed to `/opt/home-override/` (staging, not in volume path)
2. Runtime: entrypoint `cp -a /opt/home-override/. /home/agent/` on every start

This ensures tracked configs always win over runtime state, even with a persistent volume.

### Combined (Recommended)

```yaml
home:
  persist: true       # runtime state persists
  override: ./home/   # tracked configs override on every start
```

---

## Custom Packages

### Declarative (Simple)

```yaml
packages:
  - ripgrep           # apt-get install
  - fd-find
  - name: typescript
    type: npm         # npm install -g
```

### Script (Complex)

```
my-agent/
  packages.sh         ← runs during docker build
```

```bash
#!/bin/bash
# Custom install logic
curl -fsSL https://example.com/install.sh | sh
pip install some-package
```

If `packages.sh` exists, it runs after declarative packages.

---

## Network & Security

### Egress Default: Allow All + Inject Where Needed

Unlike agent-fleet (default deny), agent-sandbox defaults to **allow all traffic**. MITM only happens for hosts where credential injection is needed.

Rationale: Dev agents need to `npm install`, `pip install`, `curl` arbitrary URLs. Default deny creates too much friction for the primary use case.

```
Agent connects to registry.npmjs.org
  → Gateway: no matching plugin rule → passthrough (end-to-end TLS)

Agent connects to api.github.com
  → Gateway: github plugin rule matches → MITM + inject PAT
```

### Transparent Proxy (iptables)

```bash
# Inside container (entrypoint, as root)
iptables -t nat -A OUTPUT -p tcp -m owner ! --uid-owner gateway -j REDIRECT --to-port 8443
```

- All TCP from agent/bridge → redirected to gateway
- Gateway's own connections → exempt (it reaches the internet)
- Agent cannot bypass (kernel enforced)

### Credential Injection

| Plugin | Mechanism |
|--------|-----------|
| github | MITM → inject `Authorization: token <PAT>` header |
| openai | MITM → inject `Authorization: Bearer <key>` header |
| telegram | MITM → URL path rewrite (`/botPLACEHOLDER/` → `/bot<REAL>/`) |
| notion | MITM → inject `Authorization: Bearer <token>` + auto-refresh |

Agent never sees real credentials. Even if agent reads bridge's env vars, it only finds dummy/placeholder values.

---

## Docker Access (docker plugin)

When `docker: true`:

```yaml
plugins:
  docker: true
```

The plugin contributes:
- DinD sidecar container (Docker daemon)
- `docker` CLI package in agent container
- `DOCKER_HOST=tcp://dind:2376` env var
- Egress rule allowing connection to DinD

### Agent-Spawned Container Egress

Agent-spawned containers also need egress through the gateway. Solution:

1. Gateway listens on `0.0.0.0:8443` (not just localhost) inside the agent container
2. Agent container exposes this port on the internal Docker network
3. Spawned containers' entrypoint configures iptables to redirect to agent container's IP:8443
4. The docker plugin contributes a "spawn template" — a minimal entrypoint script that DinD injects into spawned containers

```
Agent container (IP: 172.20.0.2)
  └── Gateway listening on 0.0.0.0:8443

Spawned container (IP: 172.20.0.5)
  └── iptables -t nat -A OUTPUT -p tcp -j DNAT --to 172.20.0.2:8443
```

The docker plugin's DinD sidecar is configured to:
- Force all spawned containers onto the internal network
- Inject the gateway redirect entrypoint wrapper
- Block `--privileged`, `--network host`, host volume mounts

Spawned containers cannot spawn further containers (no Docker socket access).

### Multi-Agent Docker Sharing

If multiple agents have `docker: true`, they share one DinD instance. Each agent's containers are isolated by Docker's built-in namespace (different container names/IDs). Each agent's spawned containers route through their own agent's gateway (different IPs).

---

## Multi-Agent Topology

```
┌─ Internal Network ──────────────────────────────────────────┐
│                                                              │
│  ┌─ coder ───────────────────────────────────────────────┐  │
│  │  Gateway (github + openai + docker + telegram rules)   │  │
│  │  Bridge → codex                                        │  │
│  │  iptables → gateway                                    │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─ reviewer ────────────────────────────────────────────┐  │
│  │  Gateway (github + anthropic + telegram rules)         │  │
│  │  Bridge → claude-code                                  │  │
│  │  iptables → gateway                                    │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─ dind (shared) ───────────────────────────────────────┐  │
│  │  Docker daemon                                         │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─ Agent-spawned containers ────────────────────────────┐  │
│  │  Also on internal network, egress via agent's gateway  │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

- Each agent has its own gateway instance (same universal binary, different config)
- Agents can communicate with each other (same network, no restriction)
- No special inter-agent protocol built-in
- DinD shared if multiple agents need Docker

---

## CLI Commands

```bash
# Single agent
agent-sandbox init                      # scaffold agent.yaml + home/
agent-sandbox init --runtime codex      # with specific runtime
agent-sandbox up                        # build + start
agent-sandbox down                      # stop + remove
agent-sandbox exec [cmd]                # shell into container (default: bash)
agent-sandbox logs                      # stream logs

# Multi-agent (when fleet.yaml exists)
agent-sandbox up                        # start all agents
agent-sandbox up coder                  # start specific agent
agent-sandbox down                      # stop all
agent-sandbox exec coder [cmd]          # shell into specific agent
agent-sandbox logs reviewer             # logs for specific agent

# Utility
agent-sandbox upgrade                   # self-update
agent-sandbox plugins                   # list available plugins
agent-sandbox validate                  # validate config
```

---

## Build Flow

```
agent-sandbox up
  │
  ├── Detect mode: agent.yaml (single) or fleet.yaml (multi)
  │
  ├── For each agent:
  │     ├── Load agent.yaml
  │     ├── Auto-enable runtime plugin (from runtime: field)
  │     ├── Merge shared plugins (if fleet mode)
  │     ├── For each plugin: call Contribute()
  │     ├── Merge all Contributions
  │     └── Validate (e.g., only one BaseImage, no conflicts)
  │
  ├── Generate build context (.build/):
  │     ├── gateway-src/ (extracted from go:embed — gateway Go source)
  │     ├── bridge/ (extracted from go:embed — bridge TypeScript source)
  │     ├── bridge-plugins/ (extracted from plugin embed.FS — channel TypeScript)
  │     ├── Dockerfile (multi-stage: gateway compile + runtime)
  │     ├── gateway-config.yaml (merged egress rules from all plugins)
  │     ├── bridge-config.json (which channel plugins to load, runtime to spawn)
  │     ├── home-override/ (from user's home/ dir)
  │     ├── hooks/ (entrypoint hooks from plugins)
  │     └── packages.sh (from user, if exists)
  │
  ├── Generate docker-compose.yml:
  │     ├── Agent service (build from .build/Dockerfile)
  │     ├── Sidecar services (from plugin contributions, e.g., DinD)
  │     ├── Networks (internal)
  │     └── Volumes (named volumes from plugins + user config)
  │
  ├── Generate .env.example (scan ${VAR} patterns)
  │
  └── docker compose up -d --build
```

---

## Generated Dockerfile (Example)

For an agent with: `runtime: codex`, `plugins: [github, telegram, docker]`, `packages: [ripgrep]`, `home.override: ./home/`

```dockerfile
# Stage 1: Build gateway from source
FROM golang:1.24 AS gateway-builder
COPY gateway-src/ /src/
RUN cd /src && CGO_ENABLED=0 go build -o /gateway ./cmd/gateway

# Stage 2: Agent runtime
FROM node:22-slim

# System packages (runtime + plugins + user)
RUN apt-get update && apt-get install -y --no-install-recommends \
    git curl ca-certificates iptables gosu \
    docker.io \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

# Runtime agent CLI
RUN npm install -g @openai/codex

# Bridge (always present)
COPY bridge/ /opt/bridge/
RUN cd /opt/bridge && npm install --production

# Channel plugins (from telegram plugin)
COPY bridge-plugins/telegram/ /opt/bridge/plugins/telegram/

# User home override
COPY home-override/ /opt/home-override/

# User packages.sh (if exists)
COPY packages.sh /tmp/packages.sh
RUN chmod +x /tmp/packages.sh && /tmp/packages.sh && rm /tmp/packages.sh

# Gateway binary (built in stage 1)
COPY --from=gateway-builder /gateway /usr/local/bin/gateway

# Entrypoint hooks (from plugins)
COPY hooks/ /opt/entrypoint-hooks/

# Users: agent (unprivileged) + gateway (proxy process)
RUN useradd -m -s /bin/bash agent && useradd -r -s /usr/sbin/nologin gateway

# Entrypoint
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["node", "/opt/bridge/src/index.js"]
```

---

## Entrypoint Flow

```bash
#!/bin/sh
set -eu

# 1. iptables redirect (transparent proxy)
iptables -t nat -A OUTPUT -p tcp -m owner ! --uid-owner gateway -j REDIRECT --to-port 8443

# 2. Start gateway (background, as gateway user)
su -c "gateway --config /etc/gateway/config.yaml" gateway &

# 3. Home override (if /opt/home-override exists)
if [ -d /opt/home-override ]; then
    cp -a /opt/home-override/. /home/agent/
    chown -R agent:agent /home/agent
fi

# 4. Run entrypoint hooks (from plugins, as root)
for hook in /opt/entrypoint-hooks/*.sh; do
    [ -f "$hook" ] && . "$hook"
done

# 5. Drop to agent user, run bridge
exec gosu agent "$@"
```

---

## Project Structure

```
agent-sandbox/
  go.work                           ← Go workspace (multi-module)

  cmd/agent-sandbox/                ← CLI binary
    go.mod
    main.go
    cmd/
      root.go
      up.go
      down.go
      exec.go
      logs.go
      init.go
      validate.go
      upgrade.go
    plugins.go                      ← plugin registry (imports all plugins)

  sdk/                              ← Plugin SDK (shared types + interface)
    go.mod
    plugin.go                       ← Plugin interface
    contributions.go                ← Contributions struct
    config.go                       ← ConfigSchema types
    egress.go                       ← EgressRule, Injector types
    bridge.go                       ← BridgePlugin type

  gateway/                          ← Universal gateway binary
    go.mod
    cmd/gateway/main.go
    proxy.go                        ← transparent proxy (iptables listener)
    sni.go                          ← TLS SNI extraction
    mitm.go                         ← MITM + credential injection
    injector_registry.go            ← loads injectors from config

  bridge/                           ← Bridge runtime (TypeScript)
    package.json
    src/
      index.ts                      ← main: spawn agent, load channel plugins
      agent.ts                      ← child process management
      plugin-loader.ts              ← dynamic channel plugin loading
      types.ts                      ← ACP protocol types

  plugins/
    github/
      go.mod
      plugin.go
      plugin_test.go
    openai/
      go.mod
      plugin.go
    anthropic/
      go.mod
      plugin.go
    docker/
      go.mod
      plugin.go
    telegram/
      go.mod
      plugin.go
      bridge/                       ← TypeScript channel plugin
        src/telegram.ts
        package.json
    slack/
      go.mod
      plugin.go
      bridge/
        src/slack.ts
        package.json

  internal/
    compose/                        ← docker-compose.yml generation
    dockerfile/                     ← Dockerfile generation
    config/                         ← agent.yaml + fleet.yaml parsing
    merge/                          ← contribution merging + conflict detection

  templates/                        ← entrypoint.sh, bridge base, etc.
```

---

## Key Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Single plugin type | Simpler mental model. Plugin contributes what it needs, no artificial categorization. |
| 2 | Universal gateway binary | Build once, configure per-agent. Avoids per-agent Go compilation. |
| 3 | Bridge always entrypoint | No WrapCmd hack. Bridge is the orchestrator, agent is always a child. |
| 4 | Runtime is a plugin (auto-enabled) | Avoids hardcoded mappings. New runtime = new plugin. `custom-runtime` plugin for unsupported agents. |
| 5 | Allow-all egress default | Dev agents need unrestricted package installs. MITM only where injection needed. |
| 6 | Gateway inside each container | Self-contained. No cross-agent credential leakage. Per-agent config without routing complexity. |
| 7 | Compile-time plugin import | Single binary distribution. Type-safe. No runtime discovery overhead. |
| 8 | Home override via /opt staging | Named volume hides image's /home/agent. Staging + entrypoint cp ensures tracked configs always win. |
| 9 | Channel plugins are bridge sub-plugins | Channels are messaging concern, not sandbox concern. Bridge owns channel lifecycle. |
| 10 | Credential injection via gateway | Even bridge gets dummy tokens. Agent in same container can't steal real credentials. |
| 11 | Per-agent override shared plugins | Simple precedence: per-agent wins. No complex merge. |
| 12 | Agents share network, no special protocol | Can communicate if needed, but no built-in inter-agent tooling. |
| 13 | Optional fleet.yaml for multi-agent | Single agent is first-class. Multi-agent is additive, not required. |
| 14 | Declarative packages + optional script | Simple cases: YAML list. Complex cases: packages.sh. |

---

## Comparison with agent-fleet

| Aspect | agent-fleet | agent-sandbox |
|--------|-------------|---------------|
| Config files | fleet.yaml + agent.yaml | One agent.yaml (fleet.yaml optional) |
| Egress rules | User writes manually | Auto-derived from plugins |
| Runtime | Provider with render.sh | Config value (name) |
| Plugins | Shell scripts (render.sh) | Go modules (declarative contributions) |
| Home customization | user_base + init_scripts + Dockerfile | `home/` dir (auto-override) |
| Package install | Write Dockerfile template | `packages:` list or `packages.sh` |
| Docker access | Egress rule provider | `docker: true` |
| Deploy workflow | generate → compose up (2 steps) | `up` (1 step) |
| Egress default | Default deny | Allow all |
| Gateway | Separate container (sidecar) | Inside agent container |
| Multi-agent | Built-in (fleet-first) | Optional (single-agent-first) |
| Extensibility | Remote providers (git clone) | Compiled plugins (Go modules) |

---

## Resolved Design Questions

### Q1: How do agent-spawned containers route through the gateway?

Gateway listens on `0.0.0.0:8443` (network-accessible, not just localhost). The docker plugin configures DinD to inject an iptables redirect wrapper into spawned containers, pointing to the agent container's IP on the internal network.

### Q2: Where does the gateway binary come from?

Docker multi-stage build. The gateway source code is extracted to the build context by the CLI (from `go:embed`). Stage 1 compiles it with `golang:1.24`. Stage 2 copies the binary. No pre-built downloads needed.

### Q3: Where does the bridge TypeScript code come from?

Embedded in the CLI binary via `go:embed`. During `agent-sandbox up`, the CLI extracts the bridge source to `.build/bridge/`. Same for channel plugin TypeScript from each plugin's embedded FS.

### Q4: How to add new runtimes without CLI release?

Runtimes ARE plugins (they set `BaseImage`). Adding a new runtime = adding a new plugin to the registry + rebuild CLI. However, since all plugins are compiled in, this does require a CLI release. This is acceptable because new runtimes are rare events (new agent CLIs don't appear weekly).

For users who need a custom runtime before it's officially supported, they can use a `custom-runtime` plugin:
```yaml
plugins:
  custom-runtime:
    base_image: "python:3.12-slim"
    packages: ["git", "my-agent-cli"]
    cmd: "my-agent-cli"
```

### Q5: Gateway user in Dockerfile?

The generated Dockerfile creates both users: `agent` (unprivileged, runs bridge + agent) and `gateway` (system user, runs proxy). The iptables rule exempts `gateway` uid so the proxy's own outbound connections aren't redirected.

---

## Open Questions

1. **Plugin versioning** — How to handle breaking changes in plugin interface? SDK version pinning?
2. **Custom egress restrictions** — Some users may want default-deny. Add `egress: deny-all` option?
3. **Plugin marketplace** — External plugins beyond built-in? Go module proxy? Git clone?
4. **Resource limits** — CPU/memory limits per agent? Per-agent or fleet-level?
5. **Health checks** — How to detect agent crash vs idle? Bridge responsibility?
6. **Logging** — Structured logs? Log aggregation for multi-agent?
7. **Auth flow** — How does user authenticate agent runtimes (codex device flow, claude login)?
