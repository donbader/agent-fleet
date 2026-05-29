package gateway

import (
	"testing"
)

func TestExtractSNI(t *testing.T) {
	// Minimal TLS 1.2 ClientHello with SNI "example.com"
	// This is a hand-crafted minimal ClientHello
	hello := buildClientHello("example.com")

	sni := extractSNI(hello)
	if sni != "example.com" {
		t.Errorf("extractSNI() = %q, want %q", sni, "example.com")
	}
}

func TestExtractSNI_NoSNI(t *testing.T) {
	// ClientHello without SNI extension
	hello := buildClientHelloNoSNI()

	sni := extractSNI(hello)
	if sni != "" {
		t.Errorf("extractSNI() = %q, want empty", sni)
	}
}

func TestExtractSNI_LongHostname(t *testing.T) {
	hostname := "very-long-subdomain.api.internal.example.com"
	hello := buildClientHello(hostname)

	sni := extractSNI(hello)
	if sni != hostname {
		t.Errorf("extractSNI() = %q, want %q", sni, hostname)
	}
}

func TestIsTLSClientHello(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want bool
	}{
		{"TLS 1.0", []byte{0x16, 0x03, 0x01, 0x00, 0x05}, true},
		{"TLS 1.2", []byte{0x16, 0x03, 0x03, 0x00, 0x05}, true},
		{"HTTP GET", []byte("GET /"), false},
		{"too short", []byte{0x16, 0x03}, false},
		{"wrong type", []byte{0x17, 0x03, 0x03, 0x00, 0x05}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTLSClientHello(tt.buf)
			if got != tt.want {
				t.Errorf("isTLSClientHello(%v) = %v, want %v", tt.buf, got, tt.want)
			}
		})
	}
}

func TestPeekedConn_Read(t *testing.T) {
	// Test that peekedConn replays peeked bytes then reads from underlying conn
	// We can't easily test with a real net.Conn here, but we can test the logic
	// by verifying the Read method behavior with the peeked buffer
	peeked := []byte("hello")
	pc := &peekedConn{peeked: peeked}

	buf := make([]byte, 3)
	n, err := pc.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != 3 || string(buf[:n]) != "hel" {
		t.Errorf("Read() = %q, want %q", string(buf[:n]), "hel")
	}

	n, err = pc.Read(buf)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if n != 2 || string(buf[:n]) != "lo" {
		t.Errorf("Read() = %q, want %q", string(buf[:n]), "lo")
	}
}

// buildClientHello creates a minimal TLS ClientHello handshake body with SNI.
func buildClientHello(hostname string) []byte {
	sniExt := buildSNIExtension(hostname)
	extensions := sniExt

	// Extensions length (2 bytes)
	extLenBytes := []byte{byte(len(extensions) >> 8), byte(len(extensions))}

	// Compression methods: 1 method (null)
	compression := []byte{0x01, 0x00}

	// Cipher suites: 2 suites (4 bytes)
	cipherSuites := []byte{0x00, 0x04, 0x00, 0x2f, 0x00, 0x35}

	// Session ID: empty
	sessionID := []byte{0x00}

	// Random: 32 bytes of zeros
	random := make([]byte, 32)

	// Client version: TLS 1.2
	version := []byte{0x03, 0x03}

	// Assemble ClientHello body (after type + length)
	body := append(version, random...)
	body = append(body, sessionID...)
	body = append(body, cipherSuites...)
	body = append(body, compression...)
	body = append(body, extLenBytes...)
	body = append(body, extensions...)

	// Handshake header: Type(1) + Length(3)
	bodyLen := len(body)
	header := []byte{
		0x01, // ClientHello
		byte(bodyLen >> 16), byte(bodyLen >> 8), byte(bodyLen),
	}

	return append(header, body...)
}

// buildClientHelloNoSNI creates a minimal ClientHello without SNI extension.
func buildClientHelloNoSNI() []byte {
	// No extensions
	compression := []byte{0x01, 0x00}
	cipherSuites := []byte{0x00, 0x04, 0x00, 0x2f, 0x00, 0x35}
	sessionID := []byte{0x00}
	random := make([]byte, 32)
	version := []byte{0x03, 0x03}

	body := append(version, random...)
	body = append(body, sessionID...)
	body = append(body, cipherSuites...)
	body = append(body, compression...)
	// Extensions length = 0
	body = append(body, 0x00, 0x00)

	bodyLen := len(body)
	header := []byte{
		0x01,
		byte(bodyLen >> 16), byte(bodyLen >> 8), byte(bodyLen),
	}

	return append(header, body...)
}

// buildSNIExtension creates an SNI extension for the given hostname.
func buildSNIExtension(hostname string) []byte {
	nameBytes := []byte(hostname)
	nameLen := len(nameBytes)

	// Server Name entry: type(1) + length(2) + name
	entry := []byte{0x00, byte(nameLen >> 8), byte(nameLen)}
	entry = append(entry, nameBytes...)

	// Server Name List: length(2) + entries
	listLen := len(entry)
	list := []byte{byte(listLen >> 8), byte(listLen)}
	list = append(list, entry...)

	// Extension: type(2) + length(2) + data
	extData := list
	ext := []byte{
		0x00, 0x00, // SNI extension type
		byte(len(extData) >> 8), byte(len(extData)),
	}
	ext = append(ext, extData...)

	return ext
}
