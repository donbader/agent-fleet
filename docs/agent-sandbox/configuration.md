# Configuration

## Single Agent

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

## Multi-Agent (Optional)

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

## Home Directory

| Mode | Config | Behavior |
|------|--------|----------|
| Ephemeral (default) | — | Home resets on restart. Auth token persists via small named volume. |
| Persistent | `home.persist: true` | Named volume at /home/agent. Runtime state survives. |
| Override | `home.override: ./home/` | Files staged to /opt/home-override/, cp'd on every start. |
| Combined | Both | Persistent + override always wins on start. |

Override mechanism uses `/opt/home-override/` staging (not in volume path). Entrypoint `cp -a` on every start ensures tracked configs always win over runtime state.

## Custom Packages

Declarative list (apt by default, specify type for others):
```yaml
packages:
  - ripgrep
  - name: typescript
    type: npm
```

For complex installs, add `packages.sh` (runs during docker build after declarative packages).

## Plugin Config

Plugins accept config via the `plugins:` map:

```yaml
plugins:
  github:
    token: "${GITHUB_PAT}"       # secret reference
  docker: true                    # shorthand for {} (all defaults)
  telegram:
    bot_token: "${BOT_TOKEN}"
    allowed_users: ["donbader"]
```

`true` is shorthand for `{}` (enable with all defaults). CLI validates against each plugin's `ConfigSchema()`.
