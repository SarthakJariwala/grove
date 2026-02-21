package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grove/internal/config"
	"grove/internal/tmux"
)

const refreshInterval = 2 * time.Second

type rowType int

const (
	rowFolder rowType = iota
	rowSession
)

type treeRow struct {
	typeOf      rowType
	folderIndex int
	sessionName string
	leafName    string
	status      string
	windows     int
}

type promptMode int

const (
	promptNone promptMode = iota
	promptNewSession
	promptRenameSession
	promptRunCommand
	promptFilter
)

type Model struct {
	cfg    config.Config
	client *tmux.Client

	width  int
	height int

	rows      []treeRow
	selected  int
	sessions  map[int][]tmux.Session
	statusMsg string
	errMsg    string

	filterQuery       string
	confirmKillTarget string

	prompt     textinput.Model
	promptMode promptMode
}

type sessionsLoadedMsg struct {
	sessions map[int][]tmux.Session
	err      error
}

type actionResultMsg struct {
	status       string
	err          error
	attachTarget string
}

type attachedMsg struct {
	err error
}

func NewModel(cfg config.Config, client *tmux.Client) Model {
	t := textinput.New()
	t.CharLimit = 512
	t.Prompt = "> "

	m := Model{
		cfg:    cfg,
		client: client,

		sessions: map[int][]tmux.Session{},
		prompt:   t,
	}
	m.rebuildRows()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadSessionsCmd(), tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.promptMode != promptNone {
		return m.updatePrompt(msg)
	}
	if m.confirmKillTarget != "" {
		return m.updateKillConfirm(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "down", "j":
			if m.selected < len(m.rows)-1 {
				m.selected++
			}
			return m, nil
		case "r":
			return m, m.loadSessionsCmd()
		case "/":
			m.openPrompt(promptFilter, m.filterQuery, "filter folders and sessions")
			return m, textinput.Blink
		case "n":
			folder, ok := m.selectedFolder()
			if !ok {
				m.errMsg = "select a folder or one of its sessions"
				return m, nil
			}
			m.openPrompt(promptNewSession, m.defaultSessionLeaf(folder), "new session name")
			return m, textinput.Blink
		case "R":
			row, ok := m.selectedSessionRow()
			if !ok {
				m.errMsg = "select a session to rename"
				return m, nil
			}
			m.openPrompt(promptRenameSession, row.leafName, "rename session")
			return m, textinput.Blink
		case "c":
			_, ok := m.selectedSessionRow()
			if !ok {
				m.errMsg = "select a session to run command"
				return m, nil
			}
			m.openPrompt(promptRunCommand, "", "command to run")
			return m, textinput.Blink
		case "K":
			row, ok := m.selectedSessionRow()
			if !ok {
				m.errMsg = "select a session to kill"
				return m, nil
			}
			m.confirmKillTarget = row.sessionName
			m.statusMsg = ""
			m.errMsg = ""
			return m, nil
		case "enter":
			row, ok := m.selectedSessionRow()
			if !ok {
				return m, nil
			}
			m.statusMsg = "attached to " + row.sessionName + " (detach with Ctrl-b d)"
			m.errMsg = ""
			return m, tea.ExecProcess(m.client.AttachCommand(row.sessionName), func(err error) tea.Msg {
				return attachedMsg{err: err}
			})
		}

	case sessionsLoadedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.sessions = msg.sessions
		m.rebuildRows()
		m.errMsg = ""
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, m.loadSessionsCmd()
		}
		m.statusMsg = msg.status
		m.errMsg = ""
		if msg.attachTarget != "" {
			return m, tea.Batch(
				m.loadSessionsCmd(),
				tea.ExecProcess(m.client.AttachCommand(msg.attachTarget), func(err error) tea.Msg {
					return attachedMsg{err: err}
				}),
			)
		}
		return m, m.loadSessionsCmd()

	case attachedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, m.loadSessionsCmd()
		}
		m.statusMsg = "detached from session"
		m.errMsg = ""
		return m, m.loadSessionsCmd()

	case time.Time:
		return m, tea.Batch(tickCmd(), m.loadSessionsCmd())
	}

	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("grove - managed tmux sessions")
	help := "up/k down/j navigate | Enter attach | n new | R rename | K kill | c command | / filter | r refresh | q quit"
	if m.confirmKillTarget != "" {
		help = "confirm kill: y yes | n/esc cancel"
	}
	if m.filterQuery != "" {
		title += lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Render("  [filter: " + m.filterQuery + "]")
	}

	left := m.renderTreePane()
	right := m.renderDetailPane()

	content := ""
	if m.width > 70 {
		leftWidth := (m.width * 45) / 100
		if leftWidth < 30 {
			leftWidth = 30
		}
		rightWidth := m.width - leftWidth - 1
		if rightWidth < 20 {
			rightWidth = 20
		}
		leftStyled := lipgloss.NewStyle().Width(leftWidth).Render(left)
		rightStyled := lipgloss.NewStyle().Width(rightWidth).Render(right)
		content = lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, " ", rightStyled)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, left, "", right)
	}

	status := ""
	if m.errMsg != "" {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("error: " + m.errMsg)
	} else if m.statusMsg != "" {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.statusMsg)
	}

	if m.promptMode != promptNone {
		promptTitle := m.promptTitle()
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Render(promptTitle + ": " + m.prompt.View() + " (Enter submit, Esc cancel)")
	} else if m.confirmKillTarget != "" {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("kill session " + m.confirmKillTarget + "? press y to confirm, n to cancel")
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, help, "", content, "", status)
}

