package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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
	sentTo   []string
	sentCmds []string
}

func (f *trackingSessionManager) LoadSnapshot() (tmux.SessionSnapshot, error) {
	return tmux.SessionSnapshot{
		Sessions:       []tmux.Session{},
		SessionWindows: map[string][]int{},
		ActiveWindows:  map[string]int{},
		PaneDataFresh:  true,
	}, nil
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

func (f *trackingSessionManager) SendKeys(target, command string) error {
	f.sentTo = append(f.sentTo, target)
	f.sentCmds = append(f.sentCmds, command)
	return nil
}

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

func TestUpdateTickContinuesWhileFilterPromptOpen(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{}, "config.toml", &trackingSessionManager{})
	m.openPrompt(promptFilter, "", "filter folders and sessions")

	model, cmd := m.Update(time.Now())
	got := model.(Model)

	if got.promptMode != promptFilter {
		t.Fatalf("promptMode = %v, want promptFilter", got.promptMode)
	}
	if cmd == nil {
		t.Fatal("expected refresh command while filter prompt is open")
	}

	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want tea.BatchMsg", cmd())
	}
	if len(batch) != 2 {
		t.Fatalf("len(batch) = %d, want 2 refresh commands", len(batch))
	}

	msgCh := make(chan tea.Msg, len(batch))
	for _, subcmd := range batch {
		go func(c tea.Cmd) {
			msgCh <- c()
		}(subcmd)
	}

	select {
	case msg := <-msgCh:
		if _, ok := msg.(sessionsLoadedMsg); !ok {
			t.Fatalf("first completed batch message = %T, want sessionsLoadedMsg", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for sessions refresh command")
	}
}

func TestTickCmdUsesShortGlobalRefreshInterval(t *testing.T) {
	t.Parallel()

	if refreshInterval != 500*time.Millisecond {
		t.Fatalf("refreshInterval = %v, want %v", refreshInterval, 500*time.Millisecond)
	}

	cmd := tickCmd()
	if cmd == nil {
		t.Fatal("expected tick command")
	}

	start := time.Now()
	msgCh := make(chan tea.Msg, 1)
	go func() {
		msgCh <- cmd()
	}()

	select {
	case msg := <-msgCh:
		if _, ok := msg.(time.Time); !ok {
			t.Fatalf("tick message = %T, want time.Time", msg)
		}
		if elapsed := time.Since(start); elapsed > 1200*time.Millisecond {
			t.Fatalf("tick elapsed = %v, want <= %v", elapsed, 1200*time.Millisecond)
		}
	case <-time.After(1200 * time.Millisecond):
		t.Fatal("timed out waiting for tick within 1200ms")
	}
}

func TestRunCommandPromptDoesNotDriftToReplacementSession(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)
	m.sessions = map[int][]tmux.Session{
		0: {{Name: "api/one"}},
	}
	m.rebuildRows()
	m.setSelected(1)

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	withPrompt := model.(Model)
	if withPrompt.promptMode != promptRunCommand {
		t.Fatalf("promptMode = %v, want promptRunCommand", withPrompt.promptMode)
	}

	reloadedModel, _ := withPrompt.Update(sessionsLoadedMsg{
		sessions: map[int][]tmux.Session{
			0: {{Name: "api/two"}},
		},
		panesFresh: true,
	})
	reloaded := reloadedModel.(Model)
	reloaded.prompt.SetValue("pwd")

	finalModel, cmd := reloaded.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := finalModel.(Model)

	if len(fake.sentTo) != 0 {
		t.Fatalf("sentTo = %#v, want no command sent to replacement session", fake.sentTo)
	}
	if cmd != nil {
		t.Fatal("expected no send command when original prompt target is gone")
	}
	if final.promptMode != promptRunCommand {
		t.Fatalf("promptMode = %v, want promptRunCommand to remain open", final.promptMode)
	}
	if final.errMsg == "" {
		t.Fatal("expected error when original prompt target no longer exists")
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

func TestUpdateAOnChildRowUsesFolderContext(t *testing.T) {
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
		t.Fatal("expected no command when opening the agent picker on a child row")
	}
	if got.overlayMode != overlayAgentPicker {
		t.Fatalf("overlayMode = %v, want overlayAgentPicker", got.overlayMode)
	}
	if got.overlayFolderIndex != 0 {
		t.Fatalf("overlayFolderIndex = %d, want 0", got.overlayFolderIndex)
	}
	if len(got.agentChoices) != 1 || !got.agentChoices[0].IsNew {
		t.Fatalf("agentChoices = %#v, want only add-new choice", got.agentChoices)
	}
	if got.errMsg != "" {
		t.Fatalf("errMsg = %q, want empty", got.errMsg)
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

func TestUpdateDOnChildRowUsesFolderContext(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", &trackingSessionManager{})
	m.rebuildRows()
	m.setSelected(1) // [0]=folder, [1]=command

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := model.(Model)
	if cmd == nil {
		t.Fatal("expected prompt blink command when add-command is used on a child row")
	}
	if got.promptMode != promptAddCommandName {
		t.Fatalf("promptMode = %v, want promptAddCommandName", got.promptMode)
	}
	if got.promptFolderIndex != 0 {
		t.Fatalf("promptFolderIndex = %d, want 0", got.promptFolderIndex)
	}
	if got.errMsg != "" {
		t.Fatalf("errMsg = %q, want empty", got.errMsg)
	}
}

func TestAddDevCommandPromptPersistsAndSelectsNewCommand(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.toml")
	cfg := config.Config{Folders: []config.Folder{
		{Name: "API", Path: "/tmp/api", Namespace: "api"},
		{
			Name:      "Web",
			Path:      "/tmp/web",
			Namespace: "web",
			Commands:  []config.Command{{Name: "start", Command: "make start"}},
		},
	}}
	if err := configfile.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	m := NewModel(cfg, cfgPath, &trackingSessionManager{})
	m.rebuildRows()
	m.setSelected(2) // [0]=API folder, [1]=Web folder, [2]=start command

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	withNamePrompt := model.(Model)
	if cmd == nil {
		t.Fatal("expected prompt blink command when entering add-command flow")
	}
	if withNamePrompt.promptMode != promptAddCommandName {
		t.Fatalf("promptMode = %v, want promptAddCommandName", withNamePrompt.promptMode)
	}

	withNamePrompt.prompt.SetValue("Dev")
	model, cmd = withNamePrompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	withCommandPrompt := model.(Model)
	if cmd == nil {
		t.Fatal("submitting command name should return a prompt blink command")
	}
	if withCommandPrompt.promptMode != promptAddCommandCommand {
		t.Fatalf("promptMode = %v, want promptAddCommandCommand", withCommandPrompt.promptMode)
	}

	withCommandPrompt.prompt.SetValue("make dev")
	model, cmd = withCommandPrompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterSubmit := model.(Model)
	if cmd == nil {
		t.Fatal("expected command persistence command")
	}
	if afterSubmit.promptMode != promptNone {
		t.Fatalf("promptMode = %v, want promptNone", afterSubmit.promptMode)
	}

	msg := cmd()
	persistedModel, persistedCmd := afterSubmit.Update(msg)
	got := persistedModel.(Model)
	if persistedCmd == nil {
		t.Fatal("expected follow-up refresh/status command after command persistence")
	}
	if len(got.cfg.Folders[1].Commands) != 2 || got.cfg.Folders[1].Commands[1].Name != "Dev" {
		t.Fatalf("folder commands = %#v, want appended Dev command", got.cfg.Folders[1].Commands)
	}
	selected, ok := got.selectedRow()
	if !ok {
		t.Fatal("expected selected row after command persistence")
	}
	if selected.typeOf != rowCommand || selected.sessionName != "web/cmd-dev" {
		t.Fatalf("selected row = %#v, want web/cmd-dev command row", selected)
	}

	reloaded, err := configfile.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(reloaded.Folders[1].Commands) != 2 || reloaded.Folders[1].Commands[1].Command != "make dev" {
		t.Fatalf("reloaded web folder commands = %#v, want persisted Dev command", reloaded.Folders[1].Commands)
	}
}

func TestUpdateAddFolderPromptPersistsFolderAndClosesPrompt(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	folderPath := filepath.Join(tempDir, "workspace")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	cfgPath := filepath.Join(tempDir, "config.toml")
	if err := configfile.Save(cfgPath, config.Config{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	m := NewModel(config.Config{}, cfgPath, &trackingSessionManager{})
	m.rebuildRows()

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	step0 := model.(Model)
	if cmd == nil || step0.promptMode != promptAddFolder {
		t.Fatalf("prompt start = %#v, cmd=%v; want promptAddFolder with blink", step0.promptMode, cmd)
	}

	step0.prompt.SetValue("API")
	model, cmd = step0.Update(tea.KeyMsg{Type: tea.KeyEnter})
	step1 := model.(Model)
	if cmd == nil || step1.promptStep != 1 {
		t.Fatalf("after folder name: promptStep=%d, cmd=%v; want 1 with blink", step1.promptStep, cmd)
	}

	step1.prompt.SetValue(folderPath)
	model, cmd = step1.Update(tea.KeyMsg{Type: tea.KeyEnter})
	step2 := model.(Model)
	if cmd == nil || step2.promptStep != 2 {
		t.Fatalf("after folder path: promptStep=%d, cmd=%v; want 2 with blink", step2.promptStep, cmd)
	}

	step2.prompt.SetValue("zed .")
	model, cmd = step2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	afterSubmit := model.(Model)
	if cmd == nil {
		t.Fatal("expected folder persistence command")
	}
	if afterSubmit.promptMode != promptNone {
		t.Fatalf("promptMode = %v, want promptNone", afterSubmit.promptMode)
	}

	msg := cmd()
	persisted, _ := afterSubmit.Update(msg)
	got := persisted.(Model)
	if len(got.cfg.Folders) != 1 || got.cfg.Folders[0].Namespace != "api" {
		t.Fatalf("folders = %#v, want one persisted API folder", got.cfg.Folders)
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

func TestUpdateCDoesNotOpenPromptForCommandRow(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", &trackingSessionManager{})
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", commandText: "make start", status: "running"}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := model.(Model)
	if cmd != nil {
		t.Fatal("expected no prompt command for command row")
	}
	if got.errMsg != "select an agent or terminal to run command" {
		t.Fatalf("errMsg = %q, want %q", got.errMsg, "select an agent or terminal to run command")
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
