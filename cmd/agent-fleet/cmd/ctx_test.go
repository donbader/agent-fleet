package cmd

import (
	"testing"
)

func TestNavigatePath(t *testing.T) {
	data := map[string]any{
		"name":       "coder",
		"fleet_name": "my-fleet",
		"options": map[string]any{
			"auth_port": float64(1455),
			"nested": map[string]any{
				"deep": "value",
			},
			"channels": []any{
				map[string]any{"provider": "telegram", "opts": "val"},
			},
		},
		"env": map[string]any{
			"OPENAI_API_KEY": "sk-test",
		},
		"gateway_host": "gateway",
		"gateway_port": "8080",
	}

	tests := []struct {
		name    string
		path    string
		want    any
		found   bool
	}{
		{"top-level string", ".name", "coder", true},
		{"top-level string 2", ".fleet_name", "my-fleet", true},
		{"nested number", ".options.auth_port", float64(1455), true},
		{"deep nested", ".options.nested.deep", "value", true},
		{"env value", ".env.OPENAI_API_KEY", "sk-test", true},
		{"not found", ".missing", nil, false},
		{"nested not found", ".options.missing", nil, false},
		{"path through non-map", ".name.invalid", nil, false},
		{"array index", ".options.channels.0", map[string]any{"provider": "telegram", "opts": "val"}, true},
		{"array index nested", ".options.channels.0.provider", "telegram", true},
		{"array out of bounds", ".options.channels.5", nil, false},
		{"empty path returns whole map", ".", data, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := navigatePath(data, tt.path)
			if found != tt.found {
				t.Errorf("navigatePath(%q) found = %v, want %v", tt.path, found, tt.found)
				return
			}
			if !found {
				return
			}
			// For map comparison, just check it's not nil
			if _, isMap := tt.want.(map[string]any); isMap {
				if got == nil {
					t.Errorf("navigatePath(%q) = nil, want map", tt.path)
				}
				return
			}
			if got != tt.want {
				t.Errorf("navigatePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
