# Customizing Your Agent

agent-fleet is designed so you create your own folder (or repo) outside of agent-fleet and customize there. agent-fleet is just the CLI tool — your fleet config, home directories, and custom Dockerfiles live in your own repo.

```
my-fleet/                    ← your repo
  fleet.yaml
  .env
  agents/
    coder/
      agent.yaml
      home-override/           ← read-only config overlays
        .gitconfig
      Dockerfile             ← custom base image (optional)
    reviewer/
      agent.yaml
```

## Home Directory

### Default: Named Volume + home-override (Strategy 1)

By default, `/home/agent` is a Docker named volume (persists across container restarts). On first run, Docker populates the volume from the image contents.

To pre-configure files in the home directory, use your custom Dockerfile (`user_base_image_stage`):

```dockerfile
# agents/coder/Dockerfile
FROM node:22-slim

RUN apt-get update && apt-get install -y ripgrep fd-find jq && rm -rf /var/lib/apt/lists/*

# Pre-configure home directory
COPY home-override/.gitconfig /home/agent/.gitconfig
```

Docker populates the named volume from the image on first run, so your config files will be there.

**WORKDIR:** `/home/agent/workspace`

### Alternative: Bind Mount (Strategy 2)

If you want to version-control the entire home directory with GitHub, use the `volumes` field in agent.yaml:

```yaml
# agent.yaml
volumes:
  - "./home:/home/agent"
```

```yaml
# Generated compose:
services:
  coder:
    volumes:
      - ./agents/coder/home-override:/home/agent
```

**WORKDIR:** `/home/agent`

**When to use:**
- You want the agent's entire home directory in version control
- You want to see/edit agent files from the host
- You're sharing agent config across machines via git

**Tradeoffs:**
- Permission issues on Linux (container UID vs host UID)
- Agent writes are visible on host (can be noisy in git)
- Need `.gitignore` for transient files (node_modules, .cache, etc.)

## Custom Base Image (user_base_image_stage)

Add extra tools to your agent container without modifying the runtime:

```yaml
# agents/coder/agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/channels-bridge"
  options:
    user_base_image_stage: "./Dockerfile"
```

Your Dockerfile is a **template** — the provider injects it into its own Dockerfile and substitutes magic variables:

```dockerfile
# agents/coder/Dockerfile (partial template — not a standalone Dockerfile)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*

COPY home-override/.gitconfig ${AGENT_HOME}/.gitconfig
```

### Provider Magic Variables

Each provider defines variables for things it controls:

| Variable | Description | Example |
|----------|-------------|--------|
| `${AGENT_HOME}` | Agent's home directory | `/home/agent` |
| `${AGENT_USER}` | Agent's OS username | `agent` |

Providers may define additional variables in their own docs. Users should not hardcode internal paths — use variables instead.

### How It Works

1. Provider reads your partial Dockerfile
2. Substitutes magic variables
3. Injects it into the runtime's Dockerfile (between base setup and finalize)
4. Build context is set to your agent directory (so `COPY` paths are relative to `agents/coder/`)

You don't need to include runtime setup (bridge, iptables, entrypoint) — the provider handles that.

## Putting It Together

A typical setup for a team:

```
my-team-agents/              ← your repo (not agent-fleet)
├── fleet.yaml
├── .env                     ← secrets (gitignored)
├── .env.example
├── .gitignore
└── agents/
    ├── coder/
    │   ├── agent.yaml
    │   ├── Dockerfile       ← extra tools (ripgrep, etc.)
    │   └── home-override/
    │       └── .gitconfig   ← read-only override
    └── reviewer/
        ├── agent.yaml
        └── home-override/
            └── .gitconfig
```

Run:
```bash
cd my-team-agents
agent-fleet up
```

agent-fleet resolves remote providers (runtimes, egress-rules) automatically. Your repo only contains your config and customizations.