func (m *Model) rebuildRows() {
	rows := make([]treeRow, 0)
	query := strings.ToLower(strings.TrimSpace(m.filterQuery))
	for folderIndex, folder := range m.cfg.Folders {
		sessions := append([]tmux.Session(nil), m.sessions[folderIndex]...)
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].Name < sessions[j].Name
		})

		folderMatches := query == "" || containsAny(strings.ToLower(folder.Name), strings.ToLower(folder.Path), strings.ToLower(folder.Namespace), query)
		matchedSessions := make([]treeRow, 0, len(sessions))
		for _, s := range sessions {
			leaf := strings.TrimPrefix(s.Name, folder.Namespace+"/")
			status := "detached"
			if s.Attached {
				status = "attached"
			}
			row := treeRow{
				typeOf:      rowSession,
				folderIndex: folderIndex,
				sessionName: s.Name,
				leafName:    leaf,
				status:      status,
				windows:     s.Windows,
			}

			if folderMatches || query == "" || containsAny(strings.ToLower(leaf), strings.ToLower(s.Name), strings.ToLower(status), query) {
				matchedSessions = append(matchedSessions, row)
			}
		}

		if !folderMatches && len(matchedSessions) == 0 {
			continue
		}

		rows = append(rows, treeRow{typeOf: rowFolder, folderIndex: folderIndex})
		rows = append(rows, matchedSessions...)
	}

	m.rows = rows
	if m.selected >= len(m.rows) && len(m.rows) > 0 {
		m.selected = len(m.rows) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m Model) renderRow(row treeRow) string {
	if row.typeOf == rowFolder {
		folder := m.cfg.Folders[row.folderIndex]
		count := len(m.sessions[row.folderIndex])
		return fmt.Sprintf("[folder] %s [%d]", folder.Name, count)
	}

	return fmt.Sprintf("  - %s (%s, %d windows)", row.leafName, row.status, row.windows)
}

func (m Model) renderTreePane() string {
	rows := make([]string, 0, len(m.rows))
	for i, row := range m.rows {
		line := m.renderRow(row)
		if i == m.selected {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("> " + line)
		} else {
			line = "  " + line
		}
		rows = append(rows, line)
	}

	body := strings.Join(rows, "\n")
	if body == "" {
		body = "no folders or sessions match current filter"
	}

	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Render("Tree\n" + body)
}

func (m Model) renderDetailPane() string {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Render("Details\nNo selection")
	}

	row := m.rows[m.selected]
	if row.typeOf == rowFolder {
		folder := m.cfg.Folders[row.folderIndex]
		sessions := append([]tmux.Session(nil), m.sessions[row.folderIndex]...)
		sort.Slice(sessions, func(i, j int) bool { return sessions[i].Name < sessions[j].Name })

		lines := []string{
			"Details",
			"Type: folder",
			"Name: " + folder.Name,
			"Namespace: " + folder.Namespace,
			"Path: " + folder.Path,
		}
		if folder.DefaultCommand != "" {
			lines = append(lines, "Default command: "+folder.DefaultCommand)
		} else {
			lines = append(lines, "Default command: <none>")
		}
		lines = append(lines, fmt.Sprintf("Managed sessions: %d", len(sessions)))
		for _, s := range sessions {
			state := "detached"
			if s.Attached {
				state = "attached"
			}
			lines = append(lines, "- "+s.Name+" ("+state+")")
		}

		return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
	}

	folder := m.cfg.Folders[row.folderIndex]
	lines := []string{
		"Details",
		"Type: session",
		"Name: " + row.leafName,
		"Full name: " + row.sessionName,
		"Folder: " + folder.Name,
		"Path: " + folder.Path,
		"Status: " + row.status,
		fmt.Sprintf("Windows: %d", row.windows),
		"Actions: Enter attach, c run command, R rename, K kill",
	}
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
}

