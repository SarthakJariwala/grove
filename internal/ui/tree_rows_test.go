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
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{}, map[string]tmux.Session{})
	// Empty Agents and Terminals sections are collapsed; only folder + Commands section + command row
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if rows[0].typeOf != rowFolder {
		t.Fatalf("rows[0] = %#v, want Folder row", rows[0])
	}
	if rows[1].typeOf != rowSection || rows[1].section != sectionCommands {
		t.Fatalf("rows[1] = %#v, want Commands section", rows[1])
	}
	if rows[2].typeOf != rowCommand || rows[2].displayName != "start" || rows[2].status != "stopped" {
		t.Fatalf("rows[2] = %#v, want stopped command row", rows[2])
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

func TestCommandRowUsesShellIdleAsStoppedState(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{0: {{Name: "api/cmd-start", CurrentCommand: "zsh"}}}, map[string]tmux.Session{"api/cmd-start": {Name: "api/cmd-start", CurrentCommand: "zsh"}})
	// folder(0) + Terminals section(1) + terminal row(2) + Commands section(3) + command row(4)
	// The cmd-start session appears as a terminal AND a command (terminal because it's a legacy session too)
	// Actually: cmd-start is parsed as managedCommand, so it's excluded from buildTerminalRows
	// So: folder(0) + Commands section(1) + command row(2)
	if rows[2].status != "stopped" {
		t.Fatalf("command row status = %q, want stopped", rows[2].status)
	}
}

func TestBuildTreeRowsHidesEmptySections(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "Empty",
		Path:      "/tmp/empty",
		Namespace: "empty",
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{}, map[string]tmux.Session{})
	// Folder with no sessions and no commands: only the folder row
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (only folder row)", len(rows))
	}
	if rows[0].typeOf != rowFolder {
		t.Fatalf("rows[0] = %#v, want Folder row", rows[0])
	}
}

func TestBuildTreeRowsShowsCommandsSectionWhenConfigured(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "build", Command: "make build"}},
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{}, map[string]tmux.Session{})
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3 (folder + Commands section + command)", len(rows))
	}
	if rows[1].typeOf != rowSection || rows[1].section != sectionCommands {
		t.Fatalf("rows[1] = %#v, want Commands section", rows[1])
	}
}
