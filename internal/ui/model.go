package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/configfile"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

const refreshInterval = 2 * time.Second
const statusClearDelay = 1500 * time.Millisecond
const previewRefreshInterval = 200 * time.Millisecond

type rowType int

const (
	rowFolder rowType = iota
	rowSession
)

type treeRow struct {
	typeOf         rowType
	folderIndex    int
	sessionName    string
	leafName       string
	status         string
	windows        int
	hasAlerts      bool
	alertsBell     bool
	alertsActivity bool
	alertsSilence  bool
	currentCommand string
	paneTitle      string
	currentPath    string
	lastActivity   int64
}

type promptMode int

const (
	promptNone promptMode = iota
	promptNewSession
	promptRenameSession
	promptRunCommand
	promptFilter
	promptAddFolder
)

type detailMode int

const (
	detailNormal detailMode = iota
	detailPreview
)

// ── Color palette (forest/grove theme) ──────────────────────────────
// Primary:   green tones for branding, active states, attached
// Muted:     grays for borders, secondary text, help
// Semantic:  amber for detached, red for errors/danger only

const (
	colorPrimary    = "#73daca" // soft green — title, selection accent, attached
	colorPrimaryDim = "#3b8070" // dim green — borders, secondary accents
	colorText       = "#c9d1d9" // light gray — primary text
	colorTextDim    = "#6e7681" // dim gray — labels, help text, metadata
	colorTextMuted  = "#484f58" // very dim gray — borders, dividers
	colorBg         = "#161b22" // dark bg (for selection row only)
	colorBgSubtle   = "#21262d" // subtle bg — chips, panes
	colorAmber      = "#d29922" // amber — detached status
	colorRed        = "#f85149" // red — errors, kill confirmation
	colorWhite      = "#e6edf3" // bright white — emphasized text
)

type Model struct {
	cfg     config.Config
	cfgPath string
	client  tmux.SessionManager
	styles  styleSet

	width  int
	height int

	rows      []treeRow
	selected  int
	sessions  map[int][]tmux.Session
	statusMsg string
	statusSeq int
	errMsg    string

	filterQuery       string
	confirmKillTarget string
	detailScroll      int

	detailMode     detailMode
	previewTarget  string
	previewContent string
	previewLoading bool
	previewErr     error
	previewSeq     int
	previewZoomed  bool

	prompt        textinput.Model
	promptMode    promptMode
	promptStep    int
	pendingFolder config.Folder
}

type styleSet struct {
	// Header
	headerTitle lipgloss.Style
	headerMeta  lipgloss.Style
	headerSep   lipgloss.Style

	// Panes
	pane      lipgloss.Style
	paneDim   lipgloss.Style // dimmed pane for prompt overlay
	paneTitle lipgloss.Style
	divider   lipgloss.Style

	// Tree rows
	rowFolder       lipgloss.Style
	rowSession      lipgloss.Style
	rowSelected     lipgloss.Style
	rowSelectedText lipgloss.Style
	selAccent       lipgloss.Style // left accent bar for selection
	rowKillTarget   lipgloss.Style // red highlight for kill confirmation

	// Status indicators
	statusDotAttached lipgloss.Style
	statusDotDetached lipgloss.Style
	windowCount       lipgloss.Style
	commandDim        lipgloss.Style
	alertIndicator    lipgloss.Style

	// Detail pane
	detailName   lipgloss.Style
	detailStatus lipgloss.Style
	detailMeta   lipgloss.Style
	infoLabel    lipgloss.Style
	infoValue    lipgloss.Style

	// Footer / help bar
	helpKey    lipgloss.Style
	helpDesc   lipgloss.Style
	helpSep    lipgloss.Style
	footerOK   lipgloss.Style
	footerErr  lipgloss.Style
	footerWarn lipgloss.Style

	// Prompt
	promptLabel lipgloss.Style
	promptHint  lipgloss.Style

	// Empty state
	emptyTitle lipgloss.Style
	emptyHint  lipgloss.Style
}

