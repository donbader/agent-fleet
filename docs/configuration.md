# Configuration Reference

## File Structure

```
my-fleet/
├── fleet.yaml          # Shared config (egress-presets + agent list)
├── .env                # Secrets (never committed)
├── .gitignore          # Should include .env
└── agents/
    └── <agent-name>/
        ├── agent.yaml  # Per-agent config (runtime, egress, channel)
        └── ...         # Optional: home-override, scripts, etc.
```

## fleet.yaml Schema

```yaml
fleet:
  name: <fleet-name>

agents:
  - <agent-name>         # References agents/<agent-name>/agent.yaml
  - <agent-name-2>

egress-presets:
  <preset-name>:
    - host: [...]                    # Domain match
      provider: "<provider-path>"    # Optional: handles injection/rewriting
      options: {}
    - endpoint: [...]                # Full URL match
      provider: "<provider-path>"
    - host: ["*"]                    # Catch-all (allow remaining traffic)
```

## agent.yaml Schema

```yaml
egress: [<preset-name>, ...]     # Ordered list of egress presets (first match wins)

runtime:
  provider: "<runtime-provider-path>"
  options: {}                    # Provider-specific options

env: {}                          # Non-secret env vars injected into sandbox
```

## Runtimes

### Runtime Providers

| Provider | Description | Use case |
|----------|-------------|----------|
| `.../runtimes/codex` | OpenAI Codex CLI | Headless agent (no messaging) |
| `.../runtimes/claude-code` | Anthropic Claude Code | Headless agent |
| `.../runtimes/pi` | Pi coding agent | Headless agent |
| `.../runtimes/channels-bridge` | Bridge + channels | Agent with messaging (Telegram, web-ui) |

### channels-bridge Options

```yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/channels-bridge"
  options:
    agent_provider: "<runtime-provider-path>"   # Which agent to spawn
    user_base: "./Dockerfile"       # Optional: custom image stage
    channels:
      - provider: "<channel-provider-path>"
        options: {}                             # Channel-specific options
```

### Image Customization (template injection)

When `user_base` is set, the provider reads your partial Dockerfile and injects it into the runtime's build. Your Dockerfile is a template — not a standalone image:

```dockerfile
# agents/coder/Dockerfile (partial template)
# Magic variables: ${AGENT_HOME}, ${AGENT_USER}

RUN apt-get update && apt-get install -y ripgrep fd-find jq
COPY home-override/.gitconfig ${AGENT_HOME}/.gitconfig
```

The provider substitutes magic variables and injects your content between its base setup and finalize steps. Build context is set to your agent directory, so `COPY` paths are relative to `agents/<name>/`.

See `docs/user-base.md` for full details.

Both stages cache independently — user dep changes don't rebuild provider, and vice versa.

### Egress Presets (Composable)

Each agent selects one or more egress presets. Rules are evaluated in order across presets (first preset's rules first, then second preset's, etc.). First match wins.

```yaml
agents:
  coder:
    egress: [telegram-bot-1, notion-mcp, main]
    # Evaluation order:
    # 1. telegram-bot-1 rules (Telegram API with bot token injection)
    # 2. notion-mcp rules (Notion MCP with OAuth injection)
    # 3. main rules (GitHub PAT + catch-all)
```

Put the preset with `host: ["*"]` catch-all **last** in the array.

### Channel Configuration

Each agent has its own channel (its own bot instance). Channel provider does NOT manage credentials — the proxy handles all credential injection (including bot tokens).

```yaml
agents:
  my-agent:
    channel:
      provider: "github.com/donbader/agent-fleet/channel-providers/telegram"
      options:
        allowed_users: ["@coreyortea"]
        groups:
          "-100987654321":
            allowed_users: ["*"]
            required_mention: true
```

The channel provider uses a dummy bot token. The transparent proxy intercepts requests to `api.telegram.org` and rewrites the URL with the real token.

#### Telegram Channel Options

| Option | Type | Description |
|--------|------|-------------|
| `allowed_users` | string[] | Telegram usernames allowed to interact |
| `groups` | map | Group chat configurations |
| `groups.<id>.allowed_users` | string[] | Users allowed in this group (`["*"]` = all) |
| `groups.<id>.required_mention` | bool | Must @mention the bot to trigger |

