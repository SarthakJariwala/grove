# Left Pane Status-First Minimal Redesign

**Date:** 2026-04-13
**Status:** Approved

## Goal

Redesign Grove's left sidebar into a denser, status-first tree that makes active folders scannable at a glance, visually deemphasizes idle folders, and removes section-header noise from expanded folders.

This is primarily a visual pass on the existing tree interaction model, with one deliberate behavior simplification: folder-level actions replace section-level actions where possible.

## Current State

The left pane currently renders a tree with these row kinds:

- folder row
- optional `Agents` section row
- agent instance rows
- optional `Terminals` section row
- terminal instance rows
- optional `Commands` section row
- command rows

The current presentation has a few problems:

- folder activity requires reading right-aligned numeric counts instead of scanning a status mark
- idle folders have nearly the same visual weight as active folders
- expanded folders use extra vertical space because every child group introduces a labeled section row
- selection is visible, but not strong enough to dominate adjacent rows
- the `Agents` section row currently exists partly to provide an action target for adding agents

## Approved Decisions

### Tree Structure

The left pane tree keeps folder expansion and inline children, but section rows are removed from the tree model and from rendering.

Expanded folders render direct children only, in this order:

1. running agent rows
2. running terminal rows
3. configured command rows

There are no visible `Agents`, `Terminals`, or `Commands` header rows in the left pane.

This keeps the tree structure intact while making the expanded view lighter and more direct.

### Folder-Level Actions

`add agent` moves from the `Agents` section selection model to the folder row.

- any selected folder can trigger `add agent`
- the help bar should advertise that action whenever a folder row is selected
- no left-pane action should require selecting a now-removed section row

This is the only intentional interaction-model simplification in scope for the redesign.

### Header Row

The left pane header becomes a compact single line:

```text
grove                                    9 folders
```

- left side: app/root name in standard title styling
- right side: folder count in dimmed secondary styling
- below the header: one subtle separator line using the dim border color

The current aggregate session count shown in the header is removed from this pane.

### Expand/Collapse Affordance

Folders keep a visible expand/collapse indicator in addition to the status dot.

- collapsed folders show a small caret such as `▸`
- expanded folders show the corresponding open caret such as `▾`
- the caret stays visually secondary to the status dot
- the status dot remains the primary activity indicator

## Visual Design

### Folder Rows

Each folder row renders four visual elements in order:

```text
[caret] [dot] [name]                      [count]
```

#### Status Dot

The dot is a Unicode `●` followed by one space.

Folder status is derived from the highest-priority child session state in this order:

1. `attention`
2. `active`
3. `idle`

Status rules:

- `attention`: folder has at least one child row with tmux alert/activity flags set
- `active`: folder has at least one running agent, running terminal, or running command, but no higher-priority attention state
- `idle`: folder has no live sessions in the folder namespace

For the initial implementation, `attention` is derived from the tmux alert flags already available on session rows (`HasAlerts`, `AlertsBell`, `AlertsActivity`, `AlertsSilence`) rather than from a new unread-output model.

Color mapping:

- active dot: `#a6e3a1`
- attention dot: `#f9e2af`
- idle dot: `#45475a`

Name color mapping:

- active or attention folder name: default foreground
- idle folder name: `#6c7086`

Session count rules:

- count remains right-aligned
- count is shown only when greater than zero
- idle folders omit `0` entirely
- the count is never truncated; folder names truncate first

#### Selection Styling

The selected row, whether folder or child, uses the same strong highlight treatment:

- background: `#1e3a5f` or closest equivalent already supported by the theme
- selected folder name: accent blue `#89b4fa`
- selected child text: same selected-row treatment used consistently across row types
- status dot color does not change when selected

### Expanded Child Rows

Expanded children render directly under the folder without spacer lines or section labels.

Base layout:

```text
  ◆ Pi #1                                active
  ○ Terminal #1
  ▸ dev
```

Rules:

