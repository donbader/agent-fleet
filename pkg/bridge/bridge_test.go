package bridge

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestACPMessage_Encode(t *testing.T) {
	msg := &ACPMessage{
		Type:      MessageTypeUserMessage,
		SessionID: "chat-123",
		Content:   "hello world",
		Metadata:  map[string]string{"sender": "@user"},
	}

	data, err := msg.Encode()
	if err != nil {
		t.Fatalf("Encode() error: %v", err)
	}

	// Should end with newline
	if data[len(data)-1] != '\n' {
		t.Error("encoded message should end with newline")
	}

	// Should be valid JSON
	var decoded ACPMessage
	if err := json.Unmarshal(data[:len(data)-1], &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != MessageTypeUserMessage {
		t.Errorf("Type = %q, want %q", decoded.Type, MessageTypeUserMessage)
	}
	if decoded.SessionID != "chat-123" {
		t.Errorf("SessionID = %q, want %q", decoded.SessionID, "chat-123")
	}
	if decoded.Content != "hello world" {
		t.Errorf("Content = %q, want %q", decoded.Content, "hello world")
	}
}

func TestDecodeACPMessage(t *testing.T) {
	input := `{"type":"agent_response","session_id":"s1","content":"hi"}`

	msg, err := DecodeACPMessage([]byte(input))
	if err != nil {
		t.Fatalf("DecodeACPMessage() error: %v", err)
	}
	if msg.Type != MessageTypeAgentResponse {
		t.Errorf("Type = %q, want %q", msg.Type, MessageTypeAgentResponse)
	}
	if msg.Content != "hi" {
		t.Errorf("Content = %q, want %q", msg.Content, "hi")
	}
}

func TestDecodeACPMessage_Invalid(t *testing.T) {
	_, err := DecodeACPMessage([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// mockChannel implements ChannelProvider for testing.
type mockChannel struct {
	mu         sync.Mutex
	started    bool
	stopped    bool
	commands   map[string]CommandHandler
	msgHandler MessageHandler
	sentMsgs   []sentMsg
	startErr   error
}

type sentMsg struct {
	chatID string
	msg    OutgoingMessage
}

func newMockChannel() *mockChannel {
	return &mockChannel{
		commands: make(map[string]CommandHandler),
	}
}

func (m *mockChannel) Start(ctx context.Context) error {
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

	if m.startErr != nil {
		return m.startErr
	}

	<-ctx.Done()
	return nil
}

func (m *mockChannel) Stop(ctx context.Context) error {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	return nil
}

func (m *mockChannel) RegisterCommand(name string, handler CommandHandler) {
	m.mu.Lock()
	m.commands[name] = handler
	m.mu.Unlock()
}

func (m *mockChannel) OnMessage(handler MessageHandler) {
	m.mu.Lock()
	m.msgHandler = handler
	m.mu.Unlock()
}

func (m *mockChannel) Send(ctx context.Context, chatID string, msg OutgoingMessage) error {
	m.mu.Lock()
	m.sentMsgs = append(m.sentMsgs, sentMsg{chatID, msg})
	m.mu.Unlock()
	return nil
}

func (m *mockChannel) PromptUser(ctx context.Context, chatID string, prompt Prompt) (string, error) {
	return "yes", nil
}

func (m *mockChannel) SimulateMessage(msg IncomingMessage) {
	m.mu.Lock()
	handler := m.msgHandler
	m.mu.Unlock()

	if handler != nil {
		handler(context.Background(), msg)
	}
}

func TestBridge_RegisterCommand(t *testing.T) {
	b := New(Config{
		AgentCommand: []string{"echo", "test"},
		Channels:     []ChannelProvider{newMockChannel()},
	})

	called := false
	b.RegisterCommand("test", func(ctx context.Context, chatID string, args string) error {
		called = true
		return nil
	})

	b.mu.RLock()
	_, exists := b.commands["test"]
	b.mu.RUnlock()

	if !exists {
		t.Error("command 'test' not registered")
	}
	_ = called
}

func TestBridge_ChannelRegistration(t *testing.T) {
	ch := newMockChannel()
	b := New(Config{
		AgentCommand: []string{"cat"}, // cat echoes stdin to stdout
		Channels:     []ChannelProvider{ch},
	})

	// Register a custom command
	b.RegisterCommand("hello", func(ctx context.Context, chatID string, args string) error {
		return nil
	})

	// Run bridge briefly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go b.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	// Verify channel got the commands registered
	ch.mu.Lock()
	_, hasStatus := ch.commands["status"]
	_, hasHello := ch.commands["hello"]
	ch.mu.Unlock()

	if !hasStatus {
		t.Error("channel should have 'status' command registered")
	}
	if !hasHello {
		t.Error("channel should have 'hello' command registered")
	}
}

func TestStartAgent_InvalidCommand(t *testing.T) {
	_, err := StartAgent(context.Background(), AgentConfig{
		Command: []string{},
	})
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestStartAgent_NonexistentCommand(t *testing.T) {
	_, err := StartAgent(context.Background(), AgentConfig{
		Command: []string{"/nonexistent/binary"},
	})
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestStartAgent_CatEcho(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	agent, err := StartAgent(ctx, AgentConfig{
		Command: []string{"cat"},
	})
	if err != nil {
		t.Fatalf("StartAgent() error: %v", err)
	}
	defer agent.Stop()

	if agent.PID() == 0 {
		t.Error("PID should be non-zero")
	}

	// Send a message via ACP
	msg := &ACPMessage{
		Type:      MessageTypeUserMessage,
		SessionID: "test",
		Content:   "hello",
	}
	if err := agent.Send(msg); err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	// Read it back (cat echoes stdin to stdout)
	reply, err := agent.Recv()
	if err != nil {
		t.Fatalf("Recv() error: %v", err)
	}
	if reply.Type != MessageTypeUserMessage {
		t.Errorf("reply.Type = %q, want %q", reply.Type, MessageTypeUserMessage)
	}
	if reply.Content != "hello" {
		t.Errorf("reply.Content = %q, want %q", reply.Content, "hello")
	}
}
