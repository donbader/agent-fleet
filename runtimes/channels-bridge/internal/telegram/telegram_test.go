package telegram

import (
	"testing"
)

func TestIsAllowed_NoFilter(t *testing.T) {
	c := &Channel{cfg: Config{AllowedUsers: nil}}

	// No filter = allow all
	if !c.isAllowed(&user{ID: 123, Username: "anyone"}) {
		t.Error("should allow all when no filter set")
	}
}

func TestIsAllowed_ByNumericID(t *testing.T) {
	c := &Channel{cfg: Config{AllowedUsers: []string{"12345"}}}

	if !c.isAllowed(&user{ID: 12345, Username: "bob"}) {
		t.Error("should allow by numeric ID")
	}
	if c.isAllowed(&user{ID: 99999, Username: "bob"}) {
		t.Error("should deny non-matching ID")
	}
}

func TestIsAllowed_ByAtUsername(t *testing.T) {
	c := &Channel{cfg: Config{AllowedUsers: []string{"@alice"}}}

	if !c.isAllowed(&user{ID: 1, Username: "alice"}) {
		t.Error("should allow @alice")
	}
	if !c.isAllowed(&user{ID: 1, Username: "Alice"}) {
		t.Error("should be case-insensitive")
	}
	if c.isAllowed(&user{ID: 1, Username: "bob"}) {
		t.Error("should deny non-matching username")
	}
}

func TestIsAllowed_ByBareUsername(t *testing.T) {
	c := &Channel{cfg: Config{AllowedUsers: []string{"charlie"}}}

	if !c.isAllowed(&user{ID: 1, Username: "charlie"}) {
		t.Error("should allow bare username")
	}
	if !c.isAllowed(&user{ID: 1, Username: "Charlie"}) {
		t.Error("should be case-insensitive")
	}
}

func TestIsAllowed_NilUser(t *testing.T) {
	c := &Channel{cfg: Config{AllowedUsers: []string{"@alice"}}}

	if c.isAllowed(nil) {
		t.Error("should deny nil user")
	}
}

func TestIsAllowed_MultipleUsers(t *testing.T) {
	c := &Channel{cfg: Config{AllowedUsers: []string{"@alice", "12345", "bob"}}}

	tests := []struct {
		name    string
		u       *user
		allowed bool
	}{
		{"alice by username", &user{ID: 1, Username: "alice"}, true},
		{"user by ID", &user{ID: 12345, Username: "unknown"}, true},
		{"bob by bare name", &user{ID: 2, Username: "bob"}, true},
		{"unauthorized", &user{ID: 999, Username: "eve"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.isAllowed(tt.u); got != tt.allowed {
				t.Errorf("isAllowed() = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestHandleUpdate_FiltersUnauthorized(t *testing.T) {
	var received []string
	c := &Channel{
		cfg:     Config{AllowedUsers: []string{"@alice"}},
		pending: make(map[string]*pendingMsg),
		send: func(chatID string, text string) {
			received = append(received, text)
		},
	}

	// Authorized user
	c.handleUpdate(update{
		UpdateID: 1,
		Message: &message{
			From: &user{ID: 1, Username: "alice"},
			Chat: chat{ID: 100},
			Text: "hello",
		},
	})

	// Unauthorized user
	c.handleUpdate(update{
		UpdateID: 2,
		Message: &message{
			From: &user{ID: 2, Username: "eve"},
			Chat: chat{ID: 200},
			Text: "hack",
		},
	})

	if len(received) != 1 {
		t.Fatalf("expected 1 message forwarded, got %d", len(received))
	}
	if received[0] != "hello" {
		t.Errorf("forwarded text = %q, want hello", received[0])
	}
}

func TestHandleUpdate_IgnoresEmptyText(t *testing.T) {
	var received []string
	c := &Channel{
		cfg:     Config{AllowedUsers: nil}, // allow all
		pending: make(map[string]*pendingMsg),
		send: func(chatID string, text string) {
			received = append(received, text)
		},
	}

	c.handleUpdate(update{
		UpdateID: 1,
		Message: &message{
			From: &user{ID: 1, Username: "alice"},
			Chat: chat{ID: 100},
			Text: "", // empty
		},
	})

	if len(received) != 0 {
		t.Error("should not forward empty text messages")
	}
}

func TestHandleUpdate_IgnoresNilMessage(t *testing.T) {
	var received []string
	c := &Channel{
		cfg:     Config{AllowedUsers: nil},
		pending: make(map[string]*pendingMsg),
		send: func(chatID string, text string) {
			received = append(received, text)
		},
	}

	c.handleUpdate(update{UpdateID: 1, Message: nil})

	if len(received) != 0 {
		t.Error("should not forward nil message updates")
	}
}

func TestJsonString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", `"hello"`},
		{`has "quotes"`, `"has \"quotes\""`},
		{"has\nnewline", `"has\nnewline"`},
	}

	for _, tt := range tests {
		got := jsonString(tt.input)
		if got != tt.want {
			t.Errorf("jsonString(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
