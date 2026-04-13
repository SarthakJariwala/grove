# Session Selection Preview — Design Spec

**Date:** 2026-04-13  
**Status:** Approved

## Overview

When the selected row in the left pane is a running tmux session, the right pane should render that session's pane preview instead of the current operational detail summary. Folder rows should continue to render the existing detail view unchanged. The existing explicit preview mode remains available for interactive controls such as window cycling and zoom.

---

## Behavior

### Folder selection

Folder rows keep the current detail pane output:
- folder name and path
- section counts and overview
- sessions summary list

### Session selection

Running session rows should automatically render the preview pane in the right pane:
- agent rows
- terminal rows
- running command rows

Stopped command rows are not previewable and should continue using the existing command detail view.

### Explicit preview mode

The `v` shortcut still enters the existing explicit preview mode. That mode remains responsible for:
- left/right window navigation
- zoom toggle
- preview-only help bar
- attach-from-preview behavior

Automatic preview on selection does not change keyboard routing. Normal navigation and actions should continue working while a session row is selected.

---

## Implementation Notes

### Preview rendering

`renderDetailPane()` should choose between:
- folder/command detail lines for non-session rows
- preview rendering for previewable selected session rows
- explicit preview mode when `detailMode == detailPreview`

### Preview state synchronization

Preview capture state should be synchronized from the selected row while the model is in normal mode:
- selecting a running session initializes preview target state and starts a pane capture
- selecting a folder or stopped command clears the automatic preview state
- session refreshes should re-capture the selected session without forcing the explicit preview mode

### Stale capture protection

Captured pane messages should still be ignored unless they match the current preview target and sequence. This is required because selection can move before a tmux capture returns.

---

## Testing

Add focused tests for:
- rendering preview content in the detail pane for selected running sessions in normal mode
- triggering automatic preview capture when selection moves onto a running session
- accepting captured pane content for automatic preview without explicit preview mode

## Non-Goals

- changing folder detail content
- changing the layout or styling of the preview pane
- changing explicit preview controls or help copy beyond what is required for correctness
