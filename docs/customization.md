# Customization

agent-fleet is just the CLI tool. Your fleet config, home directories, and custom Dockerfiles live in your own repo.

```
my-fleet/                    ← your repo
  fleet.yaml
  .env
  agents/
    coder/
      agent.yaml
      Dockerfile             ← partial template (optional)
      home-override/         ← config files to bake in (optional)
```

## What You Can Customize

### persist_auth_token

Controls whether the agent's auth token persists across container restarts. Default: `true`.

```yaml
# agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
  options:
    persist_auth_token: false  # fully ephemeral, re-login every restart
```

By default, containers are **ephemeral** (home directory resets on restart) but auth tokens persist so you don't need to re-login. Set to `false` for a fully clean sandbox every time.

### volumes

Mount named volumes or other paths. Use this to persist the home directory.

```yaml
# agent.yaml
volumes:
  - "coder-home:/home/agent"    # named volume — allowed
  - "./workspace:/workspace"    # bind mount to non-home path — allowed
```

> ⛔ **Bind mounts to `/home/agent` are banned.** The compose generator automatically removes them during `agent-fleet generate`. This is a security measure — bind mounts create a file channel that bypasses the sandbox. Use named volumes or `user_base` instead.

### user_base

A partial Dockerfile template injected into the runtime's build. Use it to install extra tools or bake config files into the image.

```yaml
# agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
  options:
    user_base: "./Dockerfile"
```

```dockerfile
# agents/coder/Dockerfile (partial — not standalone)
RUN apt-get update && apt-get install -y ripgrep fd-find jq

COPY --chown=${AGENT_USER}:${AGENT_USER} home-override/ ${AGENT_HOME}/
```

The provider substitutes magic variables and injects your content into the runtime Dockerfile. Build context is your agent directory (`agents/coder/`).

**Magic variables** (provider-defined):

| Variable | Value |
|----------|-------|
| `${AGENT_HOME}` | `/home/agent` |
| `${AGENT_USER}` | `agent` |

### init_scripts

Scripts to run on every container start, before the agent process. Useful for overriding home directory files from a staging location.

```yaml
# agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
  options:
    user_base: "./Dockerfile"
    init_scripts:
      - "/scripts/override-home.sh"
```

Scripts must be COPYed into the image via `user_base` and made executable. They run as root before the agent user takes over.

### env

Environment variables passed to the agent container.

```yaml
# agent.yaml
env:
  GH_TOKEN: proxy_dummy_token
  EDITOR: vim
```

### ports

Expose container ports to the host.

```yaml
# agent.yaml
ports:
  - "1455:1455"
```

## Examples

See [examples/fleet-home-strategy-demo](../examples/fleet-home-strategy-demo/) for a walkthrough of different home directory approaches (named volume, bind mount, home overriding).
