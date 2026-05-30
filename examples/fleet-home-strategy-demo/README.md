# Home Strategy Demo

Demonstrates three approaches to managing the agent's home directory. Each agent in this fleet uses a different strategy.

## Agents

### named-volume (simplest)

The runtime provider outputs a named volume for `/home/agent`. Docker populates it from the image on first run. Both the **codex** and **channels-bridge** providers do this by default — no home directory configuration needed.

```yaml
# agents/named-volume/agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
egress:
  - allow-all
```

**How it works:**
- Provider's render.sh outputs `coder-home:/home/agent` in the compose fragment
- Docker creates the volume on first `compose up`
- Data persists across container restarts
- Rebuilding the image doesn't affect existing volume data
- Delete the volume to reset: `agent-fleet compose down -v`

**Best for:** Most use cases. Zero config, persistent home.

---

### bind-mount (host-visible)

Bind-mounts a host directory as the home. The agent's files are directly visible and editable on the host.

```yaml
# agents/bind-mount/agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
egress:
  - allow-all
volumes:
  - "./agents/bind-mount/home:/home/agent"
```

**How it works:**
- The `volumes` field in agent.yaml adds volumes to the compose service
- Path is relative to fleet root (where docker-compose.yml lives)
- Agent writes are immediately visible on host and vice versa

**Best for:** Debugging (inspecting agent state from host), or sharing files between host and container in real-time.

**Not recommended for git version control of configs** — the agent writes runtime state (caches, temp files, auth tokens) into the home directory. You'd need extensive `.gitignore` and it's hard to separate intentional configs from agent-generated artifacts. Use [custom-base](#custom-base-home-overriding) instead.

**Security considerations:**
- ⚠️ Creates a bidirectional file channel that partially bypasses the sandbox
- Agent can write files to the host filesystem (within the mounted path)
- If mount path is too broad, agent could access sensitive host files
- Conflicts with agent-fleet's sandbox isolation philosophy

**Other tradeoffs:**
- Permission issues on Linux (container UID ≠ host UID)
- Need `.gitignore` for transient files (.cache, node_modules, etc.)

---

### custom-base (home overriding)

Combines a named volume with a partial Dockerfile template. Bakes config files into the image so they become the initial state of the home directory.

```yaml
# agents/custom-base/agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
  options:
    user_base: "./Dockerfile"
egress:
  - allow-all
```

```dockerfile
# agents/custom-base/Dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep fd-find jq \
    && rm -rf /var/lib/apt/lists/*

COPY --chown=${AGENT_USER}:${AGENT_USER} home-override/ ${AGENT_HOME}/
```

**How it works:**
1. Provider reads your partial Dockerfile
2. Substitutes `${AGENT_HOME}` and `${AGENT_USER}` with actual values
3. Injects it into the runtime's Dockerfile
4. Build context is `agents/custom-base/` (so COPY paths work)
5. Named volume is populated from image on first run

**Best for:** Git version control of agent configs. Track `.gitconfig`, `.bashrc`, tool settings in your repo. Agent files a PR → merge → rebuild → configs persist. This is the recommended approach for persistent, reviewable customization.

**To reset home to tracked state:** delete the volume and rebuild:
```bash
agent-fleet compose down -v
agent-fleet compose up -d --build
```

## Running This Example

```bash
cd examples/fleet-home-strategy-demo
cp .env.example .env
agent-fleet generate
agent-fleet compose up -d --build
```
