package ui

import (
	"os/exec"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type trackingSessionManager struct {
	killed   []string
	captured []string
	attached []string
}

func (f *trackingSessionManager) ListSessions() ([]tmux.Session, error) { return nil, nil }

func (f *trackingSessionManager) ListPanes() ([]tmux.PaneInfo, error) { return nil, nil }

func (f *trackingSessionManager) NewSession(name, cwd string) error { return nil }

func (f *trackingSessionManager) SendKeys(target, command string) error { return nil }

func (f *trackingSessionManager) RenameSession(oldName, newName string) error { return nil }

func (f *trackingSessionManager) KillSession(name string) error {
	f.killed = append(f.killed, name)
	return nil
}

func (f *trackingSessionManager) CapturePane(target string) (string, error) {
	f.captured = append(f.captured, target)
	return "", nil
}

func (f *trackingSessionManager) AttachCommand(name string) *exec.Cmd {
	f.attached = append(f.attached, name)
	return exec.Command("sh", "-c", "true")
}

func TestUpdateSlashOpensFilterPrompt(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", &trackingSessionManager{})
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got := model.(Model)

	if got.promptMode != promptFilter {
		t.Fatalf("promptMode = %v, want promptFilter", got.promptMode)
	}
}

func TestUpdateEscClearsFilter(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", &trackingSessionManager{})
	m.filterQuery = "api"
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := model.(Model)

	if got.filterQuery != "" {
		t.Fatalf("filterQuery = %q, want empty", got.filterQuery)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil status clear command")
	}
}

func TestUpdateKillConfirmFlow(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/one"}}}
	m.rebuildRows()
	m.setSelected(1)

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	withConfirm := model.(Model)
	if withConfirm.confirmKillTarget != "api/one" {
		t.Fatalf("confirmKillTarget = %q, want %q", withConfirm.confirmKillTarget, "api/one")
	}

	model2, cmd := withConfirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	afterConfirm := model2.(Model)
	if afterConfirm.confirmKillTarget != "" {
		t.Fatalf("confirmKillTarget = %q, want empty", afterConfirm.confirmKillTarget)
	}
	if cmd == nil {
		t.Fatalf("expected kill command")
	}

	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok {
		t.Fatalf("kill cmd returned %T, want actionResultMsg", msg)
	}
	if res.err != nil {
		t.Fatalf("kill action returned error: %v", res.err)
	}
	if len(fake.killed) != 1 || fake.killed[0] != "api/one" {
		t.Fatalf("killed targets = %#v, want [api/one]", fake.killed)
	}
}

func TestUpdateNOpensNewSessionPrompt(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", &trackingSessionManager{})
	m.rebuildRows()

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := model.(Model)

	if got.promptMode != promptNewSession {
		t.Fatalf("promptMode = %v, want promptNewSession", got.promptMode)
	}
}

func TestUpdatePreviewLeftRightCyclesWindows(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{}, "config.toml", fake)
	m.detailMode = detailPreview
	m.previewSession = "api/one"
	m.previewWindow = 0
	m.previewSeq = 1
	m.sessionWindows = map[string][]int{"api/one": {0, 2, 5}}

	model, cmd := m.updatePreview(tea.KeyMsg{Type: tea.KeyRight})
	got := model.(Model)
	if got.previewWindow != 2 {
		t.Fatalf("previewWindow after right = %d, want 2", got.previewWindow)
	}
	if cmd == nil {
		t.Fatal("expected capture command for right navigation")
	}
	_ = cmd()
	if len(fake.captured) != 1 || fake.captured[0] != "api/one:2" {
		t.Fatalf("captured targets = %#v, want [api/one:2]", fake.captured)
	}

	model, cmd = got.updatePreview(tea.KeyMsg{Type: tea.KeyLeft})
	got = model.(Model)
	if got.previewWindow != 0 {
		t.Fatalf("previewWindow after left = %d, want 0", got.previewWindow)
	}
	if cmd == nil {
		t.Fatal("expected capture command for left navigation")
	}
	_ = cmd()
	if len(fake.captured) != 2 || fake.captured[1] != "api/one:0" {
		t.Fatalf("captured targets = %#v, want second target api/one:0", fake.captured)
	}
}

func TestUpdatePreviewQTreatsAsBack(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", &trackingSessionManager{})
	m.detailMode = detailPreview
	m.previewSession = "api/one"
	m.previewWindow = 2
	m.previewZoomed = true

	model, cmd := m.updatePreview(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := model.(Model)
	if cmd != nil {
		t.Fatal("expected no command when q exits zoomed preview")
	}
	if got.detailMode != detailPreview {
		t.Fatalf("detailMode = %v, want detailPreview", got.detailMode)
	}
	if got.previewSession != "api/one" {
		t.Fatalf("previewSession = %q, want api/one", got.previewSession)
	}
	if got.previewZoomed {
		t.Fatal("expected q to unzoom preview before exiting it")
	}

	model, cmd = got.updatePreview(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = model.(Model)
	if cmd != nil {
		t.Fatal("expected no command when q exits preview")
	}
	if got.detailMode != detailNormal {
		t.Fatalf("detailMode = %v, want detailNormal", got.detailMode)
	}
	if got.previewSession != "" {
		t.Fatalf("previewSession = %q, want empty", got.previewSession)
	}
}

func TestPreviewEnterAttachesPreviewSession(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/one"}}}
	m.rebuildRows()
	m.setSelected(0) // folder row, to ensure preview attach does not rely on selected session row
	m.detailMode = detailPreview
	m.previewSession = "api/one"

	model, cmd := m.updatePreview(tea.KeyMsg{Type: tea.KeyEnter})
	got := model.(Model)
	if cmd == nil {
		t.Fatal("expected attach command")
	}
	if got.detailMode != detailNormal {
		t.Fatalf("detailMode = %v, want detailNormal", got.detailMode)
	}
	if len(fake.attached) != 1 || fake.attached[0] != "api/one" {
		t.Fatalf("attached targets = %#v, want [api/one]", fake.attached)
	}
}
