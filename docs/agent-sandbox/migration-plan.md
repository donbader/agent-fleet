# Migration Plan: agent-fleet → agent-sandbox

## Strategy

New repo (`donbader/agent-sandbox`). agent-fleet stays in maintenance mode (security fixes only). No in-place migration — clean break.

**Principle:** Every phase produces a working `agent-sandbox generate && agent-sandbox compose up --build`. Each phase adds capabilities, never breaks what's already working.

## Phases

### Phase 1: Bare Container

**What works after this phase:**
```bash
agent-sandbox generate && agent-sandbox compose up --build
# → codex agent running in a container (no proxy, no bridge, no channels)
```

**Scope:**
- SDK interfaces (RuntimePlugin, FeaturePlugin)
- `codex` RuntimePlugin (sets base image, installs codex CLI)
- `generate` command (reads agent.yaml → writes .build/)
- `compose` passthrough command
- Dockerfile generation (single stage, no gateway)
- docker-compose.yml generation

**Config:**
```yaml
name: coder
runtime: codex
```

**What's missing:** No gateway (agent has unrestricted network). No bridge (codex runs directly as entrypoint). No channels. No features.

---

### Phase 2: Gateway (Network Enforcement)

**What works after this phase:**
```bash
agent-sandbox generate && agent-sandbox compose up --build
# → codex agent with transparent proxy (all traffic passthrough, no MITM yet)
```

**Scope:**
- Gateway binary (TCP proxy, SNI extraction, passthrough mode)
- Multi-stage Dockerfile (compile gateway + runtime)
- Entrypoint sets up iptables → starts gateway → starts agent
- DNS resolver (redirects UDP DNS to gateway)
- go:embed gateway source in CLI

**Port from agent-fleet:** `pkg/gateway/` (proxy.go, sni.go)

**Config:** Same as Phase 1 (no config change needed — gateway is always-on infrastructure).

**What's missing:** No MITM, no credential injection. All traffic passes through unchanged.

---

### Phase 3: Credential Injection (github feature)

**What works after this phase:**
```bash
agent-sandbox generate && agent-sandbox compose up --build
# → codex agent with GitHub PAT injection (MITM on github.com, passthrough rest)
```

**Scope:**
- MITM logic in gateway (TLS termination, HTTP interception)
- `github` FeaturePlugin (contributes Hosts + NewHandler)
- Gateway config generation (hosts → handler mapping)
- Sandbox CA generation (.build/ca.key, ca.crt)
- RequestHandler interface wired end-to-end

**Port from agent-fleet:** `pkg/gateway/mitm.go`

**Config:**
```yaml
name: coder
runtime: codex
features:
  github:
    token: "${GITHUB_PAT}"
```

**What's missing:** No channels (can't talk to agent remotely). Agent runs headless.

---

### Phase 4: Bridge + Telegram

**What works after this phase:**
```bash
agent-sandbox generate && agent-sandbox compose up --build
# → codex agent reachable via Telegram (send message → agent responds)
```

**Scope:**
- Bridge runtime (TypeScript: spawn agent, plugin loader)
- `telegram` FeaturePlugin (contributes gateway rules + bridge TypeScript)
- BridgeContribution wiring (extract TypeScript to .build/, bridge-config.json)
- Entrypoint: gateway → bridge → agent (process tree)
- Telegram bot token injection via gateway (URL rewrite)

**Port from agent-fleet:** `runtimes/channels-bridge/src/` (bridge.ts, telegram.ts)

**Config:**
```yaml
name: coder
runtime: codex
features:
  github:
    token: "${GITHUB_PAT}"
  telegram:
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    allowed_users: ["donbader"]
```

**What's missing:** No Docker access, no home customization, no extra packages.

---

### Phase 5: Docker Feature

**What works after this phase:**
```bash
agent-sandbox generate && agent-sandbox compose up --build
# → codex agent can run docker commands (DinD sidecar, validated by gateway)
```

**Scope:**
- `docker` FeaturePlugin (contributes DinD service, DockerHandler, DOCKER_HOST env)
- DockerHandler in gateway (HTTP inspection, block privileged, inject gateway redirect)
- ComposeContribution wiring (additional services in docker-compose.yml)
- Spawned container egress → agent's gateway

**Port from agent-fleet:** New (no equivalent in agent-fleet).

**Config:**
```yaml
name: coder
runtime: codex
features:
  github:
    token: "${GITHUB_PAT}"
  telegram:
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    allowed_users: ["donbader"]
  docker: true
```

**What's missing:** No home customization, no extra packages.

---

### Phase 6: home-version-control Feature

