# grove

A terminal UI for managing agent sessions, built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Grove lets you organize tmux sessions under named folders defined in a simple TOML config. Create, attach, rename, kill, and send commands to sessions — all from a two-pane TUI with a tree view and details panel.

## Features

- Organize tmux sessions under named folders with slug-based namespaces
- Two-pane layout: tree view (left) + session details (right)
- Create new sessions that auto-attach, with optional default commands
- Attach to existing sessions and return to the TUI on detach
- Rename, kill, and send commands to sessions
- Keyboard-driven filter to quickly find folders and sessions
- Auto-refreshes tmux state every 2 seconds
- Auto-scaffolds a config template on first run

## Requirements

- [Go](https://go.dev/) 1.23+
- [tmux](https://github.com/tmux/tmux) in your `PATH`

## Install

```bash
go install github.com/SarthakJariwala/grove/cmd/grove@latest
```

## Configuration

Grove reads its config from `~/.config/grove/config.toml` by default. On first run, it creates a template config if none exists. Use `-config` to specify a custom path.

Each `[[folder]]` entry defines a project folder with a name, path, and optional default command to run when creating new sessions:

```toml
[[folder]]
name = "Main API"
path = "/Users/you/dev/main-api"
default_command = "bin/dev"

[[folder]]
name = "Website"
path = "/Users/you/dev/website"
```

| Field             | Required | Description                                      |
|-------------------|----------|--------------------------------------------------|
| `name`            | yes      | Display name for the folder (also used to generate the session namespace) |
| `path`            | yes      | Absolute path to the project directory           |
| `default_command` | no       | Command to run automatically in new sessions     |

Folder names are slugified into namespaces (e.g. "Main API" becomes `main-api`). Sessions are named `<namespace>/<session-name>` so they stay grouped in tmux.

## Usage

Run grove directly with `go run .` or run the compiled binary:

    ./grove
    ./grove -config /path/to/config.toml

## Keybindings

| Key              | Action                                                  |
|------------------|---------------------------------------------------------|
| `↑` / `k`       | Move up                                                 |
| `↓` / `j`       | Move down                                               |
| `Enter`          | Attach to selected session                              |
| `n`              | Create new session under the selected folder            |
| `R`              | Rename selected session                                 |
| `K`              | Kill selected session                                   |
| `c`              | Send command to selected session                        |
| `/`              | Filter folders and sessions                             |
| `Esc`            | Clear filter                                            |
| `PgUp` / `PgDn` | Scroll the details pane                                 |
| `r`              | Manual refresh                                          |
| `q`              | Quit                                                    |
| `y`              | Confirm kill (when prompted)                            |
| `n` / `Esc`      | Cancel kill (when prompted)                             |

## Development

```bash
go build -o grove .   # build binary
go vet ./...          # lint
go test ./...         # run all tests
gofmt -w .            # format
```

## Architecture

    main.go                  Entrypoint: flag parsing, config loading, Bubble Tea program
    internal/
      config/config.go       TOML config parsing, slug-based namespaces, template scaffolding
      tmux/client.go         Wraps tmux CLI (list/new/kill/rename/attach sessions)
      ui/model.go            Bubble Tea model: two-pane layout, prompt modes, polling refresh

## License

[MIT](LICENSE)
