# AGENTS.md — Grove

## Build & Run
- Build: `go build -o grove ./cmd/grove/`
- Run: `go run ./cmd/grove/`
- Test all: `go test ./...`
- Test single package: `go test ./internal/config`
- Lint: `go vet ./...`

## Architecture
Grove is a TUI tmux session manager built with Go 1.23 and Bubble Tea (charmbracelet).
- `cmd/grove/main.go` — entrypoint: parses flags, wires dependencies, runs Bubble Tea program
- `internal/config/` — pure types (`Config`, `Folder`), slug generation, normalization/validation
- `internal/configfile/` — TOML config file I/O (`Load`, `EnsureTemplate`, `AppendFolder`)
- `internal/tmux/` — `SessionManager` interface + `Client` implementation wrapping tmux CLI
- `internal/ui/` — Bubble Tea `Model` with tree view of folders→sessions, prompt modes, filtering

## Code Style
- Standard Go conventions; no linter config — use `go vet` and `gofmt`
- Imports: stdlib first, blank line, third-party, blank line, internal (`grove/internal/...`)
- Alias `bubbletea` as `tea`; use `lipgloss` for styling
- Error handling: wrap with `fmt.Errorf("context: %w", err)`, return early on errors
- Naming: unexported helpers (e.g. `slug`, `sanitizeLeaf`); exported constructors (`NewClient`, `NewModel`)
- Types: use `iota` const blocks for enums (e.g. `rowType`, `promptMode`)
- No tests exist yet — add `_test.go` files next to source when writing tests
