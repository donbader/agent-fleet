# Customizing Your Agent

agent-fleet is designed so you create your own folder (or repo) outside of agent-fleet and customize there. agent-fleet is just the CLI tool — your fleet config, home directories, and custom Dockerfiles live in your own repo.

```
my-fleet/                    ← your repo
  fleet.yaml
  .env
  agents/
    coder/
      agent.yaml
      Dockerfile             ← custom base image (optional, Strategy 3)
      home-override/         ← config files to bake into image (optional, Strategy 3)
        .gitconfig
    reviewer/
      agent.yaml
```

## Home Directory Strategies

The agent's home directory (`/home/agent`) can be set up in different ways depending on your needs.

### Strategy 1.1: Named Volume

The provider's render.sh outputs a named volume for the home directory. Docker populates it from the image on first run. No extra configuration needed — this is what the codex and channels-bridge runtimes do out of the box.

```yaml
# agent.yaml — nothing special, just pick a runtime
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
egress:
  - allow-all
```

**Generated compose (by provider's render.sh):**
```yaml
volumes:
  - coder-home:/home/agent
```

**Behavior:**
- Home directory persists across container restarts
- Agent can write freely
- Rebuild image doesn't affect existing volume data

### Strategy 1.2: Bind Mount

Use the `volumes` field in agent.yaml to bind-mount a host directory as the home. Good for version-controlling the home directory with git.

```yaml
# agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
egress:
  - allow-all
volumes:
  - "./agents/coder/home:/home/agent"
```

Note: the path is relative to the compose file (fleet root), not relative to agent.yaml.

**Behavior:**
- Agent writes are visible on host
- Changes on host are immediately visible in container
- Version-control the home directory with git

**Tradeoffs:**
- Permission issues on Linux (container UID vs host UID)
- Need `.gitignore` for transient files (node_modules, .cache, etc.)

### Strategy 3: Named Volume + Custom Base Template

Combine a named volume with a custom Dockerfile template to pre-install tools and bake config files into the image. Docker populates the volume from the image on first run.

```yaml
# agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
  options:
    user_base_image_stage: "./Dockerfile"
egress:
  - allow-all
```

```dockerfile
# agents/coder/Dockerfile (partial template — not standalone)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*

COPY home-override/.gitconfig ${AGENT_HOME}/.gitconfig
```

**Behavior:**
- Extra tools available in the container
- Config files baked into image → populate volume on first run
- Rebuild image + delete volume → fresh home with updated config

## Custom Base Image (user_base_image_stage)

Your Dockerfile is a **template** — the provider reads it, substitutes magic variables, and injects it into the runtime's Dockerfile.

### Provider Magic Variables

Each provider defines variables for things it controls:

| Variable | Description | Example |
|----------|-------------|--------|
| `${AGENT_HOME}` | Agent's home directory | `/home/agent` |
| `${AGENT_USER}` | Agent's OS username | `agent` |

Providers may define additional variables in their own docs. Use variables instead of hardcoding internal paths.

### How It Works

1. Provider reads your partial Dockerfile
2. Substitutes magic variables
3. Injects it into the runtime's Dockerfile (between base setup and finalize)
4. Build context is set to your agent directory (so `COPY` paths are relative to `agents/coder/`)

You don't need to include runtime setup (bridge, iptables, entrypoint) — the provider handles that.

## Putting It Together

A typical setup:

```
my-team-agents/              ← your repo (not agent-fleet)
├── fleet.yaml
├── .env                     ← secrets (gitignored)
├── .env.example
├── .gitignore
└── agents/
    ├── coder/
    │   ├── agent.yaml
    │   ├── Dockerfile       ← extra tools (Strategy 3)
    │   └── home-override/
    │       └── .gitconfig
    └── reviewer/
        └── agent.yaml       ← Strategy 1.1 (no customization)
```

Run:
```bash
cd my-team-agents
agent-fleet up
```

agent-fleet resolves remote providers (runtimes, egress-rules) automatically. Your repo only contains your config and customizations.
