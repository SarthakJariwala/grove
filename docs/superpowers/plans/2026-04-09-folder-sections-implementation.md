# Folder Sections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Grove's new folder view with `Agents`, `Terminals`, and `Commands`, backed by persisted agent/command config and managed tmux session lifecycle.

**Architecture:** Extend config to store global agent templates plus per-folder agents and commands, then derive a typed tree model from persisted config plus live tmux state. Keep tmux as the execution layer, add one launch primitive for command/agent startup, and refactor the UI to act on typed rows instead of raw session-name strings.

**Tech Stack:** Go 1.23, Bubble Tea, Lip Gloss, BurntSushi TOML, tmux CLI

---

## File Structure

- Modify: `internal/config/config.go` — add `Agent` and `Command` types, remove `DefaultCommand`, normalize nested config values.
- Modify: `internal/config/config_test.go` — cover normalization of global agents, folder agents, and folder commands.
- Modify: `internal/configfile/configfile.go` — add rewrite-based `Save`, keep `Load`, update `AppendFolder` to use `Save`, and refresh the template content.
- Modify: `internal/configfile/configfile_test.go` — add round-trip persistence coverage for nested `folder.agent` and `folder.command` blocks.
- Modify: `internal/tmux/client.go` — add `NewSessionWithCommand` to create detached sessions with an initial command in one tmux call.
- Modify: `internal/tmux/client_test.go` — verify the new tmux primitive and keep existing parsing coverage green.
- Create: `internal/ui/managed_sessions.go` — centralize managed naming, parsing, and next-index helpers.
- Create: `internal/ui/managed_sessions_test.go` — cover naming, parsing, legacy fallback, and next-index logic.
- Create: `internal/ui/tree_rows.go` — define row/section kinds and build folder section rows from config plus live tmux sessions.
- Create: `internal/ui/tree_rows_test.go` — verify section ordering, command visibility, and managed-session classification.
- Modify: `internal/ui/model.go` — integrate typed rows, agent picker flow, command lifecycle actions, row-aware detail/help rendering, and managed row restrictions.
- Modify: `internal/ui/model_test.go` — update rendering tests to assert section headers and stopped command rows.
- Modify: `internal/ui/model_update_test.go` — add keybinding/action coverage for add-agent, start/stop/restart command, and managed-row restrictions.
- Modify: `config.example.toml` — document the new config layout.
- Modify: `README.md` — document the new sections, config shape, and keybindings.

### Task 1: Add Config Types And Normalization

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing normalization tests**

Add these tests to `internal/config/config_test.go`:

```go
func TestConfigNormalizeTrimsAgentsAndCommands(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cfg := Config{
		EditorCommand: " code . ",
		Agents: []Agent{{Name: " Codex ", Command: " codex "}},
		Folders: []Folder{{
			Name:          " API ",
			Path:          " ./api ",
			EditorCommand: " zed . ",
			Agents:        []Agent{{Name: " Amp ", Command: " amp "}},
			Commands:      []Command{{Name: " Start ", Command: " make start "}},
		}},
	}

	if err := cfg.Normalize(base); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if got := cfg.Agents[0]; got.Name != "Codex" || got.Command != "codex" {
		t.Fatalf("global agent = %#v, want trimmed fields", got)
	}

	folder := cfg.Folders[0]
	if got := folder.Agents[0]; got.Name != "Amp" || got.Command != "amp" {
		t.Fatalf("folder agent = %#v, want trimmed fields", got)
	}
	if got := folder.Commands[0]; got.Name != "Start" || got.Command != "make start" {
		t.Fatalf("folder command = %#v, want trimmed fields", got)
	}
	if folder.Namespace != "api" {
		t.Fatalf("folder.Namespace = %q, want %q", folder.Namespace, "api")
	}
}

func TestConfigNormalizeRejectsEmptyNestedEntries(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "global agent missing command",
			cfg:     Config{Agents: []Agent{{Name: "Codex"}}},
			wantErr: "agent[0] command is required",
		},
		{
			name: "folder agent missing name",
			cfg: Config{Folders: []Folder{{
				Name:   "API",
				Path:   "./api",
				Agents: []Agent{{Command: "amp"}},
			}}},
			wantErr: "folder[0] agent[0] name is required",
		},
		{
			name: "folder command missing command",
			cfg: Config{Folders: []Folder{{
				Name:     "API",
				Path:     "./api",
				Commands: []Command{{Name: "start"}},
			}}},
			wantErr: "folder[0] command[0] command is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Normalize(base)
			if err == nil {
				t.Fatalf("Normalize() error = nil, want contains %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Normalize() error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run the config tests to verify they fail**

Run: `go test ./internal/config`

Expected: FAIL with compile errors for missing `Agent` / `Command` types or failed assertions because `Normalize` ignores nested entries.

- [ ] **Step 3: Implement the config schema and normalization**

Update `internal/config/config.go` to this shape:

```go
type Agent struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
}

type Command struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
}

type Config struct {
	EditorCommand string   `toml:"editor_command"`
	Agents        []Agent  `toml:"agent"`
	Folders       []Folder `toml:"folder"`
}

type Folder struct {
	Name          string    `toml:"name"`
	Path          string    `toml:"path"`
	EditorCommand string    `toml:"editor_command"`
	Agents        []Agent   `toml:"agent"`
	Commands      []Command `toml:"command"`
	Namespace     string    `toml:"-"`
}