func defaultStyles() styleSet {
	return styleSet{
		// Header
		headerTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPrimary)),
		headerMeta:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		headerSep:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),

		// Panes
		pane:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colorTextMuted)).Padding(0, 1),
		paneDim:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colorTextMuted)).Padding(0, 1).Faint(true),
		paneTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPrimary)),
		divider:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),

		// Tree rows
		rowFolder:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorText)),
		rowSession:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)),
		rowSelected:     lipgloss.NewStyle(),
		rowSelectedText: lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)).Bold(true),
		selAccent:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)),
		rowKillTarget:   lipgloss.NewStyle().Background(lipgloss.Color("#3d1214")),

		// Status indicators
		statusDotAttached: lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)),
		statusDotDetached: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		windowCount:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		commandDim:        lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Faint(true),
		alertIndicator:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorAmber)).Bold(true),

		// Detail pane
		detailName:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorWhite)),
		detailStatus: lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)),
		detailMeta:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		infoLabel:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		infoValue:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)),

		// Footer / help bar
		helpKey:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)).Bold(true),
		helpDesc:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		helpSep:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),
		footerOK:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)),
		footerErr:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)),
		footerWarn: lipgloss.NewStyle().Foreground(lipgloss.Color(colorAmber)),

		// Prompt
		promptLabel: lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)).Bold(true),
		promptHint:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Faint(true),

		// Empty state
		emptyTitle: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		emptyHint:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),
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

type clearStatusMsg struct {
	seq int
}

type folderAddedMsg struct {
	folder config.Folder
	err    error
}

type paneCapturedMsg struct {
	target  string
	content string
	err     error
	seq     int
}

type previewTickMsg struct{}

func NewModel(cfg config.Config, cfgPath string, client tmux.SessionManager) Model {
	t := textinput.New()
	t.CharLimit = 512
	t.Prompt = ""

	m := Model{
		cfg:     cfg,
		cfgPath: cfgPath,
		client:  client,
		styles:  defaultStyles(),

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
				return m, clearCmd
			}
			return m, nil
		case "up", "k":
			m.setSelected(m.selected - 1)
			return m, nil
		case "down", "j":
			m.setSelected(m.selected + 1)
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
		if msg.seq != m.previewSeq {
			return m, nil
		}
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
		return m, tea.Batch(
			m.capturePaneCmd(m.previewTarget, m.previewSeq),
			previewTickCmd(),
		)

	case folderAddedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.cfg.Folders = append(m.cfg.Folders, msg.folder)
		m.rebuildRows()
		clearCmd := m.setStatus("added folder: " + msg.folder.Name)
		return m, tea.Batch(clearCmd, m.loadSessionsCmd())

	case clearStatusMsg:
		if msg.seq == m.statusSeq {
			m.statusMsg = ""
		}
		return m, nil

	case time.Time:
		return m, tea.Batch(tickCmd(), m.loadSessionsCmd())
	}

	return m, nil
}

// ── View ────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	// Calculate content height: total - header(1) - blank line(1) - footer(1)
	contentH := m.height - 3
	if contentH < 4 {
		contentH = 4
	}

	// Pane heights = content height minus the border overhead (2 for top/bottom border)
	paneInnerH := contentH - 2
	if paneInnerH < 3 {
		paneInnerH = 3
	}

	dimPanes := m.promptMode != promptNone

	var content string
	if m.detailMode == detailPreview && m.previewZoomed {
		// Zoomed preview: full-width, no tree pane
		paneWidth := m.width
		paneInner := paneWidth - 4
		if paneInner < 10 {
			paneInner = 10
		}
		content = m.renderDetailPane(paneInnerH, paneInner, paneWidth, dimPanes)
	} else if m.width > 70 {
		leftWidth := (m.width * 30) / 100
		if leftWidth < 30 {
			leftWidth = 30
		}
		if leftWidth > 50 {
			leftWidth = 50
		}
		rightWidth := m.width - leftWidth - 1
		if rightWidth < 20 {
			rightWidth = 20
		}

		// innerWidth = totalWidth - border(2) - padding(2)
		leftInner := leftWidth - 4
		rightInner := rightWidth - 4

		left := m.renderTreePane(paneInnerH, leftInner, leftWidth, dimPanes)
		right := m.renderDetailPane(paneInnerH, rightInner, rightWidth, dimPanes)

		content = lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	} else {
		halfH := paneInnerH / 2
		if halfH < 3 {
			halfH = 3
		}
		left := m.renderTreePane(halfH, m.width-4, m.width, dimPanes)
		right := m.renderDetailPane(halfH, m.width-4, m.width, dimPanes)
		content = lipgloss.JoinVertical(lipgloss.Left, left, right)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, "", content, footer)
}

