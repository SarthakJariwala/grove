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
	styles styleSet

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

type styleSet struct {
	headerTitle      lipgloss.Style
	headerMeta       lipgloss.Style
	helpBar          lipgloss.Style
	pane             lipgloss.Style
	paneTitle        lipgloss.Style
	rowFolder        lipgloss.Style
	rowSession       lipgloss.Style
	rowSelected      lipgloss.Style
	statusAttached   lipgloss.Style
	statusDetached   lipgloss.Style
	infoLabel        lipgloss.Style
	infoValue        lipgloss.Style
	footerOK         lipgloss.Style
	footerErr        lipgloss.Style
	footerPrompt     lipgloss.Style
	footerWarn       lipgloss.Style
	actionChip       lipgloss.Style
	actionChipActive lipgloss.Style
}

func defaultStyles() styleSet {
	return styleSet{
		headerTitle:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")),
		headerMeta:       lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		helpBar:          lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
		pane:             lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1),
		paneTitle:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111")),
		rowFolder:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("225")),
		rowSession:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		rowSelected:      lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true),
		statusAttached:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		statusDetached:   lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		infoLabel:        lipgloss.NewStyle().Foreground(lipgloss.Color("109")),
		infoValue:        lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		footerOK:         lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		footerErr:        lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		footerPrompt:     lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Bold(true),
		footerWarn:       lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
		actionChip:       lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236")).Padding(0, 1),
		actionChipActive: lipgloss.NewStyle().Foreground(lipgloss.Color("231")).Background(lipgloss.Color("33")).Padding(0, 1).Bold(true),
	}
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
		styles: defaultStyles(),

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
	header := m.renderHeader()
	help := m.renderHelpBar()

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
		status = m.styles.footerErr.Render("error: " + m.errMsg)
	} else if m.statusMsg != "" {
		status = m.styles.footerOK.Render(m.statusMsg)
	}

	if m.promptMode != promptNone {
		promptTitle := m.promptTitle()
		status = m.styles.footerPrompt.Render(promptTitle + ": " + m.prompt.View() + " (Enter submit, Esc cancel)")
	} else if m.confirmKillTarget != "" {
		status = m.styles.footerWarn.Render("kill session " + m.confirmKillTarget + "? press y to confirm, n to cancel")
	}

	if status == "" {
		status = m.styles.headerMeta.Render("ready")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, help, "", content, "", status)
}

func (m Model) renderHeader() string {
	title := m.styles.headerTitle.Render("grove")
	metaParts := []string{
		fmt.Sprintf("folders %d", len(m.cfg.Folders)),
		fmt.Sprintf("sessions %d", m.totalManagedSessions()),
	}
	if m.filterQuery != "" {
		metaParts = append(metaParts, "filter: "+m.filterQuery)
	}
	meta := m.styles.headerMeta.Render(strings.Join(metaParts, " | "))
	return lipgloss.JoinVertical(lipgloss.Left, title, meta)
}

func (m Model) renderHelpBar() string {
	if m.confirmKillTarget != "" {
		return m.styles.helpBar.Render("confirm kill mode: y confirm | n or esc cancel")
	}

	chips := []string{
		m.styles.actionChipActive.Render("enter attach"),
		m.styles.actionChip.Render("n new"),
		m.styles.actionChip.Render("R rename"),
		m.styles.actionChip.Render("K kill"),
		m.styles.actionChip.Render("c command"),
		m.styles.actionChip.Render("/ filter"),
		m.styles.actionChip.Render("r refresh"),
		m.styles.actionChip.Render("q quit"),
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, chips...)
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
		return fmt.Sprintf("[folder] %s (%d)", folder.Name, count)
	}

	return fmt.Sprintf("  %s  [%s]  %d windows", row.leafName, row.status, row.windows)
}

