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
    docker:                      # Optional Docker API Proxy
      enabled: false
      allowed_images: []
      max_containers: 5
      resource_limits: {}
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

#### Channel Provider Interface

Channel providers must implement:
- Connect to messaging platform
- Receive messages from allowed users/groups
- Route messages to agent via ACP (one session per chat/thread)
- Forward agent responses back to platform
- Handle OAuth commands (`/oauth <provider>`)

#### Telegram Channel Options

| Option | Type | Description |
|--------|------|-------------|
| `bot_token_env` | string | Env var name containing the bot token |
| `allowed_users` | string[] | Telegram usernames allowed to interact |
| `groups` | map | Group chat configurations |
| `groups.<id>.allowed_users` | string[] | Users allowed in this group (`["*"]` = all) |
| `groups.<id>.required_mention` | bool | Must @mention the bot to trigger |

### Docker Configuration (Optional)

```yaml
agents:
  my-agent:
    docker:
      enabled: true
      allowed_images:
        - "node:20-*"
        - "python:3.12-*"
        - "postgres:16-*"
      max_containers: 5
      resource_limits:
        memory: "2g"
        cpus: "2"
        pids: 256
```

### Environment Variables

Non-secret env vars injected into the sandbox:

```yaml
agents:
  my-agent:
    env:
      GH_TOKEN: proxy_dummy_token    # Dummy token so gh CLI doesn't complain
      LOG_LEVEL: info
      NODE_ENV: production
```

Note: `GH_TOKEN: proxy_dummy_token` is a pattern where the agent sees a dummy value, but the gateway's auth provider injects the real PAT at the network boundary.

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
        auth:
          provider: "github.com/donbader/agent-fleet/auth-providers/github-pat"
          options:
            token_env: GITHUB_PAT_TOKEN

      # Rule 2: MCP OAuth endpoints
      - endpoint: [https://mcp.notion.com/mcp, https://mcp.atlassian.com/v1/mcp]
        auth:
          provider: "github.com/donbader/agent-fleet/auth-providers/mcp-oauth"

      # Rule 3: Allow all other traffic (no auth injection)
      - host: ["*"]
```

### Rule Format

Each rule can use either `host:` or `endpoint:`:

| Field | Description | Example |
|-------|-------------|---------|
| `host` | Domain(s) to match | `["api.github.com"]` or `["*"]` |
| `endpoint` | Full URL(s) to match | `[https://mcp.notion.com/mcp]` |
| `auth` | Optional auth provider for credential injection | See below |
| `auth.provider` | Provider identifier (Go module path) | `"github.com/donbader/agent-fleet/auth-providers/github-pat"` |
| `auth.options` | Provider-specific options | `{ token_env: GITHUB_PAT_TOKEN }` |

- `host: ["*"]` — matches all hosts (use as last rule to allow remaining traffic)
- `endpoint:` — matches exact URL paths (useful for MCP endpoints)
- Rules without `auth:` allow traffic without credential injection

### Shared vs Separate Gateways

```yaml
# Shared gateway — both agents use same egress proxy
agents:
  frontend:
    gateway: gw-shared
  backend:
    gateway: gw-shared

gateways:
  gw-shared:
    egress: [...]

# Separate gateways — each agent has its own egress proxy
agents:
  frontend:
    gateway: gw-frontend
  backend:
    gateway: gw-backend

gateways:
  gw-frontend:
    egress: [...]
  gw-backend:
    egress: [...]
```

## Auth Providers

Auth providers handle credential injection at the gateway proxy boundary. They are referenced by Go module path.

### Built-in Auth Providers

| Provider | Purpose | Injection |
|----------|---------|-----------|
| `auth-providers/github-pat` | GitHub Personal Access Token | `Authorization: token <pat>` |
| `auth-providers/mcp-oauth` | MCP OAuth2 (Notion, Jira, etc.) | `Authorization: Bearer <token>` with auto-refresh |
| `auth-providers/mcp-token` | MCP with app credentials | App-specific auth headers |
| `auth-providers/telegram` | Telegram Bot API | Token in URL path |
| `auth-providers/api-key` | Generic API key injection | Custom header |

### github-pat

```yaml
auth:
  provider: "github.com/donbader/agent-fleet/auth-providers/github-pat"
  options:
    token_env: GITHUB_PAT_TOKEN    # Env var in .env file
```

Injects `Authorization: token <value>` on matching requests.

### mcp-oauth

```yaml
auth:
  provider: "github.com/donbader/agent-fleet/auth-providers/mcp-oauth"
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
auth:
  provider: "github.com/donbader/agent-fleet/auth-providers/mcp-token"
  options:
    app_id_env: SLACK_APP_ID
    app_secret_env: SLACK_APP_SECRET
```

For MCP services that use app credentials instead of OAuth.

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

# Datadog
DD_API_KEY=your-datadog-api-key
```

## Multi-Session Behavior

Each chat/thread maps to a separate ACP session. One agent handles multiple concurrent conversations independently:

```
Telegram DM from @alice  → Session "alice-001"
Telegram DM from @bob    → Session "bob-001"
Group chat mention        → Session "group-987654321"
```

Sessions are independent — each has its own context and history.

## Environment Variables (Auto-injected)

These are set automatically by agent-fleet inside the sandbox:

| Variable | Description |
|----------|-------------|
| `AGENT_FLEET_AGENT_NAME` | Name of this agent |
| `AGENT_FLEET_GATEWAY` | Gateway name this agent uses |
| `AGENT_FLEET_CHANNEL_SOCKET` | Path to ACP Unix socket for channel communication |
| `AGENT_FLEET_DOCKER_HOST` | Docker API Proxy endpoint (if docker.enabled) |
