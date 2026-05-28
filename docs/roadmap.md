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
- [ ] `cmd/agent-fleet/` — CLI with all commands (init, up, down, status, logs, exec, auth)
- [ ] `pkg/config/` — fleet.yaml parser and validator
- [ ] `pkg/sandbox/` — OpenShell interface (stubbed)
- [ ] `pkg/bridge/` — Bridge lifecycle (stubbed)
- [ ] `pkg/egress/` — Egress rule compiler (stubbed)
- [ ] `pkg/docker-proxy/` — Docker API Proxy (stubbed)
- [ ] `bridges/telegram/` — Telegram bridge Dockerfile + entrypoint (stubbed)
- [ ] `go.mod`, `go.sum`
- [ ] Example fleet.yaml that works with `agent-fleet init`

**Features:**
- Agent: Codex with ACP bridge + Telegram
- OpenShell sandbox with .env-based secrets
- OAuth provider support (Notion, Jira, Datadog, Gmail)
- GitHub PAT provider
- Docker API Proxy (optional)
- Multi-agent fleet support
- Shared vs separate proxy option
- Multi-session per agent (different chats = different sessions)

## Phase 3: Implementation with TDD

Fill in the implementations with integration tests driving development.

**Order of implementation:**
1. Config parsing + validation (unit tests)
2. Egress rule compilation (unit tests)
3. OpenShell sandbox provisioning (integration tests)
4. Bridge lifecycle + ACP protocol (integration tests)
5. Telegram bridge (integration tests with mock Telegram API)
6. Docker API Proxy (integration tests)
7. Fleet orchestration (end-to-end tests)
8. OAuth flow (integration tests)

**Testing strategy:**
- Unit tests for pure logic (config parsing, egress compilation, policy validation)
- Integration tests for external interactions (OpenShell, Docker, Telegram)
- Mock external services where possible
- Real OpenShell + Docker for CI integration tests

## Phase 4: CI Setup

**Deliverables:**
- [ ] GitHub Actions workflow for unit tests (fast, every PR)
- [ ] GitHub Actions workflow for integration tests (requires Docker + OpenShell)
- [ ] Linting (golangci-lint)
- [ ] Build verification
- [ ] Release automation (goreleaser)

## Future Phases

### Phase 5: Additional Runtimes
- Claude Code adapter (claude-headless-to-acp)
- Pi adapter (pi-rpc-to-acp)

### Phase 6: Additional Bridges
- Slack bridge
- Discord bridge

### Phase 7: Advanced Features
- Policy advisor (denied requests → suggested rules)
- Fleet dashboard (web UI)
- Remote gateway support (deploy to cloud)
- MicroVM compute driver support
