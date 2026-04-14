package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

const refreshInterval = 500 * time.Millisecond
const statusClearDelay = 1500 * time.Millisecond
const previewRefreshInterval = 200 * time.Millisecond

type rowType int

const (
	rowFolder rowType = iota
	rowSection
	rowAgentInstance
	rowTerminalInstance
	rowCommand
)

type treeRow struct {
	typeOf         rowType
	section        sectionKind
	folderIndex    int
	sessionName    string
	displayName    string
	commandText    string
	status         string
	attached       bool
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

type overlayMode int

const (
	overlayNone overlayMode = iota
	overlayAgentPicker
)

type agentChoice struct {
	Label   string
	Agent   config.Agent
	Persist bool
	IsNew   bool
}

type promptMode int

const (
	promptNone promptMode = iota
	promptNewSession
	promptRenameSession
	promptRunCommand
	promptFilter
	promptAddFolder
	promptAddAgentName
	promptAddAgentCommand
	promptAddCommandName
	promptAddCommandCommand
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
	colorPrimary    = "#5eead4"
	colorPrimaryDim = "#2dd4a8"
	colorText       = "#d1d9e0"
	colorTextDim    = "#768390"
	colorTextMuted  = "#444c56"
	colorTextFaint  = "#3b444d"
	colorBg         = "#0d1117"
	colorBgSubtle   = "#161b22"
	colorBgElevated = "#1a3a35"
	colorBgChip     = "#21262d"
	colorAmber      = "#e3b341"
	colorRed        = "#f85149"
	colorBlue       = "#58a6ff"
	colorWhite      = "#ecf2f8"

	colorTreeActive    = "#a6e3a1"
	colorTreeAttention = "#f9e2af"
	colorTreeDim       = "#6c7086"
	colorTreeDark      = "#45475a"
	colorTreeAccent    = "#89b4fa"
	colorTreeHighlight = "#1e3a5f"
)

type Model struct {
	cfg     config.Config
	cfgPath string
	client  tmux.SessionManager
	styles  styleSet

	width  int
	height int

	rows           []treeRow
	selected       int
	sessions       map[int][]tmux.Session
	sessionWindows map[string][]int
	activeWindows  map[string]int
	statusMsg      string
	statusSeq      int
	errMsg         string

	filterQuery        string
	confirmKillTarget  string
	detailScroll       int
	overlayMode        overlayMode
	overlayIndex       int
	overlayFolderIndex int
	agentChoices       []agentChoice

	detailMode      detailMode
	previewSession  string
	previewWindow   int
	previewContent  string
	previewLoading  bool
	previewErr      error
	previewSeq      int
	previewZoomed   bool
	previewInFlight bool

	prompt            textinput.Model
	promptMode        promptMode
	promptTarget      string
	promptFolderIndex int
	promptStep        int
	pendingFolder     config.Folder
	pendingAgent      config.Agent
	pendingCommand    config.Command
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
	rowFolder          lipgloss.Style
	rowFolderIdle      lipgloss.Style
	rowFolderCount     lipgloss.Style // muted session count
	rowSession         lipgloss.Style
	rowSelected        lipgloss.Style
	rowSelectedText    lipgloss.Style
	rowSelectedBg      lipgloss.Style
	selAccent          lipgloss.Style // left accent bar for selection
	rowKillTarget      lipgloss.Style // red highlight for kill confirmation
	rowDim             lipgloss.Style // dimmed row when prompt is active
	folderDotActive    lipgloss.Style
	folderDotAttention lipgloss.Style
	folderDotIdle      lipgloss.Style
	childIconActive    lipgloss.Style
	childIconDim       lipgloss.Style
	badgeActive        lipgloss.Style

	// Status indicators
	statusDotAttached lipgloss.Style
	statusDotDetached lipgloss.Style
	windowCount       lipgloss.Style
	commandDim        lipgloss.Style
	alertIndicator    lipgloss.Style

	// Detail pane
	detailName          lipgloss.Style
	detailMeta          lipgloss.Style
	detailSection       lipgloss.Style
	detailSectionHeader lipgloss.Style // uppercase section headers in detail pane
	infoLabel           lipgloss.Style
	infoValue           lipgloss.Style
	chipMuted           lipgloss.Style
	chipPrimary         lipgloss.Style
	chipWarn            lipgloss.Style

	// Footer / help bar
	helpBracket lipgloss.Style // bracket framing for help keys
	helpKey     lipgloss.Style
	helpDesc    lipgloss.Style
	helpSep     lipgloss.Style
	footerOK    lipgloss.Style
	footerErr   lipgloss.Style
	footerWarn  lipgloss.Style

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
		headerSep:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextFaint)),

		// Panes — very dim borders to recede behind content
		pane:      lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colorTextFaint)).Padding(0, 1),
		paneDim:   lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colorTextFaint)).Padding(0, 1).Faint(true),
		paneTitle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPrimary)),
		divider:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextFaint)),

		// Tree rows
		rowFolder:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorWhite)),
		rowFolderIdle:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeDim)),
		rowFolderCount:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeDim)),
		rowSession:         lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)),
		rowSelected:        lipgloss.NewStyle().Background(lipgloss.Color(colorTreeHighlight)),
		rowSelectedText:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeAccent)).Bold(true),
		rowSelectedBg:      lipgloss.NewStyle().Background(lipgloss.Color(colorTreeHighlight)),
		selAccent:          lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)),
		rowKillTarget:      lipgloss.NewStyle().Background(lipgloss.Color("#3d1214")),
		rowDim:             lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Faint(true),
		folderDotActive:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeActive)),
		folderDotAttention: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeAttention)),
		folderDotIdle:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeDark)),
		childIconActive:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeActive)),
		childIconDim:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeDim)),
		badgeActive:        lipgloss.NewStyle().Foreground(lipgloss.Color(colorTreeActive)).Bold(true),

		// Status indicators
		statusDotAttached: lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)),
		statusDotDetached: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		windowCount:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlue)),
		commandDim:        lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Faint(true),
		alertIndicator:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorAmber)).Bold(true),

		// Detail pane
		detailName:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorWhite)),
		detailMeta:          lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		detailSection:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		detailSectionHeader: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Bold(true),
		infoLabel:           lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		infoValue:           lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)),
		chipMuted:           lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		chipPrimary:         lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)).Bold(true),
		chipWarn:            lipgloss.NewStyle().Foreground(lipgloss.Color(colorAmber)).Bold(true),

		// Footer / help bar
		helpBracket: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),
		helpKey:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)).Bold(true),
		helpDesc:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		helpSep:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),
		footerOK:    lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)),
		footerErr:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)),
		footerWarn:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorAmber)),

		// Prompt
		promptLabel: lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimary)).Bold(true),
		promptHint:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)).Faint(true),

		// Empty state
		emptyTitle: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim)),
		emptyHint:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),
	}
}

