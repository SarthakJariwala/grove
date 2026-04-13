package ui

import (
	"testing"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

func TestBuildTreeRowsOmitsSectionsAndKeepsStoppedCommands(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{}, map[string]tmux.Session{})
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (folder + command)", len(rows))
	}
	if rows[0].typeOf != rowFolder {
		t.Fatalf("rows[0] = %#v, want folder row", rows[0])
	}
	if rows[1].typeOf != rowCommand || rows[1].displayName != "start" || rows[1].status != "stopped" {
		t.Fatalf("rows[1] = %#v, want stopped command row", rows[1])
	}
}

func TestBuildTreeRowsOrdersDirectChildren(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	sessions := map[int][]tmux.Session{0: {
		{Name: "api/agent-codex-1", Windows: 1},
		{Name: "api/term-1", Windows: 1},
		{Name: "api/legacy-shell", Windows: 1},
		{Name: "api/cmd-start", Windows: 1, CurrentCommand: "make"},
	}}

	rows := buildTreeRows(cfg, sessions, map[string]tmux.Session{
		"api/cmd-start": {Name: "api/cmd-start", CurrentCommand: "make"},
	})

	got := []rowType{rows[0].typeOf, rows[1].typeOf, rows[2].typeOf, rows[3].typeOf, rows[4].typeOf}
	want := []rowType{rowFolder, rowAgentInstance, rowTerminalInstance, rowTerminalInstance, rowCommand}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row types = %#v, want %#v", got, want)
		}
	}
	if rows[1].displayName != "Codex #1" {
		t.Fatalf("rows[1].displayName = %q, want Codex #1", rows[1].displayName)
	}
	if rows[2].displayName != "Terminal #1" {
		t.Fatalf("rows[2].displayName = %q, want Terminal #1", rows[2].displayName)
	}
	if rows[3].displayName != "legacy-shell" {
		t.Fatalf("rows[3].displayName = %q, want legacy-shell", rows[3].displayName)
	}
	if rows[4].status != "running" {
		t.Fatalf("rows[4].status = %q, want running", rows[4].status)
	}
}

func TestBuildTreeRowsKeepsEmptyFoldersVisible(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "Empty",
		Path:      "/tmp/empty",
		Namespace: "empty",
	}}}

	rows := buildTreeRows(cfg, map[int][]tmux.Session{}, map[string]tmux.Session{})
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0].typeOf != rowFolder {
		t.Fatalf("rows[0] = %#v, want folder row", rows[0])
	}
}

func TestCommandRowUsesShellIdleAsStoppedState(t *testing.T) {
	cfg := config.Config{Folders: []config.Folder{{
		Name:      "API",
		Path:      "/tmp/api",
		Namespace: "api",
		Commands:  []config.Command{{Name: "start", Command: "make start"}},
	}}}

	rows := buildTreeRows(
		cfg,
		map[int][]tmux.Session{0: {{Name: "api/cmd-start", CurrentCommand: "zsh"}}},
		map[string]tmux.Session{"api/cmd-start": {Name: "api/cmd-start", CurrentCommand: "zsh"}},
	)

	if rows[1].status != "stopped" {
		t.Fatalf("command row status = %q, want stopped", rows[1].status)
	}
}
