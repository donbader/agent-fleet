// Package gateway implements the transparent egress proxy.
// It intercepts all outbound TCP traffic (via iptables redirect) and applies
// egress rules — credential injection, MITM, or passthrough.
package gateway

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/donbader/agent-fleet/pkg/config"
)

// Gateway is the transparent egress proxy.
type Gateway struct {
	listenAddr string
	rules      []config.EgressRule
	listener   net.Listener
	mitm_cfg   *MITMConfig
	mu         sync.Mutex
}

// Config holds gateway startup configuration.
type Config struct {
	// ListenAddr is the address the proxy listens on (e.g., ":8080").
	ListenAddr string

	// Presets are the named egress presets from fleet.yaml.
	Presets map[string]config.EgressPreset

	// ActivePresets is the ordered list of preset names for this agent.
	ActivePresets []string

	// MITM is the MITM configuration (CA cert/key). If nil, MITM falls back to passthrough.
	MITM *MITMConfig
}

// New creates a new Gateway instance.
func New(cfg Config) (*Gateway, error) {
	// Compile ordered rules from active presets
	var rules []config.EgressRule
	for _, name := range cfg.ActivePresets {
		preset, ok := cfg.Presets[name]
		if !ok {
			return nil, fmt.Errorf("undefined egress preset: %q", name)
		}
		rules = append(rules, preset...)
	}

	return &Gateway{
		listenAddr: cfg.ListenAddr,
		rules:      rules,
		mitm_cfg:   cfg.MITM,
	}, nil
}

// Run starts the gateway proxy. Blocks until context is cancelled.
func (g *Gateway) Run(ctx context.Context) error {
	var err error
	g.listener, err = net.Listen("tcp", g.listenAddr)
	if err != nil {
		return fmt.Errorf("gateway listen: %w", err)
	}
	defer g.listener.Close()

	log.Printf("[gateway] listening on %s (%d rules)", g.listenAddr, len(g.rules))

	// Close listener when context is cancelled
	go func() {
		<-ctx.Done()
		g.listener.Close()
	}()

	for {
		conn, err := g.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil // graceful shutdown
			default:
				return fmt.Errorf("gateway accept: %w", err)
			}
		}
		go g.handleConnection(conn)
	}
}

// Addr returns the listener address (useful for tests).
func (g *Gateway) Addr() net.Addr {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.listener == nil {
		return nil
	}
	return g.listener.Addr()
}

// CompiledRules returns the ordered list of egress rules for inspection/testing.
func (g *Gateway) CompiledRules() []config.EgressRule {
	return g.rules
}

// handleConnection processes a single incoming connection.
func (g *Gateway) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Peek at the first bytes to determine if this is TLS (ClientHello)
	peekBuf := make([]byte, 5) // TLS record header is 5 bytes
	n, err := io.ReadFull(conn, peekBuf)
	if err != nil {
		log.Printf("[gateway] peek error: %v", err)
		return
	}

	// Create a peeked connection that replays the peeked bytes
	peeked := &peekedConn{Conn: conn, peeked: peekBuf[:n]}

	// Check if this is a TLS ClientHello
	if isTLSClientHello(peekBuf) {
		g.handleTLS(peeked)
	} else {
		g.handlePlaintext(peeked)
	}
}

// handleTLS handles a TLS connection — extract SNI, match rules, passthrough or MITM.
func (g *Gateway) handleTLS(conn net.Conn) {
	// Read enough to extract SNI from ClientHello
	hello, remaining, err := peekClientHello(conn)
	if err != nil {
		log.Printf("[gateway] TLS peek error: %v", err)
		return
	}

	hostname := hello.ServerName
	if hostname == "" {
		log.Printf("[gateway] TLS connection with no SNI, denying")
		return
	}

	// Match against rules
	rule, matched := matchRule(g.rules, hostname)
	if !matched {
		log.Printf("[gateway] DENY: %s (no matching rule)", hostname)
		return
	}

	log.Printf("[gateway] ALLOW: %s (provider: %s)", hostname, rule.Provider)

	if rule.Provider == "" {
		// Passthrough — connect to destination and pipe bytes
		g.passthrough(remaining, hostname, 443)
	} else {
		// MITM — intercept TLS, modify HTTP, re-encrypt
		g.mitm(remaining, hostname, rule)
	}
}

// handlePlaintext handles a non-TLS (HTTP) connection.
func (g *Gateway) handlePlaintext(conn net.Conn) {
	// For plaintext HTTP, read the Host header
	host, remaining, err := peekHTTPHost(conn)
	if err != nil {
		log.Printf("[gateway] HTTP peek error: %v", err)
		return
	}

	if host == "" {
		log.Printf("[gateway] HTTP connection with no Host header, denying")
		return
	}

	// Match against rules
	rule, matched := matchRule(g.rules, host)
	if !matched {
		log.Printf("[gateway] DENY: %s (no matching rule)", host)
		return
	}

	log.Printf("[gateway] ALLOW: %s (provider: %s)", host, rule.Provider)

	if rule.Provider == "" {
		// Passthrough
		g.passthrough(remaining, host, 80)
	} else {
		// Inject credentials into HTTP request
		g.injectHTTP(remaining, host, rule)
	}
}

// passthrough connects to the destination and pipes bytes bidirectionally.
func (g *Gateway) passthrough(clientConn net.Conn, host string, port int) {
	dest := fmt.Sprintf("%s:%d", host, port)
	upstream, err := net.Dial("tcp", dest)
	if err != nil {
		log.Printf("[gateway] dial %s: %v", dest, err)
		return
	}
	defer upstream.Close()

	pipe(clientConn, upstream)
}

// mitm performs MITM TLS interception for credential injection.
func (g *Gateway) mitm(clientConn net.Conn, hostname string, rule config.EgressRule) {
	g.performMITM(clientConn, hostname, rule)
}

// injectHTTP modifies plaintext HTTP requests to inject credentials.
func (g *Gateway) injectHTTP(clientConn net.Conn, host string, rule config.EgressRule) {
	g.performHTTPInjection(clientConn, host, rule)
}

// pipe copies data bidirectionally between two connections.
func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(b, a)
		// Signal write-done to the other side
		if tc, ok := b.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	go func() {
		defer wg.Done()
		io.Copy(a, b)
		if tc, ok := a.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	wg.Wait()
}
