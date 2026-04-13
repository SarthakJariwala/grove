package ui

import (
	"os/exec"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/configfile"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type trackingSessionManager struct {
	killed   []string
	captured []string
	attached []string
	created  []string
	launched []string
	commands []string
}

func (f *trackingSessionManager) ListSessions() ([]tmux.Session, error) { return nil, nil }

func (f *trackingSessionManager) ListPanes() ([]tmux.PaneInfo, error) { return nil, nil }

func (f *trackingSessionManager) NewSession(name, cwd string) error {
	f.created = append(f.created, name)
	return nil
}

func (f *trackingSessionManager) NewSessionWithCommand(name, cwd, command string) error {
	f.launched = append(f.launched, name)
	f.commands = append(f.commands, command)
	return nil
}

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

func TestUpdateNavigationStartsAutoPreviewForSelectedSession(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)
	m.sessions = map[int][]tmux.Session{
		0: {{
			Name:           "api/one",
			CurrentCommand: "bash",
		}},
	}
	m.sessionWindows = map[string][]int{"api/one": {0, 2}}
	m.activeWindows = map[string]int{"api/one": 2}
	m.rebuildRows()

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := model.(Model)

	if got.detailMode != detailNormal {
		t.Fatalf("detailMode = %v, want detailNormal", got.detailMode)
	}
	if got.previewSession != "api/one" {
		t.Fatalf("previewSession = %q, want api/one", got.previewSession)
	}
	if got.previewWindow != 2 {
		t.Fatalf("previewWindow = %d, want 2", got.previewWindow)
	}
	if cmd == nil {
		t.Fatal("expected capture command")
	}
	_ = cmd()
	if len(fake.captured) != 1 || fake.captured[0] != "api/one:2" {
		t.Fatalf("captured targets = %#v, want [api/one:2]", fake.captured)
	}
}

func TestUpdateNCreatesManagedTerminal(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)
	m.rebuildRows()

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected terminal create command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.attachTarget != "api/term-1" {
		t.Fatalf("terminal result = %#v, want attachTarget api/term-1", msg)
	}
	if len(fake.created) != 1 || fake.created[0] != "api/term-1" {
		t.Fatalf("created sessions = %#v, want [api/term-1]", fake.created)
	}
	if len(fake.launched) != 0 {
		t.Fatalf("launched sessions = %#v, want none", fake.launched)
	}
	if len(fake.attached) != 0 {
		t.Fatalf("attached sessions = %#v, command should only prepare attach result", fake.attached)
	}
}

func TestUpdateAOpensAgentPicker(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{
		Agents:  []config.Agent{{Name: "Codex", Command: "codex"}},
		Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}},
	}, "config.toml", &trackingSessionManager{})
	m.rebuildRows()

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := model.(Model)

	if got.overlayMode != overlayAgentPicker {
		t.Fatalf("overlayMode = %v, want overlayAgentPicker", got.overlayMode)
	}
	if got.overlayFolderIndex != 0 {
		t.Fatalf("overlayFolderIndex = %d, want 0", got.overlayFolderIndex)
	}
	if len(got.agentChoices) != 2 {
		t.Fatalf("len(agentChoices) = %d, want 2", len(got.agentChoices))
	}
	if got.agentChoices[0].Label != "Codex" || !got.agentChoices[0].Persist {
		t.Fatalf("agentChoices[0] = %#v, want global Codex choice marked for persistence", got.agentChoices[0])
	}
	if !got.agentChoices[1].IsNew {
		t.Fatalf("agentChoices[1] = %#v, want add new agent choice", got.agentChoices[1])
	}
}

func TestUpdateAOnChildRowRequiresFolderSelection(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", &trackingSessionManager{})
	m.rebuildRows()
	m.setSelected(1) // [0]=folder, [1]=command

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := model.(Model)
	if cmd != nil {
		t.Fatal("expected no command when add-agent is used on a child row")
	}
	if got.errMsg != "select a folder" {
		t.Fatalf("errMsg = %q, want %q", got.errMsg, "select a folder")
	}
}

