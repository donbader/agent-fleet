// Package bridge implements the channels-bridge runtime.
// The bridge is PID 1 in the agent container — it spawns the agent process
// and manages channel provider instances.
package bridge

import (
	"context"
	"fmt"
)

// Bridge manages the agent process and channel providers.
type Bridge struct {
	agentProvider string
	channels      []ChannelConfig
}

// ChannelConfig holds configuration for a single channel provider.
type ChannelConfig struct {
	Provider string
	Options  map[string]any
}

// Config holds bridge startup configuration.
type Config struct {
	AgentProvider string
	Channels      []ChannelConfig
}

// New creates a new Bridge instance.
func New(cfg Config) *Bridge {
	return &Bridge{
		agentProvider: cfg.AgentProvider,
		channels:      cfg.Channels,
	}
}

// Run starts the bridge (agent process + channels). Blocks until context is cancelled.
func (b *Bridge) Run(ctx context.Context) error {
	// TODO: implement bridge runtime
	// 1. Start agent process (based on agentProvider)
	// 2. Start channel providers
	// 3. Register commands on channels
	// 4. Route messages between channels and agent (ACP)
	// 5. Handle graceful shutdown on context cancellation
	return fmt.Errorf("bridge runtime not implemented yet")
}
