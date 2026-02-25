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
	Name           string
	Windows        int
	Attached       bool
	HasAlerts      bool
	AlertsBell     bool
	AlertsActivity bool
	AlertsSilence  bool
	LastActivity   int64
	CurrentCommand string
	PaneTitle      string
	CurrentPath    string
}

type PaneInfo struct {
	SessionName  string
	WindowIndex  int
	Command      string
	PaneActive   bool
	WindowActive bool
	PaneTitle    string
	ActivityFlag bool
	BellFlag     bool
	SilenceFlag  bool
	CurrentPath  string
}

type SessionManager interface {
	ListSessions() ([]Session, error)
	ListPanes() ([]PaneInfo, error)
	NewSession(name, cwd string) error
	SendKeys(target, command string) error
	RenameSession(oldName, newName string) error
	KillSession(name string) error
	CapturePane(session string) (string, error)
	AttachCommand(name string) *exec.Cmd
}

type Client struct{}

func NewClient() SessionManager {
	return &Client{}
}

func (c *Client) ListSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F",
		"#{session_name}:#{session_windows}:#{?session_attached,attached,detached}:#{session_alerts}:#{session_activity}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if bytes.Contains(out, []byte("no server running")) ||
			bytes.Contains(out, []byte("error connecting to")) {
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

		parts := strings.SplitN(line, ":", 5)
		if len(parts) < 3 {
			continue
		}

		windows, err := strconv.Atoi(parts[1])
		if err != nil {
			windows = 0
		}

		s := Session{
			Name:     parts[0],
			Windows:  windows,
			Attached: parts[2] == "attached",
		}

		if len(parts) >= 4 {
			alertStr := strings.TrimSpace(parts[3])
			s.HasAlerts = alertStr != ""
			s.AlertsBell = strings.Contains(alertStr, "!")
			s.AlertsActivity = strings.Contains(alertStr, "#")
			s.AlertsSilence = strings.Contains(alertStr, "~")
		}
		if len(parts) >= 5 {
			if ts, err := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64); err == nil {
				s.LastActivity = ts
			}
		}

		sessions = append(sessions, s)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan tmux sessions: %w", err)
	}

	return sessions, nil
}

func (c *Client) ListPanes() ([]PaneInfo, error) {
	cmd := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{session_name}\t#{window_index}\t#{pane_current_command}\t#{?pane_active,1,0}\t#{?window_active,1,0}\t#{window_activity_flag}\t#{window_bell_flag}\t#{window_silence_flag}\t#{pane_title}\t#{pane_current_path}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if bytes.Contains(out, []byte("no server running")) ||
			bytes.Contains(out, []byte("no current")) {
			return []PaneInfo{}, nil
		}
		return nil, fmt.Errorf("tmux list-panes: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	panes := make([]PaneInfo, 0)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 10)
		if len(parts) < 5 {
			continue
		}

		winIdx, _ := strconv.Atoi(parts[1])
		p := PaneInfo{
			SessionName:  parts[0],
			WindowIndex:  winIdx,
			Command:      parts[2],
			PaneActive:   parts[3] == "1",
			WindowActive: parts[4] == "1",
		}
		if len(parts) >= 6 {
			p.ActivityFlag = parts[5] == "1"
		}
		if len(parts) >= 7 {
			p.BellFlag = parts[6] == "1"
		}
		if len(parts) >= 8 {
			p.SilenceFlag = parts[7] == "1"
		}
		if len(parts) >= 9 {
			p.PaneTitle = parts[8]
		}
		if len(parts) >= 10 {
			p.CurrentPath = parts[9]
		}
		panes = append(panes, p)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan tmux panes: %w", err)
	}

	return panes, nil
}

type ActivePaneState struct {
	Command      string
	PaneTitle    string
	CurrentPath  string
	BellFlag     bool
	ActivityFlag bool
	SilenceFlag  bool
}

func ActivePaneStates(panes []PaneInfo) map[string]ActivePaneState {
	result := make(map[string]ActivePaneState)
	for _, p := range panes {
		state, exists := result[p.SessionName]

		// Active window+pane provides command, title, and path
		if p.WindowActive && p.PaneActive {
			state.Command = p.Command
			state.PaneTitle = stripTitleBranding(strings.TrimSpace(p.PaneTitle))
			state.CurrentPath = p.CurrentPath
		}

		// Aggregate alert flags across all windows in the session
		if p.BellFlag {
			state.BellFlag = true
		}
		if p.ActivityFlag {
			state.ActivityFlag = true
		}
		if p.SilenceFlag {
			state.SilenceFlag = true
		}

		if p.WindowActive && p.PaneActive || exists {
			result[p.SessionName] = state
		}
	}
	return result
}

// stripTitleBranding removes common app-branding prefixes from pane titles
// (e.g. Claude Code's ✳ prefix) for cleaner display.
func stripTitleBranding(title string) string {
	for _, prefix := range []string{"✳ ", "* "} {
		if strings.HasPrefix(title, prefix) {
			return strings.TrimPrefix(title, prefix)
		}
	}
	return title
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

func (c *Client) CapturePane(session string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-e", "-t", session, "-p")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (c *Client) AttachCommand(name string) *exec.Cmd {
	return exec.Command("tmux", "attach", "-t", name)
}