type sessionsLoadedMsg struct {
	sessions       map[int][]tmux.Session
	sessionWindows map[string][]int
	activeWindows  map[string]int
	panesFresh     bool
	err            error
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

type commandAddedMsg struct {
	folderIndex int
	command     config.Command
	err         error
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

		sessions:          map[int][]tmux.Session{},
		sessionWindows:    map[string][]int{},
		activeWindows:     map[string]int{},
		previewWindow:     -1,
		promptFolderIndex: -1,
		prompt:            t,
	}
	m.rebuildRows()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadSessionsCmd(), tickCmd())
}

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
		case "up", "k":
			if m.setSelected(m.selected - 1) {
				return m, m.syncSelectionPreview(true, true)
			}
			return m, nil
		case "down", "j":
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
		leftWidth := 32
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

	if m.overlayMode == overlayAgentPicker {
		content = lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center, m.renderAgentPickerOverlay())
	}

	headerSep := m.styles.divider.Render(strings.Repeat("─", m.width))
	return lipgloss.JoinVertical(lipgloss.Left, header, headerSep, content, footer)
}

func (m Model) renderHeader() string {
	left := m.styles.headerTitle.Render("grove")
	rightText := fmt.Sprintf("%d folders", len(m.cfg.Folders))
	if m.filterQuery != "" {
		rightText += " · filter: " + m.filterQuery
	}
	right := m.styles.headerMeta.Render(rightText)
	if m.width <= 0 {
		return left + "  " + right
	}
	maxRight := m.width - lipgloss.Width(left) - 1
	if maxRight < 1 {
		maxRight = 1
	}
	if lipgloss.Width(rightText) > maxRight {
		rightText = truncateRight(rightText, maxRight)
		right = m.styles.headerMeta.Render(rightText)
	}
	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		padding = 1
	}
	return left + strings.Repeat(" ", padding) + right
}

// ── Footer (merged help bar + status) ───────────────────────────────

