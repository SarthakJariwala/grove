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
