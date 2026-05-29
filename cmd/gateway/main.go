// Command gateway runs the transparent egress proxy as a standalone process.
// It is the entrypoint for the gateway Docker container.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/donbader/agent-fleet/pkg/config"
	"github.com/donbader/agent-fleet/pkg/gateway"
	"gopkg.in/yaml.v3"
)

// gatewayConfig is the config file format for the gateway container.
type gatewayConfig struct {
	Rules []config.EgressRule `yaml:"rules"`
}

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	configPath := envOr("GATEWAY_CONFIG", "/etc/gateway/rules.yaml")

	// Load rules from config file, or default to allow-all
	rules := loadRules(configPath)

	// Build gateway config using a single "default" preset containing all rules
	cfg := gateway.Config{
		ListenAddr:    listenAddr,
		Presets:       map[string]config.EgressPreset{"default": rules},
		ActivePresets: []string{"default"},
	}

	gw, err := gateway.New(cfg)
	if err != nil {
		log.Fatalf("[gateway] init: %v", err)
	}

	// Graceful shutdown on SIGTERM/SIGINT
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Printf("[gateway] received %s, shutting down", sig)
		cancel()
	}()

	log.Printf("[gateway] starting (listen=%s, rules=%d)", listenAddr, len(rules))
	if err := gw.Run(ctx); err != nil {
		log.Fatalf("[gateway] fatal: %v", err)
	}
}

// loadRules reads egress rules from a YAML config file.
// If the file doesn't exist, returns a default allow-all rule.
func loadRules(path string) []config.EgressRule {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[gateway] no config at %s, defaulting to allow-all", path)
			return []config.EgressRule{{Host: []string{"*"}}}
		}
		log.Fatalf("[gateway] read config: %v", err)
	}

	var cfg gatewayConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("[gateway] parse config: %v", err)
	}

	if len(cfg.Rules) == 0 {
		log.Printf("[gateway] config has no rules, defaulting to allow-all")
		return []config.EgressRule{{Host: []string{"*"}}}
	}

	return cfg.Rules
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
