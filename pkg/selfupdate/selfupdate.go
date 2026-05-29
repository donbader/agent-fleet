// Package selfupdate handles checking for and applying binary updates from GitHub Releases.
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

// Config holds self-update configuration.
type Config struct {
	// Repo is the GitHub repository (e.g., "donbader/agent-fleet").
	Repo string

	// CurrentVersion is the currently running version (e.g., "v0.1.0" or "dev").
	CurrentVersion string

	// Token is an optional GitHub token for private repos.
	// If empty, checks GITHUB_TOKEN env var.
	Token string
}

// Release represents an available update.
type Release struct {
	Version     string
	DownloadURL string
}

// Updater checks for and applies updates.
type Updater struct {
	cfg    Config
	client *http.Client
}

// New creates a new Updater.
func New(cfg Config) *Updater {
	token := cfg.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	cfg.Token = token

	return &Updater{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// CheckUpdate checks if a newer version is available.
// Returns nil if already on the latest version.
func (u *Updater) CheckUpdate(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.cfg.Repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if u.cfg.Token != "" {
		req.Header.Set("Authorization", "token "+u.cfg.Token)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found (is this a private repo? set GITHUB_TOKEN)")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, body)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding release: %w", err)
	}

	// Compare versions
	latestVersion := release.TagName
	if latestVersion == u.cfg.CurrentVersion {
		return nil, nil // already up to date
	}

	// Find the right asset for this platform
	assetName := u.assetName(strings.TrimPrefix(latestVersion, "v"))
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, latestVersion)
	}

	return &Release{
		Version:     latestVersion,
		DownloadURL: downloadURL,
	}, nil
}

// Apply downloads and replaces the current binary with the new version.
func (u *Updater) Apply(ctx context.Context, release *Release) error {
	// Download the archive
	req, err := http.NewRequestWithContext(ctx, "GET", release.DownloadURL, nil)
	if err != nil {
		return err
	}
	if u.cfg.Token != "" {
		req.Header.Set("Authorization", "token "+u.cfg.Token)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "agent-fleet-update-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing download: %w", err)
	}
	tmpFile.Close()

	// Extract the binary from the archive
	binaryPath, err := extractBinary(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}
	defer os.Remove(binaryPath)

	// Replace the current binary
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current binary: %w", err)
	}

	// Resolve symlinks
	currentBinary, err = resolveSymlink(currentBinary)
	if err != nil {
		return fmt.Errorf("resolving symlink: %w", err)
	}

	// Atomic replace: rename new over old
	if err := atomicReplace(binaryPath, currentBinary); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

// assetName returns the expected archive filename for the current platform.
func (u *Updater) assetName(version string) string {
	return fmt.Sprintf("agent-fleet_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
}

// resolveSymlink resolves a path that might be a symlink.
func resolveSymlink(path string) (string, error) {
	resolved, err := os.Readlink(path)
	if err != nil {
		// Not a symlink, use as-is
		return path, nil
	}
	if !strings.HasPrefix(resolved, "/") {
		// Relative symlink — resolve relative to the directory
		dir := path[:strings.LastIndex(path, "/")+1]
		resolved = dir + resolved
	}
	return resolved, nil
}

// atomicReplace replaces dst with src atomically (rename).
func atomicReplace(src, dst string) error {
	// Try rename first (works if same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fallback: copy + remove
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Write to a temp file next to the destination
	tmpDst := dst + ".new"
	dstFile, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		os.Remove(tmpDst)
		return err
	}
	dstFile.Close()

	// Rename temp to final destination
	return os.Rename(tmpDst, dst)
}

// githubRelease is the GitHub API response for a release.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset is a release asset.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}
