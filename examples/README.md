# Examples

Example fleet configurations demonstrating different agent-fleet features.

| Example | Description |
|---------|-------------|
| [fleet-home-strategy-demo](fleet-home-strategy-demo/) | Three home directory strategies: named volume, bind mount, home overriding |
| [fleet-bridge-demo](fleet-bridge-demo/) | Channels-bridge runtime with Telegram integration |

## Running an Example

```bash
cd examples/fleet-home-strategy-demo
cp .env.example .env
# Edit .env with your values
agent-fleet generate
agent-fleet compose up -d --build
```
