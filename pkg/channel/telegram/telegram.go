// Package telegram implements a Telegram channel provider for the bridge.
// It long-polls the Telegram Bot API for updates and dispatches messages
// to the bridge via the ChannelProvider interface.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/donbader/agent-fleet/pkg/bridge"
)

// Config holds Telegram channel provider configuration.
type Config struct {
	// Token is the bot token (may be dummy — proxy injects real one).
	Token string

	// AllowedUsers is the list of usernames allowed to interact (e.g., ["@myuser"]).
	AllowedUsers []string

	// BaseURL overrides the Telegram API base URL (for testing).
	BaseURL string

	// PollTimeout is the long-poll timeout in seconds (default: 30).
	PollTimeout int
}

// Provider implements bridge.ChannelProvider for Telegram.
type Provider struct {
	cfg            Config
	client         *http.Client
	baseURL        string
	commands       map[string]bridge.CommandHandler
	msgHandler     bridge.MessageHandler
	pendingPrompts map[string]chan string
	mu             sync.RWMutex
	offset         int
}

// New creates a new Telegram channel provider.
func New(cfg Config) *Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://api.telegram.org/bot%s", cfg.Token)
	}

	pollTimeout := cfg.PollTimeout
	if pollTimeout == 0 {
		pollTimeout = 30
	}

	return &Provider{
		cfg:            cfg,
		baseURL:        baseURL,
		client: &http.Client{
			Timeout: time.Duration(pollTimeout+5) * time.Second,
		},
		commands:       make(map[string]bridge.CommandHandler),
		pendingPrompts: make(map[string]chan string),
	}
}

// Start begins long-polling for updates. Blocks until context is cancelled.
func (p *Provider) Start(ctx context.Context) error {
	log.Printf("[telegram] starting long-poll (allowed users: %v)", p.cfg.AllowedUsers)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		updates, err := p.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // context cancelled
			}
			log.Printf("[telegram] getUpdates error: %v", err)
			time.Sleep(time.Second) // backoff on error
			continue
		}

		for _, update := range updates {
			p.offset = update.UpdateID + 1
			p.handleUpdate(ctx, update)
		}
	}
}

// Stop gracefully stops the provider.
func (p *Provider) Stop(ctx context.Context) error {
	log.Printf("[telegram] stopped")
	return nil
}

// RegisterCommand registers a command handler.
func (p *Provider) RegisterCommand(name string, handler bridge.CommandHandler) {
	p.mu.Lock()
	p.commands[name] = handler
	p.mu.Unlock()
}

// OnMessage sets the handler for non-command messages.
func (p *Provider) OnMessage(handler bridge.MessageHandler) {
	p.mu.Lock()
	p.msgHandler = handler
	p.mu.Unlock()
}

// Send sends a message to a Telegram chat.
func (p *Provider) Send(ctx context.Context, chatID string, msg bridge.OutgoingMessage) error {
	parseMode := ""
	if msg.Markdown {
		parseMode = "MarkdownV2"
	}

	body := SendMessageRequest{
		ChatID:    chatID,
		Text:      msg.Text,
		ParseMode: parseMode,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal sendMessage: %w", err)
	}

	url := p.baseURL + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("sendMessage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sendMessage status %d: %s", resp.StatusCode, respBody)
	}

	return nil
}

// PromptUser sends a prompt and waits for the user's reply.
func (p *Provider) PromptUser(ctx context.Context, chatID string, prompt bridge.Prompt) (string, error) {
	// Send the prompt
	if err := p.Send(ctx, chatID, bridge.OutgoingMessage{Text: prompt.Text}); err != nil {
		return "", err
	}

	// Create a channel to wait for the reply
	replyCh := make(chan string, 1)
	p.mu.Lock()
	p.pendingPrompts[chatID] = replyCh
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.pendingPrompts, chatID)
		p.mu.Unlock()
	}()

	// Wait for reply or context cancellation
	select {
	case reply := <-replyCh:
		return reply, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// getUpdates calls the Telegram getUpdates API.
func (p *Provider) getUpdates(ctx context.Context) ([]Update, error) {
	timeout := p.cfg.PollTimeout
	if timeout == 0 {
		timeout = 30
	}

	url := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=%d", p.baseURL, p.offset, timeout)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getUpdates status %d: %s", resp.StatusCode, body)
	}

	var result GetUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode getUpdates: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("getUpdates returned ok=false")
	}

	return result.Result, nil
}

// handleUpdate processes a single Telegram update.
func (p *Provider) handleUpdate(ctx context.Context, update Update) {
	msg := update.Message
	if msg == nil || msg.Text == "" {
		return
	}

	// Check if user is allowed
	if !p.isAllowed(msg.From) {
		log.Printf("[telegram] ignoring message from unauthorized user: @%s", msg.From.Username)
		return
	}

	chatID := strconv.FormatInt(msg.Chat.ID, 10)

	// Check if there's a pending prompt for this chat
	p.mu.RLock()
	replyCh, hasPending := p.pendingPrompts[chatID]
	p.mu.RUnlock()

	if hasPending {
		// Send reply to the waiting PromptUser call
		select {
		case replyCh <- msg.Text:
		default:
		}
		return
	}

	// Check if this is a command
	if strings.HasPrefix(msg.Text, "/") {
		p.handleCommand(ctx, chatID, msg.Text)
		return
	}

	// Forward as regular message
	p.mu.RLock()
	handler := p.msgHandler
	p.mu.RUnlock()

	if handler != nil {
		handler(ctx, bridge.IncomingMessage{
			ChatID:   chatID,
			Sender:   fmt.Sprintf("@%s", msg.From.Username),
			Text:     msg.Text,
			Platform: "telegram",
		})
	}
}

// handleCommand dispatches a slash command.
func (p *Provider) handleCommand(ctx context.Context, chatID string, text string) {
	// Parse command: "/command args"
	parts := strings.SplitN(text, " ", 2)
	cmdName := strings.TrimPrefix(parts[0], "/")
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	// Strip @botname suffix (e.g., "/status@mybot" → "status")
	if idx := strings.Index(cmdName, "@"); idx != -1 {
		cmdName = cmdName[:idx]
	}

	p.mu.RLock()
	handler, exists := p.commands[cmdName]
	p.mu.RUnlock()

	if !exists {
		log.Printf("[telegram] unknown command: /%s", cmdName)
		return
	}

	if err := handler(ctx, chatID, args); err != nil {
		log.Printf("[telegram] command /%s error: %v", cmdName, err)
		_ = p.Send(ctx, chatID, bridge.OutgoingMessage{
			Text: fmt.Sprintf("Error: %v", err),
		})
	}
}

// isAllowed checks if a user is in the allowed list.
func (p *Provider) isAllowed(user *User) bool {
	if user == nil {
		return false
	}
	if len(p.cfg.AllowedUsers) == 0 {
		return true // no filter = allow all
	}

	username := "@" + user.Username
	for _, allowed := range p.cfg.AllowedUsers {
		if strings.EqualFold(allowed, username) {
			return true
		}
	}
	return false
}
