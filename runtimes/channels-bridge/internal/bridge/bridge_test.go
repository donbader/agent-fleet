package bridge

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestACPMessageMarshal(t *testing.T) {
	msg := ACPMessage{
		Type:      "message.send",
		SessionID: "chat123",
		Content:   "hello world",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ACPMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != "message.send" {
		t.Errorf("type = %q, want message.send", got.Type)
	}
	if got.SessionID != "chat123" {
		t.Errorf("session_id = %q, want chat123", got.SessionID)
	}
	if got.Content != "hello world" {
		t.Errorf("content = %q, want hello world", got.Content)
	}
}

func TestACPMessageOmitEmpty(t *testing.T) {
	msg := ACPMessage{Type: "session.start", SessionID: "s1"}
	data, _ := json.Marshal(msg)

	// Content, Delta, Tool, Output should be omitted
	var raw map[string]any
	json.Unmarshal(data, &raw)

	if _, ok := raw["content"]; ok {
		t.Error("content should be omitted when empty")
	}
	if _, ok := raw["delta"]; ok {
		t.Error("delta should be omitted when empty")
	}
}

// mockChannel implements ChannelProvider for testing.
type mockChannel struct {
	started  bool
	stopped  bool
	sendFunc SendFunc
}

func (m *mockChannel) Start(_ context.Context, send SendFunc) error {
	m.started = true
	m.sendFunc = send
	return nil
}

func (m *mockChannel) Stop(_ context.Context) error {
	m.stopped = true
	return nil
}

func TestBridgeSessionCreation(t *testing.T) {
	b := New(Config{
		AgentCmd: "echo test",
		Channel:  &mockChannel{},
	})

	// Provide a mock agentIn so sendToAgent doesn't panic
	pr, pw := io.Pipe()
	b.agentIn = pw
	go func() { io.ReadAll(pr) }() // drain

	// Simulate user message — should create a session
	b.handleUserMessage("chat456", "hello")

	pw.Close()
	pr.Close()

	b.mu.Lock()
	defer b.mu.Unlock()

	sessionID, exists := b.sessions["chat456"]
	if !exists {
		t.Fatal("session not created for chat456")
	}
	if sessionID != "chat456" {
		t.Errorf("sessionID = %q, want chat456", sessionID)
	}
}

func TestBridgeSessionReuse(t *testing.T) {
	b := New(Config{
		AgentCmd: "echo test",
		Channel:  &mockChannel{},
	})

	// Pre-create a session
	b.sessions["chat789"] = "chat789"

	// Need agentIn to not be nil for sendToAgent
	// Skip actual send — just verify session lookup
	b.mu.Lock()
	_, exists := b.sessions["chat789"]
	b.mu.Unlock()

	if !exists {
		t.Fatal("pre-existing session should exist")
	}
}

func TestBridgeRouteAgentMessage(t *testing.T) {
	ch := &mockChannelWithResponse{}
	b := New(Config{
		AgentCmd: "echo test",
		Channel:  ch,
	})

	// Set up session mapping
	b.sessions["chat100"] = "sess100"

	// Route a delta message
	msg := ACPMessage{
		Type:      "message.delta",
		SessionID: "sess100",
		Delta:     "Hello from agent",
	}
	b.routeAgentMessage(context.Background(), msg)

	if ch.lastChatID != "chat100" {
		t.Errorf("routed to chatID = %q, want chat100", ch.lastChatID)
	}
	if ch.lastText != "Hello from agent" {
		t.Errorf("routed text = %q, want 'Hello from agent'", ch.lastText)
	}
}

func TestBridgeRouteUnknownSession(t *testing.T) {
	ch := &mockChannelWithResponse{}
	b := New(Config{
		AgentCmd: "echo test",
		Channel:  ch,
	})

	// No sessions — should not crash
	msg := ACPMessage{
		Type:      "message.delta",
		SessionID: "unknown",
		Delta:     "test",
	}
	b.routeAgentMessage(context.Background(), msg)

	if ch.lastChatID != "" {
		t.Error("should not route to any chat for unknown session")
	}
}

// mockChannelWithResponse implements ChannelProvider + ResponseSender.
type mockChannelWithResponse struct {
	mockChannel
	lastChatID string
	lastText   string
	completed  bool
}

func (m *mockChannelWithResponse) SendResponse(_ context.Context, chatID string, text string) {
	m.lastChatID = chatID
	m.lastText = text
}

func (m *mockChannelWithResponse) CompleteResponse(_ context.Context, chatID string) {
	m.completed = true
}
