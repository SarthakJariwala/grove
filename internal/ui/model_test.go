package ui

import (
	"errors"
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

func colorsEqual(a, b lipgloss.TerminalColor) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

type fakeSessionManager struct {
	listSessionsFn func() ([]tmux.Session, error)
	listPanesFn    func() ([]tmux.PaneInfo, error)
	capturePaneFn  func(target string) (string, error)
}

func (f fakeSessionManager) LoadSnapshot() (tmux.SessionSnapshot, error) {
	sessions, err := f.ListSessions()
	if err != nil {
		return tmux.SessionSnapshot{}, err
	}

	panes, err := f.ListPanes()
	if err != nil {
		return tmux.SessionSnapshot{
			Sessions:       append([]tmux.Session(nil), sessions...),
			SessionWindows: map[string][]int{},
			ActiveWindows:  map[string]int{},
			PaneDataFresh:  false,
		}, nil
	}

	return tmux.AssembleSessionSnapshot(sessions, panes), nil
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

func (f fakeSessionManager) NewSessionWithCommand(name, cwd, command string) error { return nil }

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

func TestRenderTreePaneOmitsSessionRuntimeMetadata(t *testing.T) {
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

	for _, want := range []string{"● API", "● one"} {
		if !strings.Contains(got, want) {
			t.Fatalf("tree view = %q, want %q", got, want)
		}
	}
	for _, hidden := range []string{"Claude Code", "nvim", "#"} {
		if strings.Contains(got, hidden) {
			t.Fatalf("tree view = %q, should not include %q", got, hidden)
		}
	}
}

func TestInstanceDetailLinesKeepOnlyOperationalFields(t *testing.T) {
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
	m.setSelected(3)

	row, ok := m.selectedSessionRow()
	if !ok {
		t.Fatal("expected selected session row")
	}
	got := strings.Join(m.instanceDetailLines(row, 80), "\n")

	for _, removed := range []string{"Full name", "Folder:", "Path:"} {
		if strings.Contains(got, removed) {
			t.Fatalf("detail lines = %q, should not include %q", got, removed)
		}
	}

	for _, keep := range []string{"Running", "Active", "activity", "node"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("detail lines = %q, want %q", got, keep)
		}
	}
}

func TestRenderDetailPaneSessionShowsPreviewInNormalMode(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "SQL", Path: "/tmp/sql", Namespace: "sql"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{
			Name:           "sql/sdk",
			Windows:        2,
			CurrentCommand: "node",
		}},
	}
	m.previewSession = "sql/sdk"
	m.previewWindow = 1
	m.previewContent = "top line\npreview output"
	m.rebuildRows()
	m.setSelected(1)

	got := m.renderDetailPane(20, 80, 84, false)

	for _, want := range []string{"Preview", "preview output"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail pane = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "Running") {
		t.Fatalf("detail pane = %q, should not include session detail labels", got)
	}
}

func TestRenderDetailPaneFolderSummarizesSections(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands: []config.Command{
			{Name: "start", Command: "make start"},
			{Name: "worker", Command: "make worker"},
		},
	}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {
			{Name: "api/agent-codex-1", Windows: 1, CurrentCommand: "codex"},
			{Name: "api/term-1", Windows: 1},
			{Name: "api/cmd-start", Windows: 1, CurrentCommand: "make"},
		},
	}
	m.rebuildRows()
	m.setSelected(0)

	got := m.renderDetailPane(20, 80, 84, false)

	for _, want := range []string{"API", "1 agent", "1 terminal", "2 commands", "OVERVIEW", "SESSIONS"} {
		if !strings.Contains(got, want) {
			t.Fatalf("detail pane = %q, want %q", got, want)
		}
	}
}

func TestRenderHeaderShowsFolderCountOnly(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API"}, {Name: "Web"}}}, "config.toml", fakeSessionManager{})
	got := m.renderHeader()

	if !strings.Contains(got, "2 folders") {
		t.Fatalf("header = %q, want folder count", got)
	}
	if strings.Contains(got, "sessions") {
		t.Fatalf("header = %q, should not include session count", got)
	}
	if strings.Contains(got, "▸ grove") {
		t.Fatalf("header = %q, should not include old title prefix", got)
	}
}

func TestRenderHeaderShowsActiveFilter(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API"}, {Name: "Web"}}}, "config.toml", fakeSessionManager{})
	m.filterQuery = "beta"
	got := m.renderHeader()

	if !strings.Contains(got, "2 folders") {
		t.Fatalf("header = %q, want folder count", got)
	}
	if !strings.Contains(got, "filter: beta") {
		t.Fatalf("header = %q, want active filter", got)
	}
}

