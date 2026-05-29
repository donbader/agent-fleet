package gateway

import (
	"net/http"
	"testing"

	"github.com/donbader/agent-fleet/pkg/config"
)

func TestApplyInjection_TelegramBot(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.telegram.org/botDUMMY_TOKEN/getUpdates?offset=0", nil)
	req.URL.Path = "/botDUMMY_TOKEN/getUpdates"
	req.RequestURI = "/botDUMMY_TOKEN/getUpdates?offset=0"

	rule := config.EgressRule{
		Host:     []string{"api.telegram.org"},
		Provider: "github.com/donbader/agent-fleet/egress-rules/telegram-bot",
		Options:  map[string]any{"token": "123456:ABC-DEF"},
	}

	applyInjection(req, "api.telegram.org", rule)

	wantPath := "/bot123456:ABC-DEF/getUpdates"
	if req.URL.Path != wantPath {
		t.Errorf("URL.Path = %q, want %q", req.URL.Path, wantPath)
	}

	wantURI := "/bot123456:ABC-DEF/getUpdates?offset=0"
	if req.RequestURI != wantURI {
		t.Errorf("RequestURI = %q, want %q", req.RequestURI, wantURI)
	}
}

func TestApplyInjection_GithubPAT(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/owner/repo", nil)

	rule := config.EgressRule{
		Host:     []string{"api.github.com"},
		Provider: "github.com/donbader/agent-fleet/egress-rules/github-pat",
		Options:  map[string]any{"token": "ghp_abc123"},
	}

	applyInjection(req, "api.github.com", rule)

	got := req.Header.Get("Authorization")
	want := "Bearer ghp_abc123"
	if got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestApplyInjection_APIKey(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/v1/data", nil)

	rule := config.EgressRule{
		Host:     []string{"api.example.com"},
		Provider: "github.com/donbader/agent-fleet/egress-rules/api-key",
		Options:  map[string]any{"header": "X-API-Key", "value": "secret123"},
	}

	applyInjection(req, "api.example.com", rule)

	got := req.Header.Get("X-API-Key")
	if got != "secret123" {
		t.Errorf("X-API-Key = %q, want %q", got, "secret123")
	}
}

func TestInjectTelegramToken(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		query     string
		token     string
		wantPath  string
		wantURI   string
	}{
		{
			name:     "getUpdates",
			path:     "/botOLD/getUpdates",
			query:    "offset=5",
			token:    "NEW_TOKEN",
			wantPath: "/botNEW_TOKEN/getUpdates",
			wantURI:  "/botNEW_TOKEN/getUpdates?offset=5",
		},
		{
			name:     "sendMessage",
			path:     "/botdummy123/sendMessage",
			query:    "",
			token:    "real:token",
			wantPath: "/botreal:token/sendMessage",
			wantURI:  "/botreal:token/sendMessage",
		},
		{
			name:     "non-bot path unchanged",
			path:     "/webhook/callback",
			query:    "",
			token:    "token",
			wantPath: "/webhook/callback",
			wantURI:  "/webhook/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://api.telegram.org"+tt.path, nil)
			req.URL.Path = tt.path
			req.URL.RawQuery = tt.query
			req.RequestURI = tt.path
			if tt.query != "" {
				req.RequestURI += "?" + tt.query
			}

			injectTelegramToken(req, tt.token)

			if req.URL.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", req.URL.Path, tt.wantPath)
			}
			if req.RequestURI != tt.wantURI {
				t.Errorf("RequestURI = %q, want %q", req.RequestURI, tt.wantURI)
			}
		})
	}
}

func TestNewMITMConfig(t *testing.T) {
	cfg, err := NewMITMConfig()
	if err != nil {
		t.Fatalf("NewMITMConfig() error: %v", err)
	}

	if cfg.CA == nil {
		t.Error("CA is nil")
	}
	if cfg.CAKey == nil {
		t.Error("CAKey is nil")
	}
	if !cfg.CA.IsCA {
		t.Error("CA.IsCA should be true")
	}
}

func TestMITMConfig_GenerateCert(t *testing.T) {
	cfg, err := NewMITMConfig()
	if err != nil {
		t.Fatalf("NewMITMConfig() error: %v", err)
	}

	cert, err := cfg.generateCert("api.telegram.org")
	if err != nil {
		t.Fatalf("generateCert() error: %v", err)
	}

	if cert == nil {
		t.Fatal("cert is nil")
	}
	if len(cert.Certificate) == 0 {
		t.Error("cert has no certificate data")
	}
}