- indent children by two spaces from the folder content column
- truncate child display names when needed to preserve right-side badges
- keep one visual row per child, with no blank lines between siblings

Type icons and colors:

- agent: `◆`
- terminal: `○`
- command: `▸`

Child icon styling:

- running agent icon uses green `#a6e3a1`
- stopped or inactive agent icon uses dimmed gray `#6c7086`
- terminal icon uses dimmed secondary color `#6c7086`
- command icon uses dimmed secondary color `#6c7086`

Agent badge rules:

- only running agents show a right-aligned `active` badge
- badge uses green foreground `#a6e3a1`
- no background fill or pill treatment
- stopped agents show no badge

Terminal and command rows do not gain new text badges in this redesign.

### Spacing

The current extra vertical rhythm in the tree is removed:

- no blank lines before folder rows
- no separator rule after each folder row
- no blank lines between section groupings because section rows no longer exist

The pane becomes a compact list optimized for scannability.

## Behavior and Data Rules

### Expansion Model

Folder expansion and collapse behavior stays the same.

- folders remain the only top-level expandable rows
- expanding reveals child rows inline beneath the folder
- collapsing hides them

The implementation may keep or restyle an expand/collapse affordance, but the status dot is the primary leading visual indicator.

### Folder Count Semantics

Folder counts continue to reflect live tmux sessions in that folder namespace.

That count includes:

- running agent sessions
- running terminal sessions
- managed command sessions that currently exist

Configured-but-stopped commands do not contribute to the count because they are not live tmux sessions.

### Attention Semantics

`attention` should be visually distinct from `active`, but it does not introduce new persisted state.

For this redesign, attention is derived from existing tmux session flags and applies when any child session indicates recent activity, bell, or silence alerts. This is an acceptable approximation of "recent output or waiting input" until Grove grows explicit unread-state tracking.

## Implementation Outline

### Tree Model

Update `internal/ui/tree_rows.go` so `buildTreeRows` emits only:

- `rowFolder`
- `rowAgentInstance`
- `rowTerminalInstance`
- `rowCommand`

`rowSection` remains unnecessary for the left pane and should be removed from left-pane tree construction.

Folder visual state should be computed from the rows that belong to that folder rather than hard-coded from the session count alone.

### Rendering

Update `internal/ui/model.go` to:

- render the compact header with folder count only
- render status dots and folder counts with separate styling control
- render direct child icons and optional agent `active` badge
- omit idle counts
- use a blue-tinted selected-row background for every selected row type
- remove the existing blank-line and folder-separator rendering logic from the tree pane

Truncation calculations must account for:

- status dot plus spacing
- icon plus indentation on child rows
- right-aligned count or badge width

### Selection and Actions

Update selection helpers and key handling in `internal/ui/model.go` so:

- folder-specific actions no longer depend on `rowSection`
- `add agent` is available from any selected folder
- row matching and retained selection work correctly after section rows disappear

### Detail Pane Compatibility

The right pane is out of scope visually, but selection behavior must remain coherent once section rows are removed.

That means any detail or help-bar behavior that was previously section-specific must either:

- move to the folder row, or
- become unnecessary because there is no selectable section row anymore

No new right-pane redesign is required.

## Testing

Add or update focused tests before implementation in:

- `internal/ui/tree_rows_test.go`
- `internal/ui/model_test.go`

Tests should cover:

- tree rows no longer include section rows
- folder rows remain present for empty folders
- command rows still appear for configured-but-stopped commands
- folder selection supports `add agent`
- folder rendering omits idle `0` counts
- folder rendering shows attention/active/idle visual states correctly
- child rows render direct icons without section headers
- selected child rows and selected folder rows both use the shared highlight treatment

## Non-Goals

- changes to the right pane layout or copy beyond compatibility updates caused by removed section rows
- new persisted unread-output state
- regrouping folders by status
- new metadata in the tree such as git branch or path
- keyboard shortcut changes outside moving `add agent` to folder selection
