# Folder Sections Design

**Date:** 2026-04-09

## Goal

Evolve Grove from a flat `folder -> tmux sessions` manager into a folder-centric workspace UI with three explicit sections under every folder: `Agents`, `Terminals`, and `Commands`.

The new design must keep Grove lightweight and tmux-first while making agent launches, plain terminals, and managed long-running commands feel like distinct concepts in both configuration and the TUI.

## Current State

Today Grove models only folders and tmux sessions.

- Config supports global `editor_command` and per-folder `name`, `path`, `default_command`, and `editor_command`.
- The UI tree renders folder rows plus matching tmux session rows.
- Grove discovers managed sessions by folder namespace prefix only.
- New sessions are generic tmux terminals.
- Config persistence supports appending folders, but not editing nested folder configuration.

This is too limited for the next workflow because it does not distinguish:

- configured agent templates vs running agent instances
- generic terminal sessions vs managed commands
- running commands vs stopped configured commands

## Product Decisions

The following decisions are in scope for this design and are considered approved.

### Folder Structure

Every folder renders three sections in this fixed order:

1. `Agents`
2. `Terminals`
3. `Commands`

The tree is organized by folder first, then by section.

### Agents

Agents have both global and folder-local configuration.

- Global agents mean "available to add in any folder".
- Folder-local agents mean "this folder knows about this template".
- Global agents do not appear in the tree by themselves.
- Folder-local agents do not appear in the tree by themselves.
- The `Agents` section shows only running agent instances.
- Users can launch multiple instances of the same agent template.
- When the user adds a global agent template to a folder at runtime, Grove persists that template into the folder's config.
- When the user creates a brand new agent at runtime, Grove persists it only to the folder, not globally.

### Terminals

Terminals remain runtime-only tmux sessions.

- They are not persisted to config.
- They behave similarly to Grove's current session creation flow.
- The `Terminals` section shows only running terminal sessions.

### Commands

Commands are configured per folder only.

- Every configured command always appears in the `Commands` section.
- Commands are single-instance per folder.
- On Grove startup, commands should appear as `stopped` unless Grove detects that their managed tmux session is currently running the configured command.
- A command row should be shown as `stopped` when the command has exited, even if tmux is still sitting at an interactive shell.
- Command `stop` kills the managed tmux session entirely.
- Command `start` recreates the tmux session from scratch.
- Command `restart` kills any existing managed tmux session and recreates it.

### Persistence

Runtime additions should persist automatically.

- Added agents and commands are written back to `config.toml` immediately.
- Terminals are never persisted.
- `default_command` can be removed from the config model because it is no longer needed and is not used.

### Session Naming

Grove fully manages tmux naming for the new item types.

- Agent instance sessions use `<folder namespace>/agent-<agent slug>-<n>`.
- Command sessions use `<folder namespace>/cmd-<command slug>`.
- Terminal sessions use `<folder namespace>/term-<n>`.

These names are internal identifiers first. The UI can render cleaner display labels derived from config names and instance indexes.

### Legacy Sessions

Existing unmatched sessions using older Grove naming should remain visible after upgrade.

- If a session matches the folder namespace but not the new managed `agent-`, `term-`, or `cmd-` patterns, Grove should classify it as a terminal session for display purposes.

## Configuration Model

### Top-Level Config

Add global agent templates:

```toml
editor_command = "code ."

[[agent]]
name = "Codex"
command = "codex"

[[agent]]
name = "Amp"
command = "amp"
```

### Per-Folder Config

Each folder keeps its own local agents and commands:

```toml
[[folder]]
name = "django-ocean"
path = "/Users/you/dev/django-ocean"

  [[folder.agent]]
  name = "Codex"
  command = "codex"

  [[folder.agent]]
  name = "Pi"
  command = "pi"

  [[folder.command]]
  name = "start"
  command = "make start"

  [[folder.command]]
  name = "worker"
  command = "make worker"
```

