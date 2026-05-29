package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_LocalPath(t *testing.T) {
	r := NewResolver(t.TempDir())

	// Absolute path
	got, err := r.Resolve("/some/local/path")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "/some/local/path" {
		t.Errorf("got %q, want /some/local/path", got)
	}

	// Relative path
	got, err = r.Resolve("./relative/path")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "./relative/path" {
		t.Errorf("got %q, want ./relative/path", got)
	}
}

func TestResolve_InvalidRemotePath(t *testing.T) {
	r := NewResolver(t.TempDir())

	_, err := r.Resolve("invalid")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestParseRemotePath(t *testing.T) {
	tests := []struct {
		input   string
		repo    string
		subdir  string
		wantErr bool
	}{
		{
			input:  "github.com/donbader/agent-fleet/runtimes/channels-bridge",
			repo:   "https://github.com/donbader/agent-fleet.git",
			subdir: "runtimes/channels-bridge",
		},
		{
			input:  "github.com/donbader/agent-fleet",
			repo:   "https://github.com/donbader/agent-fleet.git",
			subdir: "",
		},
		{
			input:   "invalid/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			repo, subdir, err := parseRemotePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repo != tt.repo {
				t.Errorf("repo = %q, want %q", repo, tt.repo)
			}
			if subdir != tt.subdir {
				t.Errorf("subdir = %q, want %q", subdir, tt.subdir)
			}
		})
	}
}

func TestRepoCacheKey(t *testing.T) {
	got := repoCacheKey("https://github.com/donbader/agent-fleet.git")
	want := "github.com-donbader-agent-fleet"
	if got != want {
		t.Errorf("repoCacheKey() = %q, want %q", got, want)
	}
}

func TestResolve_RemotePathNotFound(t *testing.T) {
	// Create a fake "cloned" repo in cache
	cacheDir := t.TempDir()
	repoDir := filepath.Join(cacheDir, "github.com-donbader-agent-fleet")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	r := NewResolver(cacheDir)

	// Provider path points to a subdir that doesn't exist
	_, err := r.Resolve("github.com/donbader/agent-fleet/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent subdir")
	}
}

func TestResolve_RemotePathFound(t *testing.T) {
	// Create a fake "cloned" repo in cache with the expected subdir
	cacheDir := t.TempDir()
	repoDir := filepath.Join(cacheDir, "github.com-donbader-agent-fleet")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	os.MkdirAll(filepath.Join(repoDir, "runtimes", "codex"), 0755)

	r := NewResolver(cacheDir)

	got, err := r.Resolve("github.com/donbader/agent-fleet/runtimes/codex")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	want := filepath.Join(repoDir, "runtimes", "codex")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