func TestTreeLineTextFolderOmitsIdleZeroCount(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fakeSessionManager{})
	m.rebuildRows()

	got := m.treeLineText(m.rows[0], 40)
	if strings.Contains(got, "0") {
		t.Fatalf("treeLineText(folder) = %q, should omit idle zero count", got)
	}
	if !strings.Contains(got, "▸") {
		t.Fatalf("treeLineText(folder) = %q, want collapsed caret", got)
	}
	if !strings.Contains(got, "●") {
		t.Fatalf("treeLineText(folder) = %q, want status dot", got)
	}
}

func TestTreeLineTextChildrenUseDeeperIndent(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "dev", Command: "make dev"}},
	}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {
			{Name: "api/agent-pi-1", Windows: 1, CurrentCommand: "pi"},
			{Name: "api/term-1", Windows: 1},
		},
	}
	m.rebuildRows()

	for idx, want := range map[int]string{
		1: "    ◆ Pi #1",
		2: "    ○ Terminal #1",
		3: "    ■ dev",
	} {
		got := m.treeLineText(m.rows[idx], 40)
		if !strings.HasPrefix(got, want) {
			t.Fatalf("treeLineText(row %d) = %q, want prefix %q", idx, got, want)
		}
	}
}

func TestRenderTreePaneShowsDirectChildrenWithoutSectionHeadings(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "dev", Command: "make dev"}},
	}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {
			{Name: "api/agent-pi-1", Windows: 1, CurrentCommand: "pi"},
			{Name: "api/term-1", Windows: 1},
		},
	}
	m.rebuildRows()

	got := m.renderTreePane(8, 40, 44, false)

	for _, want := range []string{"◆ Pi #1", "active", "○ Terminal #1", "■ dev"} {
		if !strings.Contains(got, want) {
			t.Fatalf("tree view = %q, want %q", got, want)
		}
	}
	for _, hidden := range []string{"Agents", "Terminals", "Commands"} {
		if strings.Contains(got, hidden) {
			t.Fatalf("tree view = %q, should not contain %q", got, hidden)
		}
	}
}

func TestRenderTreePaneAddsBreathingRoomAfterExpandedFolderOnly(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{
		{Name: "API", Path: "/tmp/api", Namespace: "api", Commands: []config.Command{{Name: "dev", Command: "make dev"}}},
		{Name: "Web", Path: "/tmp/web", Namespace: "web"},
		{Name: "Docs", Path: "/tmp/docs", Namespace: "docs"},
	}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{Name: "api/term-1", Windows: 1}},
	}
	m.rebuildRows()

	got := m.renderTreePane(8, 40, 44, false)
	if !strings.Contains(got, "○ Terminal #1") || !strings.Contains(got, "● Web") {
		t.Fatalf("tree view = %q, expected expanded then collapsed folders", got)
	}
	if !strings.Contains(got, "│     ■ dev                                │\n│                                          │\n│ ▸ ● Web") {
		t.Fatalf("tree view = %q, want one blank line between expanded folder content and next folder", got)
	}
	if strings.Contains(got, "▸ ● Web                               │\n│                                          │\n│ ▸ ● Docs") {
		t.Fatalf("tree view = %q, should not add blank line between collapsed folders", got)
	}
}

func TestRenderTreePaneShowsRunningCommandIcon(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "dev", Command: "make dev"}},
	}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {
			{Name: "api/cmd-dev", Windows: 1, CurrentCommand: "make"},
		},
	}
	m.rebuildRows()

	got := m.renderTreePane(6, 40, 44, false)
	if !strings.Contains(got, "▶ dev") {
		t.Fatalf("tree view = %q, want running command icon", got)
	}
	if strings.Contains(got, "■ dev") {
		t.Fatalf("tree view = %q, should not show stopped command icon", got)
	}
}

func TestTerminalTreeIconUsesFilledCircleForActiveCommands(t *testing.T) {
	t.Parallel()

	if got := terminalTreeIcon(treeRow{typeOf: rowTerminalInstance, currentCommand: "npm"}); got != "●" {
		t.Fatalf("terminalTreeIcon(active) = %q, want filled circle", got)
	}
	if got := terminalTreeIcon(treeRow{typeOf: rowTerminalInstance, currentCommand: "zsh"}); got != "○" {
		t.Fatalf("terminalTreeIcon(shell) = %q, want hollow circle", got)
	}
}