func (c *Config) Normalize(baseDir string) error {
	c.EditorCommand = strings.TrimSpace(c.EditorCommand)
	for i := range c.Agents {
		if err := normalizeAgent(&c.Agents[i], fmt.Sprintf("agent[%d]", i)); err != nil {
			return err
		}
	}

	seen := map[string]string{}
	for i := range c.Folders {
		folder := &c.Folders[i]
		folder.Name = strings.TrimSpace(folder.Name)
		folder.Path = strings.TrimSpace(folder.Path)
		folder.EditorCommand = strings.TrimSpace(folder.EditorCommand)

		if folder.Name == "" {
			return fmt.Errorf("folder[%d] name is required", i)
		}
		if folder.Path == "" {
			return fmt.Errorf("folder[%d] path is required", i)
		}

		for j := range folder.Agents {
			if err := normalizeAgent(&folder.Agents[j], fmt.Sprintf("folder[%d] agent[%d]", i, j)); err != nil {
				return err
			}
		}
		for j := range folder.Commands {
			if err := normalizeCommand(&folder.Commands[j], fmt.Sprintf("folder[%d] command[%d]", i, j)); err != nil {
				return err
			}
		}

		folder.Path = ExpandHome(folder.Path)
		if !filepath.IsAbs(folder.Path) {
			folder.Path = filepath.Join(baseDir, folder.Path)
		}

		absPath, err := filepath.Abs(folder.Path)
		if err != nil {
			return fmt.Errorf("resolve path for folder %q: %w", folder.Name, err)
		}
		folder.Path = absPath

		namespace := Slug(folder.Name)
		if namespace == "" {
			return fmt.Errorf("folder %q produced empty namespace", folder.Name)
		}
		if existing, exists := seen[namespace]; exists {
			return fmt.Errorf("folder %q conflicts with folder %q (both produce namespace %q)", folder.Name, existing, namespace)
		}
		seen[namespace] = folder.Name
		folder.Namespace = namespace
	}

	return nil
}

func normalizeAgent(agent *Agent, scope string) error {
	agent.Name = strings.TrimSpace(agent.Name)
	agent.Command = strings.TrimSpace(agent.Command)
	if agent.Name == "" {
		return fmt.Errorf("%s name is required", scope)
	}
	if agent.Command == "" {
		return fmt.Errorf("%s command is required", scope)
	}
	return nil
}

func normalizeCommand(command *Command, scope string) error {
	command.Name = strings.TrimSpace(command.Name)
	command.Command = strings.TrimSpace(command.Command)
	if command.Name == "" {
		return fmt.Errorf("%s name is required", scope)
	}
	if command.Command == "" {
		return fmt.Errorf("%s command is required", scope)
	}
	return nil
}
```

- [ ] **Step 4: Run the config tests to verify they pass**

Run: `go test ./internal/config`

Expected: PASS with all tests in `internal/config` green.

- [ ] **Step 5: Commit the schema work**

Run:

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "refactor: add agent and command config types"
```

### Task 2: Add Rewrite-Based Config Persistence

**Files:**
- Modify: `internal/configfile/configfile.go`
- Modify: `internal/configfile/configfile_test.go`

- [ ] **Step 1: Write failing persistence tests**

Add these tests to `internal/configfile/configfile_test.go`:

```go
func TestSaveRoundTripsNestedAgentsAndCommands(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	want := config.Config{
		EditorCommand: "code .",
		Agents: []config.Agent{{Name: "Codex", Command: "codex"}},
		Folders: []config.Folder{{
			Name:          "API",
			Path:          tmp,
			EditorCommand: "zed .",
			Agents:        []config.Agent{{Name: "Amp", Command: "amp"}},
			Commands:      []config.Command{{Name: "start", Command: "make start"}},
		}},
	}

	if err := Save(cfgPath, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Agents) != 1 || got.Agents[0].Command != "codex" {
		t.Fatalf("got.Agents = %#v, want preserved global agent", got.Agents)
	}
	if len(got.Folders[0].Agents) != 1 || got.Folders[0].Agents[0].Command != "amp" {
		t.Fatalf("got.Folders[0].Agents = %#v, want preserved folder agent", got.Folders[0].Agents)
	}
	if len(got.Folders[0].Commands) != 1 || got.Folders[0].Commands[0].Command != "make start" {
		t.Fatalf("got.Folders[0].Commands = %#v, want preserved folder command", got.Folders[0].Commands)
	}
}

func TestAppendFolderUsesSaveAndPreservesExistingAgents(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	if err := Save(cfgPath, config.Config{
		Agents: []config.Agent{{Name: "Codex", Command: "codex"}},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := AppendFolder(cfgPath, config.Folder{Name: "API", Path: tmp}); err != nil {
		t.Fatalf("AppendFolder() error = %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Agents) != 1 || loaded.Agents[0].Name != "Codex" {
		t.Fatalf("loaded.Agents = %#v, want preserved global agents", loaded.Agents)
	}
	if len(loaded.Folders) != 1 || loaded.Folders[0].Name != "API" {
		t.Fatalf("loaded.Folders = %#v, want appended folder", loaded.Folders)
	}
}
```

Add this import block to `internal/configfile/configfile_test.go`:

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SarthakJariwala/grove/internal/config"
)
```

- [ ] **Step 2: Run the configfile tests to verify they fail**

Run: `go test ./internal/configfile`

Expected: FAIL with `undefined: Save` or with round-trip assertions failing because nested tables are not persisted.

- [ ] **Step 3: Implement full-config save and update the template**

Update `internal/configfile/configfile.go` with these functions:

```go
func Save(path string, cfg config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory %q: %w", filepath.Dir(path), err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config %q: %w", path, err)
	}
	defer file.Close()

	if err := toml.NewEncoder(file).Encode(cfg); err != nil {
		return fmt.Errorf("encode config %q: %w", path, err)
	}
	return nil
}