### Environment Variables

Non-secret env vars injected into the sandbox:

```yaml
agents:
  my-agent:
    env:
      GH_TOKEN: proxy_dummy_token    # Dummy token so gh CLI doesn't complain
      LOG_LEVEL: info
```

## Egress Presets

Presets are named, reusable sets of egress rules. Multiple agents can share the same preset.

### Rule Format

| Field | Description | Example |
|-------|-------------|---------|
| `host` | Domain(s) to match | `["api.github.com"]` or `["*"]` |
| `endpoint` | Full URL(s) to match | `[https://mcp.notion.com/mcp]` |
| `provider` | Egress rule provider (Go module path) | `"github.com/donbader/agent-fleet/egress-rules/github-pat"` |
| `options` | Provider-specific options (defined by provider's schema) | `{ token: "${GITHUB_PAT_TOKEN}" }` |

**Combinations:**
- `host:` only — allow traffic (passthrough, no processing)
- `host:` + `provider:` — match hosts, delegate to provider
- `endpoint:` + `provider:` — match URLs, delegate to provider
- `provider:` only — provider exposes its own internal endpoint (e.g., Docker API Proxy)
- `host: ["*"]` — catch-all, allow all remaining traffic

### Built-in Providers

| Provider | Behavior | Options Schema |
|----------|----------|---------------|
| `egress-rules/github-pat` | Injects `Authorization: token <pat>` | `{ token: "${...}" }` |
| `egress-rules/mcp-oauth` | OAuth2 flow + Bearer injection + auto-refresh | `{}` (tokens managed via /oauth command) |
| `egress-rules/telegram-bot` | Rewrites URL path with bot token | `{ token: "${...}" }` |
| `egress-rules/docker-api-proxy` | Exposes Docker API with policy enforcement | `{ max_containers, disk_quota, resources }` |
| `egress-rules/header-inject` | Generic header injection (config-based) | `{ headers: map[string]string }` |

### Provider Interface

Each provider defines its own options schema. The proxy delegates request handling to the provider — it doesn't need to know about injection strategies:

```go
type EgressRuleProvider interface {
    // What options this provider accepts
    OptionsSchema() Schema

    // Handle a matched request (inject, rewrite, proxy, etc.)
    HandleRequest(req *http.Request, opts Options) (*http.Request, error)
}
```

### Example Presets

```yaml
egress-presets:
  # Telegram bot token injection (URL rewrite)
  telegram-bot-1:
    - host: ["api.telegram.org"]
      provider: "github.com/donbader/agent-fleet/egress-rules/telegram-bot"
      options:
        token: "${TELEGRAM_BOT_TOKEN}"

  # Notion MCP OAuth
  notion-mcp:
    - endpoint: [https://mcp.notion.com/mcp]
      provider: "github.com/donbader/agent-fleet/egress-rules/mcp-oauth"

  # Common base: GitHub + allow-all
  main:
    - host: ["api.github.com", "github.com"]
      provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
      options:
        token: "${GITHUB_PAT_TOKEN}"
    - host: ["*"]

  # Docker access
  docker:
    - provider: "github.com/donbader/agent-fleet/egress-rules/docker-api-proxy"
      options:
        max_containers: 5
        disk_quota: "10Gi"
        resources:
          limits:
            memory: "2Gi"
            cpu: "2"
```

## .env File

Secrets referenced by `*_env` options in egress-presets:

```bash
# Telegram bot token (injected by proxy via URL rewrite)
TELEGRAM_BOT_TOKEN=123456:ABC-DEF-your-bot-token

# GitHub PAT (injected by proxy via header)
GITHUB_PAT_TOKEN=ghp_your-github-pat

# MCP OAuth uses dynamic client registration — no client ID/secret needed
```

## Multi-Session Behavior

Each chat/thread maps to a separate ACP session. One agent handles multiple concurrent conversations independently.

## Environment Variables (Auto-injected)

| Variable | Description |
|----------|-------------|
| `AGENT_FLEET_AGENT_NAME` | Name of this agent |
| `AGENT_FLEET_CHANNEL_SOCKET` | Path to ACP Unix socket |
| `AGENT_FLEET_DOCKER_HOST` | Docker API Proxy endpoint (if docker-api-proxy preset used) |
