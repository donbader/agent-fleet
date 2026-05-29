package gateway

import (
	"testing"

	"github.com/donbader/agent-fleet/pkg/config"
)

func TestNew_CompilesRulesInOrder(t *testing.T) {
	presets := map[string]config.EgressPreset{
		"telegram": {
			{Host: []string{"api.telegram.org"}, Provider: "telegram-bot"},
		},
		"main": {
			{Host: []string{"api.github.com"}, Provider: "github-pat"},
			{Host: []string{"*"}},
		},
	}

	gw, err := New(Config{
		ListenAddr:    ":8080",
		Presets:       presets,
		ActivePresets: []string{"telegram", "main"},
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	rules := gw.CompiledRules()
	if len(rules) != 3 {
		t.Fatalf("CompiledRules() = %d rules, want 3", len(rules))
	}

	// First rule from telegram preset
	if rules[0].Host[0] != "api.telegram.org" {
		t.Errorf("rules[0].Host = %v", rules[0].Host)
	}
	// Second rule from main preset
	if rules[1].Host[0] != "api.github.com" {
		t.Errorf("rules[1].Host = %v", rules[1].Host)
	}
	// Third rule: catch-all
	if rules[2].Host[0] != "*" {
		t.Errorf("rules[2].Host = %v", rules[2].Host)
	}
}

func TestNew_UndefinedPreset(t *testing.T) {
	presets := map[string]config.EgressPreset{
		"main": {{Host: []string{"*"}}},
	}

	_, err := New(Config{
		ListenAddr:    ":8080",
		Presets:       presets,
		ActivePresets: []string{"nonexistent"},
	})
	if err == nil {
		t.Fatal("expected error for undefined preset")
	}
}