func AppendFolder(path string, f config.Folder) error {
	var cfg config.Config
	if _, err := os.Stat(path); err == nil {
		loaded, loadErr := Load(path)
		if loadErr != nil {
			return loadErr
		}
		cfg = loaded
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check config %q: %w", path, err)
	}

	cfg.Folders = append(cfg.Folders, f)
	return Save(path, cfg)
}
```

Replace the template string in `EnsureTemplate` with:

```go
const tmpl = `# Example grove config
#
# editor_command = "code ."
#
# [[agent]]
# name = "Codex"
# command = "codex"
#
# [[folder]]
# name = "Main API"
# path = "/Users/you/dev/main-api"
#
#   [[folder.agent]]
#   name = "Amp"
#   command = "amp"
#
#   [[folder.command]]
#   name = "start"
#   command = "make start"
#
#   editor_command = "zed ."
`
```

- [ ] **Step 4: Run the persistence tests to verify they pass**

Run: `go test ./internal/configfile ./internal/config`

Expected: PASS with round-trip coverage green.

- [ ] **Step 5: Commit the persistence work**

Run:

```bash
git add internal/configfile/configfile.go internal/configfile/configfile_test.go
git commit -m "refactor: save grove config by rewriting toml"
```

### Task 3: Add Managed Session Naming And Tmux Launch Helpers

**Files:**
- Create: `internal/ui/managed_sessions.go`
- Create: `internal/ui/managed_sessions_test.go`
- Modify: `internal/tmux/client.go`
- Modify: `internal/tmux/client_test.go`
- Modify: `internal/ui/model_test.go`
- Modify: `internal/ui/model_update_test.go`

- [ ] **Step 1: Write the failing naming and tmux tests**

Create `internal/ui/managed_sessions_test.go` with:

```go
package ui

import (
	"testing"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

func TestManagedSessionNames(t *testing.T) {
	folder := config.Folder{Name: "API", Namespace: "api"}
	if got := agentSessionName(folder, "codex", 2); got != "api/agent-codex-2" {
		t.Fatalf("agentSessionName() = %q, want %q", got, "api/agent-codex-2")
	}
	if got := terminalSessionName(folder, 3); got != "api/term-3" {
		t.Fatalf("terminalSessionName() = %q, want %q", got, "api/term-3")
	}
	if got := commandSessionName(folder, "start"); got != "api/cmd-start" {
		t.Fatalf("commandSessionName() = %q, want %q", got, "api/cmd-start")
	}
}

func TestParseManagedSession(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		wantKind    managedSessionKind
		wantSlug    string
		wantIndex   int
	}{
		{name: "agent", sessionName: "api/agent-codex-2", wantKind: managedAgent, wantSlug: "codex", wantIndex: 2},
		{name: "terminal", sessionName: "api/term-4", wantKind: managedTerminal, wantIndex: 4},
		{name: "command", sessionName: "api/cmd-start", wantKind: managedCommand, wantSlug: "start"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := parseManagedSession("api", tt.sessionName)
			if !ok {
				t.Fatalf("parseManagedSession(%q) ok = false, want true", tt.sessionName)
			}
			if id.kind != tt.wantKind || id.slug != tt.wantSlug || id.index != tt.wantIndex {
				t.Fatalf("parsed id = %#v, want kind=%v slug=%q index=%d", id, tt.wantKind, tt.wantSlug, tt.wantIndex)
			}
		})
	}
}

func TestNextTerminalIndex(t *testing.T) {
	folder := config.Folder{Namespace: "api"}
	sessions := []tmux.Session{{Name: "api/term-1"}, {Name: "api/term-3"}, {Name: "api/agent-codex-1"}}
	if got := nextTerminalIndex(folder, sessions); got != 4 {
		t.Fatalf("nextTerminalIndex() = %d, want 4", got)
	}
}
```

Add this test to `internal/tmux/client_test.go`:

```go
func TestNewSessionWithCommandIncludesStartupCommand(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	restore := stubExecCommand(t, func(name string, args ...string) *exec.Cmd {
		_ = name
		gotArgs = append([]string(nil), args...)
		return helperCommand(t, "mutate_ok")
	})
	defer restore()

	client := &Client{}
	if err := client.NewSessionWithCommand("api/cmd-start", "/tmp/api", "make start"); err != nil {
		t.Fatalf("NewSessionWithCommand() error = %v", err)
	}

	want := []string{"new-session", "-d", "-s", "api/cmd-start", "-c", "/tmp/api", "make start"}
	if diff := fmt.Sprint(gotArgs); diff != fmt.Sprint(want) {
		t.Fatalf("tmux args = %v, want %v", gotArgs, want)
	}
}
```

Add this helper case to `TestHelperProcess` in `internal/tmux/client_test.go`:

```go
case "mutate_ok":
	os.Exit(0)
```

- [ ] **Step 2: Run the naming and tmux tests to verify they fail**

Run: `go test ./internal/tmux ./internal/ui`

Expected: FAIL with `undefined: agentSessionName`, `undefined: parseManagedSession`, or `Client.NewSessionWithCommand undefined`.

- [ ] **Step 3: Implement managed-session helpers and the tmux launch primitive**

Create `internal/ui/managed_sessions.go`:

```go
package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type managedSessionID struct {
	kind  managedSessionKind
	slug  string
	index int
}

type managedSessionKind int

const (
	managedUnknown managedSessionKind = iota
	managedAgent
	managedTerminal
	managedCommand
)

func agentSessionName(folder config.Folder, slug string, index int) string {
	return fmt.Sprintf("%s/agent-%s-%d", folder.Namespace, sanitizeLeaf(slug), index)
}

