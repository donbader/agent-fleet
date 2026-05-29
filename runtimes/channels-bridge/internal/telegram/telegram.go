// Package telegram implements a Telegram channel provider for the channels-bridge.
// It long-polls the Telegram Bot API, filters messages by allowed users,
// and forwards them to the bridge via the SendFunc callback.
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/donbader/agent-fleet/channels-bridge/internal/bridge"
)

// Config holds Telegram channel configuration.
type Config struct {
	Token        string
	AllowedUsers []string // numeric IDs, @username, or bare username
}

// Channel implements bridge.ChannelProvider for Telegram.
type Channel struct {
	cfg        Config
	client     *http.Client
	send       bridge.SendFunc
	offset     int64
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	pending    map[string]*pendingMsg // chatID → accumulated response
}

type pendingMsg struct {
	text      string
	messageID int64
}

// New creates a new Telegram channel.
func New(cfg Config) *Channel {
	return &Channel{
		cfg:     cfg,
		client:  &http.Client{Timeout: 60 * time.Second},
		pending: make(map[string]*pendingMsg),
	}
}

// Start begins long-polling Telegram for updates.
func (c *Channel) Start(ctx context.Context, send bridge.SendFunc) error {
	c.send = send

	pollCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.pollLoop(pollCtx)

	slog.Info("telegram channel started", "allowed_users", c.cfg.AllowedUsers)
	return nil
}

// Stop gracefully stops the Telegram channel.
func (c *Channel) Stop(_ context.Context) error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	slog.Info("telegram channel stopped")
	return nil
}

// SendResponse accumulates streaming text and edits the Telegram message.
func (c *Channel) SendResponse(ctx context.Context, chatID string, text string) {
	c.mu.Lock()
	p, exists := c.pending[chatID]
	if !exists {
		c.mu.Unlock()
		// Send initial message
		msgID := c.sendMessage(chatID, text)
		c.mu.Lock()
		c.pending[chatID] = &pendingMsg{text: text, messageID: msgID}
		c.mu.Unlock()
		return
	}
	p.text += text
	fullText := p.text
	msgID := p.messageID
	c.mu.Unlock()

	// Edit message with accumulated text
	if msgID > 0 {
		c.editMessage(chatID, msgID, fullText)
	}
}

// CompleteResponse marks the response as complete.
func (c *Channel) CompleteResponse(_ context.Context, chatID string) {
	c.mu.Lock()
	delete(c.pending, chatID)
	c.mu.Unlock()
}

func (c *Channel) pollLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := c.getUpdates(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("getUpdates failed", "error", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, u := range updates {
			if u.UpdateID >= c.offset {
				c.offset = u.UpdateID + 1
			}
			c.handleUpdate(u)
		}
	}
}

func (c *Channel) handleUpdate(u update) {
	msg := u.Message
	if msg == nil {
		return
	}

	// Check allowed users
	if !c.isAllowed(msg.From) {
		slog.Warn("ignoring unauthorized user",
			"username", msg.From.Username,
			"user_id", msg.From.ID,
		)
		return
	}

	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	text := msg.Text
	if text == "" {
		return
	}

	slog.Info("received message",
		"from", msg.From.Username,
		"chat_id", chatID,
		"text_len", len(text),
	)

	// Forward to bridge
	c.send(chatID, text)
}

func (c *Channel) isAllowed(from *user) bool {
	if len(c.cfg.AllowedUsers) == 0 {
		return true // No filter = allow all
	}
	if from == nil {
		return false
	}

	userID := strconv.FormatInt(from.ID, 10)
	username := strings.ToLower(from.Username)

	for _, allowed := range c.cfg.AllowedUsers {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		// Match by numeric ID
		if allowed == userID {
			return true
		}
		// Match by @username or bare username (case-insensitive)
		normalized := strings.ToLower(strings.TrimPrefix(allowed, "@"))
		if normalized == username {
			return true
		}
	}
	return false
}

func (c *Channel) getUpdates(ctx context.Context) ([]update, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30",
		c.cfg.Token, c.offset)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result apiResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram API error: %s", string(body))
	}

	return result.Result, nil
}

func (c *Channel) sendMessage(chatID string, text string) int64 {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.cfg.Token)

	payload := fmt.Sprintf(`{"chat_id":%s,"text":%s}`, chatID, jsonString(text))
	resp, err := c.client.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		slog.Error("sendMessage failed", "error", err)
		return 0
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.OK {
		return result.Result.MessageID
	}
	return 0
}

func (c *Channel) editMessage(chatID string, messageID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", c.cfg.Token)

	payload := fmt.Sprintf(`{"chat_id":%s,"message_id":%d,"text":%s}`,
		chatID, messageID, jsonString(text))
	resp, err := c.client.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		slog.Error("editMessage failed", "error", err)
		return
	}
	resp.Body.Close()
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Telegram API types
type apiResponse struct {
	OK     bool     `json:"ok"`
	Result []update `json:"result"`
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	MessageID int64  `json:"message_id"`
	From      *user  `json:"from"`
	Chat      chat   `json:"chat"`
	Text      string `json:"text"`
}

type user struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type chat struct {
	ID int64 `json:"id"`
}
