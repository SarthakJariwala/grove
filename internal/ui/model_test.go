package ui

import (
	"os/exec"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type fakeSessionManager struct {
	listSessionsFn func() ([]tmux.Session, error)
	listPanesFn    func() ([]tmux.PaneInfo, error)
}

func (f fakeSessionManager) ListSessions() ([]tmux.Session, error) {
	if f.listSessionsFn == nil {
		return nil, nil
	}
	return f.listSessionsFn()
}

func (f fakeSessionManager) ListPanes() ([]tmux.PaneInfo, error) {
	if f.listPanesFn == nil {
		return nil, nil
	}
	return f.listPanesFn()
}

func (f fakeSessionManager) NewSession(name, cwd string) error { return nil }

func (f fakeSessionManager) SendKeys(target, command string) error { return nil }

func (f fakeSessionManager) RenameSession(oldName, newName string) error { return nil }

func (f fakeSessionManager) KillSession(name string) error { return nil }

func (f fakeSessionManager) CapturePane(session string) (string, error) { return "", nil }

func (f fakeSessionManager) AttachCommand(name string) *exec.Cmd {
	return exec.Command("sh", "-c", "true")
}

func TestWindowAround(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		selected, total, n int
		wantStart, wantEnd int
	}{
		{name: "empty", selected: 0, total: 0, n: 5, wantStart: 0, wantEnd: 0},
		{name: "all fit", selected: 2, total: 5, n: 10, wantStart: 0, wantEnd: 5},
		{name: "middle window", selected: 5, total: 20, n: 7, wantStart: 2, wantEnd: 9},
		{name: "near start", selected: 1, total: 20, n: 7, wantStart: 0, wantEnd: 7},
		{name: "near end", selected: 19, total: 20, n: 7, wantStart: 13, wantEnd: 20},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start, end := windowAround(tt.selected, tt.total, tt.n)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("windowAround(%d,%d,%d) = (%d,%d), want (%d,%d)", tt.selected, tt.total, tt.n, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestSanitizeLeaf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: " Feature/ABC 123 ", want: "featureabc-123"},
		{in: "----", want: "session"},
		{in: "my__name", want: "my-name"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeLeaf(tt.in); got != tt.want {
				t.Fatalf("sanitizeLeaf(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeANSI(t *testing.T) {
	t.Parallel()

	in := "a\x1b[31mred\x1b[0m\x1b[2Jb"
	want := "a\x1b[31mred\x1b[0mb"

	if got := sanitizeANSI(in); got != want {
		t.Fatalf("sanitizeANSI() = %q, want %q", got, want)
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Duration
		want string
	}{
		{name: "negative", in: -time.Second, want: "just now"},
		{name: "under 30s", in: 29 * time.Second, want: "just now"},
		{name: "one minute", in: 89 * time.Second, want: "1 min ago"},
		{name: "minutes", in: 10 * time.Minute, want: "10 mins ago"},
		{name: "one hour", in: 90 * time.Minute, want: "1 hour ago"},
		{name: "hours", in: 5 * time.Hour, want: "5 hours ago"},
		{name: "one day", in: 25 * time.Hour, want: "1 day ago"},
		{name: "days", in: 49 * time.Hour, want: "2 days ago"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatDuration(tt.in); got != tt.want {
				t.Fatalf("formatDuration(%s) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestPaneDisplayTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "hostname filtered", in: "my-host", want: ""},
		{name: "path kept", in: "/Users/me/project", want: "/Users/me/project"},
		{name: "space kept", in: "Claude Code", want: "Claude Code"},
		{name: "extension kept", in: "main.go", want: "main.go"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := paneDisplayTitle(treeRow{paneTitle: tt.in})
			if got != tt.want {
				t.Fatalf("paneDisplayTitle(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadSessionsCmdGroupsAndEnriches(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{
		{Name: "API", Path: "/tmp/api", Namespace: "api"},
		{Name: "Web", Path: "/tmp/web", Namespace: "web"},
	}}

	fake := fakeSessionManager{
		listSessionsFn: func() ([]tmux.Session, error) {
			return []tmux.Session{
				{Name: "api/one"},
				{Name: "web/two"},
				{Name: "other/skip"},
			}, nil
		},
		listPanesFn: func() ([]tmux.PaneInfo, error) {
			return []tmux.PaneInfo{
				{SessionName: "api/one", WindowActive: true, PaneActive: true, Command: "go", PaneTitle: "* Claude", CurrentPath: "/tmp/api", BellFlag: true},
				{SessionName: "api/one", ActivityFlag: true},
			}, nil
		},
	}

	m := NewModel(cfg, "config.toml", fake)
	msg := m.loadSessionsCmd()()
	loaded, ok := msg.(sessionsLoadedMsg)
	if !ok {
		t.Fatalf("loadSessionsCmd() returned %T, want sessionsLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("sessionsLoadedMsg.err = %v", loaded.err)
	}

	if len(loaded.sessions[0]) != 1 || loaded.sessions[0][0].Name != "api/one" {
		t.Fatalf("folder 0 sessions = %#v, want only api/one", loaded.sessions[0])
	}
	if len(loaded.sessions[1]) != 1 || loaded.sessions[1][0].Name != "web/two" {
		t.Fatalf("folder 1 sessions = %#v, want only web/two", loaded.sessions[1])
	}

	api := loaded.sessions[0][0]
	if api.CurrentCommand != "go" {
		t.Fatalf("api.CurrentCommand = %q, want %q", api.CurrentCommand, "go")
	}
	if api.PaneTitle != "Claude" {
		t.Fatalf("api.PaneTitle = %q, want %q", api.PaneTitle, "Claude")
	}
	if !api.AlertsBell || !api.AlertsActivity {
		t.Fatalf("api alerts = bell:%v activity:%v, want both true", api.AlertsBell, api.AlertsActivity)
	}
}

func TestLoadSessionsCmdError(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}
	fake := fakeSessionManager{
		listSessionsFn: func() ([]tmux.Session, error) {
			return nil, assertErr("boom")
		},
	}

	m := NewModel(cfg, "config.toml", fake)
	msg := m.loadSessionsCmd()()
	loaded, ok := msg.(sessionsLoadedMsg)
	if !ok {
		t.Fatalf("loadSessionsCmd() returned %T, want sessionsLoadedMsg", msg)
	}
	if loaded.err == nil || loaded.err.Error() != "boom" {
		t.Fatalf("sessionsLoadedMsg.err = %v, want boom", loaded.err)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

var _ tea.Msg = sessionsLoadedMsg{}
