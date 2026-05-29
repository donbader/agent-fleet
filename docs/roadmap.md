# Phases & Roadmap

## Phase 1: Documentation & Design ✅

Setup repo with architecture docs, design decisions, and configuration reference.

**Deliverables:**
- [x] README.md
- [x] AGENTS.md
- [x] docs/architecture.md
- [x] docs/configuration.md
- [x] docs/security-model.md
- [x] docs/bridge-protocol.md
- [x] docs/docker-api-proxy.md
- [x] docs/adr/ (001-005)
- [x] schemas/ (fleet.schema.json, agent.schema.json)
- [x] examples/ (simple + multi-agent)

## Phase 2: CLI Skeleton ✅

Go module with CLI commands and config parser.

**Deliverables:**
- [x] `cmd/agent-fleet/` — CLI with commands (up, down, status, validate)
- [x] `pkg/config/` — fleet.yaml + agent.yaml parser and validator
- [x] `pkg/compose/` — Docker Compose generation (stubbed)
- [x] `go.mod`, `go.sum` (cobra + yaml.v3)
- [x] Makefile

## Phase 3: Core Implementation ✅

Full implementations with unit tests.

**Completed:**
- [x] Config parsing + validation (6 fleet + 2 agent validation tests)
- [x] Egress rule compilation (first match wins, ordered presets)
- [x] Docker Compose generation (gateway + agents + networks)
- [x] Gateway proxy — TCP listener, SNI extraction, rule matching, passthrough
- [x] MITM TLS interception — dynamic cert generation, credential injection
- [x] Credential injection — Telegram URL rewrite, GitHub PAT header, API key
- [x] Bridge runtime — process spawning, ACP protocol, channel lifecycle
- [x] Telegram channel provider — long-poll, user filtering, command dispatch
- [x] Fleet orchestration — up/down/status via docker compose
- [x] Integration tests — proxy passthrough, default deny, MITM handshake

**Not yet implemented:**
- [ ] MCP OAuth2 flow (Notion dynamic client registration)
- [ ] Docker API Proxy (DinD container)
- [ ] iptables setup script (for sandbox container)

## Phase 4: CI Setup

**Deliverables:**
- [ ] GitHub Actions workflow for unit tests (fast, every PR)
- [ ] GitHub Actions workflow for integration tests (requires Docker)
- [ ] Linting (golangci-lint)
- [ ] Build verification
- [ ] Release automation (goreleaser)

## Future Phases

### Phase 5: Docker Images & Deployment
- Base sandbox Dockerfile (iptables + CA + proxy)
- Docker API Proxy image
- Pre-built runtime images (codex, claude-code)
- `agent-fleet init` command (scaffold new fleet)

### Phase 6: Additional Channels
- Slack channel provider
- Discord channel provider
- Web UI channel

### Phase 7: Advanced Features
- Policy advisor (denied requests → suggested egress rules)
- Fleet dashboard (web UI)
- Remote gateway support (deploy to cloud)
- External provider plugin mechanism
- MicroVM compute driver support