**What works after this phase:**
```bash
agent-sandbox generate && agent-sandbox compose up --build
# → codex agent with custom packages, startup hooks, persistent home
```

**Scope:**
- `home-version-control` FeaturePlugin
- ImageContribution.Commands wiring (RUN in Dockerfile)
- EntrypointContribution.Hooks wiring (scripts in entrypoint)
- ComposeContribution.Volumes wiring (named volumes)
- Home override directory (./home/ → /opt/home-override/ → cp on start)

**Port from agent-fleet:** `runtimes/codex/entrypoint.sh` (override logic)

**Config:**
```yaml
name: coder
runtime: codex
features:
  github:
    token: "${GITHUB_PAT}"
  telegram:
    bot_token: "${TELEGRAM_BOT_TOKEN}"
    allowed_users: ["donbader"]
  docker: true
  home-version-control:
    commands:
      - "apt-get install -y ripgrep fd-find"
    entrypoint_hooks:
      - ./scripts/sync-dotfiles.sh
    runtime_volumes:
      - "agent-home:/home/agent"
```

**What's missing:** No init wizard, no validate, no upgrade, no multi-agent.

---

### Phase 7: CLI Polish + Multi-Agent

**What works after this phase:**
```bash
agent-sandbox init                      # interactive scaffold
agent-sandbox validate                  # config check
agent-sandbox generate && agent-sandbox compose up --build
agent-sandbox upgrade                   # self-update
```

**Scope:**
- `init` command (interactive, detect gh auth, suggest features)
- `validate` command (config check + helpful errors)
- `plugins` command (list/info)
- `upgrade` command (self-update)
- fleet.yaml support (multi-agent, shared features)
- Additional RuntimePlugins: `claude-code`, `pi`
- Additional FeaturePlugins: `mcp-oauth`, `static-header`

**Port from agent-fleet:** `cmd/agent-fleet/cmd/init.go`, `pkg/selfupdate/`

---

### Phase 8: CI + Release

**Scope:**
- GitHub Actions (lint, test, build)
- GoReleaser (multi-arch binaries)
- install.sh one-liner
- README with quickstart
- Migration guide for agent-fleet users

**Port from agent-fleet:** `.github/workflows/`, `.goreleaser.yml`, `install.sh`

---

## Code Reuse Summary

| agent-fleet source | agent-sandbox destination | Phase | Reuse % |
|-------------------|--------------------------|-------|---------|
| `pkg/gateway/` (proxy, sni) | `gateway/` | 2 | 80% |
| `pkg/gateway/mitm.go` | `gateway/mitm.go` | 3 | 80% |
| `runtimes/channels-bridge/src/` | `bridge/src/` | 4 | 70% |
| `pkg/compose/` | `internal/compose/` | 1 | 50% |
| `pkg/selfupdate/` | `internal/selfupdate/` | 7 | 90% |
| `cmd/agent-fleet/cmd/` | `cmd/agent-sandbox/cmd/` | 1, 7 | 40% |
| `runtimes/codex/` | `plugins/codex/` | 1 | 30% |
| `pkg/config/` | `internal/config/` | 1 | 20% |

## What Gets Dropped

- `runtimes/*/render.sh` — replaced by plugin Contribute()
- `pkg/provider/resolver.go` — no remote providers, all compiled in
- `images/gateway/` — gateway source embedded in CLI
- `agent-fleet tools ctx` — no render scripts to support
- Template injection / user_base — replaced by home-version-control
- Default-deny egress model — replaced by allow-all + MITM where needed

## agent-fleet Disposition

- Tag final release (v0.12.0)
- Update README: "maintenance mode, see agent-sandbox for new development"
- Keep repo for reference
- No new features, security fixes only

## Estimated Effort

| Phase | Deliverable | Size | Dependencies |
|-------|-------------|------|-------------|
| 1. Bare Container | `generate` + `compose up` works | 3 days | None |
| 2. Gateway | Transparent proxy enforced | 3 days | Phase 1 |
| 3. Credentials | GitHub PAT injection | 2 days | Phase 2 |
| 4. Bridge + Telegram | Remote messaging | 3 days | Phase 2 |
| 5. Docker | DinD + DockerHandler | 3 days | Phase 3 |
| 6. home-vc | Packages + hooks + volumes | 2 days | Phase 1 |
| 7. CLI + Multi-Agent | Full CLI, fleet.yaml | 3 days | Phase 4 |
| 8. CI + Release | Automated releases | 1 day | Phase 7 |

Phases 3-4 can run in parallel. Phase 6 can start after Phase 1.
Total: ~3 weeks sequential, ~2 weeks with parallelism.