func TestConfirmAgentPickerPersistsAndLaunchesManagedAgent(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.toml")
	cfg := config.Config{
		Agents: []config.Agent{{Name: "Codex", Command: "codex"}},
		Folders: []config.Folder{{
			Name:      "API",
			Path:      "/tmp/api",
			Namespace: "api",
		}},
	}
	if err := configfile.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fake := &trackingSessionManager{}
	m := NewModel(cfg, cfgPath, fake)
	m.rebuildRows()
	m.overlayMode = overlayAgentPicker
	m.overlayFolderIndex = 0
	m.agentChoices = []agentChoice{{
		Label:   "Codex",
		Agent:   config.Agent{Name: "Codex", Command: "codex"},
		Persist: true,
	}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := model.(Model)
	if cmd == nil {
		t.Fatal("expected agent create command")
	}
	if got.overlayMode != overlayNone {
		t.Fatalf("overlayMode = %v, want overlayNone", got.overlayMode)
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.attachTarget != "api/agent-codex-1" {
		t.Fatalf("agent result = %#v, want attachTarget api/agent-codex-1", msg)
	}
	if len(fake.launched) != 1 || fake.launched[0] != "api/agent-codex-1" {
		t.Fatalf("launched sessions = %#v, want [api/agent-codex-1]", fake.launched)
	}
	if len(fake.commands) != 1 || fake.commands[0] != "codex" {
		t.Fatalf("launch commands = %#v, want [codex]", fake.commands)
	}

	reloaded, err := configfile.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded.Folders[0].Agents) != 1 || reloaded.Folders[0].Agents[0].Name != "Codex" {
		t.Fatalf("reloaded folder agents = %#v, want persisted Codex template", reloaded.Folders[0].Agents)
	}
}

func TestAddNewAgentPromptPreservesSelectedFolder(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.toml")
	cfg := config.Config{Folders: []config.Folder{
		{Name: "API", Path: "/tmp/api", Namespace: "api"},
		{Name: "Web", Path: "/tmp/web", Namespace: "web"},
	}}
	if err := configfile.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fake := &trackingSessionManager{}
	m := NewModel(cfg, cfgPath, fake)
	m.rebuildRows()
	m.setSelected(1)

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	withOverlay := model.(Model)
	if withOverlay.overlayMode != overlayAgentPicker {
		t.Fatalf("overlayMode = %v, want overlayAgentPicker", withOverlay.overlayMode)
	}

	model, cmd := withOverlay.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withNamePrompt := model.(Model)
	if cmd == nil {
		t.Fatal("entering add-new agent should return a prompt blink command")
	}
	if withNamePrompt.promptMode != promptAddAgentName {
		t.Fatalf("promptMode = %v, want promptAddAgentName", withNamePrompt.promptMode)
	}

	withNamePrompt.prompt.SetValue("Codex")
	model, cmd = withNamePrompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withCommandPrompt := model.(Model)
	if cmd == nil {
		t.Fatal("submitting agent name should return a prompt blink command")
	}
	if withCommandPrompt.promptMode != promptAddAgentCommand {
		t.Fatalf("promptMode = %v, want promptAddAgentCommand", withCommandPrompt.promptMode)
	}

	withCommandPrompt.prompt.SetValue("codex")
	model, cmd = withCommandPrompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected agent launch command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.attachTarget != "web/agent-codex-1" {
		t.Fatalf("agent result = %#v, want attachTarget web/agent-codex-1", msg)
	}
	if len(fake.launched) != 1 || fake.launched[0] != "web/agent-codex-1" {
		t.Fatalf("launched sessions = %#v, want [web/agent-codex-1]", fake.launched)
	}

	reloaded, err := configfile.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded.Folders[1].Agents) != 1 || reloaded.Folders[1].Agents[0].Name != "Codex" {
		t.Fatalf("reloaded web folder agents = %#v, want persisted Codex template", reloaded.Folders[1].Agents)
	}
}