func (m Model) renderHeader() string {
	title := m.styles.headerTitle.Render("▸ grove")
	sep := m.styles.headerSep.Render("  ·  ")

	metaParts := []string{
		fmt.Sprintf("%d folders", len(m.cfg.Folders)),
		fmt.Sprintf("%d sessions", m.totalManagedSessions()),
	}
	if m.filterQuery != "" {
		metaParts = append(metaParts, "filter: "+m.filterQuery)
	}
	meta := m.styles.headerMeta.Render(strings.Join(metaParts, m.styles.headerSep.Render(" · ")))

	return title + sep + meta
}

// ── Footer (merged help bar + status) ───────────────────────────────

func (m Model) renderFooter() string {
	// Prompt mode: show prompt input
	if m.promptMode != promptNone {
		label := m.styles.promptLabel.Render(m.promptTitle() + " ")
		enterHint := "enter confirm"
		if m.promptMode == promptAddFolder && m.promptStep < 3 {
			enterHint = "enter next"
		}
		extra := ""
		if m.promptMode == promptAddFolder && m.promptStep == 1 {
			extra = " · tab complete"
		}
		hint := m.styles.promptHint.Render("  " + enterHint + " · esc cancel" + extra)
		return label + m.prompt.View() + hint
	}

	// Kill confirmation mode
	if m.confirmKillTarget != "" {
		warn := m.styles.footerWarn.Render("kill " + m.confirmKillTarget + "?")
		hint := m.styles.helpDesc.Render("  y/enter confirm · n cancel")
		return warn + hint
	}

	// Status message takes precedence
	if m.errMsg != "" {
		return m.styles.footerErr.Render("error: " + m.errMsg)
	}
	if m.statusMsg != "" {
		return m.styles.footerOK.Render(m.statusMsg)
	}

	// Default: context-sensitive help bar
	return m.renderHelpBar()
}

func (m Model) renderHelpBar() string {
	type binding struct {
		key  string
		desc string
	}

	// Context-sensitive hint at the start
	var bindings []binding
	if m.detailMode == detailPreview {
		zoomHint := "zoom in"
		if m.previewZoomed {
			zoomHint = "zoom out"
		}
		bindings = []binding{
			{"⏎", "attach"},
			{"z", zoomHint},
			{"esc", "back"},
			{"q", "quit"},
		}
	} else if _, ok := m.selectedSessionRow(); ok {
		bindings = []binding{
			{"⏎", "attach"},
			{"v", "preview"},
			{"e", "editor"},
			{"n", "new"},
			{"R", "rename"},
			{"K", "kill"},
			{"c", "cmd"},
			{"A", "add folder"},
		}
		if m.filterQuery != "" {
			bindings = append(bindings, binding{"esc", "clear filter"})
		}
		bindings = append(bindings, []binding{
			{"/", "filter"},
			{"r", "refresh"},
			{"q", "quit"},
		}...)
	} else {
		bindings = []binding{
			{"n", "new session"},
			{"e", "editor"},
			{"A", "add folder"},
			{"j/k", "navigate"},
		}
		if m.filterQuery != "" {
			bindings = append(bindings, binding{"esc", "clear filter"})
		}
		bindings = append(bindings, []binding{
			{"/", "filter"},
			{"r", "refresh"},
			{"q", "quit"},
		}...)
	}

	parts := make([]string, 0, len(bindings)*2)
	sep := m.styles.helpSep.Render(" · ")
	for i, b := range bindings {
		parts = append(parts, m.styles.helpKey.Render(b.key)+" "+m.styles.helpDesc.Render(b.desc))
		if i < len(bindings)-1 {
			parts = append(parts, sep)
		}
	}

	return strings.Join(parts, "")
}

// ── Tree Pane ───────────────────────────────────────────────────────

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
				typeOf:         rowSession,
				folderIndex:    folderIndex,
				sessionName:    s.Name,
				leafName:       leaf,
				status:         status,
				windows:        s.Windows,
				hasAlerts:      s.HasAlerts,
				alertsBell:     s.AlertsBell,
				alertsActivity: s.AlertsActivity,
				alertsSilence:  s.AlertsSilence,
				currentCommand: s.CurrentCommand,
				paneTitle:      s.PaneTitle,
				currentPath:    s.CurrentPath,
				lastActivity:   s.LastActivity,
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
	m.detailScroll = 0
}

func (m Model) isLastSessionInFolder(idx int) bool {
	if idx < 0 || idx >= len(m.rows) || m.rows[idx].typeOf != rowSession {
		return false
	}
	if idx+1 >= len(m.rows) {
		return true
	}
	return m.rows[idx+1].typeOf != rowSession
}

