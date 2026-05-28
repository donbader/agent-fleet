# Configuration Reference

## File Structure

```
my-fleet/
├── fleet.yaml          # Main configuration
├── .env                # Secrets (never committed)
├── .gitignore          # Should include .env
└── policies/           # Optional custom OpenShell policies
    └── custom.yaml
```

## fleet.yaml Schema

```yaml
# Fleet metadata
fleet:
  name: my-agents                    # Fleet identifier
  gateway: local                     # OpenShell gateway (default: local)

# Agent definitions
agents:
  <agent-name>:
    runtime: codex | claude-code | pi
    bridge: <bridge-name>            # Which bridge this agent uses
    image: <custom-image>            # Optional custom sandbox image
    sandbox:
      egress: []                     # Network egress rules
      secrets:
        env_file: .env               # Path to .env file for this agent
        providers: []                # OpenShell provider definitions
      docker:                        # Optional Docker API Proxy config
        enabled: false
        allowed_images: []
        max_containers: 5
        resource_limits:
          memory: "2g"
          cpus: "2"
          pids: 256
      resources:                     # Sandbox resource limits
        memory: "4g"
        cpus: "4"
    env:                             # Non-secret environment variables
      NODE_ENV: production
      LOG_LEVEL: info

# Bridge definitions
bridges:
  <bridge-name>:
    type: telegram | slack | discord
    # Type-specific config below
    token_env: TELEGRAM_BOT_TOKEN    # Env var name (value from .env)
    allowed_chats: []                # Chat IDs allowed to interact
    sessions:
      mode: per-chat                 # per-chat | shared | manual
      max_concurrent: 10             # Max concurrent sessions

# Secret provider definitions (fleet-wide)
secrets:
  providers:
    - name: <provider-name>
      type: generic | github-pat | oauth | api-key
      # Type-specific fields below

# Shared proxy configuration
proxy:
  shared: true                       # All agents share one proxy (default)
  # shared: false                    # Each agent gets its own proxy
```

## Agent Configuration

### Runtime Options

| Runtime | Description | Protocol | Headless Mode |
|---------|-------------|----------|---------------|
| `codex` | OpenAI Codex CLI | ACP native | `codex --acp` |
| `claude-code` | Anthropic Claude Code | ACP via adapter | `claude -p` + stream-json |
| `pi` | Pi coding agent | Pi RPC via adapter | `pi --mode rpc` |

### Egress Rules

```yaml
agents:
  coder:
    sandbox:
      egress:
        # Simple: host + port + access level
        - host: api.github.com
          port: 443
          access: full

        # Wildcard subdomain
        - host: "*.googleapis.com"
          port: 443
          access: full

        # Read-only access
        - host: registry.npmjs.org
          port: 443
          access: read-only

        # Custom methods
        - host: api.example.com
          port: 443
          access: custom
          methods: [GET, POST]

        # With path restriction
        - host: api.notion.com
          port: 443
          access: full
          paths:
            - /v1/pages/*
            - /v1/databases/*
```

### Docker Configuration

```yaml
agents:
  coder:
    sandbox:
      docker:
        enabled: true
        allowed_images:
          - "node:20-*"
          - "python:3.12-*"
          - "golang:1.22-*"
          - "ubuntu:24.04"
          - "postgres:16-*"
          - "redis:7-*"
        denied_options:
          - privileged
          - network=host
          - cap-add
          - pid=host
        max_containers: 5
        resource_limits:
          memory: "2g"
          cpus: "2"
          pids: 256
        network: inherit              # Containers join sandbox network
```

## Bridge Configuration

### Telegram

```yaml
bridges:
  telegram:
    type: telegram
    token_env: TELEGRAM_BOT_TOKEN    # Read from .env
    allowed_chats:
      - 123456789                    # Your personal chat ID
      - -100987654321                # A group chat ID
    sessions:
      mode: per-chat                 # Each chat gets its own agent session
      max_concurrent: 10
      idle_timeout: 30m              # Kill idle sessions after 30 min
    commands:                        # Bot commands
      start: "Start a new session"
      reset: "Reset current session"
      status: "Show agent status"
```

### Slack (planned)

```yaml
bridges:
  slack:
    type: slack
    app_token_env: SLACK_APP_TOKEN
    bot_token_env: SLACK_BOT_TOKEN
    allowed_channels:
      - C01234ABCDE
    sessions:
      mode: per-thread              # Each thread gets its own session
```