func TestAddNewAgentPromptValidationStaysOpen(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", &trackingSessionManager{})
	m.rebuildRows()
	m.overlayMode = overlayAgentPicker
	m.overlayFolderIndex = 0
	m.agentChoices = []agentChoice{{Label: "Add new agent...", IsNew: true}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withNamePrompt := model.(Model)
	if cmd == nil {
		t.Fatal("expected prompt blink command when entering add-new agent flow")
	}
	if withNamePrompt.promptMode != promptAddAgentName {
		t.Fatalf("promptMode = %v, want promptAddAgentName", withNamePrompt.promptMode)
	}

	withNamePrompt.prompt.SetValue("")
	model, cmd = withNamePrompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterInvalid := model.(Model)
	if cmd != nil {
		t.Fatalf("invalid add-agent name should not return a command, got %v", cmd)
	}
	if afterInvalid.promptMode != promptAddAgentName {
		t.Fatalf("promptMode = %v, want promptAddAgentName to stay open", afterInvalid.promptMode)
	}
	if afterInvalid.errMsg != "agent name is required" {
		t.Fatalf("errMsg = %q, want agent name is required", afterInvalid.errMsg)
	}
}

func TestUpdateSStartsStoppedCommandWithoutAttaching(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", fake)
	m.rebuildRows()
	m.setSelected(1)

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected start command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.attachTarget != "" {
		t.Fatalf("start result = %#v, want background start", msg)
	}
	if len(fake.launched) != 1 || fake.launched[0] != "api/cmd-start" {
		t.Fatalf("launched sessions = %#v, want [api/cmd-start]", fake.launched)
	}
	if len(fake.commands) != 1 || fake.commands[0] != "make start" {
		t.Fatalf("launch commands = %#v, want [make start]", fake.commands)
	}
	if len(fake.attached) != 0 {
		t.Fatalf("attached sessions = %#v, want none", fake.attached)
	}
}

func TestUpdateXStopsRunningCommand(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", fake)
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", commandText: "make start", status: "running"}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected stop command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.err != nil {
		t.Fatalf("stop result = %#v, want successful actionResultMsg", msg)
	}
	if len(fake.killed) != 1 || fake.killed[0] != "api/cmd-start" {
		t.Fatalf("killed sessions = %#v, want [api/cmd-start]", fake.killed)
	}
}

func TestUpdateKDoesNotKillRunningCommand(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", fake)
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", commandText: "make start", status: "running"}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	got := model.(Model)
	if cmd != nil {
		t.Fatal("expected no kill command for running command row")
	}
	if got.errMsg != "select an agent or terminal to kill" {
		t.Fatalf("errMsg = %q, want %q", got.errMsg, "select an agent or terminal to kill")
	}
	if len(fake.killed) != 0 {
		t.Fatalf("killed sessions = %#v, want none", fake.killed)
	}
}

func TestUpdateRRestartsCommandSession(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", fake)
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", commandText: "make start", status: "running"}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected restart command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.err != nil || res.attachTarget != "" {
		t.Fatalf("restart result = %#v, want successful background start", msg)
	}
	if len(fake.killed) != 1 || fake.killed[0] != "api/cmd-start" {
		t.Fatalf("killed sessions after restart = %#v, want [api/cmd-start]", fake.killed)
	}
	if len(fake.launched) != 1 || fake.launched[0] != "api/cmd-start" {
		t.Fatalf("launched sessions after restart = %#v, want [api/cmd-start]", fake.launched)
	}
	if len(fake.commands) != 1 || fake.commands[0] != "make start" {
		t.Fatalf("launch commands after restart = %#v, want [make start]", fake.commands)
	}
}

func TestUpdateEnterOnStoppedCommandDoesNothing(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", &trackingSessionManager{})
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", commandText: "make start", status: "stopped"}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := model.(Model)
	if cmd != nil {
		t.Fatal("expected no attach command for stopped command row")
	}
	if got.errMsg != "select a running session" {
		t.Fatalf("errMsg = %q, want %q", got.errMsg, "select a running session")
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

func TestPaneCapturedMsgUpdatesAutoPreviewOutsideExplicitMode(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", &trackingSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{
			Name:           "api/one",
			CurrentCommand: "bash",
		}},
	}
	m.rebuildRows()
	m.setSelected(1)
	m.previewSession = "api/one"
	m.previewSeq = 3

	updated, _ := m.Update(paneCapturedMsg{target: "api/one", content: "preview", seq: 3})
	got := updated.(Model)
	if got.previewContent != "preview" {
		t.Fatalf("previewContent = %q, want preview", got.previewContent)
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
