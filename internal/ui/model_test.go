package ui

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

func TestMain(m *testing.M) {
	lipgloss.SetDefaultRenderer(
		lipgloss.NewRenderer(os.Stderr, termenv.WithProfile(termenv.TrueColor)),
	)
	os.Exit(m.Run())
}

type fakeSessionManager struct {
	listSessionsFn func() ([]tmux.Session, error)
	listPanesFn    func() ([]tmux.PaneInfo, error)
	capturePaneFn  func(target string) (string, error)
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

func (f fakeSessionManager) CapturePane(target string) (string, error) {
	if f.capturePaneFn == nil {
		return "", nil
	}
	return f.capturePaneFn(target)
}

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

func TestRenderTreePaneShowsOnlyAlertIndicators(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{
			Name:           "api/one",
			Windows:        1,
			CurrentCommand: "nvim",
			PaneTitle:      "Claude Code",
			AlertsActivity: true,
		}},
	}
	m.rebuildRows()
	m.setSelected(1)

	got := m.renderTreePane(8, 60, 64, false)

	if !strings.Contains(got, "#") {
		t.Fatalf("tree view = %q, want activity indicator", got)
	}
	if strings.Contains(got, "Claude Code") {
		t.Fatalf("tree view = %q, should not include pane title", got)
	}
	if strings.Contains(got, "nvim") {
		t.Fatalf("tree view = %q, should not include current command", got)
	}
}

func TestRenderDetailPaneSessionKeepsOnlyOperationalFields(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "SQL", Path: "/tmp/sql", Namespace: "sql"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{
			Name:           "sql/sdk-configurability",
			Windows:        2,
			CurrentCommand: "node",
			PaneTitle:      "amp - Architecture improvements",
			LastActivity:   time.Now().Add(-17 * time.Minute).Unix(),
			AlertsActivity: true,
			HasAlerts:      true,
		}},
	}
	m.rebuildRows()
	m.setSelected(1)

	got := m.renderDetailPane(20, 80, 84, false)

	for _, removed := range []string{"Full name", "Folder:", "Path:"} {
		if strings.Contains(got, removed) {
			t.Fatalf("detail pane = %q, should not include %q", got, removed)
		}
	}

	for _, keep := range []string{"Running", "Active", "activity", "node"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("detail pane = %q, want %q", got, keep)
		}
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

func TestStartPreviewUsesActiveWindowTarget(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{
		capturePaneFn: func(target string) (string, error) {
			return "captured:" + target, nil
		},
	})
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/one"}}}
	m.sessionWindows = map[string][]int{"api/one": {0, 2, 5}}
	m.activeWindows = map[string]int{"api/one": 2}
	m.rebuildRows()
	m.setSelected(1)

	cmd := m.startPreview()
	if m.previewSession != "api/one" {
		t.Fatalf("previewSession = %q, want api/one", m.previewSession)
	}
	if m.previewWindow != 2 {
		t.Fatalf("previewWindow = %d, want 2", m.previewWindow)
	}
	if cmd == nil {
		t.Fatal("startPreview() returned nil command")
	}

	msg := cmd()
	captured, ok := msg.(paneCapturedMsg)
	if !ok {
		t.Fatalf("capture cmd returned %T, want paneCapturedMsg", msg)
	}
	if captured.target != "api/one:2" {
		t.Fatalf("captured target = %q, want api/one:2", captured.target)
	}
}

func TestMovePreviewWindowWrapsSparseIndexes(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", fakeSessionManager{})
	m.previewSession = "api/one"
	m.previewWindow = 0
	m.previewSeq = 1
	m.sessionWindows = map[string][]int{"api/one": {0, 2, 5}}

	cmd := m.movePreviewWindow(-1)
	if m.previewWindow != 5 {
		t.Fatalf("previewWindow after left = %d, want 5", m.previewWindow)
	}
	if m.previewSeq != 2 {
		t.Fatalf("previewSeq after left = %d, want 2", m.previewSeq)
	}
	if cmd == nil {
		t.Fatal("movePreviewWindow(-1) returned nil command")
	}
	if msg, ok := cmd().(paneCapturedMsg); !ok || msg.target != "api/one:5" {
		t.Fatalf("left capture = %#v, want target api/one:5", msg)
	}

	cmd = m.movePreviewWindow(1)
	if m.previewWindow != 0 {
		t.Fatalf("previewWindow after right = %d, want 0", m.previewWindow)
	}
	if m.previewSeq != 3 {
		t.Fatalf("previewSeq after right = %d, want 3", m.previewSeq)
	}
	if cmd == nil {
		t.Fatal("movePreviewWindow(1) returned nil command")
	}
	if msg, ok := cmd().(paneCapturedMsg); !ok || msg.target != "api/one:0" {
		t.Fatalf("right capture = %#v, want target api/one:0", msg)
	}
}

