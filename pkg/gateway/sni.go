package gateway

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
)

// isTLSClientHello checks if the first bytes look like a TLS record.
// TLS record: ContentType(1) + Version(2) + Length(2)
// ClientHello ContentType = 0x16 (Handshake)
func isTLSClientHello(buf []byte) bool {
	if len(buf) < 3 {
		return false
	}
	// ContentType 0x16 = Handshake
	// Version: 0x0301 (TLS 1.0), 0x0302 (TLS 1.1), 0x0303 (TLS 1.2/1.3)
	return buf[0] == 0x16 && buf[1] == 0x03 && buf[2] >= 0x01 && buf[2] <= 0x03
}

// ClientHello holds parsed TLS ClientHello information.
type ClientHello struct {
	ServerName string
}

// peekClientHello reads the full TLS ClientHello and extracts SNI.
// Returns the parsed hello and a connection that replays all read bytes.
func peekClientHello(conn net.Conn) (*ClientHello, net.Conn, error) {
	// Read the full TLS record
	// We already have a peekedConn with the first 5 bytes
	// Read the record header to get the length
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, nil, fmt.Errorf("reading TLS header: %w", err)
	}

	// Record length is in bytes 3-4 (big-endian)
	recordLen := int(header[3])<<8 | int(header[4])
	if recordLen > 16384 { // max TLS record size
		return nil, nil, fmt.Errorf("TLS record too large: %d", recordLen)
	}

	// Read the full record body
	body := make([]byte, recordLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, nil, fmt.Errorf("reading TLS body: %w", err)
	}

	// Parse SNI from the ClientHello
	sni := extractSNI(body)

	// Create a connection that replays header + body
	fullRecord := append(header, body...)
	replayed := &peekedConn{Conn: conn, peeked: fullRecord}

	return &ClientHello{ServerName: sni}, replayed, nil
}

// extractSNI parses the SNI extension from a TLS ClientHello handshake body.
func extractSNI(data []byte) string {
	// Handshake message: Type(1) + Length(3) + ...
	if len(data) < 4 {
		return ""
	}
	// Type should be ClientHello (0x01)
	if data[0] != 0x01 {
		return ""
	}

	// Skip: Type(1) + Length(3) + ClientVersion(2) + Random(32)
	pos := 1 + 3 + 2 + 32
	if pos >= len(data) {
		return ""
	}

	// Session ID (variable length)
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen
	if pos+2 > len(data) {
		return ""
	}

	// Cipher Suites (variable length)
	cipherSuitesLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2 + cipherSuitesLen
	if pos+1 > len(data) {
		return ""
	}

	// Compression Methods (variable length)
	compressionLen := int(data[pos])
	pos += 1 + compressionLen
	if pos+2 > len(data) {
		return ""
	}

	// Extensions length
	extensionsLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2
	end := pos + extensionsLen
	if end > len(data) {
		end = len(data)
	}

	// Parse extensions looking for SNI (type 0x0000)
	for pos+4 <= end {
		extType := int(data[pos])<<8 | int(data[pos+1])
		extLen := int(data[pos+2])<<8 | int(data[pos+3])
		pos += 4

		if extType == 0x0000 && pos+extLen <= end {
			// SNI extension
			return parseSNIExtension(data[pos : pos+extLen])
		}

		pos += extLen
	}

	return ""
}

// parseSNIExtension parses the SNI extension data.
func parseSNIExtension(data []byte) string {
	// ServerNameList length (2 bytes)
	if len(data) < 2 {
		return ""
	}
	listLen := int(data[0])<<8 | int(data[1])
	if listLen+2 > len(data) {
		return ""
	}

	pos := 2
	end := 2 + listLen

	for pos+3 <= end {
		nameType := data[pos]
		nameLen := int(data[pos+1])<<8 | int(data[pos+2])
		pos += 3

		if nameType == 0x00 && pos+nameLen <= end { // host_name
			return string(data[pos : pos+nameLen])
		}
		pos += nameLen
	}

	return ""
}

// peekHTTPHost reads enough of an HTTP request to extract the Host header.
// Returns the host and a connection that replays all read bytes.
func peekHTTPHost(conn net.Conn) (string, net.Conn, error) {
	// Buffer the connection to read the HTTP request line + headers
	var buf bytes.Buffer
	reader := io.TeeReader(conn, &buf)
	bufReader := bufio.NewReader(reader)

	// Parse the HTTP request (just headers)
	req, err := http.ReadRequest(bufReader)
	if err != nil {
		// Replay what we read so far
		replayed := &peekedConn{Conn: conn, peeked: buf.Bytes()}
		return "", replayed, fmt.Errorf("parsing HTTP request: %w", err)
	}
	req.Body.Close()

	host := req.Host
	// Strip port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// Create replayed connection with all the bytes we consumed
	replayed := &peekedConn{Conn: conn, peeked: buf.Bytes()}
	return host, replayed, nil
}

// peekedConn wraps a net.Conn and prepends previously-read bytes.
type peekedConn struct {
	net.Conn
	peeked []byte
	offset int
}

func (c *peekedConn) Read(b []byte) (int, error) {
	if c.offset < len(c.peeked) {
		n := copy(b, c.peeked[c.offset:])
		c.offset += n
		return n, nil
	}
	return c.Conn.Read(b)
}