func (m Model) renderTreePane(innerH, maxWidth, paneWidth int, dim bool) string {
	if maxWidth < 10 {
		maxWidth = 10
	}

	bodyHeight := innerH - 1 // -1 for title
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Empty state
	if len(m.rows) == 0 {
		body := m.renderEmptyTree()
		title := m.styles.paneTitle.Render("Sessions")
		padded := padToHeight(title+"\n"+body, innerH)
		return m.styledPane(padded, paneWidth, dim)
	}

	rows := make([]string, 0, len(m.rows))
	start, end := windowAround(m.selected, len(m.rows), bodyHeight)

	for i := start; i < end; i++ {
		row := m.rows[i]
		isSelected := i == m.selected
		isKillTarget := m.confirmKillTarget != "" && row.sessionName == m.confirmKillTarget

		if row.typeOf == rowFolder {
			folder := m.cfg.Folders[row.folderIndex]
			count := len(m.sessions[row.folderIndex])
			text := fmt.Sprintf("▸ %s (%d)", folder.Name, count)
			text = truncateRight(text, maxWidth-2)

			if isSelected {
				rows = append(rows, m.selectedLine("▎"+text, maxWidth))
			} else {
				rows = append(rows, " "+m.styles.rowFolder.Render(text))
			}
		} else {
			isLast := m.isLastSessionInFolder(i)
			connector := "├"
			if isLast {
				connector = "└"
			}

			dotChar := "○"
			if row.status == "attached" {
				dotChar = "●"
			}

			winStr := fmt.Sprintf("(%dw)", row.windows)

			// Build suffix: alert indicators only.
			suffix := ""
			plainSuffix := ""
			if alertStr := alertIndicatorStr(row); alertStr != "" {
				suffix = " " + m.styles.alertIndicator.Render(alertStr)
				plainSuffix = " " + alertStr
			}

			// Layout: [indent 2][connector 1][ 1][dot 1][ 1][name][ 1][winStr][suffix]
			nameMax := maxWidth - 2 - 1 - 1 - 1 - 1 - 1 - len(winStr) - len(plainSuffix)
			if nameMax < 6 {
				nameMax = 6
			}
			name := truncateRight(row.leafName, nameMax)

			if isSelected {
				plain := "▎" + connector + " " + dotChar + " " + name + " " + winStr + plainSuffix
				rows = append(rows, m.selectedLine(plain, maxWidth))
			} else if isKillTarget {
				plain := "  " + connector + " " + dotChar + " " + name + " " + winStr + plainSuffix
				rows = append(rows, m.styles.rowKillTarget.Render(padRight(plain, maxWidth)))
			} else {
				dot := m.styles.statusDotDetached.Render(dotChar)
				if row.status == "attached" {
					dot = m.styles.statusDotAttached.Render(dotChar)
				}
				winCount := m.styles.windowCount.Render(winStr)
				line := "  " + m.styles.helpSep.Render(connector) + " " + dot + " " + m.styles.rowSession.Render(name) + " " + winCount + suffix
				rows = append(rows, line)
			}
		}
	}

	body := strings.Join(rows, "\n")
	if start > 0 {
		body = m.styles.headerMeta.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n" + body
	}
	if end < len(m.rows) {
		body += "\n" + m.styles.headerMeta.Render(fmt.Sprintf("  ↓ %d more", len(m.rows)-end))
	}

	title := m.styles.paneTitle.Render("Sessions")
	padded := padToHeight(title+"\n"+body, innerH)
	return m.styledPane(padded, paneWidth, dim)
}

func (m Model) renderEmptyTree() string {
	if len(m.cfg.Folders) == 0 {
		return m.styles.emptyTitle.Render("no folders configured") + "\n" +
			m.styles.emptyHint.Render("press A to add a folder")
	}
	if m.filterQuery != "" {
		return m.styles.emptyTitle.Render("no matches for filter") + "\n" +
			m.styles.emptyHint.Render("press / to change filter")
	}
	return m.styles.emptyTitle.Render("no sessions yet") + "\n" +
		m.styles.emptyHint.Render("press n to create a session")
}

// ── Detail Pane ─────────────────────────────────────────────────────

