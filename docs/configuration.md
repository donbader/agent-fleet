# Configuration Reference

## File Structure

```
my-fleet/
├── fleet.yaml          # Main configuration
├── .env                # Secrets (never committed)
└── .gitignore          # Should include .env
```

## fleet.yaml Schema

```yaml
fleet:
  name: <fleet-name>             # Fleet identifier

agents:
  <agent-name>:
    runtime: codex | claude-code | pi
    gateway: <gateway-name>      # Which gateway this agent uses for egress
    channel:                     # Messaging channel (one per agent, each agent = own bot)
      provider: "<provider-path>"
      options: {}                # Provider-specific options
    env: {}                      # Non-secret env vars injected into sandbox

gateways:
  <gateway-name>:
    egress: []                   # Ordered egress rules (default deny, first match wins)
```

## Agents

### Runtime Options

| Runtime | Description | Protocol | Headless Mode |
|---------|-------------|----------|---------------|
| `codex` | OpenAI Codex CLI | ACP native | `codex --acp` |
| `claude-code` | Anthropic Claude Code | ACP via adapter | `claude -p` + stream-json |
| `pi` | Pi coding agent | Pi RPC via adapter | `pi --mode rpc` |

### Channel Configuration

Each agent has its own channel (its own bot instance). Channels handle messaging platform communication.

```yaml
agents:
  my-agent:
    channel:
      provider: "github.com/donbader/agent-fleet/channel-providers/telegram"
      options:
        bot_token_env: TELEGRAM_BOT_TOKEN
        allowed_users: ["@coreyortea"]
        groups:
          "-100987654321":
            allowed_users: ["*"]
            required_mention: true
```

#### Telegram Channel Options

| Option | Type | Description |
|--------|------|-------------|
| `bot_token_env` | string | Env var name containing the bot token |
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

Note: `GH_TOKEN: proxy_dummy_token` is a pattern where the agent sees a dummy value, but the gateway's egress rule provider injects the real PAT at the network boundary.

## Gateways

Gateways are named egress proxies. Multiple agents can share a gateway.

### Egress Rules

Rules are evaluated in order. First match wins. **Default deny** — if no rule matches, the request is blocked.

```yaml
gateways:
  gw-main:
    egress:
      # Rule 1: GitHub with PAT injection
      - host: ["api.github.com", "github.com"]
        provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
        options:
          token_env: GITHUB_PAT_TOKEN

      # Rule 2: MCP OAuth endpoints
      - endpoint: [https://mcp.notion.com/mcp, https://mcp.atlassian.com/v1/mcp]
        provider: "github.com/donbader/agent-fleet/egress-rules/mcp-oauth"

      # Rule 3: Docker API Proxy (no host — exposes internal endpoint)
      - provider: "github.com/donbader/agent-fleet/egress-rules/docker-api-proxy"
        options:
          max_containers: 5
          disk_quota: "10Gi"
          resources:
            limits:
              memory: "2Gi"
              cpu: "2"

      # Rule 4: Allow all other traffic (no provider = passthrough)
      - host: ["*"]
```

### Rule Format

Each rule can have `host:`, `endpoint:`, and/or `provider:`:

| Field | Description | Example |
|-------|-------------|---------|
| `host` | Domain(s) to match | `["api.github.com"]` or `["*"]` |
| `endpoint` | Full URL(s) to match | `[https://mcp.notion.com/mcp]` |
| `provider` | Egress rule provider (Go module path) | `"github.com/donbader/agent-fleet/egress-rules/github-pat"` |
| `options` | Provider-specific options | `{ token_env: GITHUB_PAT_TOKEN }` |

**Combinations:**
- `host:` only — allow traffic to these hosts (passthrough, no processing)
- `host:` + `provider:` — match these hosts, apply provider logic (e.g., inject credentials)
- `endpoint:` + `provider:` — match these URLs, apply provider logic
- `provider:` only — provider exposes its own internal endpoint (e.g., Docker API Proxy)
- `host: ["*"]` — catch-all, allow all remaining traffic

### Shared vs Separate Gateways

