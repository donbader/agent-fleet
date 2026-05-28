# JSON Schemas

## Top-level schemas (this folder)

- `fleet.schema.json` — validates fleet.yaml structure
- `agent.schema.json` — validates agent.yaml structure

These schemas validate the overall structure. Provider-specific `options` fields are left open (`additionalProperties: true`) because each provider validates its own options.

## Provider schemas (shipped with each provider)

Each provider ships its own `schema.json` alongside its implementation:

```
runtimes/
  codex/
    schema.json              # codex runtime options
  channels-bridge/
    schema.json              # channels-bridge options (agent_provider, channels, etc.)

channel-providers/
  telegram/
    schema.json              # telegram channel options (allowed_users, groups)

egress-rules/
  github-pat/
    schema.json              # github-pat options (token)
  telegram-bot/
    schema.json              # telegram-bot options (token)
```

The CLI resolves provider schemas at validation time by fetching `schema.json` from the provider's path.
