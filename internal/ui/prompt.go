package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/config"
)

func (m *Model) openPrompt(mode promptMode, initial, placeholder string) {
	if mode != promptRunCommand {
		m.promptTarget = ""
	}
	m.promptMode = mode
	m.prompt.SetValue(initial)
	m.prompt.Placeholder = placeholder
	m.prompt.Focus()
	m.errMsg = ""
	if mode == promptRunCommand {
		m.statusMsg = "run command in selected session"
	} else {
		m.statusMsg = ""
	}
}

func (m Model) updatePrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.prompt.Blur()
			m.promptMode = promptNone
			m.promptTarget = ""
			m.promptFolderIndex = -1
			m.pendingAgent = config.Agent{}
			m.pendingCommand = config.Command{}
			m.statusMsg = ""
			return m, nil
		case "tab":
			if m.promptMode == promptAddFolder && m.promptStep == 1 {
				m.completePathInput()
				return m, nil
			}
		case "enter":
			value := strings.TrimSpace(m.prompt.Value())
			mode := m.promptMode
			closePrompt := func() {
				m.prompt.Blur()
				m.promptMode = promptNone
				m.promptTarget = ""
				m.promptFolderIndex = -1
				m.pendingAgent = config.Agent{}
				m.pendingCommand = config.Command{}
			}

			switch mode {
			case promptNewSession:
				folder, ok := m.selectedFolder()
				if !ok {
					m.errMsg = "select a folder"
					return m, nil
				}
				if value == "" {
					m.errMsg = "session name is required"
					return m, nil
				}
				closePrompt()
				return m, m.newSessionCmd(folder, value)
			case promptRenameSession:
				row, ok := m.selectedSessionRow()
				if !ok {
					m.errMsg = "select a session"
					return m, nil
				}
				if value == "" {
					m.errMsg = "new session name is required"
					return m, nil
				}
				folder := m.cfg.Folders[row.folderIndex]
				closePrompt()
				return m, m.renameSessionCmd(row.sessionName, folder.Namespace+"/"+sanitizeLeaf(value))
			case promptRunCommand:
				if m.promptTarget == "" {
					m.errMsg = "select a session"
					return m, nil
				}
				if value == "" {
					m.errMsg = "command cannot be empty"
					return m, nil
				}
				if !m.sessionExists(m.promptTarget) {
					m.errMsg = "selected session is no longer running"
					return m, nil
				}
				target := m.promptTarget
				closePrompt()
				return m, m.sendCommandCmd(target, value)
			case promptAddFolder:
				switch m.promptStep {
				case 0:
					folder, err := config.PrepareFolderName(value, m.cfg.Folders)
					if err != nil {
						m.errMsg = err.Error()
						return m, nil
					}
					m.pendingFolder.Name = folder.Name
					m.pendingFolder.Namespace = folder.Namespace
					m.promptStep = 1
					m.promptMode = promptAddFolder
					m.openPrompt(promptAddFolder, "", "folder path")
					return m, textinput.Blink
				case 1:
					path, err := config.PrepareFolderPath(value)
					if err != nil {
						m.errMsg = err.Error()
						return m, nil
					}
					m.pendingFolder.Path = path
					m.promptStep = 2
					m.promptMode = promptAddFolder
					m.openPrompt(promptAddFolder, "", "editor command (optional, e.g. code .)")
					return m, textinput.Blink
				case 2:
					m.pendingFolder.EditorCommand = value
					closePrompt()
					return m, m.addFolderCmd(m.pendingFolder)
				}
			case promptAddAgentName:
				if value == "" {
					m.errMsg = "agent name is required"
					return m, nil
				}
				m.pendingAgent.Name = value
				m.openPrompt(promptAddAgentCommand, "", "agent launch command")
				return m, textinput.Blink
			case promptAddAgentCommand:
				if value == "" {
					m.errMsg = "agent command is required"
					return m, nil
				}
				folderIndex := m.promptFolderIndex
				if folderIndex < 0 || folderIndex >= len(m.cfg.Folders) {
					m.errMsg = "select a folder"
					return m, nil
				}
				agent, err := config.PrepareAgent(config.Agent{Name: m.pendingAgent.Name, Command: value})
				if err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				if err := config.AppendFolderAgent(&m.cfg, folderIndex, agent); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				folder := m.cfg.Folders[folderIndex]
				closePrompt()
				return m, m.newAgentCmd(folderIndex, folder, agent, true)
			case promptAddCommandName:
				if value == "" {
					m.errMsg = "command name is required"
					return m, nil
				}
				folderIndex := m.promptFolderIndex
				if folderIndex < 0 || folderIndex >= len(m.cfg.Folders) {
					m.errMsg = "select a folder"
					return m, nil
				}
				if config.CommandNameExists(m.cfg.Folders[folderIndex], value) {
					m.errMsg = "command name already exists"
					return m, nil
				}
				m.pendingCommand.Name = value
				m.openPrompt(promptAddCommandCommand, "", "dev command to run")
				return m, textinput.Blink
			case promptAddCommandCommand:
				if value == "" {
					m.errMsg = "command is required"
					return m, nil
				}
				folderIndex := m.promptFolderIndex
				if folderIndex < 0 || folderIndex >= len(m.cfg.Folders) {
					m.errMsg = "select a folder"
					return m, nil
				}
				command, err := config.PrepareCommand(config.Command{Name: m.pendingCommand.Name, Command: value})
				if err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				closePrompt()
				return m, m.addCommandCmd(folderIndex, command)
			case promptFilter:
				closePrompt()
				m.filterQuery = value
				m.rebuildRows()
				var clearCmd tea.Cmd
				if value == "" {
					clearCmd = m.setStatus("filter cleared")
				} else {
					clearCmd = m.setStatus("filter set: " + value)
				}
				return m, tea.Batch(clearCmd, m.syncSelectionPreview(true, true))
			}
		}
	}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m *Model) completePathInput() {
	raw := m.prompt.Value()
	if raw == "" {
		return
	}
	expanded := config.ExpandHome(raw)
	expanded = os.ExpandEnv(expanded)

	dir, prefix := filepath.Split(expanded)
	if dir == "" {
		dir = "."
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var matches []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && e.IsDir() {
			matches = append(matches, e.Name())
		}
	}
	if len(matches) == 0 {
		return
	}

	// Find longest common prefix among matches
	common := matches[0]
	for _, m := range matches[1:] {
		for i := range common {
			if i >= len(m) || common[i] != m[i] {
				common = common[:i]
				break
			}
		}
	}

	completed := filepath.Join(dir, common)
	if len(matches) == 1 {
		completed += string(filepath.Separator)
	}

	// Convert back to use ~/  if the original input used it
	if strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && strings.HasPrefix(completed, home) {
			completed = "~" + completed[len(home):]
		}
	}

	m.prompt.SetValue(completed)
	m.prompt.SetCursor(len(completed))
}

func (m Model) promptTitle() string {
	switch m.promptMode {
	case promptNewSession:
		return "new session:"
	case promptRenameSession:
		return "rename:"
	case promptRunCommand:
		return "command:"
	case promptFilter:
		return "filter:"
	case promptAddFolder:
		return fmt.Sprintf("add folder (%d/3):", m.promptStep+1)
	case promptAddAgentName:
		return "agent name:"
	case promptAddAgentCommand:
		return "agent command:"
	case promptAddCommandName:
		return "dev command name:"
	case promptAddCommandCommand:
		return "dev command:"
	default:
		return ""
	}
}