func (m Model) renderDetailPane(innerH, maxWidth, paneWidth int, dim bool) string {
	if maxWidth < 10 {
		maxWidth = 10
	}

	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		title := m.styles.paneTitle.Render("Details")
		hint := m.styles.emptyHint.Render("select a folder or session")
		padded := padToHeight(title+"\n\n"+hint, innerH)
		return m.styledPane(padded, paneWidth, dim)
	}

	row := m.rows[m.selected]

	if m.detailMode == detailPreview && row.typeOf == rowSession {
		return m.renderPreviewPane(innerH, maxWidth, paneWidth, dim)
	}

	var lines []string

	if row.typeOf == rowFolder {
		folder := m.cfg.Folders[row.folderIndex]
		sessions := append([]tmux.Session(nil), m.sessions[row.folderIndex]...)
		sort.Slice(sessions, func(i, j int) bool { return sessions[i].Name < sessions[j].Name })

		// Folder card
		lines = []string{
			m.styles.detailName.Render(folder.Name),
			m.styles.detailMeta.Render(folder.Namespace + " · " + fmt.Sprintf("%d sessions", len(sessions))),
			"",
			m.kv("Path", truncateMiddle(folder.Path, maxWidth-6)),
		}
		if folder.DefaultCommand != "" {
			lines = append(lines, m.kv("Command", truncateRight(folder.DefaultCommand, maxWidth-10)))
		}

		if len(sessions) > 0 {
			// Activity summary
			running := 0
			bellCount := 0
			activityCount := 0
			silenceCount := 0
			for _, s := range sessions {
				if s.CurrentCommand != "" && !isShellCommand(s.CurrentCommand) {
					running++
				}
				if s.AlertsBell {
					bellCount++
				}
				if s.AlertsActivity {
					activityCount++
				}
				if s.AlertsSilence {
					silenceCount++
				}
			}
			if running > 0 || bellCount > 0 || activityCount > 0 || silenceCount > 0 {
				var parts []string
				if running > 0 {
					parts = append(parts, fmt.Sprintf("%d running", running))
				}
				if bellCount > 0 {
					parts = append(parts, fmt.Sprintf("%d bell", bellCount))
				}
				if activityCount > 0 {
					parts = append(parts, fmt.Sprintf("%d activity", activityCount))
				}
				if silenceCount > 0 {
					parts = append(parts, fmt.Sprintf("%d silence", silenceCount))
				}
				lines = append(lines, m.styles.detailMeta.Render(strings.Join(parts, ", ")))
			}

			lines = append(lines, "", m.styles.infoLabel.Render("Sessions"))
			for _, s := range sessions {
				dot := m.styles.statusDotDetached.Render("○")
				if s.Attached {
					dot = m.styles.statusDotAttached.Render("●")
				}
				leaf := strings.TrimPrefix(s.Name, folder.Namespace+"/")
				entry := dot + " " + m.styles.infoValue.Render(truncateRight(leaf, maxWidth-4))
				if s.CurrentCommand != "" && !isShellCommand(s.CurrentCommand) {
					entry += " " + m.styles.commandDim.Render(truncateRight(s.CurrentCommand, 12))
				}
				lines = append(lines, entry)
			}
		} else {
			lines = append(lines, "", m.styles.emptyHint.Render("press n to create a session"))
		}
	} else {
		folder := m.cfg.Folders[row.folderIndex]

		// Session card
		var statusLine string
		if row.status == "attached" {
			statusLine = m.styles.statusDotAttached.Render("●") + " " + m.styles.detailStatus.Render("attached")
		} else {
			statusLine = m.styles.statusDotDetached.Render("○") + " " + m.styles.detailMeta.Render("detached")
		}
		statusLine += m.styles.detailMeta.Render(fmt.Sprintf(" · %d windows", row.windows))

		lines = []string{
			m.styles.detailName.Render(truncateRight(row.leafName, maxWidth)),
			statusLine,
			"",
			m.kv("Full name", truncateRight(row.sessionName, maxWidth-12)),
			m.kv("Folder", truncateRight(folder.Name, maxWidth-9)),
			m.kv("Path", truncateMiddle(folder.Path, maxWidth-6)),
		}

		if row.currentCommand != "" {
			lines = append(lines, m.kv("Running", truncateRight(row.currentCommand, maxWidth-10)))
		}
		if title := paneDisplayTitle(row); title != "" {
			lines = append(lines, m.kv("Title", truncateRight(title, maxWidth-8)))
		}
		if row.lastActivity > 0 {
			d := time.Since(time.Unix(row.lastActivity, 0))
			lines = append(lines, m.kv("Last active", formatDuration(d)))
		}
		if row.hasAlerts {
			var alertParts []string
			if row.alertsBell {
				alertParts = append(alertParts, "bell (!)")
			}
			if row.alertsActivity {
				alertParts = append(alertParts, "activity (#)")
			}
			if row.alertsSilence {
				alertParts = append(alertParts, "silence (~)")
			}
			if len(alertParts) == 0 {
				alertParts = append(alertParts, "alerts pending")
			}
			lines = append(lines, m.styles.alertIndicator.Render(alertIndicatorStr(row)+" ")+strings.Join(alertParts, ", "))
		}
	}

	return m.renderDetailLines(lines, innerH, paneWidth, dim)
}

