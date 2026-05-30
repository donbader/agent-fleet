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

### volumes

Mount host directories or override provider volumes.

```yaml
# agent.yaml
volumes:
  - "./agents/coder/home:/home/agent"
  - "./agents/coder/workspace:/workspace"
```

Paths are relative to the fleet root (where docker-compose.yml is generated).

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
