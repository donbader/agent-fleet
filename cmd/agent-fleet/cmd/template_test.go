package cmd

import (
	"testing"
)

func TestParseVarFlags(t *testing.T) {
	tests := []struct {
		name     string
		flags    []string
		expected map[string]string
	}{
		{
			name:     "single var",
			flags:    []string{"AGENT_HOME=/home/agent"},
			expected: map[string]string{"AGENT_HOME": "/home/agent"},
		},
		{
			name:     "multiple vars",
			flags:    []string{"AGENT_HOME=/home/agent", "AGENT_USER=agent"},
			expected: map[string]string{"AGENT_HOME": "/home/agent", "AGENT_USER": "agent"},
		},
		{
			name:     "value with equals sign",
			flags:    []string{"PATH=/usr/bin:/usr/local/bin"},
			expected: map[string]string{"PATH": "/usr/bin:/usr/local/bin"},
		},
		{
			name:     "empty flags",
			flags:    nil,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVarFlags(tt.flags)
			if len(got) != len(tt.expected) {
				t.Errorf("parseVarFlags() returned %d entries, want %d", len(got), len(tt.expected))
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("parseVarFlags()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestSubstituteVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		vars     map[string]string
		expected string
	}{
		{
			name:     "single variable",
			input:    "COPY home/ ${AGENT_HOME}/",
			vars:     map[string]string{"AGENT_HOME": "/home/agent"},
			expected: "COPY home/ /home/agent/",
		},
		{
			name:     "multiple variables",
			input:    "RUN chown ${AGENT_USER}:${AGENT_USER} ${AGENT_HOME}",
			vars:     map[string]string{"AGENT_USER": "agent", "AGENT_HOME": "/home/agent"},
			expected: "RUN chown agent:agent /home/agent",
		},
		{
			name:     "no variables",
			input:    "RUN apt-get install -y ripgrep",
			vars:     map[string]string{"AGENT_HOME": "/home/agent"},
			expected: "RUN apt-get install -y ripgrep",
		},
		{
			name:     "unknown variable left as-is",
			input:    "ENV PATH=${PATH}:/usr/local/bin",
			vars:     map[string]string{"AGENT_HOME": "/home/agent"},
			expected: "ENV PATH=${PATH}:/usr/local/bin",
		},
		{
			name:     "empty input",
			input:    "",
			vars:     map[string]string{"AGENT_HOME": "/home/agent"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteVars(tt.input, tt.vars)
			if got != tt.expected {
				t.Errorf("substituteVars() = %q, want %q", got, tt.expected)
			}
		})
	}
}
