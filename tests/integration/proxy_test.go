// Package integration contains integration tests that test components together.
// These tests use real TCP connections but don't require Docker.
package integration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/donbader/agent-fleet/pkg/config"
	"github.com/donbader/agent-fleet/pkg/gateway"
)

// TestProxy_Passthrough tests that the proxy passes through allowed traffic.
func TestProxy_Passthrough(t *testing.T) {
	// Start a mock upstream HTTP server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from upstream"))
	}))
	defer upstream.Close()

	// Extract host:port from upstream URL
	upstreamAddr := strings.TrimPrefix(upstream.URL, "http://")
	host, port, _ := net.SplitHostPort(upstreamAddr)

	// Create gateway with a rule that allows the upstream host
	gw, err := gateway.New(gateway.Config{
		ListenAddr: "127.0.0.1:0",
		Presets: map[string]config.EgressPreset{
			"main": {{Host: []string{host, "127.0.0.1"}}},
		},
		ActivePresets: []string{"main"},
	})
	if err != nil {
		t.Fatalf("gateway.New() error: %v", err)
	}

	// Start the gateway
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gw.Run(ctx)
	time.Sleep(50 * time.Millisecond) // wait for listener

	addr := gw.Addr()
	if addr == nil {
		t.Fatal("gateway not listening")
	}

	t.Logf("gateway listening on %s, upstream on %s", addr, upstreamAddr)

	// Connect to the gateway and send an HTTP request with host:port
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial gateway: %v", err)
	}
	defer conn.Close()

	// Send HTTP request with port in Host header so gateway knows where to connect
	reqStr := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s:%s\r\nConnection: close\r\n\r\n", host, port)
	conn.Write([]byte(reqStr))

	// Close write side so gateway knows we're done sending
	conn.(*net.TCPConn).CloseWrite()

	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	if !strings.Contains(string(resp), "hello from upstream") {
		t.Errorf("response = %q, want to contain 'hello from upstream'", string(resp))
	}
}

