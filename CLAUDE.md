# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o grove .       # build binary
go run .                  # run directly
go run . -config /path/to/config.toml  # custom config path
```

## Test & Lint

```bash
go test ./...             # run all tests
go test ./internal/config # single package
go vet ./...              # lint
gofmt -w .               # format
```

No tests exist yet — add `_test.go` files next to source when writing tests.

## Architecture

Grove is a TUI tmux session manager built with Go 1.23 and Bubble Tea.

- `main.go` — entrypoint: parses `-config` flag, loads TOML config, creates tmux client, runs Bubble Tea program with alt screen
- `internal/config/` — TOML config parsing (`Config`, `Folder`), slug-based namespace generation, template scaffolding at `~/.config/grove/config.toml`
- `internal/tmux/` — `Client` wraps tmux CLI commands (list/new/kill/rename/attach sessions) via `os/exec`
- `internal/ui/` — Bubble Tea `Model` with two-pane layout (tree + details), prompt modes for session operations, 2-second polling refresh

The UI uses a flat `[]treeRow` list mixing folder headers and session entries. Sessions are grouped under folders by matching the `<namespace>/` prefix on tmux session names. Filtering, scrolling, and prompt input all operate on this row list.

## Code Style

- Imports: stdlib first, blank line, third-party, blank line, internal (`grove/internal/...`)
- Alias `bubbletea` as `tea`; use `lipgloss` for styling
- Error handling: wrap with `fmt.Errorf("context: %w", err)`, return early
- Naming: unexported helpers (e.g. `slug`, `sanitizeLeaf`); exported constructors (`NewClient`, `NewModel`)
- Use `iota` const blocks for enums (e.g. `rowType`, `promptMode`)
- No linter config beyond `go vet` and `gofmt`

## Config

Default config path: `~/.config/grove/config.toml`. Each `[[folder]]` entry requires `name` and `path`; `default_command` is optional. Folder names are slugified into namespaces used as tmux session name prefixes.

## Runtime Requirement

Requires `tmux` in `PATH`.
