# Plugins

## Runtime Plugins

Runtime plugins set `BaseImage` and install the agent CLI. Auto-enabled by the `runtime:` field — user doesn't list them under `plugins:`.

```yaml
runtime: codex    # auto-enables codex plugin → sets BaseImage + installs @openai/codex
```

| Runtime | Base Image | Packages |
|---------|-----------|----------|
| `codex` | node:22-slim | git, curl, @openai/codex |
| `claude-code` | node:22-slim | git, curl, @anthropic-ai/claude-code |
| `pi` | node:22-slim | git, curl, pi-coding-agent |

For unsupported runtimes:
```yaml
plugins:
  custom-runtime:
    base_image: "python:3.12-slim"
    packages: ["git", "my-agent-cli"]
    cmd: "my-agent-cli"
```

## Credential Plugins

Declare egress rules + implement `Injector` for credential injection at the gateway.

| Plugin | Hosts | Injection |
|--------|-------|-----------|
| `github` | github.com, *.github.com | Header: `Authorization: token <PAT>` |
| `mcp-oauth` | user-defined MCP server URL | OAuth2 dynamic client registration + token refresh |
| `static-header` | user-defined endpoint | Static header injection (any API key) |

Note: LLM API credentials (OpenAI, Anthropic) are handled by the runtime itself (codex device flow, claude login). No dedicated plugins needed — the agent stores its own auth token in the home directory.

### mcp-oauth plugin

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

## Channel Plugins

Contribute both egress rules (gateway side) AND bridge plugin code (channel side). One plugin, two contributions.

| Plugin | Egress | Bridge |
|--------|--------|--------|
| `telegram` | api.telegram.org → URL rewrite (bot token) | grammy bot, long-poll |
| `slack` | slack.com → OAuth token refresh | Slack socket mode |

## Feature Plugins

| Plugin | Contributes |
|--------|-------------|
| `docker` | DinD sidecar, docker CLI, DOCKER_HOST env, DockerInjector in gateway |