func (m Model) renderFooter() string {
	if m.overlayMode == overlayAgentPicker {
		return m.styles.promptLabel.Render("agent picker") + m.styles.promptHint.Render("  ↑/↓ select · enter confirm · esc cancel")
	}

	// Prompt mode: show prompt input
	if m.promptMode != promptNone {
		label := m.styles.promptLabel.Render(m.promptTitle() + " ")
		enterHint := "enter confirm"
		if m.promptMode == promptAddFolder && m.promptStep < 2 {
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
	selectedRow, hasSelectedRow := m.selectedRow()
	if m.detailMode == detailPreview {
		zoomHint := "zoom in"
		if m.previewZoomed {
			zoomHint = "zoom out"
		}
		bindings = []binding{
			{"←/→", "window"},
			{"⏎", "attach"},
			{"z", zoomHint},
			{"r", "refresh"},
			{"esc", "back"},
			{"q", "back"},
		}
	} else if hasSelectedRow && selectedRow.typeOf == rowCommand {
		bindings = []binding{{"e", "editor"}, {"d", "dev command"}}
		if selectedRow.status == "running" {
			bindings = append(bindings,
				binding{"⏎", "attach"},
				binding{"v", "preview"},
				binding{"x", "stop"},
				binding{"R", "restart"},
			)
		} else {
			bindings = append(bindings,
				binding{"s", "start"},
				binding{"R", "restart"},
			)
		}
		if m.filterQuery != "" {
			bindings = append(bindings, binding{"esc", "clear filter"})
		}
		bindings = append(bindings,
			binding{"/", "filter"},
			binding{"r", "refresh"},
			binding{"q", "quit"},
		)
	} else if _, ok := m.selectedSessionRow(); ok {
		bindings = []binding{
			{"⏎", "attach"},
			{"v", "preview"},
			{"e", "editor"},
			{"n", "terminal"},
			{"a", "agent"},
			{"d", "dev command"},
			{"K", "kill"},
			{"c", "send cmd"},
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
			{"n", "new terminal"},
			{"a", "agent"},
			{"d", "dev command"},
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

	parts := make([]string, 0, len(bindings))
	lb := m.styles.helpBracket.Render("[")
	rb := m.styles.helpBracket.Render("]")
	for _, b := range bindings {
		parts = append(parts, lb+m.styles.helpKey.Render(b.key)+rb+" "+m.styles.helpDesc.Render(b.desc))
	}

	return strings.Join(parts, "  ")
}

func (m Model) selectedRow() (treeRow, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return treeRow{}, false
	}
	return m.rows[m.selected], true
}

// ── Tree Pane ───────────────────────────────────────────────────────

func (m *Model) rebuildRows() {
	selectedRow, hadSelection := treeRow{}, false
	if m.selected >= 0 && m.selected < len(m.rows) {
		selectedRow = m.rows[m.selected]
		hadSelection = true
	}

	sessionByName := make(map[string]tmux.Session)
	for _, folderSessions := range m.sessions {
		for _, session := range folderSessions {
			sessionByName[session.Name] = session
		}
	}

	rows := buildTreeRows(m.cfg, m.sessions, sessionByName)
	m.rows = filterTreeRows(rows, m.cfg, m.filterQuery)
	if hadSelection {
		if nextSelected, ok := findMatchingRowIndex(m.rows, selectedRow); ok {
			m.selected = nextSelected
		}
	}
	if m.selected >= len(m.rows) && len(m.rows) > 0 {
		m.selected = len(m.rows) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
	m.detailScroll = 0
}

func filterTreeRows(rows []treeRow, cfg config.Config, query string) []treeRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return rows
	}

	filtered := make([]treeRow, 0, len(rows))
	for i := 0; i < len(rows); {
		folderRow := rows[i]
		if folderRow.typeOf != rowFolder {
			i++
			continue
		}

		j := i + 1
		for j < len(rows) && rows[j].typeOf != rowFolder {
			j++
		}

		children := rows[i+1 : j]
		if treeRowMatchesFilter(folderRow, cfg, query) {
			filtered = append(filtered, folderRow)
			filtered = append(filtered, children...)
			i = j
			continue
		}

		matchedChildren := make([]treeRow, 0, len(children))
		for _, child := range children {
			if treeRowMatchesFilter(child, cfg, query) {
				matchedChildren = append(matchedChildren, child)
			}
		}
		if len(matchedChildren) > 0 {
			filtered = append(filtered, folderRow)
			filtered = append(filtered, matchedChildren...)
		}

		i = j
	}

	return filtered
}

func treeRowMatchesFilter(row treeRow, cfg config.Config, query string) bool {
	parts := []string{row.displayName, row.sessionName, row.commandText}
	if row.typeOf == rowFolder && row.folderIndex >= 0 && row.folderIndex < len(cfg.Folders) {
		folder := cfg.Folders[row.folderIndex]
		parts = append(parts, folder.Namespace, folder.Path)
	}

	for _, part := range parts {
		if strings.Contains(strings.ToLower(part), query) {
			return true
		}
	}
	return false
}

func isInstanceRowType(typeOf rowType) bool {
	return typeOf == rowAgentInstance || typeOf == rowTerminalInstance
}

func (m Model) renderAgentPickerOverlay() string {
	width := 48
	if m.width > 0 && m.width-6 < width {
		width = m.width - 6
	}
	if width < 24 {
		width = 24
	}

	lines := []string{m.styles.paneTitle.Render("Add Agent"), ""}
	for i, choice := range m.agentChoices {
		prefix := "  "
		if i == m.overlayIndex {
			prefix = m.styles.selAccent.Render("▎") + " "
		}
		label := choice.Label
		if choice.Persist {
			label += "  " + m.styles.detailMeta.Render("save to folder")
		}
		if choice.IsNew {
			label = m.styles.infoValue.Render(choice.Label)
		}
		lines = append(lines, prefix+label)
	}
	return m.styledPane(strings.Join(lines, "\n"), width, 0, false)
}

func findMatchingRowIndex(rows []treeRow, selected treeRow) (int, bool) {
	for i, row := range rows {
		switch selected.typeOf {
		case rowFolder:
			if row.typeOf == rowFolder && row.folderIndex == selected.folderIndex {
				return i, true
			}
		case rowSection:
			continue
		default:
			if row.sessionName != "" && row.sessionName == selected.sessionName {
				return i, true
			}
		}
	}
	return 0, false
}

type folderVisualStatus int

const (
	folderStatusIdle folderVisualStatus = iota
	folderStatusActive
	folderStatusAttention
)

func (m Model) folderSessionCount(folderIndex int) int {
	return len(m.sessions[folderIndex])
}

func (m Model) folderStatus(folderIndex int) folderVisualStatus {
	sessions := m.sessions[folderIndex]
	if len(sessions) == 0 {
		return folderStatusIdle
	}
	for _, session := range sessions {
		if session.HasAlerts || session.AlertsBell || session.AlertsActivity || session.AlertsSilence {
			return folderStatusAttention
		}
	}
	return folderStatusActive
}

func (m Model) folderCaret(folderIndex int) string {
	for _, row := range m.rows {
		if row.typeOf != rowFolder && row.folderIndex == folderIndex {
			return "▾"
		}
	}
	return "▸"
}

func (m Model) renderTreePane(innerH, maxWidth, paneWidth int, dim bool) string {
	if maxWidth < 10 {
		maxWidth = 10
	}

	bodyHeight := innerH
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Empty state
	if len(m.rows) == 0 {
		body := m.renderEmptyTree()
		padded := padToHeight(body, innerH)
		return m.styledPane(padded, paneWidth, innerH, dim)
	}

	// Compute exact dataRowBudget by walking rows from the selection
	// and summing visual weights until we exhaust bodyHeight lines.
	dataRowBudget := treeDataRowBudget(m.rows, m.selected, bodyHeight)

	// Reserve space for scroll indicators
	effectiveBodyH := bodyHeight
	needsScroll := len(m.rows) > dataRowBudget
	if needsScroll {
		effectiveBodyH-- // reserve for possible ↓ indicator
	}

	dataRowBudget = treeDataRowBudget(m.rows, m.selected, effectiveBodyH)

	rows := make([]string, 0, len(m.rows))
	start, end := windowAround(m.selected, len(m.rows), dataRowBudget)

	if start > 0 && needsScroll {
		effectiveBodyH-- // reserve for ↑ indicator
		dataRowBudget = treeDataRowBudget(m.rows, m.selected, effectiveBodyH)
		start, end = windowAround(m.selected, len(m.rows), dataRowBudget)
	}

	visualLines := 0
	actualEnd := end
	for i := start; i < end; i++ {
		row := m.rows[i]
		isKillTarget := m.confirmKillTarget != "" && row.sessionName == m.confirmKillTarget

		visualLines++
		if visualLines > effectiveBodyH {
			actualEnd = i
			break
		}

		plain := m.treeLineText(row, maxWidth)
		if dim {
			rows = append(rows, m.styles.rowDim.Render(plain))
		} else if isKillTarget {
			rows = append(rows, m.styles.rowKillTarget.Render(padRight(plain, maxWidth)))
		} else {
			rows = append(rows, m.treeLineStyled(row, plain, maxWidth))
		}

		if treeRowNeedsBreathingRoom(m.rows, i) {
			visualLines++
			if visualLines > effectiveBodyH {
				actualEnd = i + 1
				break
			}
			rows = append(rows, "")
		}
	}

	body := strings.Join(rows, "\n")
	if start > 0 {
		body = m.styles.headerMeta.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n" + body
	}
	if actualEnd < len(m.rows) {
		body += "\n" + m.styles.headerMeta.Render(fmt.Sprintf("  ↓ %d more", len(m.rows)-actualEnd))
	}

	padded := padToHeight(body, innerH)
	return m.styledPane(padded, paneWidth, innerH, dim)
}

const treeChildIndent = "    "

func (m Model) treeLineText(row treeRow, maxWidth int) string {
	switch row.typeOf {
	case rowFolder:
		left := fmt.Sprintf("%s ● %s", m.folderCaret(row.folderIndex), row.displayName)
		right := ""
		if count := m.folderSessionCount(row.folderIndex); count > 0 {
			right = fmt.Sprintf("%d", count)
		}
		return treeJustify(left, right, maxWidth)
	case rowAgentInstance:
		return treeJustify(treeChildIndent+"◆ "+row.displayName, "active", maxWidth)
	case rowTerminalInstance:
		return treeJustify(treeChildIndent+"○ "+row.displayName, "", maxWidth)
	case rowCommand:
		return treeJustify(treeChildIndent+"▸ "+row.displayName, "", maxWidth)
	default:
		return ""
	}
}

// treeJustify places left text on the left and right text flush-right,
// like CSS justify-between, truncating left if needed.
func treeJustify(left, right string, maxWidth int) string {
	if right == "" {
		return truncateRight(left, maxWidth)
	}
	gap := maxWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		left = truncateRight(left, maxWidth-lipgloss.Width(right)-1)
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) treeLineStyled(row treeRow, plain string, maxWidth int) string {
	switch row.typeOf {
	case rowFolder:
		count := ""
		if n := m.folderSessionCount(row.folderIndex); n > 0 {
			count = m.styles.rowFolderCount.Render(fmt.Sprintf("%d", n))
		}

		nameStyle := m.styles.rowFolder
		if m.folderStatus(row.folderIndex) == folderStatusIdle {
			nameStyle = m.styles.rowFolderIdle
		}
		if selected, ok := m.selectedRow(); ok && selected.typeOf == rowFolder && selected.folderIndex == row.folderIndex {
			nameStyle = m.styles.rowSelectedText
		}

		dot := m.styles.folderDotIdle.Render("●")
		switch m.folderStatus(row.folderIndex) {
		case folderStatusAttention:
			dot = m.styles.folderDotAttention.Render("●")
		case folderStatusActive:
			dot = m.styles.folderDotActive.Render("●")
		}

		left := m.styles.detailMeta.Render(m.folderCaret(row.folderIndex)) + " " + dot + " " + nameStyle.Render(row.displayName)
		leftPlain := fmt.Sprintf("%s ● %s", m.folderCaret(row.folderIndex), row.displayName)
		gap := maxWidth - lipgloss.Width(leftPlain) - lipgloss.Width(stripANSI(count))
		if gap < 1 {
			gap = 1
		}
		line := left + strings.Repeat(" ", gap) + count
		return line
	case rowAgentInstance:
		name := m.styles.rowSession.Render(row.displayName)
		if selected, ok := m.selectedRow(); ok && selected.sessionName == row.sessionName {
			name = m.styles.rowSelectedText.Render(row.displayName)
		}
		left := treeChildIndent + m.styles.childIconActive.Render("◆") + " " + name
		right := m.styles.badgeActive.Render("active")
		leftPlain := treeChildIndent + "◆ " + row.displayName
		gap := maxWidth - lipgloss.Width(leftPlain) - lipgloss.Width("active")
		if gap < 1 {
			gap = 1
		}
		return left + strings.Repeat(" ", gap) + right
	case rowTerminalInstance:
		name := m.styles.rowSession.Render(row.displayName)
		if selected, ok := m.selectedRow(); ok && selected.sessionName == row.sessionName {
			name = m.styles.rowSelectedText.Render(row.displayName)
		}
		return treeChildIndent + m.styles.childIconDim.Render("○") + " " + name
	case rowCommand:
		name := m.styles.rowSession.Render(row.displayName)
		if selected, ok := m.selectedRow(); ok && selected.sessionName == row.sessionName {
			name = m.styles.rowSelectedText.Render(row.displayName)
		}
		return treeChildIndent + m.styles.childIconDim.Render("▸") + " " + name
	default:
		return plain
	}
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
		return m.styledPane(padded, paneWidth, innerH, dim)
	}

	row := m.rows[m.selected]

	if m.detailMode == detailPreview || m.shouldAutoPreview() {
		return m.renderPreviewPane(innerH, maxWidth, paneWidth, dim)
	}

	return m.renderDetailLines(m.detailLinesForRow(row, maxWidth), innerH, paneWidth, dim)
}

func (m Model) detailLinesForRow(row treeRow, maxWidth int) []string {
	switch row.typeOf {
	case rowFolder:
		return m.folderDetailLines(row, maxWidth)
	case rowCommand:
		return m.commandDetailLines(row, maxWidth)
	case rowAgentInstance, rowTerminalInstance:
		return m.instanceDetailLines(row, maxWidth)
	default:
		return []string{m.styles.emptyHint.Render("select a folder or session")}
	}
}

func (m Model) folderDetailLines(row treeRow, maxWidth int) []string {
	folder := m.cfg.Folders[row.folderIndex]
	sessions := m.sessions[row.folderIndex]
	agents := len(buildAgentRows(row.folderIndex, folder, sessions))
	terminals := len(buildTerminalRows(row.folderIndex, folder, sessions))
	commands := len(folder.Commands)
	runningCommands := 0
	sessionByName := make(map[string]tmux.Session, len(sessions))
	for _, session := range sessions {
		sessionByName[session.Name] = session
	}
	for _, command := range folder.Commands {
		session, ok := sessionByName[commandSessionName(folder, command.Name)]
		if ok && commandSessionRunning(session) {
			runningCommands++
		}
	}

	const lw = 13
	lines := []string{
		m.styles.detailName.Render(folder.Name),
		m.styles.detailMeta.Render(fmt.Sprintf("%d agent%s · %d terminal%s · %d command%s", agents, pluralSuffix(agents), terminals, pluralSuffix(terminals), commands, pluralSuffix(commands))),
		m.dividerLine(maxWidth),
		"",
		m.styles.detailSectionHeader.Render("PATH"),
		m.styles.infoValue.Render(truncateMiddle(folder.Path, maxWidth)),
		"",
		m.dividerLine(maxWidth),
		"",
		m.styles.detailSectionHeader.Render("OVERVIEW"),
		m.kvPad("Agents", lw, m.styles.infoValue.Render(fmt.Sprintf("%d running", agents))),
		m.kvPad("Terminals", lw, m.styles.infoValue.Render(fmt.Sprintf("%d running", terminals))),
	}
	if commands > 0 {
		cmdSummary := fmt.Sprintf("%d configured", commands)
		if runningCommands > 0 {
			cmdSummary += fmt.Sprintf(", %d active", runningCommands)
		}
		lines = append(lines, m.kvPad("Commands", lw, m.styles.infoValue.Render(cmdSummary)))
	} else {
		lines = append(lines, m.kvPad("Commands", lw, m.styles.infoValue.Render("0 configured")))
	}

	// SESSIONS mini-table: list all sessions in this folder
	agentRows := buildAgentRows(row.folderIndex, folder, sessions)
	termRows := buildTerminalRows(row.folderIndex, folder, sessions)
	cmdRows := buildCommandRows(row.folderIndex, folder, sessionByName)
	allRows := make([]treeRow, 0, len(agentRows)+len(termRows)+len(cmdRows))
	allRows = append(allRows, agentRows...)
	allRows = append(allRows, termRows...)
	allRows = append(allRows, cmdRows...)

	if len(allRows) > 0 {
		lines = append(lines, "", m.dividerLine(maxWidth), "", m.styles.detailSectionHeader.Render("SESSIONS"))
		for _, r := range allRows {
			lines = append(lines, m.sessionSummaryLine(r, maxWidth))
		}
	} else if len(sessions) == 0 && commands == 0 {
		lines = append(lines, "", m.styles.emptyHint.Render("press n to create a terminal"))
	}
	return lines
}

func (m Model) commandDetailLines(row treeRow, maxWidth int) []string {
	const lw = 13
	statusText := m.styles.chipMuted.Render("stopped")
	if row.status == "running" {
		statusText = m.styles.chipPrimary.Render("running")
	}

	lines := []string{
		m.styles.detailName.Render(truncateRight(row.displayName, maxWidth)),
		statusText,
		m.dividerLine(maxWidth),
		"",
		m.styles.detailSectionHeader.Render("COMMAND"),
		m.kvPad("Command", lw, m.styles.infoValue.Render(truncateRight(row.commandText, maxWidth-lw))),
		m.kvPad("Session", lw, m.styles.detailMeta.Render(truncateRight(row.sessionName, maxWidth-lw))),
	}

	if row.status != "running" {
		return lines
	}

	lines = append(lines, "", m.dividerLine(maxWidth), "", m.styles.detailSectionHeader.Render("STATUS"))
	lines = append(lines, m.instanceDetailBodyLines(row, maxWidth)...)
	return lines
}

func (m Model) instanceDetailLines(row treeRow, maxWidth int) []string {
	statusText := m.styles.detailMeta.Render("○ detached")
	if row.attached {
		statusText = m.styles.chipPrimary.Render("● attached")
	}
	windowLabel := "windows"
	if row.windows == 1 {
		windowLabel = "window"
	}

	lines := []string{
		m.styles.detailName.Render(truncateRight(row.displayName, maxWidth)),
		statusText + m.styles.detailMeta.Render("  ·  "+fmt.Sprintf("%d %s", row.windows, windowLabel)),
		m.dividerLine(maxWidth),
		"",
		m.styles.detailSectionHeader.Render("STATUS"),
	}
	lines = append(lines, m.instanceDetailBodyLines(row, maxWidth)...)
	return lines
}

func (m Model) instanceDetailBodyLines(row treeRow, maxWidth int) []string {
	const lw = 13
	lines := make([]string, 0, 8)

	running := strings.TrimSpace(row.currentCommand)
	if running == "" || isShellCommand(running) {
		lines = append(lines, m.kvPad("Running", lw, m.styles.detailMeta.Render("shell idle")))
	} else {
		lines = append(lines, m.kvPad("Running", lw, m.styles.infoValue.Render(truncateRight(running, maxWidth-lw))))
	}

	if title := paneDisplayTitle(row); title != "" {
		lines = append(lines, m.kvPad("Title", lw, m.styles.infoValue.Render(truncateRight(title, maxWidth-lw))))
	}

	lines = append(lines, "")
	if row.lastActivity > 0 {
		d := time.Since(time.Unix(row.lastActivity, 0))
		lines = append(lines, m.kvPad("Active", lw, m.styles.infoValue.Render(formatDuration(d))))
	} else {
		lines = append(lines, m.kvPad("Active", lw, m.styles.detailMeta.Render("unknown")))
	}

	if row.hasAlerts || row.alertsBell || row.alertsActivity || row.alertsSilence {
		lines = append(lines, "", m.dividerLine(maxWidth), "", m.styles.detailSectionHeader.Render("ALERTS"))
		chips := make([]string, 0, 3)
		if row.alertsBell {
			chips = append(chips, m.styles.chipWarn.Render("! bell"))
		}
		if row.alertsActivity {
			chips = append(chips, m.styles.chipWarn.Render("# activity"))
		}
		if row.alertsSilence {
			chips = append(chips, m.styles.chipWarn.Render("~ silence"))
		}
		if len(chips) == 0 {
			chips = append(chips, m.styles.chipWarn.Render("alerts"))
		}
		lines = append(lines, strings.Join(chips, " "))
	}

	return lines
}

func (m Model) sessionSummaryLine(row treeRow, maxWidth int) string {
	var icon, state string
	switch row.typeOf {
	case rowAgentInstance, rowTerminalInstance:
		if row.attached {
			icon = m.styles.statusDotAttached.Render("●")
			state = "attached"
		} else {
			icon = m.styles.statusDotDetached.Render("○")
			state = "detached"
		}
	case rowCommand:
		if row.status == "running" {
			icon = m.styles.statusDotAttached.Render("▶")
			state = "running"
		} else {
			icon = m.styles.statusDotDetached.Render("■")
			state = "stopped"
		}
	}
	name := truncateRight(row.displayName, maxWidth-24)
	wc := ""
	if row.windows > 0 {
		wc = m.styles.windowCount.Render(fmt.Sprintf("%dw", row.windows))
	}
	return icon + " " + m.styles.infoValue.Render(name) + "  " + m.styles.detailMeta.Render(state) + "  " + wc
}

func (m Model) renderDetailLines(lines []string, innerH, paneWidth int, dim bool) string {
	if len(lines) == 0 {
		return m.styledPane("", paneWidth, innerH, dim)
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
	return m.styledPane(padded, paneWidth, innerH, dim)
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
	sessionName := m.previewSession
	if sessionName == "" {
		if row, ok := m.selectedSessionRow(); ok {
			sessionName = row.sessionName
		}
	}

	titleMeta := sessionName
	if sessionName != "" && m.previewWindow >= 0 {
		titleMeta = fmt.Sprintf("%s · win %d", sessionName, m.previewWindow)
		if windows := m.sessionWindows[sessionName]; len(windows) > 0 {
			if pos := indexOfInt(windows, m.previewWindow); pos >= 0 {
				titleMeta = fmt.Sprintf("%s · win %d (%d/%d)", sessionName, m.previewWindow, pos+1, len(windows))
			}
		}
	}

	title := m.styles.paneTitle.Render("Preview")
	if titleMeta != "" {
		metaWidth := maxWidth - 10
		if metaWidth < 10 {
			metaWidth = 10
		}
		title += " " + m.styles.detailMeta.Render(truncateRight(titleMeta, metaWidth))
	}

	if m.previewLoading {
		padded := padToHeight(title+"\n\n"+m.styles.emptyHint.Render("capturing pane…"), innerH)
		return m.styledPane(padded, paneWidth, innerH, dim)
	}
	if m.previewErr != nil {
		padded := padToHeight(title+"\n\n"+m.styles.footerErr.Render("error: "+m.previewErr.Error()), innerH)
		return m.styledPane(padded, paneWidth, innerH, dim)
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
			return sessionsLoadedMsg{
				sessions:       map[int][]tmux.Session{},
				sessionWindows: map[string][]int{},
				activeWindows:  map[string]int{},
				panesFresh:     true,
			}
		}

		sessions, err := m.client.ListSessions()
		if err != nil {
			return sessionsLoadedMsg{err: err}
		}

		// Fetch pane info for commands, titles, alert flags, and preview window navigation
		paneDataFresh := false
		sessionWindows := map[string][]int{}
		activeWindows := map[string]int{}
		panes, paneErr := m.client.ListPanes()
		if paneErr == nil {
			paneDataFresh = true
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
			sessionWindows = tmux.SessionWindowIndexes(panes)
			activeWindows = tmux.ActiveWindowIndexes(panes)
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

		return sessionsLoadedMsg{
			sessions:       grouped,
			sessionWindows: sessionWindows,
			activeWindows:  activeWindows,
			panesFresh:     paneDataFresh,
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
	case "up", "k":
		if m.overlayIndex > 0 {
			m.overlayIndex--
		}
		return m, nil
	case "down", "j":
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
			m.addFolderAgent(folderIndex, choice.Agent)
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

func (m *Model) addFolderAgent(folderIndex int, agent config.Agent) {
	key := sanitizeLeaf(agent.Name)
	for _, existing := range m.cfg.Folders[folderIndex].Agents {
		if sanitizeLeaf(existing.Name) == key {
			return
		}
	}
	m.cfg.Folders[folderIndex].Agents = append(m.cfg.Folders[folderIndex].Agents, agent)
}

func (m Model) folderHasCommandName(folderIndex int, commandName string) bool {
	key := sanitizeLeaf(commandName)
	for _, existing := range m.cfg.Folders[folderIndex].Commands {
		if sanitizeLeaf(existing.Name) == key {
			return true
		}
	}
	return false
}

func (m Model) selectedSessionRow() (treeRow, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return treeRow{}, false
	}
	row := m.rows[m.selected]
	switch row.typeOf {
	case rowAgentInstance, rowTerminalInstance:
		return row, true
	case rowCommand:
		if row.status == "running" {
			return row, true
		}
	}
	return treeRow{}, false
}

func (m Model) shouldAutoPreview() bool {
	if m.detailMode == detailPreview {
		return false
	}
	_, ok := m.selectedSessionRow()
	return ok
}

func (m Model) selectedKillableSessionRow() (treeRow, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return treeRow{}, false
	}
	row := m.rows[m.selected]
	if row.typeOf != rowAgentInstance && row.typeOf != rowTerminalInstance {
		return treeRow{}, false
	}
	return row, true
}

func (m Model) selectedCommandRow() (treeRow, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return treeRow{}, false
	}
	row := m.rows[m.selected]
	if row.typeOf != rowCommand {
		return treeRow{}, false
	}
	return row, true
}

func (m Model) selectedFolder() (config.Folder, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return config.Folder{}, false
	}
	row := m.rows[m.selected]
	if row.folderIndex < 0 || row.folderIndex >= len(m.cfg.Folders) {
		return config.Folder{}, false
	}
	return m.cfg.Folders[row.folderIndex], true
}

func (m *Model) syncSelectionPreview(force, showLoading bool) tea.Cmd {
	if m.detailMode == detailPreview {
		return nil
	}

	row, ok := m.selectedSessionRow()
	if !ok {
		m.clearSelectionPreview()
		return nil
	}

	prevSession := m.previewSession
	prevWindow := m.previewWindow

	m.previewSession = row.sessionName
	m.previewWindow = m.resolvePreviewWindow(row.sessionName, prevWindow)

	targetChanged := prevSession != m.previewSession || prevWindow != m.previewWindow
	if !force && !targetChanged && m.previewContent != "" && m.previewErr == nil {
		return nil
	}

	m.previewSeq++
	m.previewInFlight = false
	return m.beginPreviewCapture(showLoading)
}

func (m *Model) clearSelectionPreview() {
	m.previewSession = ""
	m.previewWindow = -1
	m.previewLoading = false
	m.previewErr = nil
	m.previewContent = ""
	m.previewInFlight = false
}

func (m Model) resolvePreviewWindow(sessionName string, current int) int {
	windows := m.sessionWindows[sessionName]
	if len(windows) == 0 {
		return -1
	}
	if indexOfInt(windows, current) >= 0 {
		return current
	}
	if active, ok := m.activeWindows[sessionName]; ok && indexOfInt(windows, active) >= 0 {
		return active
	}
	return windows[0]
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
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q":
		if m.previewZoomed {
			m.previewZoomed = false
			return m, nil
		}
		m.exitPreview()
		return m, m.syncSelectionPreview(true, true)
	case "left":
		return m, m.movePreviewWindow(-1)
	case "right":
		return m, m.movePreviewWindow(1)
	case "z":
		m.previewZoomed = !m.previewZoomed
		m.detailScroll = 0
		return m, nil
	case "r":
		return m, m.beginPreviewCapture(true)
	case "enter":
		target := m.previewSession
		if target == "" {
			row, ok := m.selectedSessionRow()
			if !ok {
				return m, nil
			}
			target = row.sessionName
		}
		m.exitPreview()
		m.statusMsg = "attached to " + target + " (detach with Ctrl-b d)"
		m.errMsg = ""
		return m, tea.ExecProcess(m.client.AttachCommand(target), func(err error) tea.Msg {
			return attachedMsg{err: err}
		})
	}
	return m, nil
}

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
				agent := config.Agent{Name: m.pendingAgent.Name, Command: value}
				m.addFolderAgent(folderIndex, agent)
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
				if m.folderHasCommandName(folderIndex, value) {
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
				command := config.Command{Name: m.pendingCommand.Name, Command: value}
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

func (m Model) styledPane(content string, paneWidth, innerH int, dim bool) string {
	// paneWidth is total outer width including border+padding.
	// lipgloss Width includes padding but excludes border.
	// border = 2 (left+right), so Width = paneWidth - 2.
	// With Padding(0,1), the actual text area = Width - 2 = paneWidth - 4.
	w := paneWidth - 2
	if w < 1 {
		w = 1
	}
	style := m.styles.pane
	if dim {
		style = m.styles.paneDim
	}
	style = style.Width(w)
	if innerH > 0 {
		style = style.Height(innerH)
	}
	return style.Render(content)
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

func indexOfInt(values []int, needle int) int {
	for i, v := range values {
		if v == needle {
			return i
		}
	}
	return -1
}

func (m *Model) setSelected(next int) bool {
	if len(m.rows) == 0 {
		m.selected = 0
		m.detailScroll = 0
		return false
	}
	if next < 0 {
		next = 0
	}
	if next >= len(m.rows) {
		next = len(m.rows) - 1
	}
	changed := next != m.selected
	if next != m.selected {
		m.detailScroll = 0
	}
	m.selected = next
	return changed
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

func treeRowVisualWeight(rows []treeRow, i int, isFirstVisible bool) int {
	if treeRowNeedsBreathingRoom(rows, i) {
		return 2
	}
	return 1
}

func treeRowNeedsBreathingRoom(rows []treeRow, i int) bool {
	if i < 0 || i >= len(rows)-1 {
		return false
	}
	if rows[i].typeOf == rowFolder {
		return false
	}
	return rows[i+1].typeOf == rowFolder
}

// treeDataRowBudget computes how many data rows fit in maxVisual lines
// by expanding outward from the selection, matching windowAround's behavior.
func treeDataRowBudget(rows []treeRow, selected, maxVisual int) int {
	if len(rows) == 0 || maxVisual <= 0 {
		return 0
	}

	// Simulate: start by placing selected in the middle, expand outward.
	// This mirrors what windowAround will produce.
	count := 0
	visual := 0

	// First, walk forward from a rough start position
	// Use a simple approach: try increasing budgets until visual weight exceeds maxVisual
	for budget := 1; budget <= len(rows); budget++ {
		start, end := windowAround(selected, len(rows), budget)
		visual = 0
		for i := start; i < end; i++ {
			visual += treeRowVisualWeight(rows, i, i == start)
		}
		if visual > maxVisual {
			// budget-1 was the last that fit
			count = budget - 1
			if count < 1 {
				count = 1
			}
			return count
		}
	}
	// Everything fits
	return len(rows)
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

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func (m Model) kv(label, value string) string {
	return m.styles.infoLabel.Render(label+": ") + m.styles.infoValue.Render(value)
}

func (m Model) dividerLine(width int) string {
	if width < 1 {
		width = 1
	}
	return m.styles.divider.Render(strings.Repeat("─", width))
}

func (m Model) kvPad(label string, padTo int, value string) string {
	pad := padTo - len(label)
	if pad < 1 {
		pad = 1
	}
	return m.styles.infoLabel.Render(label+strings.Repeat(" ", pad)) + value
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

func stripANSI(s string) string {
	return ansi.Strip(s)
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
	if folderIndex < 0 || folderIndex >= len(cfg.Folders) {
		return func() tea.Msg {
			return commandAddedMsg{err: fmt.Errorf("select a folder")}
		}
	}
	cfg.Folders = append([]config.Folder(nil), cfg.Folders...)
	commands := append([]config.Command(nil), cfg.Folders[folderIndex].Commands...)
	cfg.Folders[folderIndex].Commands = append(commands, command)
	cfgPath := m.cfgPath
	return func() tea.Msg {
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

func (m *Model) startPreview() tea.Cmd {
	row, ok := m.selectedSessionRow()
	if !ok {
		m.exitPreview()
		return nil
	}
	m.previewSession = row.sessionName
	m.previewWindow = -1
	if windows := m.sessionWindows[row.sessionName]; len(windows) > 0 {
		if active, ok := m.activeWindows[row.sessionName]; ok && indexOfInt(windows, active) >= 0 {
			m.previewWindow = active
		} else {
			m.previewWindow = windows[0]
		}
	}
	m.previewSeq++
	m.previewInFlight = false
	m.previewLoading = false
	m.previewErr = nil
	m.previewContent = ""
	m.detailScroll = 0
	m.previewZoomed = false
	return m.beginPreviewCapture(true)
}

func (m *Model) reconcilePreviewAfterLoad() tea.Cmd {
	if m.previewSession == "" {
		m.exitPreview()
		return nil
	}
	if !m.sessionExists(m.previewSession) {
		m.exitPreview()
		return m.setStatus("preview closed: session ended")
	}

	windows := m.sessionWindows[m.previewSession]
	if len(windows) == 0 {
		if m.previewWindow == -1 {
			return nil
		}
		m.previewWindow = -1
		m.previewSeq++
		return m.beginPreviewCapture(true)
	}

	if indexOfInt(windows, m.previewWindow) >= 0 {
		return nil
	}

	if m.previewWindow < 0 {
		if active, ok := m.activeWindows[m.previewSession]; ok && indexOfInt(windows, active) >= 0 {
			m.previewWindow = active
		} else {
			m.previewWindow = windows[0]
		}
	} else {
		replacement := windows[len(windows)-1]
		for _, win := range windows {
			if win >= m.previewWindow {
				replacement = win
				break
			}
		}
		m.previewWindow = replacement
	}

	m.previewSeq++
	return m.beginPreviewCapture(true)
}

func (m *Model) movePreviewWindow(delta int) tea.Cmd {
	if delta == 0 || m.previewSession == "" {
		return nil
	}
	windows := m.sessionWindows[m.previewSession]
	if len(windows) <= 1 {
		return nil
	}

	cur := indexOfInt(windows, m.previewWindow)
	if cur < 0 {
		if active, ok := m.activeWindows[m.previewSession]; ok {
			cur = indexOfInt(windows, active)
		}
		if cur < 0 {
			cur = 0
		}
	}

	next := cur + delta
	n := len(windows)
	next %= n
	if next < 0 {
		next += n
	}

	newWindow := windows[next]
	if newWindow == m.previewWindow {
		return nil
	}

	m.previewWindow = newWindow
	m.previewSeq++
	m.detailScroll = 0
	return m.beginPreviewCapture(true)
}

func (m *Model) beginPreviewCapture(showLoading bool) tea.Cmd {
	target := m.previewCaptureTarget()
	if target == "" {
		return nil
	}
	m.previewInFlight = true
	if showLoading {
		m.previewLoading = true
		m.previewErr = nil
		m.previewContent = ""
	}
	return m.capturePaneCmd(target, m.previewSeq)
}

func (m Model) previewCaptureTarget() string {
	if m.previewSession == "" {
		return ""
	}
	if m.previewWindow >= 0 {
		return fmt.Sprintf("%s:%d", m.previewSession, m.previewWindow)
	}
	return m.previewSession
}

func (m *Model) exitPreview() {
	m.detailMode = detailNormal
	m.previewSession = ""
	m.previewWindow = -1
	m.previewLoading = false
	m.previewErr = nil
	m.previewContent = ""
	m.previewZoomed = false
	m.previewInFlight = false
}

func (m Model) sessionExists(name string) bool {
	for _, sessions := range m.sessions {
		for _, session := range sessions {
			if session.Name == name {
				return true
			}
		}
	}
	return false
}

func (m Model) capturePaneCmd(target string, seq int) tea.Cmd {
	return func() tea.Msg {
		content, err := m.client.CapturePane(target)
		return paneCapturedMsg{target: target, content: content, err: err, seq: seq}
	}
}