func TestTerminalTreeIconStyleHighlightsActiveCommands(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", fakeSessionManager{})
	got := m.terminalTreeIconStyle(treeRow{typeOf: rowTerminalInstance, currentCommand: "npm"}).GetForeground()
	want := lipgloss.Color(colorTreeActive)
	if !colorsEqual(got, want) {
		t.Fatalf("terminalTreeIconStyle(active).Foreground = %v, want %v", got, want)
	}
}

func TestTreeLineStyledFolderStatusPriorityPrefersAttention(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{0: {{
		Name:           "api/term-1",
		Windows:        1,
		AlertsActivity: true,
	}}}
	m.rebuildRows()

	if got := m.folderStatus(0); got != folderStatusAttention {
		t.Fatalf("folderStatus(0) = %v, want %v", got, folderStatusAttention)
	}
	if !colorsEqual(m.styles.folderDotAttention.GetForeground(), lipgloss.Color(colorTreeAttention)) {
		t.Fatalf("folder attention color = %#v, want %s", m.styles.folderDotAttention.GetForeground(), colorTreeAttention)
	}
}

func TestTreeLineStyledSelectedRowUsesTextEmphasisOnly(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/term-1", Windows: 1}}}
	m.rebuildRows()
	m.setSelected(1)

	got := m.treeLineStyled(m.rows[1], m.treeLineText(m.rows[1], 40), 40)
	if lipgloss.Width(got) >= 40 {
		t.Fatalf("selected tree line width = %d, want less than 40 without full-row highlight", lipgloss.Width(got))
	}
	if !colorsEqual(m.styles.rowSelectedText.GetForeground(), lipgloss.Color(colorTreeAccent)) {
		t.Fatalf("selected row text color = %#v, want %s", m.styles.rowSelectedText.GetForeground(), colorTreeAccent)
	}
}

func TestRenderHelpBarCommandRowsShowLifecycleBindings(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fakeSessionManager{})
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", commandText: "make start", status: "running"}}

	running := m.renderHelpBar()
	for _, want := range []string{"attach", "preview", "stop", "restart", "dev command"} {
		if !strings.Contains(running, want) {
			t.Fatalf("running help bar = %q, want %q", running, want)
		}
	}
	if strings.Contains(running, "kill") {
		t.Fatalf("running help bar = %q, should not advertise kill for command rows", running)
	}
	if strings.Contains(running, "send cmd") {
		t.Fatalf("running help bar = %q, should not advertise send cmd for command rows", running)
	}

	m.rows[0].status = "stopped"
	stopped := m.renderHelpBar()
	for _, want := range []string{"start", "restart", "dev command"} {
		if !strings.Contains(stopped, want) {
			t.Fatalf("stopped help bar = %q, want %q", stopped, want)
		}
	}
	for _, unwanted := range []string{"attach", "preview", "send cmd", "kill"} {
		if strings.Contains(stopped, unwanted) {
			t.Fatalf("stopped help bar = %q, should not include %q", stopped, unwanted)
		}
	}
}

func TestRenderHelpBarFolderRowsShowDevCommandWithoutSendCmd(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fakeSessionManager{})
	m.rebuildRows()
	m.setSelected(0)

	got := m.renderHelpBar()
	if !strings.Contains(got, "dev command") {
		t.Fatalf("folder help bar = %q, want dev command binding", got)
	}
	if strings.Contains(got, "send cmd") {
		t.Fatalf("folder help bar = %q, should not advertise send cmd on folder rows", got)
	}
}

func TestRenderHelpBarSessionRowsShowSendCmdAndDevCommand(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/term-1", Windows: 1}}}
	m.rebuildRows()
	m.setSelected(1)

	got := m.renderHelpBar()
	for _, want := range []string{"send cmd", "dev command"} {
		if !strings.Contains(got, want) {
			t.Fatalf("session help bar = %q, want %q", got, want)
		}
	}
}