func (m Model) renderTreePane() string {
	rows := make([]string, 0, len(m.rows))
	maxWidth := 40
	if m.width > 0 {
		maxWidth = (m.width * 45 / 100) - 8
		if maxWidth < 24 {
			maxWidth = 24
		}
	}
	for i, row := range m.rows {
		raw := truncateRight(m.renderRow(row), maxWidth)
		line := raw
		if i == m.selected {
			line = m.styles.rowSelected.Render(" " + raw + " ")
		} else {
			if row.typeOf == rowFolder {
				line = "  " + m.styles.rowFolder.Render(raw)
			} else {
				line = "  " + m.decorateSessionLine(raw, row.status)
			}
		}
		rows = append(rows, line)
	}

	body := strings.Join(rows, "\n")
	if body == "" {
		body = "no folders or sessions match current filter"
	}

	header := m.styles.paneTitle.Render("Sessions")
	return m.styles.pane.Render(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

func (m Model) renderDetailPane() string {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		header := m.styles.paneTitle.Render("Details")
		return m.styles.pane.Render(lipgloss.JoinVertical(lipgloss.Left, header, "No selection"))
	}

	row := m.rows[m.selected]
	if row.typeOf == rowFolder {
		folder := m.cfg.Folders[row.folderIndex]
		sessions := append([]tmux.Session(nil), m.sessions[row.folderIndex]...)
		sort.Slice(sessions, func(i, j int) bool { return sessions[i].Name < sessions[j].Name })

		lines := []string{
			m.styles.paneTitle.Render("Details"),
			m.kv("Type", "folder"),
			m.kv("Name", folder.Name),
			m.kv("Namespace", folder.Namespace),
			m.kv("Path", truncateMiddle(folder.Path, 64)),
		}
		if folder.DefaultCommand != "" {
			lines = append(lines, m.kv("Default command", truncateRight(folder.DefaultCommand, 64)))
		} else {
			lines = append(lines, m.kv("Default command", "<none>"))
		}
		lines = append(lines, m.kv("Managed sessions", fmt.Sprintf("%d", len(sessions))))
		for _, s := range sessions {
			state := "detached"
			if s.Attached {
				state = "attached"
			}
			lines = append(lines, "- "+truncateRight(s.Name, 70)+" ("+state+")")
		}

		return m.styles.pane.Render(strings.Join(lines, "\n"))
	}

	folder := m.cfg.Folders[row.folderIndex]
	lines := []string{
		m.styles.paneTitle.Render("Details"),
		m.kv("Type", "session"),
		m.kv("Name", row.leafName),
		m.kv("Full name", row.sessionName),
		m.kv("Folder", folder.Name),
		m.kv("Path", truncateMiddle(folder.Path, 64)),
		m.kv("Status", row.status),
		m.kv("Windows", fmt.Sprintf("%d", row.windows)),
		"",
		lipgloss.JoinHorizontal(lipgloss.Left,
			m.styles.actionChipActive.Render("Enter attach"), " ",
			m.styles.actionChip.Render("c command"), " ",
			m.styles.actionChip.Render("R rename"), " ",
			m.styles.actionChip.Render("K kill"),
		),
	}
	return m.styles.pane.Render(strings.Join(lines, "\n"))
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

func (m Model) totalManagedSessions() int {
	total := 0
	for _, sessions := range m.sessions {
		total += len(sessions)
	}
	return total
}

func (m Model) kv(label, value string) string {
	return m.styles.infoLabel.Render(label+": ") + m.styles.infoValue.Render(value)
}

func (m Model) decorateSessionLine(line, status string) string {
	if status == "attached" {
		return strings.Replace(line, "[attached]", "["+m.styles.statusAttached.Render("attached")+"]", 1)
	}
	return strings.Replace(line, "[detached]", "["+m.styles.statusDetached.Render("detached")+"]", 1)
}

func truncateRight(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "~"
}

func truncateMiddle(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return truncateRight(s, max)
	}
	head := (max - 1) / 2
	tail := max - 1 - head
	return string(r[:head]) + "~" + string(r[len(r)-tail:])
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
