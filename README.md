# grove

> Scale agents, not stress. Calm terminal energy for chaotic agent workflows.

A calm terminal UI for managing agent sessions.

Grove lets you organize agent sessions under named folders. Create, preview, attach, rename, kill, and send commands to sessions — all from a two-pane TUI with a tree view and details panel.

## Features

- Organize agent sessions under named folders
- Two-pane layout: tree view (left) + session details and preview (right)
- Create new sessions that auto-attach, with optional default commands
- Preview, rename, kill, and send commands to sessions
- Keyboard-driven filter to quickly find folders and sessions

## Install

```bash
brew tap SarthakJariwala/grove
brew install grove
```

## Usage

```bash
grove

# OR
grove -config /path/to/config.toml
```

## Configuration

Just run `grove` after install and it will walk you through its configuration.

## Keybindings

| Key              | Action                                                  |
|------------------|---------------------------------------------------------|
| `↑` / `k`       | Move up                                                  |
| `↓` / `j`       | Move down                                                |
| `Enter`          | Attach to selected session                              |
| `v`              | Preview selected session (live)                         |
| `z`              | Zoom in/out preview pane (in preview mode)              |
| `n`              | Create new session under the selected folder            |
| `R`              | Rename selected session                                 |
| `K`              | Kill selected session                                   |
| `c`              | Send command to selected session                        |
| `/`              | Filter folders and sessions                             |
| `Esc`            | Clear filter                                            |
| `PgUp` / `PgDn` | Scroll the details pane                                  |
| `e`              | Launch editor in the selected folder or session         |
| `r`              | Manual refresh                                          |
| `q`              | Quit                                                    |
| `y`              | Confirm kill (when prompted)                            |
| `n` / `Esc`      | Cancel kill (when prompted)                             |

## License

[MIT](LICENSE)
