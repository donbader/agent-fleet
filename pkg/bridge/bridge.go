// Package bridge implements the channels-bridge runtime.
// The bridge is PID 1 in the agent container — it spawns the agent process
// and manages channel provider instances, routing messages via ACP protocol.
package bridge

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// Bridge manages the agent process and channel providers.
type Bridge struct {
	cfg      Config
	agent    *AgentProcess
	channels []ChannelProvider
	commands map[string]CommandHandler
	mu       sync.RWMutex
}

// Config holds bridge startup configuration.
type Config struct {
	// AgentCommand is the command to spawn the agent process.
	AgentCommand []string

	// AgentDir is the working directory for the agent process.
	AgentDir string

	// AgentEnv is additional environment variables for the agent.
	AgentEnv map[string]string

	// Channels are the channel provider instances to manage.
	Channels []ChannelProvider
}

// New creates a new Bridge instance.
func New(cfg Config) *Bridge {
	return &Bridge{
		cfg:      cfg,
		channels: cfg.Channels,
		commands: make(map[string]CommandHandler),
	}
}

// RegisterCommand registers a command handler on all channels.
func (b *Bridge) RegisterCommand(name string, handler CommandHandler) {
	b.mu.Lock()
	b.commands[name] = handler
	b.mu.Unlock()
}

// Run starts the bridge (agent process + channels). Blocks until context is cancelled.
func (b *Bridge) Run(ctx context.Context) error {
	// Register built-in commands
	b.RegisterCommand("status", b.handleStatus)

	// Start the agent process
	agent, err := StartAgent(ctx, AgentConfig{
		Command: b.cfg.AgentCommand,
		Dir:     b.cfg.AgentDir,
		Env:     b.cfg.AgentEnv,
	})
	if err != nil {
		return fmt.Errorf("starting agent: %w", err)
	}
	b.agent = agent

	log.Printf("[bridge] agent started (pid %d)", agent.PID())

	// Start all channel providers
	var wg sync.WaitGroup
	errCh := make(chan error, len(b.channels)+1)

	for i, ch := range b.channels {
		// Register commands on each channel
		for name, handler := range b.commands {
			ch.RegisterCommand(name, handler)
		}

		// Set up message routing: channel → agent
		ch.OnMessage(func(ctx context.Context, msg IncomingMessage) {
			b.routeToAgent(ctx, msg)
		})

		wg.Add(1)
		go func(idx int, channel ChannelProvider) {
			defer wg.Done()
			if err := channel.Start(ctx); err != nil {
				errCh <- fmt.Errorf("channel %d: %w", idx, err)
			}
		}(i, ch)
	}

	log.Printf("[bridge] %d channel(s) started", len(b.channels))

	// Wait for agent to exit or context cancellation
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := agent.Wait(); err != nil {
			errCh <- fmt.Errorf("agent exited: %w", err)
		}
	}()

	// Wait for context cancellation or first error
	select {
	case <-ctx.Done():
		log.Printf("[bridge] shutting down...")
	case err := <-errCh:
		log.Printf("[bridge] error: %v", err)
	}

	// Graceful shutdown
	b.shutdown()
	wg.Wait()

	return nil
}

// routeToAgent sends an incoming message to the agent via ACP.
func (b *Bridge) routeToAgent(ctx context.Context, msg IncomingMessage) {
	if b.agent == nil {
		return
	}

	acpMsg := &ACPMessage{
		Type:      MessageTypeUserMessage,
		SessionID: msg.ChatID,
		Content:   msg.Text,
		Metadata: map[string]string{
			"sender":   msg.Sender,
			"platform": msg.Platform,
		},
	}

	if err := b.agent.Send(acpMsg); err != nil {
		log.Printf("[bridge] failed to send to agent: %v", err)
	}
}

// shutdown gracefully stops channels and agent.
func (b *Bridge) shutdown() {
	// Stop channels
	for _, ch := range b.channels {
		if err := ch.Stop(context.Background()); err != nil {
			log.Printf("[bridge] channel stop error: %v", err)
		}
	}

	// Stop agent
	if b.agent != nil {
		b.agent.Stop()
	}
}

// handleStatus is the built-in /status command.
func (b *Bridge) handleStatus(ctx context.Context, chatID string, args string) error {
	status := fmt.Sprintf("Bridge status:\n- Agent: running (pid %d)\n- Channels: %d active",
		b.agent.PID(), len(b.channels))

	// Send status back through all channels (find the one with this chatID)
	for _, ch := range b.channels {
		_ = ch.Send(ctx, chatID, OutgoingMessage{Text: status})
	}
	return nil
}
