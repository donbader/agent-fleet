package bridge

import "context"

// ChannelProvider is the interface that messaging channel providers must implement.
// Channels handle platform-specific communication (Telegram, Slack, etc.)
// while the bridge handles routing and command dispatch.
type ChannelProvider interface {
	// Start connects to the platform and begins listening for messages.
	// Blocks until context is cancelled.
	Start(ctx context.Context) error

	// Stop gracefully disconnects from the platform.
	Stop(ctx context.Context) error

	// RegisterCommand registers a command handler.
	// The bridge registers commands at startup; the channel dispatches them.
	RegisterCommand(name string, handler CommandHandler)

	// OnMessage sets the handler for non-command messages.
	OnMessage(handler MessageHandler)

	// Send sends a message to a specific chat.
	Send(ctx context.Context, chatID string, msg OutgoingMessage) error

	// PromptUser sends an interactive prompt and blocks until the user replies.
	// Used for OAuth flows, confirmations, etc.
	PromptUser(ctx context.Context, chatID string, prompt Prompt) (string, error)
}

// CommandHandler handles a slash command from a user.
type CommandHandler func(ctx context.Context, chatID string, args string) error

// MessageHandler handles a non-command message from a user.
type MessageHandler func(ctx context.Context, msg IncomingMessage)

// IncomingMessage represents a message received from a channel.
type IncomingMessage struct {
	ChatID   string
	Sender   string
	Text     string
	Platform string
}

// OutgoingMessage represents a message to send through a channel.
type OutgoingMessage struct {
	Text     string
	Markdown bool
}

// Prompt represents an interactive prompt sent to a user.
type Prompt struct {
	Text    string
	Options []string // If non-empty, show as buttons/choices
}
