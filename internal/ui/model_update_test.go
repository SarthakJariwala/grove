package ui

import (
	"os/exec"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type trackingSessionManager struct {
	killed []string
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

func (f *trackingSessionManager) CapturePane(session string) (string, error) { return "", nil }

func (f *trackingSessionManager) AttachCommand(name string) *exec.Cmd {
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
