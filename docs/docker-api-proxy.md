# Docker API Proxy

## Overview

The Docker API Proxy is an optional component that allows agents to spin up Docker containers in a controlled, policy-enforced way. It sits between the agent and the Docker daemon, validating every API call against a security policy.

## Why Not Direct Docker Access?

Giving an agent raw Docker socket access is equivalent to giving it root on the host:

```bash
# With raw Docker socket, an agent could:
docker run --privileged --pid=host -v /:/host ubuntu chroot /host
# → Full host access, game over
```

The Docker API Proxy prevents this by intercepting and validating every Docker API call.

## Architecture

```
┌─ OpenShell Sandbox ─────────────────────────────────────────┐
│                                                             │
│  Agent                                                      │
│    │                                                        │
│    │ DOCKER_HOST=tcp://docker-proxy:2375                    │
│    │ (standard Docker client, no special SDK needed)        │
│    ▼                                                        │
└────┼────────────────────────────────────────────────────────┘
     │
     ▼
┌─ Docker API Proxy ──────────────────────────────────────────┐
│                                                             │
│  1. Authenticate request (sandbox token)                    │
│  2. Parse Docker API call                                   │
│  3. Validate against policy                                 │
│  4. Reject or forward to Docker daemon                      │
│  5. Mutate: force network, inject labels, set limits        │
│                                                             │
└────┼────────────────────────────────────────────────────────┘
     │
     ▼
┌─ Docker Daemon ─────────────────────────────────────────────┐
│  Creates container on internal network                       │
└─────────────────────────────────────────────────────────────┘
```

## API Surface

The proxy exposes a subset of the Docker Engine API. It's compatible with standard Docker clients — the agent just sets `DOCKER_HOST`.

### Allowed Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/containers/create` | POST | Create a container (validated) |
| `/containers/{id}/start` | POST | Start a container |
| `/containers/{id}/stop` | POST | Stop a container |
| `/containers/{id}/kill` | POST | Kill a container |
| `/containers/{id}` | DELETE | Remove a container |
| `/containers/{id}/json` | GET | Inspect a container |
| `/containers/{id}/logs` | GET | Get container logs |
| `/containers/{id}/exec` | POST | Create exec instance |
| `/exec/{id}/start` | POST | Start exec instance |
| `/images/json` | GET | List images |
| `/images/create` | POST | Pull an image (validated against allowlist) |

### Blocked Endpoints

| Endpoint | Reason |
|----------|--------|
| `/volumes/*` | Prevent host filesystem access |
| `/networks/*` | Prevent network manipulation |
| `/swarm/*` | Prevent cluster operations |
| `/secrets/*` | Prevent secret access |
| `/configs/*` | Prevent config access |
| `/system/*` | Prevent system info leakage |

## Policy Enforcement

### On Container Create (`/containers/create`)

The proxy validates and mutates the create request:

```go
type DockerPolicy struct {
    AllowedImages    []string  // Glob patterns: ["node:20-*", "python:3.12-*"]
    DeniedOptions    []string  // ["privileged", "network=host", "cap-add"]
    MaxContainers    int       // Per-agent limit
    ResourceLimits   Resources // Forced on every container
    Network          string    // Force this network (e.g., "openshell_internal")
    Labels           map[string]string // Auto-applied labels for tracking
}
```

**Validation checks:**

| Check | Action on Violation |
|-------|-------------------|
| Image not in allowlist | Reject with 403 |
| `Privileged: true` | Reject with 403 |
| `NetworkMode: host` | Reject with 403 |
| `CapAdd` not empty | Reject with 403 |
| `PidMode: host` | Reject with 403 |
| `IpcMode: host` | Reject with 403 |
| Host path bind mounts | Reject with 403 |
| Container count > max | Reject with 429 |

**Mutations (always applied):**

| Field | Forced Value |
|-------|-------------|
| `NetworkMode` | `openshell_internal` (or sandbox network) |
| `Memory` | Policy limit (e.g., 2GB) |
| `NanoCPUs` | Policy limit (e.g., 2 CPUs) |
| `PidsLimit` | Policy limit (e.g., 256) |
| `Labels` | `agent-fleet.agent=<name>`, `agent-fleet.sandbox=<id>` |
| `RestartPolicy` | `no` (prevent zombie containers) |

### On Image Pull (`/images/create`)

```
1. Extract image name from ?fromImage= parameter
2. Check against allowlist (glob matching)
3. If not allowed → 403
4. If allowed → forward to Docker daemon
```

## Authentication

The proxy authenticates requests using a sandbox-scoped token:

```
Agent → Docker API Proxy
  Header: X-Sandbox-Token: <jwt>
  
Proxy validates:
  - Token signed by gateway
  - Token bound to this sandbox ID
  - Token not expired
```

Agent-spawned containers do NOT receive this token, so they cannot talk to the proxy.

## Lifecycle Management

### Container Cleanup

When the sandbox is destroyed, the proxy cleans up all agent-spawned containers:

```
1. agent-fleet down (or sandbox timeout)
2. Proxy queries Docker for containers with label agent-fleet.sandbox=<id>
3. Proxy stops and removes all matching containers
4. Proxy removes itself
```

### Health Monitoring

The proxy tracks container health and enforces timeouts:

```yaml
docker:
  container_timeout: 1h        # Kill containers running longer than 1h
  idle_timeout: 10m            # Kill containers idle for 10m
  health_check_interval: 30s   # Check container health every 30s
```

## Configuration

```yaml
# In fleet.yaml
agents:
  coder:
    sandbox:
      docker:
        enabled: true
        
        # Image allowlist (glob patterns)
        allowed_images:
          - "node:20-*"
          - "node:22-*"
          - "python:3.11-*"
          - "python:3.12-*"
          - "golang:1.22-*"
          - "golang:1.23-*"
          - "ubuntu:24.04"
          - "postgres:16-*"
          - "redis:7-*"
          - "mongo:7-*"
        
        # Hard limits
        max_containers: 5
        
        # Resource limits per container
        resource_limits:
          memory: "2g"
          cpus: "2"
          pids: 256
        
        # Timeouts
        container_timeout: 1h
        idle_timeout: 10m
        
        # Network behavior
        network: inherit    # Join sandbox's internal network
```

## Implementation Notes

### Technology Choice

The Docker API Proxy is a small Go binary (~500 lines core logic):
- HTTP reverse proxy with request interception
- JSON parsing for Docker API request bodies
- Glob matching for image allowlist
- JWT validation for authentication
- Docker client for cleanup operations

### Deployment

The proxy runs as a container on the OpenShell bridge network:
- Accessible from sandbox via network policy
- Has Docker socket mounted (it's the only thing that does)
- Managed by agent-fleet lifecycle

```
agent-fleet up
  → Creates OpenShell sandbox (agent + bridge)
  → Starts Docker API Proxy container (if docker.enabled)
  → Configures network policy to allow sandbox → proxy
  → Injects DOCKER_HOST env var into sandbox
```

## Comparison with Alternatives

| Approach | Security | Complexity | Agent UX |
|----------|----------|-----------|----------|
| **Docker API Proxy** (ours) | ✅ Policy-enforced | Medium | ✅ Standard Docker client |
| Raw Docker socket | ❌ Full host access | Low | ✅ Standard Docker client |
| Sysbox (rootless nested) | ⚠️ Partial | High | ✅ Standard Docker client |
| No Docker access | ✅ Maximum | None | ❌ Agent can't run containers |
| Gateway-mediated (OpenShell style) | ✅ Good | High | ❌ Custom API needed |
