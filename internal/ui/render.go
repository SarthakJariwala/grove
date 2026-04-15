package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/SarthakJariwala/grove/internal/tmux"
)

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

func commandTreeIcon(row treeRow) string {
	if row.status == "running" {
		return "▶"
	}
	return "■"
}

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
		return treeJustify(treeChildIndent+commandTreeIcon(row)+" "+row.displayName, "", maxWidth)
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
		icon := m.styles.childIconDim.Render(commandTreeIcon(row))
		if row.status == "running" {
			icon = m.styles.childIconActive.Render(commandTreeIcon(row))
		}
		return treeChildIndent + icon + " " + name
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