func (m Model) renderDetailLines(lines []string, innerH, paneWidth int, dim bool) string {
	if len(lines) == 0 {
		return m.styledPane("", paneWidth, dim)
	}
	bodyHeight := innerH
	if bodyHeight < 4 {
		bodyHeight = 4
	}
	scroll := m.detailScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= len(lines) {
		scroll = len(lines) - 1
	}

	start := scroll
	end := start + bodyHeight
	if end > len(lines) {
		end = len(lines)
		start = end - bodyHeight
		if start < 0 {
			start = 0
		}
	}

	body := strings.Join(lines[start:end], "\n")
	if start > 0 {
		body = m.styles.headerMeta.Render(fmt.Sprintf("  ↑ %d above", start)) + "\n" + body
	}
	if end < len(lines) {
		body += "\n" + m.styles.headerMeta.Render(fmt.Sprintf("  ↓ %d below", len(lines)-end))
	}

	padded := padToHeight(body, innerH)
	return m.styledPane(padded, paneWidth, dim)
}

func truncateLines(lines []string, maxWidth int) []string {
	if maxWidth < 1 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, ansi.Truncate(line, maxWidth, ""))
	}
	return out
}

func wrapLines(lines []string, maxWidth int) []string {
	if maxWidth < 1 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped := ansi.Hardwrap(line, maxWidth, true)
		out = append(out, strings.Split(wrapped, "\n")...)
	}
	return out
}

func (m Model) renderPreviewPane(innerH, maxWidth, paneWidth int, dim bool) string {
	row, _ := m.selectedSessionRow()
	title := m.styles.paneTitle.Render("Preview") +
		" " + m.styles.detailMeta.Render(row.sessionName)

	if m.previewLoading {
		padded := padToHeight(title+"\n\n"+m.styles.emptyHint.Render("capturing pane…"), innerH)
		return m.styledPane(padded, paneWidth, dim)
	}
	if m.previewErr != nil {
		padded := padToHeight(title+"\n\n"+m.styles.footerErr.Render("error: "+m.previewErr.Error()), innerH)
		return m.styledPane(padded, paneWidth, dim)
	}

	content := sanitizeANSI(m.previewContent)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	lines = truncateLines(lines, maxWidth)
	if maxLines := innerH - 2; len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	contentLines := append([]string{title, ""}, lines...)
	return m.renderDetailLines(contentLines, innerH, paneWidth, dim)
}

// ── Commands ────────────────────────────────────────────────────────

