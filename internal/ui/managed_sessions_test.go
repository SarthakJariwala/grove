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

func TestParseManagedSessionRejectsInvalidManagedNames(t *testing.T) {
	tests := []string{
		"api/agent-2",
		"api/agent-codex-+3",
		"api/term-foo-9",
		"api/term--3",
		"api/term-+3",
		"api/agent-codex--2",
		"api/cmd-",
		"api/cmd-Start!",
		"web/agent-codex-1",
	}

	for _, sessionName := range tests {
		t.Run(sessionName, func(t *testing.T) {
			if _, ok := parseManagedSession("api", sessionName); ok {
				t.Fatalf("parseManagedSession(%q) ok = true, want false", sessionName)
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

func TestNextAgentIndex(t *testing.T) {
	folder := config.Folder{Namespace: "api"}
	sessions := []tmux.Session{{Name: "api/agent-codex-1"}, {Name: "api/agent-codex-3"}, {Name: "api/agent-amp-1"}}
	if got := nextAgentIndex(folder, "Codex", sessions); got != 4 {
		t.Fatalf("nextAgentIndex() = %d, want 4", got)
	}
}
