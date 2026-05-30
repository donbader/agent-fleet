package compose

import (
	"testing"
)

func TestIsBindMount(t *testing.T) {
	tests := []struct {
		volume   string
		expected bool
	}{
		{"./path:/home/agent", true},
		{"/absolute/path:/home/agent", true},
		{"~/home:/home/agent", true},
		{"named-vol:/home/agent", false},
		{"coder-home:/home/agent", false},
		{"coder-codex-auth:/home/agent/.codex", false},
	}

	for _, tt := range tests {
		t.Run(tt.volume, func(t *testing.T) {
			got := isBindMount(tt.volume)
			if got != tt.expected {
				t.Errorf("isBindMount(%q) = %v, want %v", tt.volume, got, tt.expected)
			}
		})
	}
}

func TestIsBannedVolume(t *testing.T) {
	tests := []struct {
		volume   string
		expected bool
	}{
		// Bind mounts to /home/agent — banned
		{"./home:/home/agent", true},
		{"/host/path:/home/agent", true},
		{"./agents/coder/home:/home/agent", true},
		// Bind mounts to subdirs of /home/agent — also banned
		{"./config:/home/agent/.config", true},
		// Named volumes to /home/agent — allowed
		{"coder-home:/home/agent", false},
		{"coder-codex-auth:/home/agent/.codex", false},
		// Bind mounts to other paths — allowed
		{"./workspace:/workspace", false},
		{"/host/data:/data", false},
	}

	for _, tt := range tests {
		t.Run(tt.volume, func(t *testing.T) {
			got := isBannedVolume(tt.volume)
			if got != tt.expected {
				t.Errorf("isBannedVolume(%q) = %v, want %v", tt.volume, got, tt.expected)
			}
		})
	}
}

func TestSanitizeVolumes(t *testing.T) {
	volumes := []string{
		"coder-home:/home/agent",
		"./agents/coder/home:/home/agent",
		"coder-codex-auth:/home/agent/.codex",
		"./workspace:/workspace",
	}

	sanitized, removed := SanitizeVolumes(volumes)

	if len(sanitized) != 3 {
		t.Errorf("expected 3 sanitized volumes, got %d: %v", len(sanitized), sanitized)
	}
	if len(removed) != 1 {
		t.Errorf("expected 1 removed volume, got %d: %v", len(removed), removed)
	}
	if removed[0] != "./agents/coder/home:/home/agent" {
		t.Errorf("expected removed volume to be './agents/coder/home:/home/agent', got %q", removed[0])
	}
}
