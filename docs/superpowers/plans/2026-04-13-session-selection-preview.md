# Session Selection Preview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Auto-render the tmux pane preview in the right pane whenever a running session row is selected, while preserving the existing folder detail view and explicit preview mode.

**Architecture:** Keep explicit preview mode intact, but treat running session selection in normal mode as a preview-rendering state. Reuse the existing preview capture fields in `internal/ui/model.go`, add a selection-to-preview synchronization helper, and refresh automatic preview state from selection and session reload events.

**Tech Stack:** Go 1.23, Bubble Tea, Lip Gloss, tmux client abstractions, Go test

---

### Task 1: Lock Down Automatic Preview Rendering

**Files:**
- Modify: `internal/ui/model_test.go`
- Test: `internal/ui/model_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestRenderDetailPaneSessionShowsPreviewInNormalMode(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{Name: "SQL", Path: "/tmp/sql", Namespace: "sql"}}}
	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.sessions = map[int][]tmux.Session{
		0: {{Name: "sql/sdk", Windows: 2, CurrentCommand: "node"}},
	}
	m.previewSession = "sql/sdk"
	m.previewWindow = 1
	m.previewContent = "top line\npreview output"
	m.rebuildRows()
	m.setSelected(1)

	got := m.renderDetailPane(20, 80, 84, false)

	if !strings.Contains(got, "Preview") || !strings.Contains(got, "preview output") {
		t.Fatalf("detail pane = %q, want preview content", got)
	}
	if strings.Contains(got, "Running") {
		t.Fatalf("detail pane = %q, should not show operational detail body", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui -run TestRenderDetailPaneSessionShowsPreviewInNormalMode`
Expected: FAIL because `renderDetailPane()` still renders the session detail view in normal mode.

- [ ] **Step 3: Write minimal implementation**

```go
func (m Model) renderDetailPane(innerH, maxWidth, paneWidth int, dim bool) string {
	// ...existing guards...

	if m.detailMode == detailPreview || m.shouldAutoPreview() {
		return m.renderPreviewPane(innerH, maxWidth, paneWidth, dim)
	}

	return m.renderDetailLines(m.detailLinesForRow(row, maxWidth), innerH, paneWidth, dim)
}
```

```go
func (m Model) shouldAutoPreview() bool {
	if m.detailMode == detailPreview {
		return false
	}
	_, ok := m.selectedSessionRow()
	return ok
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ui -run TestRenderDetailPaneSessionShowsPreviewInNormalMode`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/model_test.go
git commit -m "feat: auto-show session preview in detail pane"
```

### Task 2: Synchronize Automatic Preview Capture With Selection

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/model_update_test.go`
- Test: `internal/ui/model_update_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func TestUpdateNavigationStartsAutoPreviewForSelectedSession(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/one", CurrentCommand: "bash"}}}
	m.sessionWindows = map[string][]int{"api/one": {0, 2}}
	m.activeWindows = map[string]int{"api/one": 2}
	m.rebuildRows()

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := model.(Model)

	if got.detailMode != detailNormal {
		t.Fatalf("detailMode = %v, want detailNormal", got.detailMode)
	}
	if got.previewSession != "api/one" || got.previewWindow != 2 {
		t.Fatalf("preview target = %q:%d, want api/one:2", got.previewSession, got.previewWindow)
	}
	if cmd == nil {
		t.Fatal("expected capture command")
	}
	_ = cmd()
	if len(fake.captured) != 1 || fake.captured[0] != "api/one:2" {
		t.Fatalf("captured targets = %#v, want [api/one:2]", fake.captured)
	}
}
```

```go
func TestPaneCapturedMsgUpdatesAutoPreviewOutsideExplicitMode(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", &trackingSessionManager{})
	m.sessions = map[int][]tmux.Session{0: {{Name: "api/one", CurrentCommand: "bash"}}}
	m.rebuildRows()
	m.setSelected(1)
	m.previewSession = "api/one"
	m.previewSeq = 3

	updated, _ := m.Update(paneCapturedMsg{target: "api/one", content: "preview", seq: 3})
	got := updated.(Model)

	if got.previewContent != "preview" {
		t.Fatalf("previewContent = %q, want preview", got.previewContent)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/ui -run 'TestUpdateNavigationStartsAutoPreviewForSelectedSession|TestPaneCapturedMsgUpdatesAutoPreviewOutsideExplicitMode'`
Expected: FAIL because selection changes do not currently start preview capture and pane capture messages are ignored outside explicit preview mode.

- [ ] **Step 3: Write minimal implementation**

```go
func (m *Model) syncSelectionPreview(force, showLoading bool) tea.Cmd {
	if m.detailMode == detailPreview {
		return nil
	}
	row, ok := m.selectedSessionRow()
	if !ok {
		m.clearSelectionPreview()
		return nil
	}

	targetChanged := m.setPreviewTarget(row.sessionName)
	if !force && !targetChanged && m.previewContent != "" && m.previewErr == nil {
		return nil
	}
	m.previewSeq++
	m.previewInFlight = false
	return m.beginPreviewCapture(showLoading)
}
```

```go
case paneCapturedMsg:
	if msg.seq != m.previewSeq || msg.target != m.previewCaptureTarget() {
		return m, nil
	}
	// apply content for explicit and automatic preview
```

```go
case "up", "k":
	if m.setSelected(m.selected - 1) {
		return m, m.syncSelectionPreview(true, true)
	}
	return m, nil
```

```go
case sessionsLoadedMsg:
	// ...existing reload work...
	if m.detailMode == detailPreview {
		return m, m.reconcilePreviewAfterLoad()
	}
	return m, m.syncSelectionPreview(true, false)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/ui -run 'TestUpdateNavigationStartsAutoPreviewForSelectedSession|TestPaneCapturedMsgUpdatesAutoPreviewOutsideExplicitMode'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/model_update_test.go
git commit -m "feat: sync session selection preview"
```

### Task 3: Verify Full UI Behavior

**Files:**
- Modify: `internal/ui/model.go` (only if verification exposes a missed edge case)
- Test: `internal/ui/model_test.go`
- Test: `internal/ui/model_update_test.go`

- [ ] **Step 1: Run focused UI tests**

Run: `go test ./internal/ui`
Expected: PASS with the new automatic preview coverage included.

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`
Expected: PASS for all packages.

- [ ] **Step 3: Commit verification-only follow-up if needed**

```bash
git add internal/ui/model.go internal/ui/model_test.go internal/ui/model_update_test.go
git commit -m "test: cover session selection preview behavior"
```