func TestSelectedHelpersAreRowTypeAware(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands: []config.Command{
			{Name: "start", Command: "make start"},
			{Name: "worker", Command: "make worker"},
		},
	}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {
			{Name: "api/term-1", Windows: 1},
			{Name: "api/cmd-start", Windows: 1, CurrentCommand: "make"},
			{Name: "api/cmd-worker", Windows: 1, CurrentCommand: "zsh"},
		},
	}
	m.rebuildRows()

	// rows: [0]=folder, [1]=term-1, [2]=cmd-start, [3]=cmd-worker
	for _, selected := range []int{0, 1, 2, 3} {
		m.setSelected(selected)
		folder, ok := m.selectedFolder()
		if !ok || folder.Namespace != "api" {
			t.Fatalf("selectedFolder() at row %d = (%#v, %v), want api folder", selected, folder, ok)
		}
	}

	m.setSelected(2)
	runningCommand, ok := m.selectedSessionRow()
	if !ok || runningCommand.typeOf != rowCommand || runningCommand.sessionName != "api/cmd-start" {
		t.Fatalf("selectedSessionRow() running command = (%#v, %v), want running command row", runningCommand, ok)
	}

	m.setSelected(3)
	if row, ok := m.selectedSessionRow(); ok {
		t.Fatalf("selectedSessionRow() on stopped command = (%#v, %v), want false", row, ok)
	}
}

func TestInstanceDetailLinesDeriveAlertsFromFlags(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{
			Name:           "api/term-1",
			Windows:        1,
			CurrentCommand: "go",
			AlertsActivity: true,
		}},
	}
	m.rebuildRows()
	m.setSelected(3)

	row, ok := m.selectedSessionRow()
	if !ok {
		t.Fatal("expected selected session row")
	}
	got := strings.Join(m.instanceDetailLines(row, 80), "\n")
	if !strings.Contains(got, "activity") {
		t.Fatalf("detail lines = %q, want derived alert chip", got)
	}
}

func TestRebuildRowsPreservesSelectedSessionByIdentity(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/term-1", Windows: 1}}}
	m.rebuildRows()
	m.setSelected(3)

	m.sessions = map[int][]tmux.Session{0: {
		{Name: "api/agent-codex-1", Windows: 1},
		{Name: "api/term-1", Windows: 1},
	}}
	m.rebuildRows()

	row, ok := m.selectedSessionRow()
	if !ok || row.sessionName != "api/term-1" {
		t.Fatalf("selectedSessionRow() after rebuild = (%#v, %v), want api/term-1", row, ok)
	}
}

func TestRebuildRowsAppliesFilterQueryToSessions(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{
		{Name: "API", Path: "/tmp/api", Namespace: "api"},
		{Name: "Web", Path: "/tmp/web", Namespace: "web"},
	}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {
			{Name: "api/alpha", Windows: 1},
			{Name: "api/beta", Windows: 1},
		},
		1: {{Name: "web/gamma", Windows: 1}},
	}
	m.filterQuery = "beta"

	m.rebuildRows()

	if len(m.rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (matching folder + matching session)", len(m.rows))
	}
	if m.rows[0].typeOf != rowFolder || m.rows[0].folderIndex != 0 {
		t.Fatalf("rows[0] = %#v, want API folder row", m.rows[0])
	}
	if m.rows[1].typeOf != rowTerminalInstance || m.rows[1].sessionName != "api/beta" {
		t.Fatalf("rows[1] = %#v, want matching beta session row", m.rows[1])
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

func TestLoadSessionsCmdGroupsSessionsWhenListPanesFails(t *testing.T) {
	t.Parallel()

	fake := fakeSessionManager{
		listSessionsFn: func() ([]tmux.Session, error) {
			return []tmux.Session{{Name: "api/one", Windows: 1}}, nil
		},
		listPanesFn: func() ([]tmux.PaneInfo, error) {
			return nil, errors.New("tmux list-panes failed")
		},
	}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)

	msg := m.loadSessionsCmd()()
	loaded, ok := msg.(sessionsLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want sessionsLoadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("err = %v, want nil", loaded.err)
	}
	if len(loaded.sessions[0]) != 1 || loaded.sessions[0][0].Name != "api/one" {
		t.Fatalf("sessions = %#v, want grouped api/one session", loaded.sessions)
	}
	if loaded.panesFresh {
		t.Fatal("panesFresh = true, want false when ListPanes fails")
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
	m.setSelected(3)

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
	m.setSelected(2) // select the session row

	dimmed := m.renderTreePane(8, 60, 64, true)

	// Dimmed pane should not contain the bright primary green
	// #5eead4 in truecolor ANSI = 38;2;94;234;212
	colorGreen := "38;2;94;234;212"
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

	// #2dd4a8 (colorPrimaryDim for folder names) in truecolor ANSI = 38;2;45;212;168
	colorFolder := "38;2;45;212;168"
	if strings.Contains(dimmed, colorFolder) {
		t.Fatalf("dimmed tree pane should not contain folder color (%s), got:\n%s", colorFolder, dimmed)
	}
}