func (m Model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if len(m.cfg.Folders) == 0 {
			return sessionsLoadedMsg{sessions: map[int][]tmux.Session{}}
		}

		sessions, err := m.client.ListSessions()
		if err != nil {
			return sessionsLoadedMsg{err: err}
		}

		// Fetch pane info for commands, titles, and alert flags (graceful degradation on failure)
		panes, paneErr := m.client.ListPanes()
		if paneErr == nil {
			states := tmux.ActivePaneStates(panes)
			for i := range sessions {
				if st, ok := states[sessions[i].Name]; ok {
					sessions[i].CurrentCommand = st.Command
					sessions[i].PaneTitle = st.PaneTitle
					sessions[i].CurrentPath = st.CurrentPath
					if st.BellFlag {
						sessions[i].AlertsBell = true
					}
					if st.ActivityFlag {
						sessions[i].AlertsActivity = true
					}
					if st.SilenceFlag {
						sessions[i].AlertsSilence = true
					}
				}
			}
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

func previewTickCmd() tea.Cmd {
	return tea.Tick(previewRefreshInterval, func(time.Time) tea.Msg {
		return previewTickMsg{}
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

func (m Model) updatePreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		if m.previewZoomed {
			m.previewZoomed = false
			return m, nil
		}
		m.detailMode = detailNormal
		return m, nil
	case "z":
		m.previewZoomed = !m.previewZoomed
		m.detailScroll = 0
		return m, nil
	case "r":
		return m, m.capturePaneCmd(m.previewTarget, m.previewSeq)
	case "enter":
		row, ok := m.selectedSessionRow()
		if !ok {
			return m, nil
		}
		m.detailMode = detailNormal
		m.previewZoomed = false
		m.statusMsg = "attached to " + row.sessionName + " (detach with Ctrl-b d)"
		m.errMsg = ""
		return m, tea.ExecProcess(m.client.AttachCommand(row.sessionName), func(err error) tea.Msg {
			return attachedMsg{err: err}
		})
	}
	return m, nil
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
		case "tab":
			if m.promptMode == promptAddFolder && m.promptStep == 1 {
				m.completePathInput()
				return m, nil
			}
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
			case promptAddFolder:
				switch m.promptStep {
				case 0:
					if value == "" {
						m.errMsg = "folder name is required"
						return m, nil
					}
					ns := config.Slug(value)
					if ns == "" {
						m.errMsg = "folder name produced empty namespace"
						return m, nil
					}
					for _, f := range m.cfg.Folders {
						if f.Namespace == ns {
							m.errMsg = fmt.Sprintf("namespace %q already exists", ns)
							return m, nil
						}
					}
					m.pendingFolder.Name = value
					m.pendingFolder.Namespace = ns
					m.promptStep = 1
					m.promptMode = promptAddFolder
					m.openPrompt(promptAddFolder, "", "folder path")
					return m, textinput.Blink
				case 1:
					if value == "" {
						m.errMsg = "folder path is required"
						return m, nil
					}
					value = config.ExpandHome(value)
					absPath, err := filepath.Abs(value)
					if err != nil {
						m.errMsg = fmt.Sprintf("invalid path: %v", err)
						return m, nil
					}
					info, err := os.Stat(absPath)
					if err != nil {
						m.errMsg = fmt.Sprintf("path not found: %v", err)
						return m, nil
					}
					if !info.IsDir() {
						m.errMsg = "path is not a directory"
						return m, nil
					}
					m.pendingFolder.Path = absPath
					m.promptStep = 2
					m.promptMode = promptAddFolder
					m.openPrompt(promptAddFolder, "", "default command (optional)")
					return m, textinput.Blink
				case 2:
					m.pendingFolder.DefaultCommand = value
					m.promptStep = 3
					m.promptMode = promptAddFolder
					m.openPrompt(promptAddFolder, "", "editor command (optional, e.g. code .)")
					return m, textinput.Blink
				case 3:
					m.pendingFolder.EditorCommand = value
					return m, m.addFolderCmd(m.pendingFolder)
				}
			case promptFilter:
				m.filterQuery = value
				m.rebuildRows()
				var clearCmd tea.Cmd
				if value == "" {
					clearCmd = m.setStatus("filter cleared")
				} else {
					clearCmd = m.setStatus("filter set: " + value)
				}
				return m, clearCmd
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
		return fmt.Sprintf("add folder (%d/4):", m.promptStep+1)
	default:
		return ""
	}
}

func (m Model) selectedLine(plain string, width int) string {
	return m.styles.rowSelectedText.Width(width).Render(plain)
}

func (m Model) styledPane(content string, paneWidth int, dim bool) string {
	// paneWidth is total outer width including border+padding.
	// lipgloss Width includes padding but excludes border.
	// border = 2 (left+right), so Width = paneWidth - 2.
	// With Padding(0,1), the actual text area = Width - 2 = paneWidth - 4.
	w := paneWidth - 2
	if w < 1 {
		w = 1
	}
	if dim {
		return m.styles.paneDim.Width(w).Render(content)
	}
	return m.styles.pane.Width(w).Render(content)
}

func (m *Model) setStatus(msg string) tea.Cmd {
	m.statusMsg = msg
	m.errMsg = ""
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(statusClearDelay, func(time.Time) tea.Msg {
		return clearStatusMsg{seq: seq}
	})
}

// ── Helpers ─────────────────────────────────────────────────────────

func containsAny(a, b, c, needle string) bool {
	return strings.Contains(a, needle) || strings.Contains(b, needle) || strings.Contains(c, needle)
}

func (m *Model) setSelected(next int) {
	if len(m.rows) == 0 {
		m.selected = 0
		m.detailScroll = 0
		return
	}
	if next < 0 {
		next = 0
	}
	if next >= len(m.rows) {
		next = len(m.rows) - 1
	}
	if next != m.selected {
		m.detailScroll = 0
	}
	m.selected = next
}

func (m Model) contentHeight() int {
	if m.height <= 0 {
		return 18
	}
	reserved := 3 // header + blank + footer
	h := m.height - reserved
	if h < 8 {
		h = 8
	}
	return h
}

func windowAround(selected, total, maxItems int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if maxItems <= 0 || maxItems >= total {
		return 0, total
	}
	start := selected - maxItems/2
	if start < 0 {
		start = 0
	}
	end := start + maxItems
	if end > total {
		end = total
		start = end - maxItems
		if start < 0 {
			start = 0
		}
	}
	return start, end
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
	return string(r[:max-1]) + "…"
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
	return string(r[:head]) + "…" + string(r[len(r)-tail:])
}

func padRight(s string, width int) string {
	// Pad with spaces to fill width (approximate — ANSI-aware padding would be better
	// but for our use this is sufficient since lipgloss handles final rendering)
	sLen := lipgloss.Width(s)
	if sLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-sLen)
}

func padToHeight(s string, height int) string {
	lines := strings.Count(s, "\n") + 1
	if lines >= height {
		return s
	}
	return s + strings.Repeat("\n", height-lines)
}

func alertIndicatorStr(row treeRow) string {
	var s string
	if row.alertsBell {
		s += "!"
	}
	if row.alertsActivity {
		s += "#"
	}
	if row.alertsSilence {
		s += "~"
	}
	return s
}

func paneDisplayTitle(row treeRow) string {
	t := strings.TrimSpace(row.paneTitle)
	if t == "" {
		return ""
	}
	// Filter out bare hostnames and default shell titles — only show titles
	// that look informative (contain path separators, dots with extensions, or spaces)
	if strings.Contains(t, "/") || strings.Contains(t, " ") {
		return t
	}
	if dot := strings.LastIndex(t, "."); dot > 0 && dot < len(t)-1 {
		return t
	}
	return ""
}

func isShellCommand(cmd string) bool {
	switch strings.ToLower(cmd) {
	case "zsh", "bash", "fish", "sh", "dash", "ksh":
		return true
	}
	return false
}

func sanitizeANSI(s string) string {
	// Strip CSI sequences that are not SGR (Select Graphic Rendition).
	// SGR sequences end with 'm'; others (cursor movement, screen clear, etc.)
	// could interfere with Bubble Tea's rendering.
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '\x1b' && s[i+1] == '[' {
			// Find end of CSI sequence (first byte in 0x40–0x7E)
			j := i + 2
			for j < len(s) && s[j] >= 0x20 && s[j] <= 0x3F {
				j++
			}
			if j < len(s) && s[j] >= 0x40 && s[j] <= 0x7E {
				if s[j] == 'm' {
					b.WriteString(s[i : j+1])
				}
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < 30*time.Second:
		return "just now"
	case d < 90*time.Second:
		return "1 min ago"
	case d < time.Hour:
		return fmt.Sprintf("%d mins ago", int(d.Minutes()))
	case d < 2*time.Hour:
		return "1 hour ago"
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		days := int(d.Hours()) / 24
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
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

func (m Model) addFolderCmd(f config.Folder) tea.Cmd {
	cfgPath := m.cfgPath
	return func() tea.Msg {
		if err := configfile.AppendFolder(cfgPath, f); err != nil {
			return folderAddedMsg{err: err}
		}
		return folderAddedMsg{folder: f}
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

func (m *Model) startPreview() tea.Cmd {
	row, ok := m.selectedSessionRow()
	if !ok {
		m.detailMode = detailNormal
		return nil
	}
	m.previewSeq++
	m.previewTarget = row.sessionName
	m.previewLoading = true
	m.previewErr = nil
	m.previewContent = ""
	m.detailScroll = 0
	m.previewZoomed = false
	return m.capturePaneCmd(row.sessionName, m.previewSeq)
}

func (m Model) capturePaneCmd(target string, seq int) tea.Cmd {
	return func() tea.Msg {
		content, err := m.client.CapturePane(target)
		return paneCapturedMsg{target: target, content: content, err: err, seq: seq}
	}
}