func (m Model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.client.ListSessions()
		if err != nil {
			return sessionsLoadedMsg{err: err}
		}

		grouped := map[int][]tmux.Session{}
		for _, session := range sessions {
			for idx, folder := range m.cfg.Folders {
				prefix := folder.Namespace + "/"
				if strings.HasPrefix(session.Name, prefix) {
					grouped[idx] = append(grouped[idx], session)
					break
				}
			}
		}

		return sessionsLoadedMsg{sessions: grouped}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return t
	})
}

func (m Model) selectedSessionRow() (treeRow, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return treeRow{}, false
	}
	row := m.rows[m.selected]
	if row.typeOf != rowSession {
		return treeRow{}, false
	}
	return row, true
}

func (m Model) selectedFolder() (config.Folder, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return config.Folder{}, false
	}
	row := m.rows[m.selected]
	return m.cfg.Folders[row.folderIndex], true
}

func (m Model) updateKillConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch strings.ToLower(key.String()) {
	case "y":
		target := m.confirmKillTarget
		m.confirmKillTarget = ""
		return m, m.killSessionCmd(target)
	case "n", "esc":
		m.confirmKillTarget = ""
		m.statusMsg = "kill cancelled"
		m.errMsg = ""
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) openPrompt(mode promptMode, initial, placeholder string) {
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
			m.statusMsg = ""
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.prompt.Value())
			m.prompt.Blur()
			mode := m.promptMode
			m.promptMode = promptNone

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
				return m, m.renameSessionCmd(row.sessionName, folder.Namespace+"/"+sanitizeLeaf(value))
			case promptRunCommand:
				row, ok := m.selectedSessionRow()
				if !ok {
					m.errMsg = "select a session"
					return m, nil
				}
				if value == "" {
					m.errMsg = "command cannot be empty"
					return m, nil
				}
				return m, m.sendCommandCmd(row.sessionName, value)
			case promptFilter:
					m.filterQuery = value
					m.rebuildRows()
					m.errMsg = ""
					if value == "" {
						m.statusMsg = "filter cleared"
					} else {
						m.statusMsg = "filter set: " + value
					}
					return m, nil
				}
			}
		}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}

func (m Model) promptTitle() string {
	switch m.promptMode {
	case promptNewSession:
		return "new session"
	case promptRenameSession:
		return "rename session"
	case promptRunCommand:
		return "run command"
	case promptFilter:
		return "set filter"
	default:
		return ""
	}
}

func containsAny(a, b, c, needle string) bool {
	return strings.Contains(a, needle) || strings.Contains(b, needle) || strings.Contains(c, needle)
}

func (m Model) defaultSessionLeaf(folder config.Folder) string {
	base := time.Now().Format("20060102-150405")
	name := folder.Namespace + "-" + base
	return sanitizeLeaf(name)
}

func sanitizeLeaf(in string) string {
	in = strings.TrimSpace(strings.ToLower(in))
	var b strings.Builder
	lastDash := false
	for _, r := range in {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '/' {
			continue
		}
		if lastDash {
			continue
		}
		b.WriteByte('-')
		lastDash = true
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "session"
	}
	return out
}

func (m Model) newSessionCmd(folder config.Folder, leaf string) tea.Cmd {
	leaf = sanitizeLeaf(leaf)
	fullName := folder.Namespace + "/" + leaf

	return func() tea.Msg {
		if err := m.client.NewSession(fullName, folder.Path); err != nil {
			return actionResultMsg{err: err}
		}
		if folder.DefaultCommand != "" {
			if err := m.client.SendKeys(fullName, folder.DefaultCommand); err != nil {
				return actionResultMsg{err: err}
			}
		}
		return actionResultMsg{status: "created " + fullName, attachTarget: fullName}
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

func (m Model) sendCommandCmd(name, command string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.SendKeys(name, command); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "sent command to " + name}
	}
}
