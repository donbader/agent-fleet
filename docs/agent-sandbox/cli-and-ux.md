# CLI & UX

## Commands

```bash
agent-sandbox init [--runtime codex]    # interactive scaffold
agent-sandbox up [agent-name]           # build + start
agent-sandbox down [agent-name]         # stop
agent-sandbox exec [agent-name] [cmd]   # shell (default: bash)
agent-sandbox logs [agent-name]         # stream logs
agent-sandbox status                    # health dashboard
agent-sandbox plugins                   # list available
agent-sandbox plugins info <name>       # plugin details
agent-sandbox validate                  # check config + suggest fixes
agent-sandbox upgrade                   # self-update
agent-sandbox up --dry-run              # preview without building
agent-sandbox up --debug                # verbose gateway + bridge logs
agent-sandbox generate                  # write .build/ without starting
agent-sandbox rebuild                   # rebuild image, keep container
agent-sandbox restart                   # restart container, keep image
```

## UX Design

### Progressive Disclosure

```yaml
# Minimal (works immediately):
name: coder
runtime: codex

# Add credentials:
plugins:
  github: { token: "${GITHUB_PAT}" }

# Add channels:
plugins:
  telegram: { bot_token: "${BOT_TOKEN}", allowed_users: ["me"] }

# Full power:
plugins:
  github: { token: "${GITHUB_PAT}" }
  docker: true
  telegram: { bot_token: "${BOT_TOKEN}", allowed_users: ["me"] }
packages: [ripgrep, fd-find]
home: { persist: true, override: ./home/ }
```

### Interactive Init

Auto-detects `gh auth token`, suggests plugins based on runtime, creates `.env` with detected credentials.

### Smart Validation

```bash
$ agent-sandbox validate
⚠ runtime 'codex' typically needs 'openai' plugin for API access.
✓ Config valid (1 warning)
```

### Helpful Errors

```bash
✗ Plugin 'github' failed: token is invalid or expired
  Fix: gh auth refresh && agent-sandbox up
```

## DX (Plugin Authors)

### Scaffold

```bash
$ agent-sandbox plugin new my-corp-api
Created plugins/my-corp-api/ (go.mod, plugin.go, plugin_test.go, README.md)
```

### Testing

```go
func TestContribute(t *testing.T) {
    p := New()
    contrib, err := p.Contribute(sdk.ContributeContext{
        AgentName: "test",
        Config:    map[string]any{"token": "ghp_test"},
    })
    require.NoError(t, err)
    assert.Equal(t, []string{"github.com", "*.github.com"}, contrib.EgressRules[0].Hosts)
}

func TestInjector(t *testing.T) {
    injector, _ := New().NewInjector(map[string]any{"token": "ghp_real"})
    req := httptest.NewRequest("GET", "https://api.github.com/repos", nil)
    injector.InjectCredentials(req)
    assert.Equal(t, "token ghp_real", req.Header.Get("Authorization"))
}
```

### Integration Test Helper

```go
sb := sdktest.NewTestSandbox(t, github.New(), telegram.New())
defer sb.Cleanup()
resp := sb.HTTPGet("https://api.github.com/user")
assert.Equal(t, 200, resp.StatusCode)
```
