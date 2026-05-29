package selfupdate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestCheckUpdate_NewVersionAvailable(t *testing.T) {
	assetName := "agent-fleet_1.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{ //nolint:errcheck
			TagName: "v1.0.0",
			Assets: []githubAsset{
				{Name: assetName, BrowserDownloadURL: "https://example.com/download"},
			},
		})
	}))
	defer server.Close()

	u := &Updater{
		cfg: Config{
			Repo:           "test/repo",
			CurrentVersion: "v0.1.0",
		},
		client: server.Client(),
	}
	// Override the URL by using a custom transport
	u.client.Transport = &rewriteTransport{base: server.URL}

	release, err := u.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdate() error: %v", err)
	}
	if release == nil {
		t.Fatal("expected a release, got nil")
	}
	if release.Version != "v1.0.0" {
		t.Errorf("Version = %q, want v1.0.0", release.Version)
	}
}

func TestCheckUpdate_AlreadyLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(githubRelease{ //nolint:errcheck
			TagName: "v0.1.0",
			Assets:  []githubAsset{},
		})
	}))
	defer server.Close()

	u := &Updater{
		cfg: Config{
			Repo:           "test/repo",
			CurrentVersion: "v0.1.0",
		},
		client: server.Client(),
	}
	u.client.Transport = &rewriteTransport{base: server.URL}

	release, err := u.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdate() error: %v", err)
	}
	if release != nil {
		t.Errorf("expected nil (already latest), got %+v", release)
	}
}

func TestCheckUpdate_PrivateRepoNoToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	u := &Updater{
		cfg: Config{
			Repo:           "test/private-repo",
			CurrentVersion: "v0.1.0",
		},
		client: server.Client(),
	}
	u.client.Transport = &rewriteTransport{base: server.URL}

	_, err := u.CheckUpdate(context.Background())
	if err == nil {
		t.Error("expected error for private repo without token")
	}
}

func TestCheckUpdate_WithToken(t *testing.T) {
	var gotAuth string
	assetName := "agent-fleet_2.0.0_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(githubRelease{ //nolint:errcheck
			TagName: "v2.0.0",
			Assets: []githubAsset{
				{Name: assetName, BrowserDownloadURL: "https://example.com/dl"},
			},
		})
	}))
	defer server.Close()

	u := &Updater{
		cfg: Config{
			Repo:           "test/repo",
			CurrentVersion: "v1.0.0",
			Token:          "ghp_secret",
		},
		client: server.Client(),
	}
	u.client.Transport = &rewriteTransport{base: server.URL}

	release, err := u.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdate() error: %v", err)
	}
	if release == nil {
		t.Fatal("expected release")
	}
	if gotAuth != "token ghp_secret" {
		t.Errorf("Authorization = %q, want 'token ghp_secret'", gotAuth)
	}
}

func TestAssetName(t *testing.T) {
	u := &Updater{}
	got := u.assetName("1.2.3")
	want := "agent-fleet_1.2.3_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if got != want {
		t.Errorf("assetName() = %q, want %q", got, want)
	}
}

// rewriteTransport rewrites all requests to point to the test server.
type rewriteTransport struct {
	base string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}