func terminalSessionName(folder config.Folder, index int) string {
	return fmt.Sprintf("%s/term-%d", folder.Namespace, index)
}

func commandSessionName(folder config.Folder, slug string) string {
	return fmt.Sprintf("%s/cmd-%s", folder.Namespace, sanitizeLeaf(slug))
}

func parseManagedSession(namespace, fullName string) (managedSessionID, bool) {
	prefix := namespace + "/"
	if !strings.HasPrefix(fullName, prefix) {
		return managedSessionID{}, false
	}
	leaf := strings.TrimPrefix(fullName, prefix)

	if slug, index, ok := parseIndexedLeaf(leaf, "agent-"); ok {
		return managedSessionID{kind: managedAgent, slug: slug, index: index}, true
	}
	if strings.HasPrefix(leaf, "cmd-") {
		return managedSessionID{kind: managedCommand, slug: strings.TrimPrefix(leaf, "cmd-")}, true
	}
	if slug, index, ok := parseIndexedLeaf(leaf, "term-"); ok {
		_ = slug
		return managedSessionID{kind: managedTerminal, index: index}, true
	}
	return managedSessionID{}, false
}

func parseIndexedLeaf(leaf, prefix string) (string, int, bool) {
	if !strings.HasPrefix(leaf, prefix) {
		return "", 0, false
	}
	remainder := strings.TrimPrefix(leaf, prefix)
	lastDash := strings.LastIndex(remainder, "-")
	if lastDash == -1 {
		index, err := strconv.Atoi(remainder)
		return "", index, err == nil
	}
	index, err := strconv.Atoi(remainder[lastDash+1:])
	if err != nil {
		return "", 0, false
	}
	return remainder[:lastDash], index, true
}

func nextTerminalIndex(folder config.Folder, sessions []tmux.Session) int {
	maxIndex := 0
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if ok && id.kind == managedTerminal && id.index > maxIndex {
			maxIndex = id.index
		}
	}
	return maxIndex + 1
}

func nextAgentIndex(folder config.Folder, agentName string, sessions []tmux.Session) int {
	slug := sanitizeLeaf(agentName)
	maxIndex := 0
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if ok && id.kind == managedAgent && id.slug == slug && id.index > maxIndex {
			maxIndex = id.index
		}
	}
	return maxIndex + 1
}
```

Update `internal/tmux/client.go`:

```go
type SessionManager interface {
	ListSessions() ([]Session, error)
	ListPanes() ([]PaneInfo, error)
	NewSession(name, cwd string) error
	NewSessionWithCommand(name, cwd, command string) error
	SendKeys(target, command string) error
	RenameSession(oldName, newName string) error
	KillSession(name string) error
	CapturePane(target string) (string, error)
	AttachCommand(name string) *exec.Cmd
}

