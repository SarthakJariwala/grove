# Left Pane UI Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix three issues in the sessions tree pane: muted/indented section labels with folder separators, scroll-indicator overflow clipping the last visible row, and unstable pane width.

**Architecture:** All changes are confined to `internal/ui/model.go`. Task 1 changes the `View()` pane-width formula. Task 2 changes style definition and two rendering functions. Task 3 restructures the `renderTreePane` render loop to add folder separators and pre-reserve space for scroll indicators.

**Tech Stack:** Go 1.23, Bubble Tea, Lipgloss

---

### Task 1: Fix pane width stability

**Files:**
- Modify: `internal/ui/model.go:607-618`

Fix the left pane width from a percentage calculation (which shifts with terminal width) to a fixed 32 characters. The right pane gets everything else.

- [ ] **Step 1: Replace percentage calculation with fixed width**

In `View()`, find this block (around line 607):

```go
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
```

Replace with:

```go
} else if m.width > 70 {
    leftWidth := 32
    rightWidth := m.width - leftWidth - 1
    if rightWidth < 20 {
        rightWidth = 20
    }
```

- [ ] **Step 2: Build and verify it compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/model.go
git commit -m "fix: use fixed left pane width of 32 chars"
```

---

### Task 2: Fix section label styling and item indentation

**Files:**
- Modify: `internal/ui/model.go` — `defaultStyles()`, `treeLineText()`, `treeLineStyled()`
- Test: `internal/ui/model_test.go`

Section labels move from bold green (`colorPrimaryDim`) to very dim gray (`colorTextMuted`) with a `·` prefix. Terminal/agent/command items increase indent from 4 to 6 spaces.

- [ ] **Step 1: Write failing tests**

Add these tests to `internal/ui/model_test.go`:

```go
func TestTreeLineSectionPrefix(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{}, "", fakeSessionManager{})
	row := treeRow{typeOf: rowSection, displayName: "Agents"}
	got := m.treeLineText(row, 40)
	const wantPrefix = "  · "
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("treeLineText(section) = %q, want prefix %q", got, wantPrefix)
	}
}