```yaml
# Shared gateway — both agents use same egress rules
agents:
  frontend:
    gateway: gw-shared
  backend:
    gateway: gw-shared

# Separate gateways — different egress policies
agents:
  frontend:
    gateway: gw-frontend
  backend:
    gateway: gw-backend

gateways:
  gw-frontend:
    egress:
      - host: ["registry.npmjs.org"]
      - host: ["*"]
  gw-backend:
    egress:
      - provider: "github.com/donbader/agent-fleet/egress-rules/docker-api-proxy"
        options: { max_containers: 3 }
      - host: ["*"]
```

## Egress Rule Providers

All providers are under the `egress-rules/` namespace. Each implements a specific behavior when traffic matches.

### Built-in Providers

| Provider | Behavior |
|----------|----------|
| `egress-rules/github-pat` | Injects `Authorization: token <pat>` |
| `egress-rules/mcp-oauth` | OAuth2 flow + Bearer token injection + auto-refresh |
| `egress-rules/mcp-token` | App credential injection |
| `egress-rules/docker-api-proxy` | Exposes controlled Docker API to sandbox |
| `egress-rules/api-key` | Generic header injection |

### github-pat

```yaml
- host: ["api.github.com", "github.com"]
  provider: "github.com/donbader/agent-fleet/egress-rules/github-pat"
  options:
    token_env: GITHUB_PAT_TOKEN    # Env var in .env file
```

Injects `Authorization: token <value>` on matching requests.

### mcp-oauth

```yaml
- endpoint: [https://mcp.notion.com/mcp, https://mcp.atlassian.com/v1/mcp]
  provider: "github.com/donbader/agent-fleet/egress-rules/mcp-oauth"
```

Handles OAuth2 flow for MCP services. Token management:
1. User sends `/oauth notion` in chat
2. Bot responds with authorization URL
3. User clicks, authorizes, pastes callback URL
4. Gateway exchanges code for token, stores it
5. Auto-refreshes before expiry
6. Injects `Authorization: Bearer <token>` on matching requests

### mcp-token

```yaml
- endpoint: [https://mcp.slack.com/mcp]
  provider: "github.com/donbader/agent-fleet/egress-rules/mcp-token"
  options:
    app_id_env: SLACK_APP_ID
    app_secret_env: SLACK_APP_SECRET
```

For MCP services that use app credentials instead of OAuth.

### docker-api-proxy

```yaml
- provider: "github.com/donbader/agent-fleet/egress-rules/docker-api-proxy"
  options:
    max_containers: 5
    disk_quota: "10Gi"
    resources:
      limits:
        memory: "2Gi"
        cpu: "2"
```

Exposes a Docker API endpoint inside the sandbox. No `host:` needed — the provider creates its own internal endpoint and sets `DOCKER_HOST` in the sandbox.

### api-key

```yaml
- host: ["*.datadoghq.com"]
  provider: "github.com/donbader/agent-fleet/egress-rules/api-key"
  options:
    key_env: DD_API_KEY
    header: DD-API-KEY
```

Generic API key injection via custom header.

## .env File

Secrets referenced by `*_env` options in fleet.yaml:

```bash
# Agent channel
TELEGRAM_BOT_TOKEN=123456:ABC-DEF-your-bot-token

# GitHub
GITHUB_PAT_TOKEN=ghp_your-github-pat

# MCP OAuth (for /oauth command flow)
NOTION_CLIENT_ID=your-notion-client-id
NOTION_CLIENT_SECRET=your-notion-client-secret
JIRA_CLIENT_ID=your-jira-client-id
JIRA_CLIENT_SECRET=your-jira-client-secret

# MCP Token
SLACK_APP_ID=your-slack-app-id
SLACK_APP_SECRET=your-slack-app-secret

# Generic API keys
DD_API_KEY=your-datadog-api-key
```

## Multi-Session Behavior

Each chat/thread maps to a separate ACP session. One agent handles multiple concurrent conversations independently:

```
Telegram DM from @alice  → Session "alice-001"
Telegram DM from @bob    → Session "bob-001"
Group chat mention        → Session "group-987654321"
```

## Environment Variables (Auto-injected)

These are set automatically by agent-fleet inside the sandbox:

| Variable | Description |
|----------|-------------|
| `AGENT_FLEET_AGENT_NAME` | Name of this agent |
| `AGENT_FLEET_GATEWAY` | Gateway name this agent uses |
| `AGENT_FLEET_CHANNEL_SOCKET` | Path to ACP Unix socket for channel communication |
| `AGENT_FLEET_DOCKER_HOST` | Docker API Proxy endpoint (if docker-api-proxy rule present) |