func (c *Client) NewSessionWithCommand(name, cwd, command string) error {
	args := []string{"new-session", "-d", "-s", name, "-c", cwd}
	if strings.TrimSpace(command) != "" {
		args = append(args, command)
	}
	cmd := execCommand("tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux new-session %q: %w (%s)", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
```

Update the fake session managers in `internal/ui/model_test.go` and `internal/ui/model_update_test.go` with a stub:

```go
func (f fakeSessionManager) NewSessionWithCommand(name, cwd, command string) error { return nil }
```

and:

```go
func (f *trackingSessionManager) NewSessionWithCommand(name, cwd, command string) error { return nil }
```

- [ ] **Step 4: Run the naming and tmux tests to verify they pass**

Run: `go test ./internal/tmux ./internal/ui`

Expected: PASS with the new helper coverage green.

- [ ] **Step 5: Commit the managed-session groundwork**

Run:

```bash
git add internal/tmux/client.go internal/tmux/client_test.go internal/ui/managed_sessions.go internal/ui/managed_sessions_test.go internal/ui/model_test.go internal/ui/model_update_test.go
git commit -m "refactor: add managed grove session helpers"
```

### Task 4: Build Typed Tree Rows And Section Rendering

**Files:**
- Create: `internal/ui/tree_rows.go`
- Create: `internal/ui/tree_rows_test.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/model_test.go`

- [ ] **Step 1: Write the failing tree-row tests**

Create `internal/ui/tree_rows_test.go` with:

```go
package ui

import (
	"testing"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

func TestBuildTreeRowsIncludesSectionsAndStoppedCommands(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands: []config.Command{{Name: "start", Command: "make start"}},
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{}, map[string]tmux.Session{})
	if len(rows) != 5 {
		t.Fatalf("len(rows) = %d, want 5", len(rows))
	}
	if rows[1].typeOf != rowSection || rows[1].section != sectionAgents {
		t.Fatalf("rows[1] = %#v, want Agents section", rows[1])
	}
	if rows[2].typeOf != rowSection || rows[2].section != sectionTerminals {
		t.Fatalf("rows[2] = %#v, want Terminals section", rows[2])
	}
	if rows[3].typeOf != rowSection || rows[3].section != sectionCommands {
		t.Fatalf("rows[3] = %#v, want Commands section", rows[3])
	}
	if rows[4].typeOf != rowCommand || rows[4].displayName != "start" || rows[4].status != "stopped" {
		t.Fatalf("rows[4] = %#v, want stopped command row", rows[4])
	}
}

func TestBuildTreeRowsPlacesManagedAndLegacySessionsInExpectedSections(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	sessions := map[int][]tmux.Session{0: {
		{Name: "api/agent-codex-1", Windows: 1},
		{Name: "api/term-1", Windows: 1},
		{Name: "api/cmd-start", Windows: 1, CurrentCommand: "make"},
		{Name: "api/legacy-shell", Windows: 1},
	}}

	rows := buildTreeRows(cfg, sessions, map[string]tmux.Session{"api/cmd-start": {Name: "api/cmd-start", CurrentCommand: "make"}})

	var sawAgent, sawTerminal, sawLegacy, sawRunningCommand bool
	for _, row := range rows {
		switch {
		case row.typeOf == rowAgentInstance && row.displayName == "Codex #1":
			sawAgent = true
		case row.typeOf == rowTerminalInstance && row.sessionName == "api/term-1":
			sawTerminal = true
		case row.typeOf == rowTerminalInstance && row.sessionName == "api/legacy-shell":
			sawLegacy = true
		case row.typeOf == rowCommand && row.displayName == "start" && row.status == "running":
			sawRunningCommand = true
		}
	}

	if !sawAgent || !sawTerminal || !sawLegacy || !sawRunningCommand {
		t.Fatalf("rows missing expected classifications: %#v", rows)
	}
}
```

Add this rendering assertion to `internal/ui/model_test.go`:

```go
func TestRenderTreePaneShowsSectionHeadings(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	m := NewModel(cfg, "config.toml", fakeSessionManager{})
	m.rebuildRows()
	got := m.renderTreePane(12, 60, 64, false)

	for _, heading := range []string{"Agents", "Terminals", "Commands", "start", "stopped"} {
		if !strings.Contains(got, heading) {
			t.Fatalf("tree view = %q, want %q", got, heading)
		}
	}
}
```

- [ ] **Step 2: Run the UI tests to verify they fail**

Run: `go test ./internal/ui`

Expected: FAIL with `undefined: buildTreeRows`, missing section metadata, or the tree not rendering section headings.

- [ ] **Step 3: Implement typed rows and section-aware rebuilding**

Create `internal/ui/tree_rows.go`:

```go
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type sectionKind int

const (
	rowFolder rowType = iota
	rowSection
	rowAgentInstance
	rowTerminalInstance
	rowCommand
)

const (
	sectionNone sectionKind = iota
	sectionAgents
	sectionTerminals
	sectionCommands
)

func buildTreeRows(cfg config.Config, sessions map[int][]tmux.Session, sessionByName map[string]tmux.Session) []treeRow {
	rows := make([]treeRow, 0)
	for folderIndex, folder := range cfg.Folders {
		rows = append(rows, treeRow{typeOf: rowFolder, folderIndex: folderIndex, displayName: folder.Name})
		rows = append(rows, treeRow{typeOf: rowSection, folderIndex: folderIndex, section: sectionAgents, displayName: "Agents"})

		for _, row := range buildAgentRows(folderIndex, folder, sessions[folderIndex]) {
			rows = append(rows, row)
		}

		rows = append(rows, treeRow{typeOf: rowSection, folderIndex: folderIndex, section: sectionTerminals, displayName: "Terminals"})
		for _, row := range buildTerminalRows(folderIndex, folder, sessions[folderIndex]) {
			rows = append(rows, row)
		}

		rows = append(rows, treeRow{typeOf: rowSection, folderIndex: folderIndex, section: sectionCommands, displayName: "Commands"})
		for _, command := range folder.Commands {
			sessionName := commandSessionName(folder, command.Name)
			session, ok := sessionByName[sessionName]
			status := "stopped"
			if ok && commandSessionRunning(session) {
				status = "running"
			}
			rows = append(rows, treeRow{
				typeOf:      rowCommand,
				folderIndex: folderIndex,
				section:     sectionCommands,
				sessionName: sessionName,
				displayName: command.Name,
				commandText: command.Command,
				status:      status,
				attached:    ok && session.Attached,
			})
		}
	}
	return rows
}

func buildAgentRows(folderIndex int, folder config.Folder, sessions []tmux.Session) []treeRow {
	rows := make([]treeRow, 0)
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if !ok || id.kind != managedAgent {
			continue
		}
		rows = append(rows, treeRow{
			typeOf:      rowAgentInstance,
			folderIndex: folderIndex,
			section:     sectionAgents,
			sessionName: session.Name,
			displayName: fmt.Sprintf("%s #%d", strings.Title(id.slug), id.index),
			status:      attachedStatus(session.Attached),
			attached:    session.Attached,
			windows:     session.Windows,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].sessionName < rows[j].sessionName })
	return rows
}

func buildTerminalRows(folderIndex int, folder config.Folder, sessions []tmux.Session) []treeRow {
	rows := make([]treeRow, 0)
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if ok && id.kind == managedCommand {
			continue
		}

		display := strings.TrimPrefix(session.Name, folder.Namespace+"/")
		if ok && id.kind == managedTerminal {
			display = fmt.Sprintf("Terminal #%d", id.index)
		}
		if ok && id.kind == managedAgent {
			continue
		}

		rows = append(rows, treeRow{
			typeOf:      rowTerminalInstance,
			folderIndex: folderIndex,
			section:     sectionTerminals,
			sessionName: session.Name,
			displayName: display,
			status:      attachedStatus(session.Attached),
			attached:    session.Attached,
			windows:     session.Windows,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].sessionName < rows[j].sessionName })
	return rows
}

func attachedStatus(attached bool) string {
	if attached {
		return "attached"
	}
	return "detached"
}

func commandSessionRunning(session tmux.Session) bool {
	command := strings.TrimSpace(session.CurrentCommand)
	return command != "" && !isShellCommand(command)
}
```

Then update the `treeRow` shape and `rebuildRows()` in `internal/ui/model.go`:

```go
type treeRow struct {
	typeOf      rowType
	section     sectionKind
	folderIndex int
	sessionName string
	displayName string
	commandText string
	status      string
	attached    bool
	windows     int
	currentPath string
	currentCommand string
	paneTitle   string
	lastActivity int64
	hasAlerts   bool
	alertsBell  bool
	alertsActivity bool
	alertsSilence bool
}