func TestReconcilePreviewWindowClampsWhenWindowRemoved(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", fakeSessionManager{})
	m.detailMode = detailPreview
	m.previewSession = "api/one"
	m.previewWindow = 2
	m.previewSeq = 10
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/one"}}}
	m.sessionWindows = map[string][]int{"api/one": {0, 5}}

	cmd := m.reconcilePreviewAfterLoad()
	if m.previewWindow != 5 {
		t.Fatalf("previewWindow = %d, want 5", m.previewWindow)
	}
	if m.previewSeq != 11 {
		t.Fatalf("previewSeq = %d, want 11", m.previewSeq)
	}
	if cmd == nil {
		t.Fatal("reconcilePreviewAfterLoad() returned nil command")
	}
	if msg, ok := cmd().(paneCapturedMsg); !ok || msg.target != "api/one:5" {
		t.Fatalf("reconcile capture = %#v, want target api/one:5", msg)
	}
}

func TestReconcilePreviewExitsWhenSessionRemoved(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", fakeSessionManager{})
	m.detailMode = detailPreview
	m.previewSession = "api/one"
	m.previewWindow = 1
	m.sessions = map[int][]tmux.Session{}

	cmd := m.reconcilePreviewAfterLoad()
	if cmd == nil {
		t.Fatal("reconcilePreviewAfterLoad() should return status clear command")
	}
	if m.detailMode != detailNormal {
		t.Fatalf("detailMode = %v, want detailNormal", m.detailMode)
	}
	if m.previewSession != "" {
		t.Fatalf("previewSession = %q, want empty", m.previewSession)
	}
	if m.statusMsg != "preview closed: session ended" {
		t.Fatalf("statusMsg = %q, want preview closed message", m.statusMsg)
	}
}

func TestPaneCapturedMsgIgnoresStalePreviewTarget(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", fakeSessionManager{})
	m.detailMode = detailPreview
	m.previewSession = "api/one"
	m.previewWindow = 2
	m.previewSeq = 4
	m.previewContent = "fresh"

	updated, _ := m.Update(paneCapturedMsg{target: "api/one:0", content: "stale", seq: 3})
	got := updated.(Model)
	if got.previewContent != "fresh" {
		t.Fatalf("previewContent = %q, want fresh", got.previewContent)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

var _ tea.Msg = sessionsLoadedMsg{}

func TestRenderTreePaneDimSessionRows(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{
			Name:     "api/service",
			Windows:  2,
			Attached: true,
		}},
	}
	m.rebuildRows()

	normal := m.renderTreePane(8, 60, 64, false)
	dimmed := m.renderTreePane(8, 60, 64, true)

	if normal == dimmed {
		t.Fatal("dim=true output should differ from dim=false output")
	}

	// #73daca in truecolor ANSI = 38;2;115;218;202
	colorGreen := "38;2;115;218;202"
	if strings.Contains(dimmed, colorGreen) {
		t.Fatalf("dimmed tree pane should not contain bright green (%s), got:\n%s", colorGreen, dimmed)
	}
}

func TestRenderTreePaneDimFolderRows(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{
		{Name: "API", Path: "/tmp/api", Namespace: "api"},
		{Name: "Web", Path: "/tmp/web", Namespace: "web"},
	}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{Name: "api/one", Windows: 1}},
		1: {},
	}
	m.rebuildRows()
	// Select the first folder so the second folder is non-selected
	m.setSelected(0)

	dimmed := m.renderTreePane(8, 60, 64, true)

	// #c9d1d9 in truecolor ANSI = 38;2;201;209;217
	colorText := "38;2;201;209;217"
	if strings.Contains(dimmed, colorText) {
		t.Fatalf("dimmed tree pane should not contain folder text color (%s), got:\n%s", colorText, dimmed)
	}
}
