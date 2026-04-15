package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

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