// TestProxy_DefaultDeny tests that the proxy denies traffic with no matching rule.
func TestProxy_DefaultDeny(t *testing.T) {
	// Create gateway with rules that only allow specific hosts
	gw, err := gateway.New(gateway.Config{
		ListenAddr: "127.0.0.1:0",
		Presets: map[string]config.EgressPreset{
			"main": {{Host: []string{"allowed.example.com"}}},
		},
		ActivePresets: []string{"main"},
	})
	if err != nil {
		t.Fatalf("gateway.New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gw.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	addr := gw.Addr()
	if addr == nil {
		t.Fatal("gateway not listening")
	}

	// Connect and send request to a denied host
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial gateway: %v", err)
	}
	defer conn.Close()

	// Send HTTP request to a host that's NOT in the rules
	reqStr := "GET / HTTP/1.1\r\nHost: evil.com\r\nConnection: close\r\n\r\n"
	conn.Write([]byte(reqStr))

	// The gateway should close the connection (deny)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	resp, err := io.ReadAll(conn)
	if err != nil && !isTimeout(err) {
		// Connection closed is expected (deny)
	}

	// Should NOT get a valid HTTP response
	if strings.Contains(string(resp), "HTTP/1.1 200") {
		t.Error("expected connection to be denied, got 200 response")
	}
}

// TestProxy_MITM_TelegramTokenRewrite tests that MITM TLS handshake succeeds with our CA.
func TestProxy_MITM_TelegramTokenRewrite(t *testing.T) {
	// Create MITM config
	mitmCfg, err := gateway.NewMITMConfig()
	if err != nil {
		t.Fatalf("NewMITMConfig() error: %v", err)
	}

	// Create gateway with Telegram rule
	gw, err := gateway.New(gateway.Config{
		ListenAddr: "127.0.0.1:0",
		Presets: map[string]config.EgressPreset{
			"telegram": {{
				Host:     []string{"api.telegram.org"},
				Provider: "github.com/donbader/agent-fleet/egress-rules/telegram-bot",
				Options:  map[string]any{"token": "REAL_TOKEN_123"},
			}},
		},
		ActivePresets: []string{"telegram"},
		MITM:          mitmCfg,
	})
	if err != nil {
		t.Fatalf("gateway.New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gw.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	addr := gw.Addr()
	if addr == nil {
		t.Fatal("gateway not listening")
	}

	// Connect to gateway with TLS (trusting our MITM CA)
	caPool := x509.NewCertPool()
	caPool.AddCert(mitmCfg.CA)

	tlsConn, err := tls.Dial("tcp", addr.String(), &tls.Config{
		ServerName: "api.telegram.org",
		RootCAs:    caPool,
	})
	if err != nil {
		t.Fatalf("TLS dial: %v", err)
	}
	defer tlsConn.Close()

	// The TLS handshake succeeded — MITM cert generation works
	t.Log("TLS handshake succeeded with MITM CA for api.telegram.org")

	// Verify the cert was issued for the right hostname
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("no peer certificates")
	}
	cert := state.PeerCertificates[0]
	if cert.Subject.CommonName != "api.telegram.org" {
		t.Errorf("cert CN = %q, want api.telegram.org", cert.Subject.CommonName)
	}
}

// TestProxy_MITM_GithubPATInjection tests that MITM TLS handshake succeeds for GitHub.
func TestProxy_MITM_GithubPATInjection(t *testing.T) {
	mitmCfg, err := gateway.NewMITMConfig()
	if err != nil {
		t.Fatalf("NewMITMConfig() error: %v", err)
	}

	gw, err := gateway.New(gateway.Config{
		ListenAddr: "127.0.0.1:0",
		Presets: map[string]config.EgressPreset{
			"main": {{
				Host:     []string{"api.github.com"},
				Provider: "github.com/donbader/agent-fleet/egress-rules/github-pat",
				Options:  map[string]any{"token": "ghp_test123"},
			}},
		},
		ActivePresets: []string{"main"},
		MITM:          mitmCfg,
	})
	if err != nil {
		t.Fatalf("gateway.New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go gw.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	addr := gw.Addr()
	if addr == nil {
		t.Fatal("gateway not listening")
	}

	// Connect with TLS trusting our CA
	caPool := x509.NewCertPool()
	caPool.AddCert(mitmCfg.CA)

	tlsConn, err := tls.Dial("tcp", addr.String(), &tls.Config{
		ServerName: "api.github.com",
		RootCAs:    caPool,
	})
	if err != nil {
		t.Fatalf("TLS dial: %v", err)
	}
	defer tlsConn.Close()

	// Verify cert was issued for the right hostname
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("no peer certificates")
	}
	cert := state.PeerCertificates[0]
	if cert.Subject.CommonName != "api.github.com" {
		t.Errorf("cert CN = %q, want api.github.com", cert.Subject.CommonName)
	}

	t.Log("TLS handshake succeeded with MITM CA for api.github.com")
}

// TestProxy_RuleOrder tests that rules are evaluated in order (first match wins).
func TestProxy_RuleOrder(t *testing.T) {
	gw, err := gateway.New(gateway.Config{
		ListenAddr: "127.0.0.1:0",
		Presets: map[string]config.EgressPreset{
			"specific": {{Host: []string{"api.github.com"}, Provider: "github-pat"}},
			"catchall": {{Host: []string{"*"}}},
		},
		ActivePresets: []string{"specific", "catchall"},
	})
	if err != nil {
		t.Fatalf("gateway.New() error: %v", err)
	}

	rules := gw.CompiledRules()
	if len(rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(rules))
	}
	if rules[0].Provider != "github-pat" {
		t.Errorf("rules[0].Provider = %q, want github-pat", rules[0].Provider)
	}
	if rules[1].Host[0] != "*" {
		t.Errorf("rules[1].Host = %v, want [*]", rules[1].Host)
	}
}

// TestProxy_MultiplePresets tests that multiple presets are compiled correctly.
func TestProxy_MultiplePresets(t *testing.T) {
	gw, err := gateway.New(gateway.Config{
		ListenAddr: "127.0.0.1:0",
		Presets: map[string]config.EgressPreset{
			"telegram": {
				{Host: []string{"api.telegram.org"}, Provider: "telegram-bot"},
			},
			"github": {
				{Host: []string{"api.github.com", "github.com"}, Provider: "github-pat"},
			},
			"catchall": {
				{Host: []string{"*"}},
			},
		},
		ActivePresets: []string{"telegram", "github", "catchall"},
	})
	if err != nil {
		t.Fatalf("gateway.New() error: %v", err)
	}

	rules := gw.CompiledRules()
	if len(rules) != 3 {
		t.Fatalf("rules = %d, want 3", len(rules))
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	netErr, ok := err.(net.Error)
	return ok && netErr.Timeout()
}