func (m *Model) rebuildRows() {
	sessionByName := map[string]tmux.Session{}
	for _, folderSessions := range m.sessions {
		for _, session := range folderSessions {
			sessionByName[session.Name] = session
		}
	}
	m.rows = buildTreeRows(m.cfg, m.sessions, sessionByName)
	if m.selected >= len(m.rows) && len(m.rows) > 0 {
		m.selected = len(m.rows) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
	m.detailScroll = 0
}
```

Update `renderTreePane`, `renderDetailPane`, `selectedSessionRow`, and `selectedFolder` to branch on `rowSection`, `rowAgentInstance`, `rowTerminalInstance`, and `rowCommand` instead of assuming only folder/session rows.

- [ ] **Step 4: Run the UI tests to verify they pass**

Run: `go test ./internal/ui`

Expected: PASS with section rows, stopped commands, and legacy terminal classification covered.

- [ ] **Step 5: Commit the tree-row refactor**

Run:

```bash
git add internal/ui/tree_rows.go internal/ui/tree_rows_test.go internal/ui/model.go internal/ui/model_test.go
git commit -m "refactor: render folders with agent terminal and command sections"
```

### Task 5: Implement Terminal Creation And Agent Picker Flows

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/model_update_test.go`

- [ ] **Step 1: Write the failing interaction tests**

Add these tests to `internal/ui/model_update_test.go`:

```go
func TestUpdateNCreatesManagedTerminal(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", fake)
	m.rebuildRows()

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected terminal create command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.attachTarget != "api/term-1" {
		t.Fatalf("terminal result = %#v, want attachTarget api/term-1", msg)
	}
}

func TestUpdateAOpensAgentPicker(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{
		Agents: []config.Agent{{Name: "Codex", Command: "codex"}},
		Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}},
	}, "config.toml", &trackingSessionManager{})
	m.rebuildRows()

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := model.(Model)

	if got.overlayMode != overlayAgentPicker {
		t.Fatalf("overlayMode = %v, want overlayAgentPicker", got.overlayMode)
	}
	if len(got.agentChoices) != 2 {
		t.Fatalf("len(agentChoices) = %d, want 2", len(got.agentChoices))
	}
}

func TestConfirmAgentPickerCreatesAndAttachesManagedAgent(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{
		Folders: []config.Folder{{
			Name:      "API",
			Path:      "/tmp/api",
			Namespace: "api",
			Agents:    []config.Agent{{Name: "Codex", Command: "codex"}},
		}},
	}, "config.toml", fake)
	m.overlayMode = overlayAgentPicker
	m.overlayFolderIndex = 0
	m.agentChoices = []agentChoice{{Label: "Codex", Agent: config.Agent{Name: "Codex", Command: "codex"}}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected agent create command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.attachTarget != "api/agent-codex-1" {
		t.Fatalf("agent result = %#v, want attachTarget api/agent-codex-1", msg)
	}
}
```

Extend `trackingSessionManager` in `internal/ui/model_update_test.go`:

```go
type trackingSessionManager struct {
	killed    []string
	captured  []string
	attached  []string
	created   []string
	launched  []string
	commands  []string
}

func (f *trackingSessionManager) NewSession(name, cwd string) error {
	f.created = append(f.created, name)
	return nil
}

func (f *trackingSessionManager) NewSessionWithCommand(name, cwd, command string) error {
	f.launched = append(f.launched, name)
	f.commands = append(f.commands, command)
	return nil
}
```

- [ ] **Step 2: Run the interaction tests to verify they fail**

Run: `go test ./internal/ui`

Expected: FAIL because `overlayAgentPicker` and managed terminal/agent creation are not implemented yet.

- [ ] **Step 3: Implement managed terminal creation and the agent picker**

Add these model fields and enums to `internal/ui/model.go`:

```go
type overlayMode int

const (
	overlayNone overlayMode = iota
	overlayAgentPicker
)

type agentChoice struct {
	Label   string
	Agent   config.Agent
	Persist bool
	IsNew   bool
}

type promptMode int

const (
	promptNone promptMode = iota
	promptRunCommand
	promptFilter
	promptAddFolder
	promptAddAgentName
	promptAddAgentCommand
)

type Model struct {
	overlayMode        overlayMode
	overlayIndex       int
	overlayFolderIndex int
	agentChoices       []agentChoice
	pendingAgent       config.Agent
}
```

Update the `n` and `a` branches inside `Update`:

```go
case "n":
	folder, ok := m.selectedFolder()
	if !ok {
		m.errMsg = "select a folder or one of its sections"
		return m, nil
	}
	return m, m.newTerminalCmd(m.rows[m.selected].folderIndex, folder)
case "a":
	folder, ok := m.selectedFolder()
	if !ok {
		m.errMsg = "select a folder or the Agents section"
		return m, nil
	}
	m.openAgentPicker(folder)
	return m, nil
```

Add these helpers to `internal/ui/model.go`:

```go
func (m *Model) openAgentPicker(folder config.Folder) {
	m.overlayMode = overlayAgentPicker
	m.overlayFolderIndex = m.rows[m.selected].folderIndex
	m.overlayIndex = 0
	m.agentChoices = buildAgentChoices(m.cfg, folder)
	m.errMsg = ""
	m.statusMsg = ""
}

func buildAgentChoices(cfg config.Config, folder config.Folder) []agentChoice {
	choices := make([]agentChoice, 0, len(folder.Agents)+len(cfg.Agents)+1)
	seen := map[string]struct{}{}
	for _, agent := range folder.Agents {
		key := sanitizeLeaf(agent.Name)
		seen[key] = struct{}{}
		choices = append(choices, agentChoice{Label: agent.Name, Agent: agent})
	}
	for _, agent := range cfg.Agents {
		key := sanitizeLeaf(agent.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		choices = append(choices, agentChoice{Label: agent.Name, Agent: agent, Persist: true})
	}
	choices = append(choices, agentChoice{Label: "Add new agent...", IsNew: true})
	return choices
}

func (m Model) newTerminalCmd(folderIndex int, folder config.Folder) tea.Cmd {
	index := nextTerminalIndex(folder, m.sessions[folderIndex])
	name := terminalSessionName(folder, index)
	return func() tea.Msg {
		if err := m.client.NewSession(name, folder.Path); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "created " + name, attachTarget: name}
	}
}

func (m Model) newAgentCmd(folderIndex int, folder config.Folder, agent config.Agent) tea.Cmd {
	index := nextAgentIndex(folder, agent.Name, m.sessions[folderIndex])
	name := agentSessionName(folder, agent.Name, index)
	return func() tea.Msg {
		if err := m.client.NewSessionWithCommand(name, folder.Path, agent.Command); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "created " + name, attachTarget: name}
	}
}
```

In `Update`, when `overlayMode == overlayAgentPicker`, handle `up`, `down`, `enter`, and `esc`; on `enter`, either open the add-agent prompts or run `newAgentCmd(m.overlayFolderIndex, folder, choice.Agent)`. Persist missing global templates into `m.cfg.Folders[folderIndex].Agents` and call `configfile.Save(m.cfgPath, m.cfg)` before launching the session.

- [ ] **Step 4: Run the interaction tests to verify they pass**

Run: `go test ./internal/ui`

Expected: PASS with managed terminal naming and agent picker behavior covered.

- [ ] **Step 5: Commit the terminal and agent flows**

Run:

```bash
git add internal/ui/model.go internal/ui/model_update_test.go
git commit -m "feat: add managed terminal and agent launch flows"
```

### Task 6: Implement Command Rows And Lifecycle Controls

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/model_update_test.go`
- Modify: `internal/ui/tree_rows_test.go`

- [ ] **Step 1: Write the failing command lifecycle tests**

Add these tests to `internal/ui/model_update_test.go`:

```go
func TestUpdateSStartsStoppedCommandWithoutAttaching(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", fake)
	m.rebuildRows()
	m.setSelected(4)

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected start command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.attachTarget != "" {
		t.Fatalf("start result = %#v, want background start", msg)
	}
	if len(fake.launched) != 1 || fake.launched[0] != "api/cmd-start" {
		t.Fatalf("launched sessions = %#v, want [api/cmd-start]", fake.launched)
	}
}

func TestUpdateXStopsRunningCommand(t *testing.T) {
	t.Parallel()

	fake := &trackingSessionManager{}
	m := NewModel(config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}, "config.toml", fake)
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", status: "running"}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	_ = model.(Model)
	if cmd == nil {
		t.Fatal("expected stop command")
	}
	msg := cmd()
	res, ok := msg.(actionResultMsg)
	if !ok || res.err != nil {
		t.Fatalf("stop result = %#v, want successful actionResultMsg", msg)
	}
	if len(fake.killed) != 1 || fake.killed[0] != "api/cmd-start" {
		t.Fatalf("killed sessions = %#v, want [api/cmd-start]", fake.killed)
	}
}

