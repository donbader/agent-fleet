package gateway

import (
	"testing"

)

func TestHostMatches(t *testing.T) {
	tests := []struct {
		pattern  string
		hostname string
		want     bool
	}{
		// Exact match
		{"api.github.com", "api.github.com", true},
		{"api.github.com", "github.com", false},
		{"api.github.com", "evil.api.github.com", false},

		// Wildcard
		{"*", "anything.com", true},
		{"*", "api.github.com", true},
		{"*", "", true},

		// Suffix wildcard
		{"*.github.com", "api.github.com", true},
		{"*.github.com", "raw.github.com", true},
		{"*.github.com", "github.com", false},
		{"*.github.com", "evil.com", false},
		{"*.example.com", "sub.example.com", true},
		{"*.example.com", "deep.sub.example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.hostname, func(t *testing.T) {
			got := hostMatches(tt.pattern, tt.hostname)
			if got != tt.want {
				t.Errorf("hostMatches(%q, %q) = %v, want %v", tt.pattern, tt.hostname, got, tt.want)
			}
		})
	}
}

func TestMatchRule(t *testing.T) {
	rules := []EgressRule{
		{Host: []string{"api.telegram.org"}, Provider: "telegram-bot"},
		{Host: []string{"api.github.com", "github.com"}, Provider: "github-pat"},
		{Host: []string{"*.npmjs.org"}},
		{Host: []string{"*"}}, // catch-all
	}

	tests := []struct {
		hostname     string
		wantProvider string
		wantMatch    bool
	}{
		{"api.telegram.org", "telegram-bot", true},
		{"api.github.com", "github-pat", true},
		{"github.com", "github-pat", true},
		{"registry.npmjs.org", "", true},
		{"random.com", "", true}, // catch-all
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			rule, matched := matchRule(rules, tt.hostname)
			if matched != tt.wantMatch {
				t.Fatalf("matchRule(%q) matched = %v, want %v", tt.hostname, matched, tt.wantMatch)
			}
			if rule.Provider != tt.wantProvider {
				t.Errorf("matchRule(%q) provider = %q, want %q", tt.hostname, rule.Provider, tt.wantProvider)
			}
		})
	}
}

func TestMatchRule_DefaultDeny(t *testing.T) {
	// No catch-all rule — should deny unknown hosts
	rules := []EgressRule{
		{Host: []string{"api.github.com"}, Provider: "github-pat"},
	}

	_, matched := matchRule(rules, "evil.com")
	if matched {
		t.Error("expected no match for evil.com (default deny)")
	}
}

func TestMatchRule_FirstMatchWins(t *testing.T) {
	rules := []EgressRule{
		{Host: []string{"api.github.com"}, Provider: "specific-provider"},
		{Host: []string{"*.github.com"}, Provider: "wildcard-provider"},
		{Host: []string{"*"}, Provider: "catch-all-provider"},
	}

	rule, matched := matchRule(rules, "api.github.com")
	if !matched {
		t.Fatal("expected match")
	}
	if rule.Provider != "specific-provider" {
		t.Errorf("provider = %q, want specific-provider (first match wins)", rule.Provider)
	}
}

func TestNew_CompilesRulesInOrder(t *testing.T) {
	presets := map[string]EgressPreset{
		"telegram": {
			{Host: []string{"api.telegram.org"}, Provider: "telegram-bot"},
		},
		"main": {
			{Host: []string{"api.github.com"}, Provider: "github-pat"},
			{Host: []string{"*"}},
		},
	}

	gw, err := New(Config{
		ListenAddr:    ":0",
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

	if rules[0].Host[0] != "api.telegram.org" {
		t.Errorf("rules[0].Host = %v", rules[0].Host)
	}
	if rules[1].Host[0] != "api.github.com" {
		t.Errorf("rules[1].Host = %v", rules[1].Host)
	}
	if rules[2].Host[0] != "*" {
		t.Errorf("rules[2].Host = %v", rules[2].Host)
	}
}

func TestNew_UndefinedPreset(t *testing.T) {
	presets := map[string]EgressPreset{
		"main": {{Host: []string{"*"}}},
	}

	_, err := New(Config{
		ListenAddr:    ":0",
		Presets:       presets,
		ActivePresets: []string{"nonexistent"},
	})
	if err == nil {
		t.Fatal("expected error for undefined preset")
	}
}
