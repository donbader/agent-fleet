package bridge

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

// AgentConfig holds configuration for spawning the agent process.
type AgentConfig struct {
	Command []string
	Dir     string
	Env     map[string]string
}

// AgentProcess manages a running agent child process.
type AgentProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

// StartAgent spawns the agent as a child process.
func StartAgent(ctx context.Context, cfg AgentConfig) (*AgentProcess, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("agent command is required")
	}

	childCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(childCtx, cfg.Command[0], cfg.Command[1:]...)
	if cfg.Dir != "" {
		cmd.Dir = cfg.Dir
	}

	// Set up environment
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set up stdin/stdout pipes for ACP communication
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	// Agent stderr goes to bridge stderr (for logging)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("starting agent: %w", err)
	}

	ap := &AgentProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
		cancel: cancel,
		done:   make(chan struct{}),
	}

	return ap, nil
}

// PID returns the agent process ID.
func (ap *AgentProcess) PID() int {
	if ap.cmd.Process == nil {
		return 0
	}
	return ap.cmd.Process.Pid
}

// Send sends an ACP message to the agent via stdin.
func (ap *AgentProcess) Send(msg *ACPMessage) error {
	data, err := msg.Encode()
	if err != nil {
		return err
	}

	ap.mu.Lock()
	defer ap.mu.Unlock()

	_, err = ap.stdin.Write(data)
	return err
}

// Recv reads the next ACP message from the agent's stdout.
// Blocks until a message is available or the scanner is closed.
func (ap *AgentProcess) Recv() (*ACPMessage, error) {
	if !ap.stdout.Scan() {
		if err := ap.stdout.Err(); err != nil {
			return nil, fmt.Errorf("reading from agent: %w", err)
		}
		return nil, io.EOF
	}

	return DecodeACPMessage(ap.stdout.Bytes())
}

// Wait waits for the agent process to exit.
func (ap *AgentProcess) Wait() error {
	err := ap.cmd.Wait()
	close(ap.done)
	return err
}

// Stop gracefully stops the agent process.
func (ap *AgentProcess) Stop() {
	// Close stdin to signal the agent to exit
	ap.stdin.Close()

	// Cancel context (sends SIGKILL after timeout)
	ap.cancel()

	// Wait for process to exit
	select {
	case <-ap.done:
	default:
		slog.Info("waiting for agent to exit")
	}
}

// Done returns a channel that's closed when the agent exits.
func (ap *AgentProcess) Done() <-chan struct{} {
	return ap.done
}
