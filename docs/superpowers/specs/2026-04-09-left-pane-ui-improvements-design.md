# Left Pane UI Improvements — Design Spec

**Date:** 2026-04-09  
**Status:** Approved

## Overview

Three targeted improvements to the sessions tree pane (`internal/ui/model.go`):

1. Visual hierarchy: section labels become clearly subordinate to folder headers
2. Scroll fix: `↑/↓ more` indicators no longer overflow the pane height budget
3. Pane width: fixed-width left pane instead of percentage-based

---

## 1. Visual Hierarchy

### Problem

Section labels (`Agents`, `Terminals`, `Commands`) currently use `colorPrimaryDim` (`#3b8070`), a dim green from the same color family as folder headers. They read as peers of folders rather than children of them. Item indent is 4 spaces, which doesn't clearly distinguish items from sections.

### Changes

**Color:** Section labels move to `colorTextMuted` (`#484f58`) — the very dim gray used for borders and dividers. This makes sections feel like structural labels, not navigable entries.

**Prefix:** Section labels gain a `·` prefix: `· Agents`, `· Terminals`, `· Commands`. Keeps indent short while adding visual subordination.

**Separator:** A thin horizontal rule is drawn on the line immediately after each folder header row. Rendered as `─────` in `colorTextMuted` at the full inner pane width. This visually closes the folder header and opens the section list below it.

**Item indent:** Terminal/agent/command items increase from 4 to 6 spaces to sit clearly under the section label (which is at 2-space indent with the `·` prefix).

### Affected code

- `defaultStyles()`: change `detailSection` color to `colorTextMuted`
- `treeLineText()` / `treeLineStyled()`: update `rowSection` formatting to `"  · " + name`; update item indentation from `"    "` to `"      "` (6 spaces)
- `renderTreePane()`: after appending the blank line before each folder row, append an additional separator line (a string of `─` repeated to `maxWidth`) styled with `colorTextMuted`

---

## 2. Scroll Cut-Off Fix

### Problem

`renderTreePane` allocates `bodyHeight = innerH - 1` rows for content (subtracting 1 for the title). It then appends `↑ N more` and/or `↓ N more` scroll indicators *after* filling up to `bodyHeight` rows. When both indicators are present, the total content is `title(1) + ↑(1) + body(bodyHeight) + ↓(1)` = `innerH + 2` lines, which overflows the pane and causes the last visible item to be clipped.

### Fix

Before the row-filling loop, compute how many lines the scroll indicators will consume:

- `start` is known before the loop (from `windowAround`). If `start > 0`, subtract 1 from `effectiveBodyH`.
- Whether a `↓ more` indicator will appear is not known until after the loop. Pre-emptively subtract 1 more from `effectiveBodyH` whenever `len(m.rows) > bodyHeight` (i.e., the list is longer than one screenful and scrolling is possible at all).

This reserves space for both indicators whenever the list could overflow, at the cost of at most 1–2 blank lines at the very top or bottom of an unscrolled list (acceptable trade-off).

### Affected code

- `renderTreePane()`: derive `effectiveBodyH` from `bodyHeight` before the filling loop; use `effectiveBodyH` as the cap in the loop and for `windowAround`.

---

## 3. Pane Width Stability

### Problem

Left pane width is computed as `(terminalWidth * 30) / 100`, clamped to `[30, 50]`. Because integer division rounds differently at different terminal widths, small terminal resizes can shift the left pane by 1–2 characters, making it feel unstable. The right pane gets the remainder so it also shifts.

### Fix

Use a fixed left pane width of **32 characters**. The right pane gets `terminalWidth - 32 - 1` (the `1` is the spacer between panes). The right pane grows/shrinks freely on resize; the sessions tree stays locked at 32 chars.

32 chars is wide enough for typical session names (e.g. `grove-20260409-120208` = 21 chars + 6-char indent + status indicator) and matches roughly what the 30% floor produces on a typical 100-char terminal.

The `leftWidth < 30` and `leftWidth > 50` guards are removed. A single guard ensures `rightWidth >= 20` to prevent degenerate narrow terminals from breaking the layout.

### Affected code

- `View()`: replace percentage calculation with `leftWidth := 32`; keep `rightWidth = m.width - leftWidth - 1` with a `rightWidth >= 20` floor guard.

---

## Non-Goals

- Collapsing empty sections
- Changing folder count display format
- Any changes to the right detail pane or footer