### Go Types

Add small shared config types:

- `Agent { Name string, Command string }`
- `Command { Name string, Command string }`

Update existing types:

- `Config` gains `Agents []Agent`
- `Folder` gains `Agents []Agent` and `Commands []Command`
- `Folder.DefaultCommand` is removed

Normalization rules should trim names and commands, reject empty required fields, and compute folder namespaces as today.

### Derived Runtime Model

The tree should no longer be derived directly from `[]tmux.Session` alone. Grove should compose a richer per-folder view model from:

1. persisted folder config
2. global agent templates
3. live tmux sessions
4. live tmux pane metadata

Each folder view derives:

- running agent instances
- running terminal instances
- configured command rows with running or stopped state
- optional legacy unmatched sessions, displayed under terminals

## UI Tree Model

Replace the current flat folder/session row model with explicit typed rows.

Recommended row kinds:

- `rowFolder`
- `rowSection`
- `rowAgentInstance`
- `rowTerminalInstance`
- `rowCommand`

Each row should carry enough metadata to support rendering, keybindings, and lifecycle actions without relying on string parsing at the call site.

Suggested row fields:

- `folderIndex`
- `sectionKind`
- `sessionName`
- `displayName`
- `templateName`
- `commandText`
- `instanceIndex`
- `running`
- `attached`
- `currentPath`
- alert and preview metadata already supported today

Tree layout per folder:

```text
Folder Name
  Agents
    Codex #1
    Amp #1
  Terminals
    Terminal #1
    legacy-session
  Commands
    start      running
    worker     stopped
```

Sections should remain visible even when empty so the folder structure is stable and learnable.

## Session Classification

For each tmux session inside a folder namespace, Grove should classify it as one of:

- managed agent instance
- managed terminal instance
- managed command instance
- unmatched legacy session

Classification should be based on managed naming conventions rather than shell command heuristics alone.

Recommended parsing rules:

- `agent-<slug>-<n>` => agent instance
- `cmd-<slug>` => command instance
- `term-...` => terminal instance
- anything else inside the folder namespace => legacy terminal instance

Command status also depends on pane state:

- `running` only if the managed command session exists and the configured command is still the active running command in that pane
- `stopped` if the session does not exist or if tmux has returned to an idle shell

## Interaction Design

### Navigation

Existing tree navigation remains:

- `j` / `k` and arrows move selection
- `/` filters
- `r` refreshes
- `e` opens editor in the folder or selected row path when relevant

### Terminals

- `n` creates a new runtime-only terminal in the selected folder
- Newly created terminals should preserve Grove's current behavior and auto-attach immediately after creation
- `Enter` attaches to a running terminal
- `v` previews a running terminal
- `c` sends a command to a running terminal
- `K` kills a running terminal session

### Agents

- `a` opens the add-agent flow for the selected folder or `Agents` section
- `Enter` attaches to a running agent instance
- `v` previews a running agent instance
- `c` sends a command to a running agent instance when useful
- `K` kills a running agent session

Add-agent flow:

1. User selects a folder row or `Agents` section
2. User presses `a`
3. Grove shows options for:
   - folder-local agent templates
   - global agent templates not yet added to the folder
   - `Add new agent...`
4. If the user picks a global template, Grove persists it into the folder config
5. Grove creates a new agent instance using the next available numeric suffix
6. Grove starts the configured command inside the new tmux session
7. Grove auto-attaches to the newly created agent session

If the user chooses `Add new agent...`, Grove prompts for:

1. agent name
2. launch command

Then Grove persists the template into the folder and creates the first instance immediately.

### Commands

Commands are controlled from visible command rows.

- `Enter` only attaches when the command is currently running
- `s` starts a stopped command
- `x` stops a running command by killing its tmux session
- `R` restarts the command by killing and recreating its tmux session
- `v` previews only when running
- `c` is only available when running

