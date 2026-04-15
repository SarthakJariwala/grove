package ui

import (
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/configfile"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

func (m Model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if len(m.cfg.Folders) == 0 {
			return sessionsLoadedMsg{
				sessions:       map[int][]tmux.Session{},
				sessionWindows: map[string][]int{},
				activeWindows:  map[string]int{},
				panesFresh:     true,
			}
		}

		snapshot, err := m.client.LoadSnapshot()
		if err != nil {
			return sessionsLoadedMsg{err: err}
		}

		grouped := map[int][]tmux.Session{}
		for _, session := range snapshot.Sessions {
			for idx, folder := range m.cfg.Folders {
				prefix := folder.Namespace + "/"
				if strings.HasPrefix(session.Name, prefix) {
					grouped[idx] = append(grouped[idx], session)
					break
				}
			}
		}

		return sessionsLoadedMsg{
			sessions:       grouped,
			sessionWindows: snapshot.SessionWindows,
			activeWindows:  snapshot.ActiveWindows,
			panesFresh:     snapshot.PaneDataFresh,
		}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return t
	})
}

func previewTickCmd() tea.Cmd {
	return tea.Tick(previewRefreshInterval, func(time.Time) tea.Msg {
		return previewTickMsg{}
	})
}

func (m Model) newSessionCmd(folder config.Folder, leaf string) tea.Cmd {
	leaf = sanitizeLeaf(leaf)
	fullName := folder.Namespace + "/" + leaf

	return func() tea.Msg {
		if err := m.client.NewSession(fullName, folder.Path); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "created " + fullName, attachTarget: fullName}
	}
}

func (m Model) newTerminalCmd(folderIndex int, folder config.Folder) tea.Cmd {
	index := nextTerminalIndex(folder, m.sessions[folderIndex])
	name := terminalSessionName(folder, index)
	return func() tea.Msg {
		if err := m.client.NewSession(name, folder.Path); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "created " + name, attachTarget: name}
	}
}

func (m Model) newAgentCmd(folderIndex int, folder config.Folder, agent config.Agent, persist bool) tea.Cmd {
	index := nextAgentIndex(folder, agent.Name, m.sessions[folderIndex])
	name := agentSessionName(folder, agent.Name, index)
	return func() tea.Msg {
		if persist {
			if err := configfile.Save(m.cfgPath, m.cfg); err != nil {
				return actionResultMsg{err: err}
			}
		}
		if err := m.client.NewSessionWithCommand(name, folder.Path, agent.Command); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "created " + name, attachTarget: name}
	}
}

func (m Model) startCommandCmd(folder config.Folder, row treeRow) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.NewSessionWithCommand(row.sessionName, folder.Path, row.commandText); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "started " + row.displayName}
	}
}

func (m Model) restartCommandCmd(folder config.Folder, row treeRow) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.KillSession(row.sessionName); err != nil {
			return actionResultMsg{err: err}
		}
		if err := m.client.NewSessionWithCommand(row.sessionName, folder.Path, row.commandText); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "restarted " + row.displayName}
	}
}

func (m Model) renameSessionCmd(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.RenameSession(oldName, newName); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "renamed to " + newName}
	}
}

func (m Model) killSessionCmd(name string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.KillSession(name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "killed " + name}
	}
}

func (m Model) addFolderCmd(f config.Folder) tea.Cmd {
	cfgPath := m.cfgPath
	return func() tea.Msg {
		if err := configfile.AppendFolder(cfgPath, f); err != nil {
			return folderAddedMsg{err: err}
		}
		return folderAddedMsg{folder: f}
	}
}

func (m Model) addCommandCmd(folderIndex int, command config.Command) tea.Cmd {
	cfg := m.cfg
	cfg.Folders = append([]config.Folder(nil), cfg.Folders...)
	cfgPath := m.cfgPath
	return func() tea.Msg {
		if err := config.AppendCommand(&cfg, folderIndex, command); err != nil {
			return commandAddedMsg{err: err}
		}
		if err := configfile.Save(cfgPath, cfg); err != nil {
			return commandAddedMsg{err: err}
		}
		return commandAddedMsg{folderIndex: folderIndex, command: command}
	}
}

func (m Model) sendCommandCmd(name, command string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.SendKeys(name, command); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "sent command to " + name}
	}
}

func (m Model) resolveEditorCommand(folder config.Folder) string {
	if folder.EditorCommand != "" {
		return folder.EditorCommand
	}
	if m.cfg.EditorCommand != "" {
		return m.cfg.EditorCommand
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	return ""
}

func (m Model) openEditorInDir(cmdStr string, dir string) tea.Cmd {
	c := exec.Command("sh", "-lc", cmdStr)
	c.Dir = dir
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return actionResultMsg{status: "editor closed", err: err}
	})
}
