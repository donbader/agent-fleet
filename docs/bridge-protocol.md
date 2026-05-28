# Bridge Protocol

## Overview

Bridges connect AI agents to messaging platforms. Every bridge speaks **ACP (Agent Client Protocol)** on the agent side and a platform-specific API on the user side.

```
User ←→ [Messaging Platform] ←→ [Bridge] ←→ [Agent]
              Telegram API           ACP protocol
              Slack API              (ndJSON over stdio/socket)
```

## ACP (Agent Client Protocol)

ACP is a multi-session protocol for communicating with AI agents. It uses newline-delimited JSON (ndJSON) over stdio or Unix sockets.

### Why ACP?

| Feature | ACP | Pi RPC | Raw CLI |
|---------|-----|--------|---------|
| Multi-session | ✅ | ❌ (single session) | ❌ |
| Structured tool calls | ✅ | ✅ | ❌ |
| Streaming responses | ✅ | ✅ | ⚠️ (stdout parsing) |
| Session resume | ✅ | ❌ | ⚠️ (--resume flag) |
| Standard protocol | ✅ | ❌ (proprietary) | ❌ |

### ACP Message Types

```jsonl
// Client → Agent: Start session
{"type":"session.start","session_id":"abc123","config":{}}

// Client → Agent: Send message
{"type":"message.send","session_id":"abc123","content":"Fix the bug in auth.ts"}

// Agent → Client: Text response (streaming)
{"type":"message.delta","session_id":"abc123","delta":"Looking at auth.ts..."}

// Agent → Client: Tool use
{"type":"tool.use","session_id":"abc123","tool":"file_read","input":{"path":"src/auth.ts"}}

// Client → Agent: Tool result
{"type":"tool.result","session_id":"abc123","output":"...file contents..."}

// Agent → Client: Message complete
{"type":"message.complete","session_id":"abc123"}

// Client → Agent: End session
{"type":"session.end","session_id":"abc123"}
```

## Bridge Architecture

### Bridge Inside Sandbox

The bridge runs inside the OpenShell sandbox alongside the agent. This means:
- Bridge's Telegram API access is egress-controlled (needs explicit policy)
- Bridge communicates with agent via local Unix socket (no network hop)
- Bridge process is also restricted by sandbox policy

```
┌─ OpenShell Sandbox ──────────────────────────────────────┐
│                                                          │
│  ┌─────────┐    Unix Socket    ┌─────────────────────┐   │
│  │  Agent  │◄─────────────────►│  Bridge (ACP↔TG)   │   │
│  │ (codex) │    /ipc/agent.sock│                     │   │
│  └─────────┘                   └──────────┬──────────┘   │
│                                           │              │
│                                    egress (allowed)      │
│                                           │              │
└───────────────────────────────────────────┼──────────────┘
                                            │
                                            ▼
                                   api.telegram.org
```

### Multi-Session Routing

One bridge instance handles multiple concurrent conversations:

```
Telegram Chat A ──┐
                  │     ┌─────────┐     ┌─────────┐
Telegram Chat B ──┼────►│ Bridge  │────►│  Agent  │
                  │     │         │     │         │
Telegram Chat C ──┘     │ Routes: │     │ ACP     │
                        │ A→sess1 │     │ sessions│
                        │ B→sess2 │     │         │
                        │ C→sess3 │     │         │
                        └─────────┘     └─────────┘
```

Each chat maps to a separate ACP session. The agent maintains independent context per session.

### Session Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| `per-chat` | Each chat/channel gets its own session | Default, most common |
| `shared` | All chats share one session | Team collaboration on one task |
| `manual` | User explicitly starts/ends sessions | Long-running projects |

## Protocol Adapters

For agents that don't speak ACP natively, adapters translate between protocols:

### Pi RPC → ACP Adapter

```
Bridge ←→ [pi-rpc-to-acp adapter] ←→ Pi Agent (stdin/stdout)
  ACP          translates              Pi RPC JSON
```

Pi RPC uses custom JSON events over stdin/stdout:
```json
// Pi RPC input
{"type":"user_message","content":"Fix the bug"}

// Pi RPC output
{"type":"assistant_message","content":"Looking at the code..."}
{"type":"tool_use","name":"read_file","input":{"path":"src/auth.ts"}}
```

The adapter maps these to/from ACP session messages.

### Claude Code → ACP Adapter

```
Bridge ←→ [claude-headless-to-acp adapter] ←→ Claude Code CLI
  ACP          translates                      stream-json output
```

Claude Code uses `claude -p "message" --output-format stream-json --resume <session>`:
```json
{"type":"assistant","subtype":"text","text":"Looking at..."}
{"type":"assistant","subtype":"tool_use","tool":"Read","input":{"file_path":"src/auth.ts"}}
{"type":"result","subtype":"success","result":"Done"}
```

The adapter:
1. Receives ACP `message.send`
2. Spawns/resumes `claude -p` with the message
3. Parses stream-json output
4. Emits ACP `message.delta` and `tool.use` events
5. Emits `message.complete` when Claude finishes

## Bridge Implementation (Telegram)

The Telegram bridge is a Node.js process using [grammy](https://grammy.dev/):

### Responsibilities

1. **Listen** for Telegram messages via long-polling or webhook
2. **Filter** by allowed chat IDs
3. **Route** messages to the correct ACP session (create if new)
4. **Forward** agent responses back to Telegram
5. **Handle** commands (/start, /reset, /status)
6. **Stream** long responses (edit message as agent streams)

### Message Flow

```
1. User sends "Fix the login bug" in Telegram
2. Bridge receives update from Telegram API
3. Bridge checks: chat_id in allowed_chats? ✅
4. Bridge looks up session for this chat (or creates new one)
5. Bridge sends ACP: {"type":"message.send","session_id":"chat_123","content":"Fix the login bug"}
6. Agent processes, streams response via ACP message.delta events
7. Bridge sends initial Telegram message, then edits it as deltas arrive
8. Agent completes → Bridge sends final message state
```

### Error Handling

| Scenario | Bridge Behavior |
|----------|----------------|
| Agent crashes | Send error message to user, attempt restart |
| Session timeout | Notify user, offer to start new session |
| Rate limit (Telegram) | Queue messages, respect backoff |
| Unauthorized chat | Silently ignore (log for audit) |
| Bridge restart | Resume existing sessions from ACP session IDs |

## Egress Requirements

The bridge needs these egress rules (auto-added by agent-fleet):

```yaml
# Auto-added for telegram bridge
egress:
  - host: api.telegram.org
    port: 443
    access: full
    # Bridge needs to send/receive messages
```

## Future: MCP over Bridge

When MCP servers need OAuth (Notion, Jira, etc.), the bridge handles the OAuth flow:

```
1. Agent calls MCP tool (e.g., notion.search)
2. MCP request goes through egress proxy
3. Proxy injects OAuth Bearer token (from OpenShell provider)
4. Request reaches Notion API with valid credentials
5. Response returns to agent
```

The bridge doesn't directly handle MCP — it's the egress proxy that injects credentials. But the bridge may need to initiate OAuth re-authorization if tokens expire and refresh fails.
