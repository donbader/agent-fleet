# Documentation

## For Users

| Doc | Description |
|-----|-------------|
| [Getting Started](getting-started.md) | Install, configure, deploy, daily workflow |
| [Configuration Reference](configuration.md) | fleet.yaml and agent.yaml options |
| [Customization](customization.md) | Home directory strategies and Dockerfile templates |

## Architecture & Design

| Doc | Description |
|-----|-------------|
| [Architecture](architecture.md) | System components and how they connect |
| [Security Model](security-model.md) | Egress rules, transparent proxy, MITM, credential injection |
| [Bridge Protocol](bridge-protocol.md) | ACP protocol, channel adapters, message routing |
| [Docker API Proxy](docker-api-proxy.md) | Controlled container spawning [PLANNED] |

## Decision Records

| ADR | Decision |
|-----|----------|
| [001](adr/001-no-openshell.md) | No OpenShell — doesn't support allow-all traffic |
| [002](adr/002-transparent-proxy.md) | Transparent proxy via iptables |
| [003](adr/003-go-proxy.md) | Go proxy (single binary, same language) |
| [004](adr/004-composable-egress-presets.md) | Composable egress presets (first match wins) |
| [005](adr/005-all-credentials-through-proxy.md) | All credentials through proxy |

## Runtime Providers

| Runtime | Docs |
|---------|------|
| Codex | [runtimes/codex/README.md](../runtimes/codex/README.md) |
| Channels Bridge | [runtimes/channels-bridge/README.md](../runtimes/channels-bridge/README.md) |
