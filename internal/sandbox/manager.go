// Package sandbox manages Docker containers for OpenTL sessions.
package sandbox

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// StartOptions configures a new sandbox container.
type StartOptions struct {
	SessionID string
	Repo      string   // "owner/repo"
	Prompt    string
	Branch    string   // git branch name
	Image     string   // Docker image name
	Env       []string // additional environment variables
	Network   string   // Docker network name
}

// Manager handles Docker sandbox lifecycle.
type Manager struct{}

// NewManager creates a new sandbox Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Start creates and starts a new sandbox container. Returns the container ID.
func (m *Manager) Start(ctx context.Context, opts StartOptions) (string, error) {
	args := []string{
		"run", "-d",
		"--name", fmt.Sprintf("opentl-%s", opts.SessionID),
		"--label", "opentl.session=" + opts.SessionID,
	}

	// Network
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	// Environment variables
	envVars := append(opts.Env,
		"OPENTL_SESSION_ID="+opts.SessionID,
		"OPENTL_REPO="+opts.Repo,
		"OPENTL_PROMPT="+opts.Prompt,
		"OPENTL_BRANCH="+opts.Branch,
	)
	for _, e := range envVars {
		args = append(args, "-e", e)
	}

	// Image
	args = append(args, opts.Image)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("starting container: %w\noutput: %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))
	return containerID, nil
}

// StreamLogs attaches to a container's stdout and returns a line-by-line reader.
func (m *Manager) StreamLogs(ctx context.Context, containerID string) (*LineScanner, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", containerID)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("attaching stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("attaching stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting log stream: %w", err)
	}

	// Merge stdout and stderr into a single reader.
	merged := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(merged)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	return &LineScanner{
		scanner: scanner,
		cmd:     cmd,
	}, nil
}

// Stop kills and removes a sandbox container.
func (m *Manager) Stop(ctx context.Context, containerID string) error {
	// Kill the container (ignore error if already stopped).
	_ = exec.CommandContext(ctx, "docker", "kill", containerID).Run()

	// Remove the container.
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("removing container: %w\noutput: %s", err, string(output))
	}
	return nil
}

// Wait blocks until the container exits and returns the exit code.
func (m *Manager) Wait(ctx context.Context, containerID string) (int, error) {
	cmd := exec.CommandContext(ctx, "docker", "wait", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("waiting for container: %w", err)
	}

	var exitCode int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &exitCode)
	if err != nil {
		return -1, fmt.Errorf("parsing exit code: %w", err)
	}
	return exitCode, nil
}

// EnsureNetwork creates the Docker network if it doesn't exist.
func (m *Manager) EnsureNetwork(ctx context.Context, name string) error {
	// Check if network exists.
	check := exec.CommandContext(ctx, "docker", "network", "inspect", name)
	if check.Run() == nil {
		return nil // Already exists.
	}

	cmd := exec.CommandContext(ctx, "docker", "network", "create", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating network %q: %w\noutput: %s", name, err, string(output))
	}
	return nil
}

// LineScanner wraps a bufio.Scanner for reading container log lines.
type LineScanner struct {
	scanner *bufio.Scanner
	cmd     *exec.Cmd
}

// Scan advances to the next line. Returns false when done.
func (ls *LineScanner) Scan() bool {
	return ls.scanner.Scan()
}

// Text returns the current line.
func (ls *LineScanner) Text() string {
	return ls.scanner.Text()
}

// Err returns any error from scanning.
func (ls *LineScanner) Err() error {
	return ls.scanner.Err()
}

// Close terminates the log stream.
func (ls *LineScanner) Close() error {
	if ls.cmd.Process != nil {
		_ = ls.cmd.Process.Kill()
	}
	return ls.cmd.Wait()
}
