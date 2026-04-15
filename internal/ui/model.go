package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/SarthakJariwala/grove/internal/config"
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
	client  sessionManager
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

func NewModel(cfg config.Config, cfgPath string, client sessionManager) Model {
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
