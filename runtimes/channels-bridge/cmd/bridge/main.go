// Package main implements the channels-bridge entrypoint.
// It wires up a channel provider (Telegram) with an agent process,
// routing messages between them via ACP protocol.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/donbader/agent-fleet/channels-bridge/internal/bridge"
	"github.com/donbader/agent-fleet/channels-bridge/internal/telegram"
)

func main() {
	// Read configuration from environment
	agentCmd := os.Getenv("AGENT_CMD")
	if agentCmd == "" {
		agentCmd = "codex"
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		// Use dummy token — real token injected by proxy
		botToken = "000000000:DUMMY"
	}

	allowedUsers := parseList(os.Getenv("TELEGRAM_ALLOWED_USERS"))

	slog.Info("channels-bridge starting",
		"agent_cmd", agentCmd,
		"allowed_users", allowedUsers,
	)

	// Create Telegram channel provider
	channel := telegram.New(telegram.Config{
		Token:        botToken,
		AllowedUsers: allowedUsers,
	})

	// Create and run bridge
	b := bridge.New(bridge.Config{
		AgentCmd: agentCmd,
		Channel:  channel,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := b.Run(ctx); err != nil {
		slog.Error("bridge exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("channels-bridge stopped")
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
