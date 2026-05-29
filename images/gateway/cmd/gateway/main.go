// Command gateway runs the transparent egress proxy as a standalone process.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	gw "github.com/donbader/agent-fleet/gateway"
	"gopkg.in/yaml.v3"
)

// rulesConfig is the config file format for the gateway container.
type rulesConfig struct {
	Rules []gw.EgressRule `yaml:"rules"`
}

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	configPath := envOr("GATEWAY_CONFIG", "/etc/gateway/rules.yaml")

	// Load rules from config file, or default to allow-all
	rules := loadRules(configPath)

	// Build gateway config using a single "default" preset containing all rules
	cfg := gw.Config{
		ListenAddr:    listenAddr,
		Presets:       map[string]gw.EgressPreset{"default": rules},
		ActivePresets: []string{"default"},
	}

	proxy, err := gw.New(cfg)
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
	if err := proxy.Run(ctx); err != nil {
		log.Fatalf("[gateway] fatal: %v", err)
	}
}

// loadRules reads egress rules from a YAML config file.
func loadRules(path string) gw.EgressPreset {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[gateway] no config at %s, defaulting to allow-all", path)
			return gw.EgressPreset{{Host: []string{"*"}}}
		}
		log.Fatalf("[gateway] read config: %v", err)
	}

	var cfg rulesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("[gateway] parse config: %v", err)
	}

	if len(cfg.Rules) == 0 {
		log.Printf("[gateway] config has no rules, defaulting to allow-all")
		return gw.EgressPreset{{Host: []string{"*"}}}
	}

	return cfg.Rules
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
