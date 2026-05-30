# Bridge Demo

Demonstrates the channels-bridge runtime connecting a codex agent to Telegram.

## What This Shows

- channels-bridge runtime spawns codex as a child process
- Telegram bot receives messages and routes them to the agent via ACP
- Gateway proxy injects the bot token (agent never sees the real token)
- Egress rules control what the agent can access

## Structure

```
fleet-bridge-demo/
  fleet.yaml                    # egress presets (telegram, github, allow-all)
  .env.example                  # required secrets
  agents/
    coder/
      agent.yaml                # channels-bridge + telegram channel config
```

## Configuration

**fleet.yaml** defines egress presets:
- `telegram-bot` — allows api.telegram.org with token injection
- `github` — allows github.com with PAT injection
- `allow-all` — catch-all for remaining traffic

**agents/coder/agent.yaml** uses channels-bridge runtime with:
- `agent_provider` — which agent CLI to spawn (codex)
- `channels` — Telegram channel with allowed users filter

## Running

```bash
cd examples/fleet-bridge-demo
cp .env.example .env
# Fill in:
#   TELEGRAM_BOT_TOKEN — from @BotFather
#   GITHUB_PAT — personal access token

agent-fleet generate
agent-fleet compose up -d --build
```

## Testing

Send a message to your bot on Telegram. The agent should respond via codex.

Check logs:
```bash
agent-fleet compose logs coder -f
```
