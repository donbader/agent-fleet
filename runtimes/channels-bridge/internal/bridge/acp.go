package bridge

// ACPMessage represents a message in the Agent Client Protocol.
// ACP uses newline-delimited JSON (ndJSON) over stdin/stdout pipes.
type ACPMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Delta     string `json:"delta,omitempty"`
	Tool      string `json:"tool,omitempty"`
	Input     any    `json:"input,omitempty"`
	Output    string `json:"output,omitempty"`
}