func TestUpdateEnterOnStoppedCommandDoesNothing(t *testing.T) {
	t.Parallel()

	m := NewModel(config.Config{Folders: []config.Folder{{Name: "API", Path: "/tmp/api", Namespace: "api"}}}, "config.toml", &trackingSessionManager{})
	m.rows = []treeRow{{typeOf: rowCommand, folderIndex: 0, sessionName: "api/cmd-start", displayName: "start", status: "stopped"}}

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := model.(Model)
	if cmd != nil {
		t.Fatal("expected no attach command for stopped command row")
	}
	if got.errMsg != "select a running session" {
		t.Fatalf("errMsg = %q, want %q", got.errMsg, "select a running session")
	}
}
```

Add this classification test to `internal/ui/tree_rows_test.go`:

```go
func TestCommandRowUsesShellIdleAsStoppedState(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{0: {{Name: "api/cmd-start", CurrentCommand: "zsh"}}}, map[string]tmux.Session{"api/cmd-start": {Name: "api/cmd-start", CurrentCommand: "zsh"}})
	if rows[4].status != "stopped" {
		t.Fatalf("command row status = %q, want stopped", rows[4].status)
	}
}
```

- [ ] **Step 2: Run the command lifecycle tests to verify they fail**

Run: `go test ./internal/ui`

Expected: FAIL because `s`, `x`, and `R` are not wired to command rows and `Enter` still assumes every non-folder row is attachable.

- [ ] **Step 3: Implement command start, stop, restart, and add-command flows**

Extend `promptMode` in `internal/ui/model.go`:

```go
const (
	promptNone promptMode = iota
	promptRunCommand
	promptFilter
	promptAddFolder
	promptAddAgentName
	promptAddAgentCommand
	promptAddCommandName
	promptAddCommandCommand
)
```

Add these helpers to `internal/ui/model.go`:

```go
func (m Model) selectedCommandRow() (treeRow, bool) {
	if len(m.rows) == 0 || m.selected < 0 || m.selected >= len(m.rows) {
		return treeRow{}, false
	}
	row := m.rows[m.selected]
	if row.typeOf != rowCommand {
		return treeRow{}, false
	}
	return row, true
}