func TestTreeLineItemIndent(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{}, "", fakeSessionManager{})
	const wantIndent = "      " // 6 spaces

	terminal := treeRow{typeOf: rowTerminalInstance, displayName: "my-session"}
	if got := m.treeLineText(terminal, 40); !strings.HasPrefix(got, wantIndent) {
		t.Fatalf("treeLineText(terminal) = %q, want 6-space indent", got)
	}

	agent := treeRow{typeOf: rowAgentInstance, displayName: "Codex #1"}
	if got := m.treeLineText(agent, 40); !strings.HasPrefix(got, wantIndent) {
		t.Fatalf("treeLineText(agent) = %q, want 6-space indent", got)
	}

	cmd := treeRow{typeOf: rowCommand, displayName: "start", status: "stopped"}
	if got := m.treeLineText(cmd, 40); !strings.HasPrefix(got, wantIndent) {
		t.Fatalf("treeLineText(command) = %q, want 6-space indent", got)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/ui/ -run "TestTreeLineSection|TestTreeLineItem" -v
```

Expected: FAIL — section line starts with `"  "` not `"  · "`, items start with `"    "` not `"      "`.

- [ ] **Step 3: Change section label style in `defaultStyles()`**

Find (around line 237):

```go
detailSection: lipgloss.NewStyle().Foreground(lipgloss.Color(colorPrimaryDim)).Bold(true),
```

Replace with:

```go
detailSection: lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextMuted)),
```

- [ ] **Step 4: Update `treeLineText()` — section prefix and item indent**

Find the `treeLineText` function (around line 954) and replace its body:

```go
func (m Model) treeLineText(row treeRow, maxWidth int) string {
	switch row.typeOf {
	case rowFolder:
		count := len(m.sessions[row.folderIndex])
		return truncateRight(fmt.Sprintf("▸ %s (%d)", row.displayName, count), maxWidth)
	case rowSection:
		return truncateRight("  · "+row.displayName, maxWidth)
	case rowCommand:
		text := fmt.Sprintf("      %s · %s", row.displayName, row.status)
		if row.status == "running" && row.windows > 0 {
			text += fmt.Sprintf(" (%dw)", row.windows)
		}
		return truncateRight(text, maxWidth)
	default:
		dot := "○"
		if row.attached {
			dot = "●"
		}
		text := fmt.Sprintf("      %s %s", dot, row.displayName)
		if row.windows > 0 {
			text += fmt.Sprintf(" (%dw)", row.windows)
		}
		if alertStr := alertIndicatorStr(row); alertStr != "" {
			text += " " + alertStr
		}
		return truncateRight(text, maxWidth)
	}
}
```

- [ ] **Step 5: Update `treeLineStyled()` — item indent**

Find the `treeLineStyled` function (around line 983) and replace its body:

```go
func (m Model) treeLineStyled(row treeRow, plain string, maxWidth int) string {
	switch row.typeOf {
	case rowFolder:
		return " " + m.styles.rowFolder.Render(plain)
	case rowSection:
		return " " + m.styles.detailSection.Render(plain)
	case rowCommand:
		status := m.styles.windowCount.Render(row.status)
		base := "      " + m.styles.rowSession.Render(truncateRight(row.displayName, maxWidth-14)) + " · " + status
		if row.status == "running" && row.windows > 0 {
			base += " " + m.styles.windowCount.Render(fmt.Sprintf("(%dw)", row.windows))
		}
		return base
	default:
		dot := m.styles.statusDotDetached.Render("○")
		if row.attached {
			dot = m.styles.statusDotAttached.Render("●")
		}
		line := "      " + dot + " " + m.styles.rowSession.Render(row.displayName)
		if row.windows > 0 {
			line += " " + m.styles.windowCount.Render(fmt.Sprintf("(%dw)", row.windows))
		}
		if alertStr := alertIndicatorStr(row); alertStr != "" {
			line += " " + m.styles.alertIndicator.Render(alertStr)
		}
		return line
	}
}
```

Note: `maxWidth-14` for commands (was `maxWidth-12`) accounts for the 2 extra indent spaces added.

- [ ] **Step 6: Run tests to confirm they pass**

```bash
go test ./internal/ui/ -run "TestTreeLineSection|TestTreeLineItem" -v
```

Expected: PASS.

- [ ] **Step 7: Run full test suite**

```bash
go test ./internal/ui/ -v
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: mute section label color, add dot prefix, increase item indent"
```

---

### Task 3: Fix scroll cut-off and add folder separator

**Files:**
- Modify: `internal/ui/model.go` — `renderTreePane()`
- Test: `internal/ui/model_test.go`

Two changes to `renderTreePane`:
1. Pre-reserve height for `↑/↓ more` scroll indicators so they don't overflow the pane.
2. Render a `─────` separator line immediately after each folder header to visually close the folder row.

- [ ] **Step 1: Write failing test for scroll overflow**

Add to `internal/ui/model_test.go`:

```go
func TestRenderTreePaneScrollNoOverflow(t *testing.T) {
	t.Parallel()

	// Build model with 10 folders — enough rows to require scrolling
	folders := make([]config.Folder, 10)
	for i := range folders {
		folders[i] = config.Folder{
			Name: fmt.Sprintf("folder-%d", i),
			Path: "/tmp",
		}
	}
	cfg := config.Config{Folders: folders}
	m := NewModel(cfg, "", fakeSessionManager{})
	// Simulate loaded sessions so rebuildRows produces rows
	m.sessions = map[int][]tmux.Session{}
	m.rebuildRows()

	const innerH = 20
	rendered := m.renderTreePane(innerH, 28, 32, false)

	// Strip ANSI codes, count lines. The pane border adds 2 lines (top + bottom),
	// so total lines must be ≤ innerH + 2.
	stripped := ansi.Strip(rendered)
	lineCount := strings.Count(stripped, "\n") + 1
	if lineCount > innerH+2 {
		t.Fatalf("renderTreePane output has %d lines (max %d): %q", lineCount, innerH+2, stripped)
	}
}
```

Also add `"fmt"` to the import block in `model_test.go` if not already present, and `"github.com/charmbracelet/x/ansi"`.

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/ui/ -run TestRenderTreePaneScrollNoOverflow -v
```

Expected: FAIL — line count exceeds `innerH + 2` when both scroll indicators are rendered.

- [ ] **Step 3: Rewrite `renderTreePane` with effective body height and folder separator**

Replace the entire `renderTreePane` function body (lines 886–952) with:

```go
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

	// Reserve height for scroll indicators when the row list exceeds one screenful.
	// Each indicator (↑ / ↓) consumes one visual line; pre-subtract both when
	// scrolling is possible so they never overflow the pane.
	effectiveBodyH := bodyHeight
	if len(m.rows) > bodyHeight {
		effectiveBodyH = bodyHeight - 2
		if effectiveBodyH < 3 {
			effectiveBodyH = 3
		}
	}

	rows := make([]string, 0, len(m.rows))
	start, end := windowAround(m.selected, len(m.rows), effectiveBodyH)

	visualLines := 0
	actualEnd := end
	for i := start; i < end; i++ {
		row := m.rows[i]
		isSelected := i == m.selected
		isKillTarget := m.confirmKillTarget != "" && row.sessionName == m.confirmKillTarget

		if row.typeOf == rowFolder {
			// blank spacer before folder
			visualLines++
			if visualLines > effectiveBodyH {
				actualEnd = i
				break
			}
			rows = append(rows, "")

			// folder name line
			visualLines++
			if visualLines > effectiveBodyH {
				actualEnd = i
				break
			}
			plain := m.treeLineText(row, maxWidth)
			if dim {
				rows = append(rows, m.styles.rowDim.Render(plain))
			} else if isSelected {
				rows = append(rows, m.selectedLine("▎"+plain, maxWidth))
			} else {
				rows = append(rows, m.treeLineStyled(row, plain, maxWidth))
			}

			// separator line under folder header
			visualLines++
			if visualLines > effectiveBodyH {
				actualEnd = i
				break
			}
			rows = append(rows, m.dividerLine(maxWidth))
			continue
		}

		// section labels and session items
		visualLines++
		if visualLines > effectiveBodyH {
			actualEnd = i
			break
		}
		plain := m.treeLineText(row, maxWidth)
		if dim {
			rows = append(rows, m.styles.rowDim.Render(plain))
		} else if isSelected {
			rows = append(rows, m.selectedLine("▎"+plain, maxWidth))
		} else if isKillTarget {
			rows = append(rows, m.styles.rowKillTarget.Render(padRight(plain, maxWidth)))
		} else {
			rows = append(rows, m.treeLineStyled(row, plain, maxWidth))
		}
	}

	body := strings.Join(rows, "\n")
	if start > 0 {
		body = m.styles.headerMeta.Render(fmt.Sprintf("  ↑ %d more", start)) + "\n" + body
	}
	if actualEnd < len(m.rows) {
		body += "\n" + m.styles.headerMeta.Render(fmt.Sprintf("  ↓ %d more", len(m.rows)-actualEnd))
	}

	title := m.styles.paneTitle.Render("Sessions")
	padded := padToHeight(title+"\n"+body, innerH)
	return m.styledPane(padded, paneWidth, dim)
}
```

- [ ] **Step 4: Add missing import `"github.com/charmbracelet/x/ansi"` to `model_test.go` if needed**

Check the import block in `model_test.go`. If `"github.com/charmbracelet/x/ansi"` is absent, add it to the third-party import group:

```go
import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)
```

- [ ] **Step 5: Run scroll overflow test to confirm it passes**

```bash
go test ./internal/ui/ -run TestRenderTreePaneScrollNoOverflow -v
```

Expected: PASS.

- [ ] **Step 6: Run full test suite**

```bash
go test ./internal/ui/ -v
```

Expected: all pass.

- [ ] **Step 7: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: fix scroll overflow and add folder separator line"
```

---

## Self-Review

**Spec coverage check:**
- ✅ Section labels → `colorTextMuted`, no bold — Task 2, Step 3
- ✅ `·` prefix on section labels — Task 2, Steps 4–5
- ✅ Separator `─────` under folder header — Task 3, Step 3
- ✅ Item indent 4→6 spaces — Task 2, Steps 4–5
- ✅ Scroll fix: pre-reserve `effectiveBodyH` — Task 3, Step 3
- ✅ Fixed 32-char left pane width — Task 1, Step 1

**Placeholder scan:** No TBD, TODO, or vague steps. All code blocks are complete.

**Type consistency:**
- `effectiveBodyH` introduced and used within same function — no cross-task references
- `treeLineText` / `treeLineStyled` / `renderTreePane` / `defaultStyles` all in same file — no interface changes
- `dividerLine` already exists in `model.go` at line 1981 — reused, not redefined

**Edge cases noted:**
- `effectiveBodyH` floored at 3 to prevent degenerate tiny panes
- `maxWidth-14` in `treeLineStyled` for commands: was `maxWidth-12`; adjusted for 2 extra indent chars so the displayName truncation still works correctly
- Folder rows no longer go through the generic rendering block (now handled inside `if rowFolder` with `continue`) — kill-target highlight for folders was never possible (you can't kill a folder), so removing `isKillTarget` from the folder branch is correct