Starting or restarting a command should not auto-attach; the command starts in the background and `Enter` remains the explicit attach action.

Optional add-command flow:

- `C` from a folder row or `Commands` section prompts for command name and command text
- Grove persists it to the selected folder
- The new row appears immediately as `stopped`

### Details Pane

The details pane should become row-type aware.

- Folder rows summarize counts of running agents, terminals, and configured commands
- Section rows explain the section purpose and available actions
- Running agent and terminal rows show current status, command, activity, alerts, and preview metadata similar to today's session detail view
- Command rows show configured command text plus lifecycle status (`running` or `stopped`)

### Keybinding Summary

Recommended user-facing bindings:

- `n` new terminal
- `a` add agent
- `C` add command
- `s` start command
- `x` stop command
- `R` restart command
- `Enter` attach to running instance or running command
- `v` preview running instance or running command
- `K` kill running agent or terminal
- `c` send command to running tmux-backed rows

The help bar should change by selected row type.

## Tmux Execution Model

Grove should continue using tmux as the execution layer.

Desired behaviors:

- Agent creation creates a session in the folder path and launches the configured agent command
- Terminal creation creates a plain tmux session in the folder path
- Command start creates a fresh tmux session in the folder path and launches the configured command
- Command stop kills the managed command session
- Command restart kills then recreates the managed command session

The tmux client may need one additional primitive if `new session` followed by `send-keys` proves too race-prone for reliable initial command launch.

## Config Persistence

Current config persistence only appends whole folders. That is insufficient for this feature.

Grove needs write support for nested per-folder updates:

- append a `folder.agent`
- append a `folder.command`
- possibly rewrite the full config document to preserve correctness and avoid brittle string appends

The preferred persistence strategy is:

1. load the full config into memory
2. mutate the relevant folder/global structures
3. rewrite the full TOML file in a stable format
4. reload the normalized config in memory

Rewriting the full file is safer than trying to surgically append nested blocks into the original file.

## Migration And Compatibility

This release should preserve visibility of older sessions while shifting config to the new model.

- Existing folders remain valid after dropping `default_command`
- Existing running sessions remain visible through namespace matching
- Old unmanaged session names are surfaced under `Terminals`
- Existing session rename behavior should be revisited because freeform renaming conflicts with Grove-managed naming

The simplest rule for the first version is:

- disallow rename for managed agent, terminal, and command rows
- continue allowing legacy session rows to be attached, previewed, and killed

## Non-Goals

The following are explicitly out of scope for this change:

- CPU or memory display in the tree
- global commands
- persisted terminals
- graceful command supervision beyond tmux session recreation
- process trees or child-process management outside tmux
- background daemonization or restart-on-crash policies

## Risks

- `internal/ui/model.go` is already large, so the typed row refactor must avoid turning rendering and behavior into a stringly-typed patchwork
- reliable command-status detection will be subtle when a tmux pane falls back to an interactive shell after the command exits
- config rewriting must preserve correctness and avoid corrupting user config files
- managed naming and next-instance numbering must avoid collisions with deleted or still-running sessions

## Implementation Slices

Recommended implementation order:

1. Extend config types and rewrite-based config persistence
2. Add managed naming and session classification helpers
3. Refactor tree rows and detail rendering around explicit row kinds
4. Add runtime terminal behavior under the new `Terminals` section
5. Add agent template persistence and instance launch flow
6. Add command rows and command lifecycle actions
7. Update docs and compatibility handling for legacy sessions

## Success Criteria

This feature is successful when:

- every folder shows `Agents`, `Terminals`, and `Commands`
- users can create multiple agent instances from configured templates
- users can add agents and commands at runtime and have them persist automatically
- terminals remain quick, runtime-only tmux sessions
- commands are always visible and correctly show `running` or `stopped`
- command lifecycle actions behave deterministically through tmux session recreation
- existing legacy sessions remain visible after upgrade
