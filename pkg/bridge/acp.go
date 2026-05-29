package bridge

import (
	"encoding/json"
	"fmt"
)

// ACP (Agent Client Protocol) message types.
const (
	MessageTypeUserMessage    = "user_message"
	MessageTypeAgentResponse  = "agent_response"
	MessageTypeSessionStart   = "session_start"
	MessageTypeSessionEnd     = "session_end"
	MessageTypeError          = "error"
)

// ACPMessage is the wire format for messages between bridge and agent.
// Messages are JSON-encoded, one per line (newline-delimited JSON).
type ACPMessage struct {
	Type      string            `json:"type"`
	SessionID string            `json:"session_id"`
	Content   string            `json:"content,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// Encode serializes an ACP message to JSON bytes (with trailing newline).
func (m *ACPMessage) Encode() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encoding ACP message: %w", err)
	}
	return append(data, '\n'), nil
}

// DecodeACPMessage deserializes an ACP message from JSON bytes.
func DecodeACPMessage(data []byte) (*ACPMessage, error) {
	var msg ACPMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("decoding ACP message: %w", err)
	}
	return &msg, nil
}
