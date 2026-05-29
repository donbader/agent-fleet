// Package provider resolves provider paths to local directories.
// Remote providers (e.g., github.com/owner/repo/path) are cloned to a local cache.
package provider

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolver resolves provider paths to local filesystem paths.
type Resolver struct {
	cacheDir string // where cloned repos are stored
}

// NewResolver creates a provider resolver with the given cache directory.
func NewResolver(cacheDir string) *Resolver {
	return &Resolver{cacheDir: cacheDir}
}

// Resolve takes a provider path and returns the local filesystem path to its directory.
// Remote providers are cloned to the cache directory.
// Local providers (starting with "./" or "/") are returned as-is.
func (r *Resolver) Resolve(providerPath string) (string, error) {
	// Local path — return as-is
	if strings.HasPrefix(providerPath, "./") || strings.HasPrefix(providerPath, "/") {
		return providerPath, nil
	}

	// Remote path — parse and clone
	repo, subdir, err := parseRemotePath(providerPath)
	if err != nil {
		return "", err
	}

	// Clone or update the repo
	localRepo, err := r.ensureCloned(repo)
	if err != nil {
		return "", fmt.Errorf("cloning %s: %w", repo, err)
	}

	// Return the subdir within the cloned repo
	resolved := filepath.Join(localRepo, subdir)
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("provider path %q not found in cloned repo (expected %s)", providerPath, resolved)
	}

	return resolved, nil
}

// parseRemotePath parses a provider path like "github.com/owner/repo/sub/dir"
// into a clone URL and subdirectory.
func parseRemotePath(path string) (repoURL string, subdir string, err error) {
	parts := strings.Split(path, "/")

	// Minimum: host/owner/repo
	if len(parts) < 3 {
		return "", "", fmt.Errorf("invalid provider path %q: expected github.com/owner/repo[/subdir]", path)
	}

	host := parts[0]
	owner := parts[1]
	repo := parts[2]

	repoURL = fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)

	if len(parts) > 3 {
		subdir = strings.Join(parts[3:], "/")
	}

	return repoURL, subdir, nil
}

// ensureCloned clones the repo if not already cached, or pulls latest.
func (r *Resolver) ensureCloned(repoURL string) (string, error) {
	// Derive a cache key from the repo URL
	key := repoCacheKey(repoURL)
	localPath := filepath.Join(r.cacheDir, key)

	if _, err := os.Stat(filepath.Join(localPath, ".git")); err == nil {
		// Already cloned — pull latest
		cmd := exec.Command("git", "-C", localPath, "pull", "--ff-only", "-q")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Ignore pull errors (might be offline)
		_ = cmd.Run()
		return localPath, nil
	}

	// Clone fresh
	if err := os.MkdirAll(r.cacheDir, 0755); err != nil {
		return "", err
	}

	fmt.Printf("[provider] Cloning %s...\n", repoURL)
	cmd := exec.Command("git", "clone", "--depth=1", "-q", repoURL, localPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone %s: %w", repoURL, err)
	}

	return localPath, nil
}

// repoCacheKey derives a filesystem-safe cache key from a repo URL.
func repoCacheKey(repoURL string) string {
	// https://github.com/donbader/agent-fleet.git -> github.com-donbader-agent-fleet
	key := repoURL
	key = strings.TrimPrefix(key, "https://")
	key = strings.TrimPrefix(key, "http://")
	key = strings.TrimSuffix(key, ".git")
	key = strings.ReplaceAll(key, "/", "-")
	return key
}
