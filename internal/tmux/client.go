package tmux

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Session struct {
	Name     string
	Windows  int
	Attached bool
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) ListSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}:#{session_windows}:#{?session_attached,attached,detached}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if bytes.Contains(out, []byte("no server running")) {
			return []Session{}, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	sessions := make([]Session, 0)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}

		windows, err := strconv.Atoi(parts[1])
		if err != nil {
			windows = 0
		}

		sessions = append(sessions, Session{
			Name:     parts[0],
			Windows:  windows,
			Attached: parts[2] == "attached",
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan tmux sessions: %w", err)
	}

	return sessions, nil
}

func (c *Client) NewSession(name, cwd string) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", cwd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) SendKeys(target, command string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", target, command, "C-m")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux send-keys: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) RenameSession(oldName, newName string) error {
	cmd := exec.Command("tmux", "rename-session", "-t", oldName, newName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux rename-session: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) KillSession(name string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux kill-session: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) AttachCommand(name string) *exec.Cmd {
	return exec.Command("tmux", "attach", "-t", name)
}
