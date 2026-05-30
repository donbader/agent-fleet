# Migration Plan: agent-fleet → agent-sandbox

## Strategy

New repo (`donbader/agent-sandbox`). agent-fleet stays in maintenance mode (security fixes only). No in-place migration — clean break.

## Phases

### Phase 1: SDK + Core Interfaces

**Goal:** Define the plugin contracts. Everything else builds on this.

```
agent-sandbox/
  sdk/
    go.mod
    plugin.go           ← RuntimePlugin, FeaturePlugin interfaces
    contributions.go    ← all Contribution structs
    handler.go          ← RequestHandler interface
  go.work
```

**Port from agent-fleet:** Nothing. Fresh design.

**Exit criteria:** Interfaces compile. A mock plugin can implement both interfaces.

---

### Phase 2: Gateway

**Goal:** Universal gateway binary that reads config + uses RequestHandler.

```
gateway/
  go.mod
  cmd/gateway/main.go
  proxy.go              ← TCP listener, iptables redirect handling
  sni.go               ← SNI extraction
  mitm.go             ← TLS termination, HTTP interception
  handler_registry.go  ← loads handlers from config
  dns.go              ← DNS resolver (UDP redirect)
```

**Port from agent-fleet:** `pkg/gateway/` (proxy, SNI, MITM logic). Refactor to use `RequestHandler` interface instead of hardcoded injectors.

**Exit criteria:** Gateway binary starts, reads config, routes traffic. Integration test with mock handler.

---

### Phase 3: Bridge

**Goal:** TypeScript bridge that spawns agent + loads channel plugins dynamically.

```
bridge/
  package.json
  src/
    index.ts            ← entrypoint
    agent.ts            ← spawn child process
    plugin-loader.ts    ← dynamic import from /opt/bridge/plugins/<name>/
    types.ts            ← ChannelPlugin interface
```

**Port from agent-fleet:** `runtimes/channels-bridge/src/` (bridge.ts, telegram.ts). Refactor to plugin-loader pattern.

**Exit criteria:** Bridge spawns a dummy agent, loads a mock channel plugin, routes messages.

---

### Phase 4: Generate Command

**Goal:** CLI reads config, calls plugins, writes .build/ artifacts.

```
cmd/agent-sandbox/
  go.mod
  main.go
  cmd/
    generate.go         ← reads config → calls Contribute() → writes .build/
    compose.go          ← passthrough
    validate.go
    init.go
    plugins.go

internal/
  config/               ← agent.yaml + fleet.yaml parsing
  merge/                ← contribution merging + conflict detection
  dockerfile/           ← Dockerfile generation from contributions
  compose/              ← docker-compose.yml generation
```

**Port from agent-fleet:** `pkg/compose/` (compose generation patterns), `cmd/agent-fleet/cmd/` (CLI structure).

**Exit criteria:** `agent-sandbox generate` produces valid .build/ from a test config. `agent-sandbox compose up --build` starts a container.

---

### Phase 5: Built-in Plugins

**Goal:** Implement core plugins. Each is a self-contained Go module.

| Plugin | Type | Port from |
|--------|------|-----------|
| `codex` | Runtime | `runtimes/codex/` (Dockerfile logic → Go) |
| `claude-code` | Runtime | New (similar to codex) |
| `pi` | Runtime | New (similar to codex) |
| `github` | Feature | `pkg/gateway/mitm.go` (PAT injection logic) |
| `telegram` | Feature | `pkg/channel/telegram/` + `runtimes/channels-bridge/src/telegram.ts` |
| `docker` | Feature | New (DockerHandler + DinD compose contribution) |

**Exit criteria:** Each plugin has unit tests. End-to-end: codex + github + telegram config → generate → compose up → agent responds via Telegram.

---

### Phase 6: home-version-control Plugin

**Goal:** Feature plugin for commands, entrypoint hooks, runtime volumes.

```
plugins/home-version-control/
  go.mod
  plugin.go
  plugin_test.go
```

**Port from agent-fleet:** `runtimes/codex/entrypoint.sh` (home-override logic → Go contribution).

**Exit criteria:** Plugin contributes commands, hooks, volumes. Generated Dockerfile includes them.

---

### Phase 7: CLI Polish

**Goal:** init, validate, plugins, upgrade commands.

- `init` — interactive scaffold (detect gh auth, suggest features)
- `validate` — config check + helpful errors
- `plugins` — list/info
- `upgrade` — self-update (port from agent-fleet)

**Port from agent-fleet:** `cmd/agent-fleet/cmd/init.go`, `pkg/selfupdate/`.

**Exit criteria:** Full CLI works end-to-end. README with quickstart.

---

### Phase 8: CI + Release

- GitHub Actions (lint, test, build)
- GoReleaser (multi-arch binaries)
- install.sh one-liner

**Port from agent-fleet:** `.github/workflows/`, `.goreleaser.yml`.

---

## Code Reuse Summary

| agent-fleet source | agent-sandbox destination | Reuse % |
|-------------------|--------------------------|---------|
| `pkg/gateway/` (proxy, sni, mitm) | `gateway/` | 80% |
| `runtimes/channels-bridge/src/` | `bridge/src/` | 70% |
| `pkg/compose/` | `internal/compose/` | 50% |
| `cmd/agent-fleet/cmd/` | `cmd/agent-sandbox/cmd/` | 40% |
| `pkg/selfupdate/` | `internal/selfupdate/` | 90% |
| `runtimes/codex/` | `plugins/codex/` | 30% (shell → Go) |
| `pkg/config/` | `internal/config/` | 20% (different schema) |

## What Gets Dropped

- `runtimes/*/render.sh` — replaced by plugin Contribute()
- `pkg/provider/resolver.go` — no remote providers, all compiled in
- `images/gateway/` — gateway source embedded in CLI
- `agent-fleet tools ctx` — no render scripts to support
- Template injection / user_base — replaced by home-version-control plugin
- Default-deny egress model — replaced by allow-all + MITM where needed

## agent-fleet Disposition

- Tag final release (v0.12.0)
- Update README: "maintenance mode, see agent-sandbox for new development"
- Keep repo for reference
- No new features, security fixes only

## Estimated Effort

| Phase | Size | Dependencies |
|-------|------|-------------|
| 1. SDK | Small (1 day) | None |
| 2. Gateway | Medium (3 days) | Phase 1 |
| 3. Bridge | Medium (2 days) | Phase 1 |
| 4. Generate | Medium (3 days) | Phase 1 |
| 5. Plugins | Large (5 days) | Phase 2, 3, 4 |
| 6. home-vc | Small (1 day) | Phase 4 |
| 7. CLI Polish | Medium (2 days) | Phase 4 |
| 8. CI + Release | Small (1 day) | Phase 7 |

Phases 2, 3, 4 can run in parallel after Phase 1.
Total: ~2-3 weeks.
