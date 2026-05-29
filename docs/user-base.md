# Customizing Your Agent

agent-fleet is designed so you create your own folder (or repo) outside of agent-fleet and customize there. agent-fleet is just the CLI tool — your fleet config, home directories, and custom Dockerfiles live in your own repo.

```
my-fleet/                    ← your repo
  fleet.yaml
  .env
  agents/
    coder/
      agent.yaml
      home/                  ← home-override (read-only config)
        .gitconfig
        .ssh/
          config
      Dockerfile             ← custom base image (optional)
    reviewer/
      agent.yaml
```

## Home Directory

### Default: Named Volume + home-override (Strategy 1)

By default, `/home/agent` is a Docker named volume (persists across container restarts). You can overlay read-only config files on top using `agents/<name>/home/`:

```
agents/coder/home/
  .gitconfig        → mounted at /home/agent/.gitconfig:ro
  .ssh/config       → mounted at /home/agent/.ssh/config:ro
```

The agent can write anywhere in `/home/agent` (it's a writable volume), but the files from `home/` are always read-only — they stay in sync with your repo.

**Docker Compose equivalent:**
```yaml
services:
  coder:
    volumes:
      - coder-home:/home/agent                              # persistent volume
      - ./agents/coder/home/.gitconfig:/home/agent/.gitconfig:ro
      - ./agents/coder/home/.ssh:/home/agent/.ssh:ro

volumes:
  coder-home:
```

**WORKDIR:** `/home/agent/workspace`

### Alternative: Bind Mount (Strategy 2)

If you want to version-control the entire home directory with GitHub:

```yaml
# agent.yaml
runtime:
  provider: "github.com/donbader/agent-fleet/runtimes/codex"
  options:
    home_mount: bind    # use bind mount instead of named volume
```

This mounts `agents/<name>/home/` directly as `/home/agent`:

```yaml
# Generated compose:
services:
  coder:
    volumes:
      - ./agents/coder/home:/home/agent
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

Your Dockerfile installs extra tools:

```dockerfile
# agents/coder/Dockerfile
FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*
```

The runtime's Dockerfile uses Docker `additional_contexts` to pull tools from your image into the final container. You don't need to include bridge/iptables setup — the runtime handles that.

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
    │   └── home/
    │       ├── .gitconfig   ← read-only override
    │       └── .ssh/
    │           └── config   ← SSH host aliases
    └── reviewer/
        ├── agent.yaml
        └── home/
            └── .gitconfig
```

Run:
```bash
cd my-team-agents
agent-fleet up
```

agent-fleet resolves remote providers (runtimes, egress-rules) automatically. Your repo only contains your config and customizations.
