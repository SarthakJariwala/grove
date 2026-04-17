package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/config"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle background/system messages before routing input to modal states so
	// periodic refreshes and async results continue to flow while prompts are open.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case sessionsLoadedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.sessions = msg.sessions
		if msg.panesFresh {
			m.sessionWindows = msg.sessionWindows
			m.activeWindows = msg.activeWindows
		}
		m.rebuildRows()
		m.errMsg = ""
		if m.detailMode == detailPreview {
			return m, m.reconcilePreviewAfterLoad()
		}
		return m, m.syncSelectionPreview(true, false)

	case actionResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, m.loadSessionsCmd()
		}
		clearCmd := m.setStatus(msg.status)
		if msg.attachTarget != "" {
			return m, tea.Batch(
				clearCmd,
				m.loadSessionsCmd(),
				tea.ExecProcess(m.client.AttachCommand(msg.attachTarget), func(err error) tea.Msg {
					return attachedMsg{err: err}
				}),
			)
		}
		return m, tea.Batch(clearCmd, m.loadSessionsCmd())

	case attachedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, m.loadSessionsCmd()
		}
		clearCmd := m.setStatus("detached from session")
		return m, tea.Batch(clearCmd, m.loadSessionsCmd())

	case paneCapturedMsg:
		if msg.seq != m.previewSeq || msg.target != m.previewCaptureTarget() {
			return m, nil
		}
		m.previewInFlight = false
		m.previewLoading = false
		if msg.err != nil {
			m.previewErr = msg.err
			m.previewContent = ""
		} else {
			m.previewErr = nil
			m.previewContent = msg.content
		}
		return m, nil

	case previewTickMsg:
		if m.detailMode != detailPreview {
			return m, nil
		}
		if m.previewInFlight {
			return m, previewTickCmd()
		}
		return m, tea.Batch(m.beginPreviewCapture(false), previewTickCmd())

	case folderAddedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.cfg.Folders = append(m.cfg.Folders, msg.folder)
		m.rebuildRows()
		clearCmd := m.setStatus("added folder: " + msg.folder.Name)
		return m, tea.Batch(clearCmd, m.loadSessionsCmd())

	case commandAddedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		if msg.folderIndex < 0 || msg.folderIndex >= len(m.cfg.Folders) {
			m.errMsg = "select a folder"
			return m, nil
		}
		m.cfg.Folders[msg.folderIndex].Commands = append(m.cfg.Folders[msg.folderIndex].Commands, msg.command)
		folder := m.cfg.Folders[msg.folderIndex]
		m.rebuildRows()
		target := treeRow{
			typeOf:      rowCommand,
			folderIndex: msg.folderIndex,
			sessionName: commandSessionName(folder, msg.command.Name),
		}
		if index, ok := findMatchingRowIndex(m.rows, target); ok {
			m.setSelected(index)
		}
		return m, m.setStatus("added dev command: " + msg.command.Name)

	case clearStatusMsg:
		if msg.seq == m.statusSeq {
			m.statusMsg = ""
		}
		return m, nil

	case time.Time:
		return m, tea.Batch(tickCmd(), m.loadSessionsCmd())
	}

	if m.promptMode != promptNone {
		return m.updatePrompt(msg)
	}
	if m.overlayMode != overlayNone {
		return m.updateOverlay(msg)
	}
	if m.confirmKillTarget != "" {
		return m.updateKillConfirm(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.detailMode == detailPreview {
			return m.updatePreview(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.filterQuery != "" {
				m.filterQuery = ""
				m.rebuildRows()
				clearCmd := m.setStatus("filter cleared")
				return m, tea.Batch(clearCmd, m.syncSelectionPreview(true, true))
			}
			return m, nil
		case "up":
			if m.setSelected(m.selected - 1) {
				return m, m.syncSelectionPreview(true, true)
			}
			return m, nil
		case "down":
			if m.setSelected(m.selected + 1) {
				return m, m.syncSelectionPreview(true, true)
			}
			return m, nil
		case "r":
			return m, m.loadSessionsCmd()
		case "/":
			m.openPrompt(promptFilter, m.filterQuery, "filter folders and sessions")
			return m, textinput.Blink
		case "pgdown", "ctrl+f":
			m.detailScroll += m.contentHeight() / 2
			return m, nil
		case "pgup", "ctrl+b":
			m.detailScroll -= m.contentHeight() / 2
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
			return m, nil
		case "n":
			folder, ok := m.selectedFolder()
			if !ok {
				m.errMsg = "select a folder or one of its sections"
				return m, nil
			}
			return m, m.newTerminalCmd(m.rows[m.selected].folderIndex, folder)
		case "a":
			folder, ok := m.selectedFolder()
			if !ok {
				m.errMsg = "select a folder or one of its sections"
				return m, nil
			}
			m.openAgentPicker(folder)
			return m, nil
		case "d":
			if _, ok := m.selectedFolder(); !ok {
				m.errMsg = "select a folder or one of its sections"
				return m, nil
			}
			m.promptFolderIndex = m.rows[m.selected].folderIndex
			m.pendingCommand = config.Command{}
			m.openPrompt(promptAddCommandName, "", "dev command name")
			return m, textinput.Blink
		case "s":
			row, ok := m.selectedCommandRow()
			if !ok || row.status == "running" {
				return m, nil
			}
			folder := m.cfg.Folders[row.folderIndex]
			return m, m.startCommandCmd(folder, row)
		case "x":
			row, ok := m.selectedCommandRow()
			if !ok || row.status != "running" {
				return m, nil
			}
			return m, m.killSessionCmd(row.sessionName)
		case "R":
			row, ok := m.selectedCommandRow()
			if !ok {
				return m, nil
			}
			folder := m.cfg.Folders[row.folderIndex]
			if row.status == "running" {
				return m, m.restartCommandCmd(folder, row)
			}
			return m, m.startCommandCmd(folder, row)
		case "c":
			row, ok := m.selectedRow()
			if !ok || (row.typeOf != rowAgentInstance && row.typeOf != rowTerminalInstance) {
				m.errMsg = "select an agent or terminal to run command"
				return m, nil
			}
			m.promptTarget = row.sessionName
			m.openPrompt(promptRunCommand, "", "command to run")
			return m, textinput.Blink
		case "K":
			row, ok := m.selectedKillableSessionRow()
			if !ok {
				m.errMsg = "select an agent or terminal to kill"
				return m, nil
			}
			m.confirmKillTarget = row.sessionName
			m.statusMsg = ""
			m.errMsg = ""
			return m, nil
		case "A":
			m.promptStep = 0
			m.pendingFolder = config.Folder{}
			m.openPrompt(promptAddFolder, "", "folder name")
			return m, textinput.Blink
		case "v":
			_, ok := m.selectedSessionRow()
			if !ok {
				m.errMsg = "select a session to preview"
				return m, nil
			}
			m.detailMode = detailPreview
			return m, tea.Batch(m.startPreview(), previewTickCmd())
		case "e":
			folder, ok := m.selectedFolder()
			if !ok {
				m.errMsg = "select a folder or session"
				return m, nil
			}
			cmd := m.resolveEditorCommand(folder)
			if cmd == "" {
				m.errMsg = "no editor configured; set editor_command in config or $EDITOR"
				return m, nil
			}
			dir := folder.Path
			if row, ok := m.selectedSessionRow(); ok && row.currentPath != "" {
				dir = row.currentPath
			}
			return m, m.openEditorInDir(cmd, dir)
		case "enter":
			row, ok := m.selectedSessionRow()
			if !ok {
				m.errMsg = "select a running session"
				return m, nil
			}
			m.statusMsg = "attached to " + row.sessionName + " (detach with Ctrl-b d)"
			m.errMsg = ""
			return m, tea.ExecProcess(m.client.AttachCommand(row.sessionName), func(err error) tea.Msg {
				return attachedMsg{err: err}
			})
		}
	}

	return m, nil
}

// ── View ────────────────────────────────────────────────────────────

func (m *Model) openAgentPicker(folder config.Folder) {
	m.overlayMode = overlayAgentPicker
	m.overlayFolderIndex = m.rows[m.selected].folderIndex
	m.overlayIndex = 0
	m.agentChoices = buildAgentChoices(m.cfg, folder)
	m.errMsg = ""
	m.statusMsg = ""
}

func buildAgentChoices(cfg config.Config, folder config.Folder) []agentChoice {
	choices := make([]agentChoice, 0, len(folder.Agents)+len(cfg.Agents)+1)
	seen := map[string]struct{}{}
	for _, agent := range folder.Agents {
		key := sanitizeLeaf(agent.Name)
		seen[key] = struct{}{}
		choices = append(choices, agentChoice{Label: agent.Name, Agent: agent})
	}
	for _, agent := range cfg.Agents {
		key := sanitizeLeaf(agent.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		choices = append(choices, agentChoice{Label: agent.Name, Agent: agent, Persist: true})
	}
	choices = append(choices, agentChoice{Label: "Add new agent...", IsNew: true})
	return choices
}

func (m Model) updateOverlay(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.overlayMode != overlayAgentPicker || len(m.agentChoices) == 0 {
		m.overlayMode = overlayNone
		m.agentChoices = nil
		return m, nil
	}

	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		m.closeOverlay()
		return m, nil
	case "up":
		if m.overlayIndex > 0 {
			m.overlayIndex--
		}
		return m, nil
	case "down":
		if m.overlayIndex < len(m.agentChoices)-1 {
			m.overlayIndex++
		}
		return m, nil
	case "enter":
		choice := m.agentChoices[m.overlayIndex]
		folderIndex := m.overlayFolderIndex
		if folderIndex < 0 || folderIndex >= len(m.cfg.Folders) {
			m.closeOverlay()
			m.errMsg = "select a folder"
			return m, nil
		}
		if choice.IsNew {
			m.closeOverlay()
			m.pendingAgent = config.Agent{}
			m.promptFolderIndex = folderIndex
			m.openPrompt(promptAddAgentName, "", "agent name")
			return m, textinput.Blink
		}
		if choice.Persist {
			if err := config.AppendFolderAgent(&m.cfg, folderIndex, choice.Agent); err != nil {
				m.closeOverlay()
				m.errMsg = err.Error()
				return m, nil
			}
		}
		folder := m.cfg.Folders[folderIndex]
		m.closeOverlay()
		return m, m.newAgentCmd(folderIndex, folder, choice.Agent, choice.Persist)
	}

	return m, nil
}

func (m *Model) closeOverlay() {
	m.overlayMode = overlayNone
	m.overlayIndex = 0
	m.agentChoices = nil
}

func (m Model) updateKillConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch strings.ToLower(key.String()) {
	case "y", "enter":
		target := m.confirmKillTarget
		m.confirmKillTarget = ""
		return m, m.killSessionCmd(target)
	case "n", "esc":
		m.confirmKillTarget = ""
		clearCmd := m.setStatus("kill cancelled")
		return m, clearCmd
	default:
		return m, nil
	}
}