func (m Model) startCommandCmd(folder config.Folder, row treeRow) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.NewSessionWithCommand(row.sessionName, folder.Path, row.commandText); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{status: "started " + row.displayName}
	}
}

func (m Model) restartCommandCmd(folder config.Folder, row treeRow) tea.Cmd {
	return tea.Sequence(
		m.killSessionCmd(row.sessionName),
		m.startCommandCmd(folder, row),
	)
}
```

Update `Update` with these branches:

```go
case "C":
	folder, ok := m.selectedFolder()
	if !ok {
		m.errMsg = "select a folder or the Commands section"
		return m, nil
	}
	m.openPrompt(promptAddCommandName, "", "command name")
	m.pendingFolder = folder
	return m, textinput.Blink
case "s":
	row, ok := m.selectedCommandRow()
	if !ok || row.status == "running" {
		return m, nil
	}
	folder := m.cfg.Folders[row.folderIndex]
	return m, m.startCommandCmd(folder, row)
case "x":
	row, ok := m.selectedCommandRow()
	if !ok || row.status != "running" {
		return m, nil
	}
	return m, m.killSessionCmd(row.sessionName)
case "R":
	row, ok := m.selectedCommandRow()
	if !ok {
		return m, nil
	}
	folder := m.cfg.Folders[row.folderIndex]
	if row.status == "running" {
		return m, m.restartCommandCmd(folder, row)
	}
	return m, m.startCommandCmd(folder, row)
case "enter":
	row, ok := m.selectedSessionRow()
	if !ok {
		m.errMsg = "select a running session"
		return m, nil
	}
	if row.typeOf == rowCommand && row.status != "running" {
		m.errMsg = "select a running session"
		return m, nil
	}
	return m, tea.ExecProcess(m.client.AttachCommand(row.sessionName), func(err error) tea.Msg {
		return attachedMsg{err: err}
	})
```

In `updatePrompt`, handle `promptAddCommandName` and `promptAddCommandCommand` by appending a `config.Command` to the selected folder, saving through `configfile.Save`, and rebuilding rows.

Update `selectedSessionRow()` so it returns `rowAgentInstance`, `rowTerminalInstance`, and only `rowCommand` when `status == "running"`.

Remove managed-row rename support by deleting the `R` rename branch and its help text.

- [ ] **Step 4: Run the command lifecycle tests to verify they pass**

Run: `go test ./internal/ui`

Expected: PASS with stopped command rows, background start, kill-based stop, and restart behavior covered.

- [ ] **Step 5: Commit the command lifecycle work**

Run:

```bash
git add internal/ui/model.go internal/ui/model_update_test.go internal/ui/tree_rows_test.go
git commit -m "feat: add managed command lifecycle controls"
```

### Task 7: Update Docs And Run Full Verification

**Files:**
- Modify: `config.example.toml`
- Modify: `README.md`

- [ ] **Step 1: Update the example config**

Replace `config.example.toml` with:

```toml
# editor_command = "code ."

[[agent]]
name = "Codex"
command = "codex"

[[agent]]
name = "Amp"
command = "amp"

[[folder]]
name = "Main API"
path = "/Users/you/dev/main-api"

  [[folder.agent]]
  name = "Codex"
  command = "codex"

  [[folder.command]]
  name = "start"
  command = "make start"
```

- [ ] **Step 2: Update the README feature and keybinding sections**

Update the feature list and keybindings in `README.md` to this content:

```md
- Organize workspaces into folders with `Agents`, `Terminals`, and `Commands`
- Launch multiple agent instances from configured templates
- Start, stop, restart, preview, and attach to managed command sessions
- Keep plain terminal sessions runtime-only and lightweight
```

```md
| Key              | Action                                                  |
|------------------|---------------------------------------------------------|
| `↑` / `k`       | Move up                                                  |
| `↓` / `j`       | Move down                                                |
| `Enter`          | Attach to selected running session                       |
| `v`              | Preview selected running session                         |
| `n`              | Create a new terminal in the selected folder             |
| `a`              | Add or launch an agent in the selected folder            |
| `C`              | Add a managed command to the selected folder             |
| `s`              | Start the selected stopped command                       |
| `x`              | Stop the selected running command                        |
| `R`              | Restart the selected command                             |
| `c`              | Send a command to the selected running session           |
| `K`              | Kill the selected running terminal or agent              |
| `/`              | Filter folders and rows                                  |
| `e`              | Open the selected folder or session path in the editor   |
| `r`              | Manual refresh                                           |
| `q`              | Quit                                                     |
```

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`

Expected: PASS with all package tests green.

- [ ] **Step 4: Run the vet check**

Run: `go vet ./...`

Expected: no output and exit code 0.

- [ ] **Step 5: Commit the docs and final verification**

Run:

```bash
git add README.md config.example.toml
git commit -m "docs: describe folder sections workflow"
```

## Self-Review Checklist

- Spec coverage:
  - Config schema and persistence: Tasks 1-2
  - Managed naming and legacy fallback: Tasks 3-4
  - Typed tree rows and stable sections: Task 4
  - Terminal runtime-only behavior: Task 5
  - Agent picker, persistence, and multi-instance launch: Task 5
  - Command rows, start/stop/restart, stopped detection: Task 6
  - Docs and examples: Task 7
- Placeholder scan:
  - No placeholder markers or vague “add tests” steps remain.
- Type consistency:
  - `Agent`, `Command`, `rowAgentInstance`, `rowTerminalInstance`, `rowCommand`, `sectionAgents`, `sectionTerminals`, `sectionCommands`, `NewSessionWithCommand`, and `buildTreeRows` are used consistently across tasks.
