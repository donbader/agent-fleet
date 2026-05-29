package gateway

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/donbader/agent-fleet/pkg/config"
)

// MITMConfig holds the CA certificate and key for MITM interception.
type MITMConfig struct {
	CA     *x509.Certificate
	CAKey  *ecdsa.PrivateKey
}

// NewMITMConfig generates a self-signed CA for MITM interception.
func NewMITMConfig() (*MITMConfig, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating CA key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"agent-fleet gateway"},
			CommonName:   "agent-fleet CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("creating CA cert: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parsing CA cert: %w", err)
	}

	return &MITMConfig{CA: cert, CAKey: key}, nil
}

// generateCert creates a TLS certificate for the given hostname, signed by the CA.
func (m *MITMConfig) generateCert(hostname string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: hostname,
		},
		DNSNames:  []string{hostname},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, m.CA, &key.PublicKey, m.CAKey)
	if err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

// performMITM intercepts a TLS connection, reads/modifies HTTP, and forwards to upstream.
func (g *Gateway) performMITM(clientConn net.Conn, hostname string, rule config.EgressRule) {
	if g.mitm_cfg == nil {
		log.Printf("[gateway] MITM not configured, falling back to passthrough for %s", hostname)
		g.passthrough(clientConn, hostname, 443)
		return
	}

	// Generate a cert for this hostname
	cert, err := g.mitm_cfg.generateCert(hostname)
	if err != nil {
		log.Printf("[gateway] cert generation failed for %s: %v", hostname, err)
		return
	}

	// Terminate TLS from client
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	clientTLS := tls.Server(clientConn, tlsConfig)
	if err := clientTLS.Handshake(); err != nil {
		log.Printf("[gateway] TLS handshake with client failed for %s: %v", hostname, err)
		return
	}
	defer clientTLS.Close()

	// Read the HTTP request from client
	req, err := http.ReadRequest(bufio.NewReader(clientTLS))
	if err != nil {
		log.Printf("[gateway] reading HTTP request from client: %v", err)
		return
	}

	// Apply credential injection
	applyInjection(req, hostname, rule)

	// Connect to upstream with TLS
	upstream, err := tls.Dial("tcp", hostname+":443", &tls.Config{
		ServerName: hostname,
	})
	if err != nil {
		log.Printf("[gateway] dial upstream %s: %v", hostname, err)
		return
	}
	defer upstream.Close()

	// Forward the modified request
	if err := req.Write(upstream); err != nil {
		log.Printf("[gateway] writing to upstream: %v", err)
		return
	}

	// Pipe the response back
	_, _ = io.Copy(clientTLS, upstream)
}

// performHTTPInjection modifies a plaintext HTTP request and forwards it.
func (g *Gateway) performHTTPInjection(clientConn net.Conn, host string, rule config.EgressRule) {
	// Read the HTTP request
	req, err := http.ReadRequest(bufio.NewReader(clientConn))
	if err != nil {
		log.Printf("[gateway] reading HTTP request: %v", err)
		return
	}

	// Apply credential injection
	applyInjection(req, host, rule)

	// Connect to upstream
	upstream, err := net.Dial("tcp", host+":80")
	if err != nil {
		log.Printf("[gateway] dial upstream %s:80: %v", host, err)
		return
	}
	defer upstream.Close()

	// Forward the modified request
	if err := req.Write(upstream); err != nil {
		log.Printf("[gateway] writing to upstream: %v", err)
		return
	}

	// Pipe the response back
	io.Copy(clientConn, upstream)
}

// applyInjection modifies an HTTP request based on the egress rule provider.
func applyInjection(req *http.Request, hostname string, rule config.EgressRule) {
	provider := rule.Provider
	options := rule.Options

	switch {
	case strings.HasSuffix(provider, "/telegram-bot"):
		// Telegram bot token URL rewrite
		// The agent uses a dummy token in the URL path: /bot<dummy>/method
		// We rewrite it to /bot<real-token>/method
		if token, ok := options["token"].(string); ok {
			injectTelegramToken(req, token)
		}

	case strings.HasSuffix(provider, "/github-pat"):
		// GitHub PAT header injection
		if token, ok := options["token"].(string); ok {
			req.Header.Set("Authorization", "Bearer "+token)
		}

	case strings.HasSuffix(provider, "/api-key"):
		// Generic API key header injection
		if header, ok := options["header"].(string); ok {
			if value, ok := options["value"].(string); ok {
				req.Header.Set(header, value)
			}
		}
	}
}

// injectTelegramToken rewrites the URL path to inject the real bot token.
// Path format: /bot<token>/method → /bot<real-token>/method
func injectTelegramToken(req *http.Request, realToken string) {
	path := req.URL.Path
	// Find /bot<anything>/ and replace with /bot<realToken>/
	if strings.HasPrefix(path, "/bot") {
		// Find the next slash after /bot
		rest := path[4:] // after "/bot"
		if idx := strings.Index(rest, "/"); idx != -1 {
			method := rest[idx:] // "/getUpdates" etc.
			req.URL.Path = "/bot" + realToken + method
			req.RequestURI = req.URL.Path
			if req.URL.RawQuery != "" {
				req.RequestURI += "?" + req.URL.RawQuery
			}
		}
	}
}
