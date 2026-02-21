# grove

`grove` is a terminal UI for managing tmux sessions grouped under configured folders.

## Features

- Read folder config from TOML.
- Show configured folders and matching managed sessions (`<folder-namespace>/<session-name>`).
- Create new sessions and immediately attach.
- Optionally run a folder default command automatically in the new session.
- Attach to a session with full tmux terminal experience and return to the TUI on detach.
- Rename, kill, and send commands to sessions.
- Left tree pane and right details pane for the current selection.
- Filter folders/sessions from the keyboard.
- Poll tmux every 2 seconds to keep status fresh.

## Install

```bash
go build ./...
```

## Config

By default the app reads `~/.config/grove/config.toml`.

If the file does not exist, grove creates a template at startup.

Example:

```toml
[[folder]]
name = "Main API"
path = "/Users/you/dev/main-api"
default_command = "bin/dev"

[[folder]]
name = "Website"
path = "/Users/you/dev/website"
```

## Run

```bash
go run .
# or
go run . -config /path/to/config.toml
```

## Keybindings

- `↑/k` move up
- `↓/j` move down
- `Enter` attach selected session
- `n` create new session under selected folder (or selected session's folder)
- `R` rename selected session
- `K` kill selected session
- `c` send command to selected session
- `/` set or clear a filter
- `PgUp`/`PgDn` scroll details pane when content is long
- `r` manual refresh
- `q` quit
- `y` confirm kill (when prompted)
- `n` or `Esc` cancel kill (when prompted)

## Notes

- Only sessions that match configured folder namespaces are shown.
- Session names are sanitized to lowercase slug-like names for consistency.
- Requires `tmux` in `PATH`.
