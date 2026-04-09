# grove

> Scale agents, not stress. Calm terminal energy for chaotic agent workflows.

A calm terminal UI for managing agent sessions.

Grove lets you organize workspaces under named folders with dedicated `Agents`, `Terminals`, and `Commands` sections. Launch agent instances, keep plain terminals lightweight, and manage long-running commands from a two-pane TUI with a tree view and details panel.

## Features

- Organize workspaces into folders with `Agents`, `Terminals`, and `Commands`
- Two-pane layout: tree view (left) + session details and preview (right)
- Launch multiple agent instances from configured templates
- Start, stop, restart, preview, and attach to managed command sessions
- Keep plain terminal sessions runtime-only and lightweight
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
| `Enter`          | Attach to selected running session                       |
| `v`              | Preview selected running session                         |
| `←` / `→`       | Cycle session windows (in preview mode)                 |
| `z`              | Zoom in/out preview pane (in preview mode)              |
| `n`              | Create a new terminal in the selected folder             |
| `a`              | Add or launch an agent in the selected folder            |
| `C`              | Add a managed command to the selected folder             |
| `s`              | Start the selected stopped command                       |
| `x`              | Stop the selected running command                        |
| `R`              | Restart the selected command                             |
| `c`              | Send a command to the selected running session           |
| `K`              | Kill the selected running terminal or agent              |
| `/`              | Filter folders and rows                                  |
| `Esc`            | Clear filter                                            |
| `PgUp` / `PgDn` | Scroll the details pane                                  |
| `e`              | Open the selected folder or session path in the editor   |
| `r`              | Manual refresh                                           |
| `q`              | Quit                                                     |
| `y`              | Confirm kill (when prompted)                            |
| `n` / `Esc`      | Cancel kill (when prompted)                             |

## License

[MIT](LICENSE)
