// Package bridge implements the channels-bridge runtime.
// It spawns an agent process and routes messages between
// a channel provider and the agent via ACP protocol.
package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
)

// ChannelProvider is the interface that channel implementations must satisfy.
type ChannelProvider interface {
	Start(ctx context.Context, send SendFunc) error
	Stop(ctx context.Context) error
}

// SendFunc is called by the channel to deliver a user message to the bridge.
type SendFunc func(chatID string, text string)

// Config holds bridge configuration.
type Config struct {
	AgentCmd string
	Channel  ChannelProvider
}

// Bridge routes messages between a channel and an agent process.
type Bridge struct {
	cfg      Config
	agent    *exec.Cmd
	agentIn  io.WriteCloser
	agentOut *bufio.Scanner
	mu       sync.Mutex
	sessions map[string]string // chatID → sessionID
}

// New creates a new Bridge.
func New(cfg Config) *Bridge {
	return &Bridge{
		cfg:      cfg,
		sessions: make(map[string]string),
	}
}

// Run starts the bridge: spawns the agent, starts the channel, and routes messages.
func (b *Bridge) Run(ctx context.Context) error {
	// Spawn agent process
	if err := b.spawnAgent(); err != nil {
		return fmt.Errorf("spawning agent: %w", err)
	}
	defer b.stopAgent()

	slog.Info("agent spawned", "cmd", b.cfg.AgentCmd, "pid", b.agent.Process.Pid)

	// Start reading agent output in background
	agentMessages := make(chan ACPMessage, 64)
	go b.readAgentOutput(agentMessages)

	// Start channel provider
	if err := b.cfg.Channel.Start(ctx, b.handleUserMessage); err != nil {
		return fmt.Errorf("starting channel: %w", err)
	}
	defer func() {
		if err := b.cfg.Channel.Stop(context.Background()); err != nil {
			slog.Error("channel stop error", "error", err)
		}
	}()

	slog.Info("channel started, bridge is ready")

	// Main loop: route agent responses back to channel
	for {
		select {
		case <-ctx.Done():
			slog.Info("bridge shutting down")
			return nil
		case msg, ok := <-agentMessages:
			if !ok {
				slog.Info("agent output closed")
				return nil
			}
			b.routeAgentMessage(ctx, msg)
		}
	}
}

// handleUserMessage is called by the channel when a user sends a message.
func (b *Bridge) handleUserMessage(chatID string, text string) {
	b.mu.Lock()
	sessionID, exists := b.sessions[chatID]
	if !exists {
		sessionID = chatID // Use chatID as sessionID for simplicity
		b.sessions[chatID] = sessionID
	}
	b.mu.Unlock()

	// Send session.start for new sessions
	if !exists {
		start := ACPMessage{
			Type:      "session.start",
			SessionID: sessionID,
		}
		if err := b.sendToAgent(start); err != nil {
			slog.Error("failed to start session", "error", err)
			return
		}
	}

	// Send message to agent
	msg := ACPMessage{
		Type:      "message.send",
		SessionID: sessionID,
		Content:   text,
	}
	if err := b.sendToAgent(msg); err != nil {
		slog.Error("failed to send to agent", "error", err)
	}
}

// routeAgentMessage sends an agent response back to the appropriate chat.
func (b *Bridge) routeAgentMessage(ctx context.Context, msg ACPMessage) {
	if msg.SessionID == "" {
		return
	}

	// Find chatID for this session
	b.mu.Lock()
	var chatID string
	for cid, sid := range b.sessions {
		if sid == msg.SessionID {
			chatID = cid
			break
		}
	}
	b.mu.Unlock()

	if chatID == "" {
		slog.Warn("no chat for session", "session_id", msg.SessionID)
		return
	}

	switch msg.Type {
	case "message.delta":
		// Streaming text — channel handles accumulation
		if sender, ok := b.cfg.Channel.(ResponseSender); ok {
			sender.SendResponse(ctx, chatID, msg.Delta)
		}
	case "message.complete":
		if sender, ok := b.cfg.Channel.(ResponseCompleter); ok {
			sender.CompleteResponse(ctx, chatID)
		}
	}
}

func (b *Bridge) spawnAgent() error {
	b.agent = exec.Command("sh", "-c", b.cfg.AgentCmd)

	var err error
	b.agentIn, err = b.agent.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := b.agent.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	b.agentOut = bufio.NewScanner(stdout)

	// Inherit stderr so agent logs are visible
	b.agent.Stderr = nil // goes to container stderr

	if err := b.agent.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	return nil
}

func (b *Bridge) stopAgent() {
	if b.agentIn != nil {
		b.agentIn.Close()
	}
	if b.agent != nil && b.agent.Process != nil {
		_ = b.agent.Process.Kill()
		_ = b.agent.Wait()
	}
}

func (b *Bridge) sendToAgent(msg ACPMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	b.mu.Lock()
	defer b.mu.Unlock()
	_, err = b.agentIn.Write(data)
	return err
}

func (b *Bridge) readAgentOutput(ch chan<- ACPMessage) {
	defer close(ch)
	for b.agentOut.Scan() {
		var msg ACPMessage
		if err := json.Unmarshal(b.agentOut.Bytes(), &msg); err != nil {
			slog.Warn("invalid ACP message from agent", "error", err, "line", b.agentOut.Text())
			continue
		}
		ch <- msg
	}
}

// ResponseSender is an optional interface for channels that support streaming.
type ResponseSender interface {
	SendResponse(ctx context.Context, chatID string, text string)
}

// ResponseCompleter is an optional interface for channels that track message completion.
type ResponseCompleter interface {
	CompleteResponse(ctx context.Context, chatID string)
}
