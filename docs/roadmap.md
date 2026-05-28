# Phases & Roadmap

## Phase 1: Documentation & Design ← CURRENT

Setup repo with architecture docs, design decisions, and configuration reference.

**Deliverables:**
- [x] README.md
- [x] AGENTS.md
- [x] docs/architecture.md
- [x] docs/configuration.md
- [x] docs/security-model.md
- [x] docs/bridge-protocol.md
- [x] docs/docker-api-proxy.md
- [x] examples/

## Phase 2: Skeleton with UX

Implement the CLI skeleton with proper UX. Empty implementations, but the full user-facing interface is defined.

**Deliverables:**
- [ ] `cmd/agent-fleet/` — CLI with all commands (init, up, down, status, logs, exec)
- [ ] `pkg/config/` — fleet.yaml parser and validator
- [ ] `pkg/compose/` — Docker Compose generation (stubbed)
- [ ] `pkg/bridge/` — channels-bridge runtime (spawn agent + manage channels, stubbed)
- [ ] `pkg/gateway/` — Transparent proxy + egress rule evaluation (stubbed)
- [ ] `pkg/egress-rules/` — Egress rule provider implementations (stubbed)
- [ ] `pkg/auth-providers/` — Auth provider implementations (stubbed)
- [ ] `pkg/docker-proxy/` — Docker API Proxy (stubbed)
- [ ] `channel-providers/telegram/` — Telegram channel provider (stubbed)
- [ ] `go.mod`, `go.sum`
- [ ] Example fleet.yaml that works with `agent-fleet init`

**Features:**
- Agent: Codex with ACP + Telegram channel (own bot per agent)
- Docker container with transparent proxy and default-deny egress
- Egress rule providers (github-pat, mcp-oauth, telegram-bot)
- OAuth UX via chat (`/oauth notion` → URL → paste callback)
- Docker API Proxy (optional per agent)
- Multi-agent fleet with composable egress presets
- Multi-session per agent (different chats = different ACP sessions)

## Phase 3: Implementation with TDD

Fill in the implementations with integration tests driving development.

**Order of implementation:**
1. Config parsing + validation (unit tests)
2. Egress rule compilation (unit tests)
3. Auth provider interfaces + github-pat (unit tests)
4. Docker Compose generation (integration tests)
5. Egress proxy wiring (integration tests)
6. Channel provider lifecycle + ACP protocol (integration tests)
7. Telegram channel provider (integration tests with mock Telegram API)
8. mcp-oauth auth provider + OAuth flow (integration tests)
9. Docker API Proxy (integration tests)
10. Fleet orchestration end-to-end (e2e tests)

**Testing strategy:**
- Unit tests for pure logic (config parsing, egress compilation, policy validation)
- Integration tests for external interactions (Docker, Telegram)
- Mock external services where possible
- Real Docker for CI integration tests

## Phase 4: CI Setup

**Deliverables:**
- [ ] GitHub Actions workflow for unit tests (fast, every PR)
- [ ] GitHub Actions workflow for integration tests (requires Docker)
- [ ] Linting (golangci-lint)
- [ ] Build verification
- [ ] Release automation (goreleaser)

## Future Phases

### Phase 5: Additional Runtimes
- Claude Code adapter (claude-headless-to-acp)
- Pi adapter (pi-rpc-to-acp)

### Phase 6: Additional Channels
- Slack channel provider
- Discord channel provider

### Phase 7: Advanced Features
- Policy advisor (denied requests → suggested egress rules)
- Fleet dashboard (web UI)
- Remote gateway support (deploy to cloud)
- External provider plugin mechanism
- MicroVM compute driver support