## Secret Providers

### Generic (API Key)

```yaml
secrets:
  providers:
    - name: openai
      type: api-key
      env: OPENAI_API_KEY
      inject:
        header: Authorization
        prefix: "Bearer "
        endpoints:
          - host: api.openai.com
```

### GitHub PAT

```yaml
secrets:
  providers:
    - name: github
      type: github-pat
      env: GITHUB_TOKEN
      # Auto-configures:
      # - Injects Authorization: token <pat> on api.github.com
      # - Adds egress rule for api.github.com:443
```

### OAuth (Notion, Jira, Gmail, Datadog)

```yaml
secrets:
  providers:
    - name: notion
      type: oauth
      provider: notion
      client_id_env: NOTION_CLIENT_ID
      client_secret_env: NOTION_CLIENT_SECRET
      scopes:
        - read_content
        - update_content
      # Token stored after `agent-fleet auth notion`
      # Auto-refreshed by OpenShell provider

    - name: jira
      type: oauth
      provider: atlassian
      client_id_env: JIRA_CLIENT_ID
      client_secret_env: JIRA_CLIENT_SECRET
      scopes:
        - read:jira-work
        - write:jira-work
      cloud_id_env: JIRA_CLOUD_ID

    - name: gmail
      type: oauth
      provider: google
      client_id_env: GOOGLE_CLIENT_ID
      client_secret_env: GOOGLE_CLIENT_SECRET
      scopes:
        - https://www.googleapis.com/auth/gmail.modify

    - name: datadog
      type: api-key
      env: DD_API_KEY
      inject:
        header: DD-API-KEY
        endpoints:
          - host: "*.datadoghq.com"
```

## .env File Format

```bash
# Agent credentials
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...

# GitHub
GITHUB_TOKEN=ghp_...

# Telegram bridge
TELEGRAM_BOT_TOKEN=123456:ABC-DEF...

# OAuth (for MCP integrations)
NOTION_CLIENT_ID=...
NOTION_CLIENT_SECRET=...
JIRA_CLIENT_ID=...
JIRA_CLIENT_SECRET=...
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...

# Datadog
DD_API_KEY=...
```

## Proxy Configuration

### Shared Proxy (default)

All agents share one OpenShell gateway and proxy. Simpler, less resource usage.

```yaml
proxy:
  shared: true
```

### Separate Proxies

Each agent gets its own proxy instance. Better isolation between agents.

```yaml
proxy:
  shared: false
```

## Multi-Agent Example

```yaml
fleet:
  name: dev-team

agents:
  frontend:
    runtime: codex
    bridge: telegram
    sandbox:
      egress:
        - host: api.github.com
          port: 443
          access: full
        - host: registry.npmjs.org
          port: 443
          access: read-only
      secrets:
        env_file: .env
        providers:
          - github
          - openai

  backend:
    runtime: claude-code
    bridge: telegram
    sandbox:
      egress:
        - host: api.github.com
          port: 443
          access: full
        - host: "*.amazonaws.com"
          port: 443
          access: full
      secrets:
        env_file: .env
        providers:
          - github
          - anthropic
      docker:
        enabled: true
        allowed_images: ["postgres:*", "redis:*"]

bridges:
  telegram:
    type: telegram
    token_env: TELEGRAM_BOT_TOKEN
    allowed_chats: [123456789]
    sessions:
      mode: per-chat

secrets:
  providers:
    - name: github
      type: github-pat
      env: GITHUB_TOKEN
    - name: openai
      type: api-key
      env: OPENAI_API_KEY
      inject:
        header: Authorization
        prefix: "Bearer "
        endpoints:
          - host: api.openai.com
    - name: anthropic
      type: api-key
      env: ANTHROPIC_API_KEY
      inject:
        header: x-api-key
        endpoints:
          - host: api.anthropic.com

proxy:
  shared: false    # Each agent gets its own proxy
```

## Environment Variables

These are set automatically by agent-fleet inside the sandbox:

| Variable | Description |
|----------|-------------|
| `AGENT_FLEET_AGENT_NAME` | Name of this agent |
| `AGENT_FLEET_SESSION_ID` | Current ACP session ID |
| `AGENT_FLEET_BRIDGE_SOCKET` | Path to ACP Unix socket |
| `AGENT_FLEET_DOCKER_HOST` | Docker API Proxy endpoint (if enabled) |
