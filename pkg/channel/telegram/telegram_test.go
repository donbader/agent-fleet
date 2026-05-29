package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/donbader/agent-fleet/pkg/bridge"
)

func TestProvider_ImplementsChannelProvider(t *testing.T) {
	// Compile-time check that Provider implements bridge.ChannelProvider
	var _ bridge.ChannelProvider = (*Provider)(nil)
}

func TestProvider_RegisterCommand(t *testing.T) {
	p := New(Config{Token: "test"})

	called := false
	p.RegisterCommand("hello", func(ctx context.Context, chatID string, args string) error {
		called = true
		return nil
	})

	p.mu.RLock()
	_, exists := p.commands["hello"]
	p.mu.RUnlock()

	if !exists {
		t.Error("command 'hello' not registered")
	}
	_ = called
}

func TestProvider_IsAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		user         *User
		want         bool
	}{
		{"nil user", []string{"@admin"}, nil, false},
		{"allowed user", []string{"@admin"}, &User{Username: "admin"}, true},
		{"case insensitive", []string{"@Admin"}, &User{Username: "admin"}, true},
		{"not allowed", []string{"@admin"}, &User{Username: "hacker"}, false},
		{"empty list allows all", []string{}, &User{Username: "anyone"}, true},
		{"multiple allowed", []string{"@a", "@b"}, &User{Username: "b"}, true},
		{"numeric ID match", []string{"123456"}, &User{ID: 123456, Username: "someone"}, true},
		{"numeric ID no match", []string{"999"}, &User{ID: 123456, Username: "someone"}, false},
		{"username without @", []string{"admin"}, &User{Username: "admin"}, true},
		{"mixed list ID match", []string{"@bob", "123456"}, &User{ID: 123456, Username: "alice"}, true},
		{"mixed list username match", []string{"@bob", "123456"}, &User{ID: 999, Username: "bob"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(Config{Token: "test", AllowedUsers: tt.allowedUsers})
			got := p.isAllowed(tt.user)
			if got != tt.want {
				t.Errorf("isAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_HandleUpdate_Command(t *testing.T) {
	p := New(Config{Token: "test", AllowedUsers: []string{"@user1"}})

	var gotChatID, gotArgs string
	p.RegisterCommand("status", func(ctx context.Context, chatID string, args string) error {
		gotChatID = chatID
		gotArgs = args
		return nil
	})

	update := Update{
		UpdateID: 1,
		Message: &Message{
			MessageID: 1,
			From:      &User{Username: "user1"},
			Chat:      &Chat{ID: 123},
			Text:      "/status some args",
		},
	}

	p.handleUpdate(context.Background(), update)

	if gotChatID != "123" {
		t.Errorf("chatID = %q, want %q", gotChatID, "123")
	}
	if gotArgs != "some args" {
		t.Errorf("args = %q, want %q", gotArgs, "some args")
	}
}

func TestProvider_HandleUpdate_CommandWithBotSuffix(t *testing.T) {
	p := New(Config{Token: "test", AllowedUsers: []string{"@user1"}})

	var called bool
	p.RegisterCommand("status", func(ctx context.Context, chatID string, args string) error {
		called = true
		return nil
	})

	update := Update{
		UpdateID: 1,
		Message: &Message{
			From: &User{Username: "user1"},
			Chat: &Chat{ID: 1},
			Text: "/status@mybot",
		},
	}

	p.handleUpdate(context.Background(), update)

	if !called {
		t.Error("command handler not called for /status@mybot")
	}
}

func TestProvider_HandleUpdate_RegularMessage(t *testing.T) {
	p := New(Config{Token: "test", AllowedUsers: []string{"@user1"}})

	var gotMsg bridge.IncomingMessage
	p.OnMessage(func(ctx context.Context, msg bridge.IncomingMessage) {
		gotMsg = msg
	})

	update := Update{
		UpdateID: 1,
		Message: &Message{
			From: &User{Username: "user1"},
			Chat: &Chat{ID: 456},
			Text: "hello world",
		},
	}

	p.handleUpdate(context.Background(), update)

	if gotMsg.ChatID != "456" {
		t.Errorf("ChatID = %q, want %q", gotMsg.ChatID, "456")
	}
	if gotMsg.Text != "hello world" {
		t.Errorf("Text = %q, want %q", gotMsg.Text, "hello world")
	}
	if gotMsg.Platform != "telegram" {
		t.Errorf("Platform = %q, want %q", gotMsg.Platform, "telegram")
	}
}

func TestProvider_HandleUpdate_UnauthorizedUser(t *testing.T) {
	p := New(Config{Token: "test", AllowedUsers: []string{"@admin"}})

	called := false
	p.OnMessage(func(ctx context.Context, msg bridge.IncomingMessage) {
		called = true
	})

	update := Update{
		UpdateID: 1,
		Message: &Message{
			From: &User{Username: "hacker"},
			Chat: &Chat{ID: 1},
			Text: "hello",
		},
	}

	p.handleUpdate(context.Background(), update)

	if called {
		t.Error("message handler should not be called for unauthorized user")
	}
}

func TestProvider_Send(t *testing.T) {
	var gotBody SendMessageRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendMessage" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(SendMessageResponse{OK: true})
	}))
	defer server.Close()

	p := New(Config{Token: "test", BaseURL: server.URL})

	err := p.Send(context.Background(), "123", bridge.OutgoingMessage{
		Text:     "hello",
		Markdown: true,
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	if gotBody.ChatID != "123" {
		t.Errorf("ChatID = %q, want %q", gotBody.ChatID, "123")
	}
	if gotBody.Text != "hello" {
		t.Errorf("Text = %q, want %q", gotBody.Text, "hello")
	}
	if gotBody.ParseMode != "MarkdownV2" {
		t.Errorf("ParseMode = %q, want %q", gotBody.ParseMode, "MarkdownV2")
	}
}

func TestProvider_GetUpdates(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(GetUpdatesResponse{
			OK: true,
			Result: []Update{
				{
					UpdateID: 42,
					Message: &Message{
						From: &User{Username: "user1"},
						Chat: &Chat{ID: 1},
						Text: "hi",
					},
				},
			},
		})
	}))
	defer server.Close()

	p := New(Config{Token: "test", BaseURL: server.URL, PollTimeout: 1})

	updates, err := p.getUpdates(context.Background())
	if err != nil {
		t.Fatalf("getUpdates() error: %v", err)
	}

	if len(updates) != 1 {
		t.Fatalf("updates count = %d, want 1", len(updates))
	}
	if updates[0].UpdateID != 42 {
		t.Errorf("UpdateID = %d, want 42", updates[0].UpdateID)
	}
}

func TestProvider_StartAndStop(t *testing.T) {
	var mu sync.Mutex
	var messages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/getUpdates" {
			json.NewEncoder(w).Encode(GetUpdatesResponse{
				OK: true,
				Result: []Update{
					{
						UpdateID: 1,
						Message: &Message{
							From: &User{Username: "user1"},
							Chat: &Chat{ID: 1},
							Text: "hello",
						},
					},
				},
			})
		} else {
			json.NewEncoder(w).Encode(SendMessageResponse{OK: true})
		}
	}))
	defer server.Close()

	p := New(Config{
		Token:        "test",
		BaseURL:      server.URL,
		AllowedUsers: []string{"@user1"},
		PollTimeout:  1,
	})

	p.OnMessage(func(ctx context.Context, msg bridge.IncomingMessage) {
		mu.Lock()
		messages = append(messages, msg.Text)
		mu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := p.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	mu.Lock()
	count := len(messages)
	mu.Unlock()

	if count == 0 {
		t.Error("expected at least one message to be processed")
	}
}
