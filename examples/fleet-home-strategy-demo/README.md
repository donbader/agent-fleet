# Home Strategy Demo

Demonstrates three approaches to managing the agent's home directory. Each agent in this fleet uses a different strategy.

## Agents

### ephemeral (default)

No home volume — container starts fresh every time. Only the auth token persists (via `persist_auth_token: true`, the default).

```yaml
# agents/named-volume/agent.yaml — this is actually ephemeral by default
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
egress:
  - allow-all
```

**How it works:**
- Provider outputs only an auth volume (`coder-codex-auth:/home/agent/.codex`)
- Home directory is fresh on every restart
- Auth token persists so you don't need to re-login
- Set `persist_auth_token: false` for fully ephemeral (re-login every time)

**Best for:** Clean sandbox environments where you want no state leakage between runs.

---

### named-volume (persistent home)

Add a named volume for `/home/agent` in your agent.yaml. Home directory persists across container restarts.

```yaml
# agents/named-volume/agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
egress:
  - allow-all
volumes:
  - "named-volume-home:/home/agent"
```

**How it works:**
- You declare a named volume targeting `/home/agent`
- Docker creates the volume on first `compose up`
- Data persists across container restarts
- Rebuilding the image doesn't affect existing volume data
- Delete the volume to reset: `agent-fleet compose down -v`

**Best for:** Persistent home without git tracking. Agent keeps all state across restarts.

---

### ~~bind-mount~~ (BANNED)

> ⛔ Bind-mounting host directories to `/home/agent` is **not allowed**. The compose generator automatically removes any bind mount targeting the agent's home directory.

**Why:** Bind mounts create a bidirectional file channel that bypasses the sandbox. The agent could write malicious files to the host filesystem, defeating the purpose of isolation.

**Alternatives:**
- Use **named-volume** for persistent home
- Use **custom-base** for git-tracked configs
- Use **ephemeral** for clean sandbox

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
