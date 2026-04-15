package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareFolderName(t *testing.T) {
	t.Parallel()

	folder, err := PrepareFolderName(" Web ", []Folder{{Name: "API", Namespace: "api"}})
	if err != nil {
		t.Fatalf("PrepareFolderName() error = %v", err)
	}
	if folder.Name != "Web" || folder.Namespace != "web" {
		t.Fatalf("folder = %#v, want trimmed name and namespace", folder)
	}

	_, err = PrepareFolderName(" API ", []Folder{{Name: "API", Namespace: "api"}})
	if err == nil || !strings.Contains(err.Error(), `namespace "api" already exists`) {
		t.Fatalf("duplicate PrepareFolderName() error = %v, want namespace already exists", err)
	}
}

func TestPrepareFolderPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	workspace := filepath.Join(home, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	filePath := filepath.Join(home, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := PrepareFolderPath("~/workspace")
	if err != nil {
		t.Fatalf("PrepareFolderPath() error = %v", err)
	}
	if got != workspace {
		t.Fatalf("PrepareFolderPath() = %q, want %q", got, workspace)
	}

	if _, err := PrepareFolderPath(filepath.Join(home, "missing")); err == nil || !strings.Contains(err.Error(), "path not found") {
		t.Fatalf("missing path error = %v, want path not found", err)
	}
	if _, err := PrepareFolderPath(filePath); err == nil || err.Error() != "path is not a directory" {
		t.Fatalf("file path error = %v, want path is not a directory", err)
	}
}

func TestPrepareAgent(t *testing.T) {
	t.Parallel()

	agent, err := PrepareAgent(Agent{Name: " Amp ", Command: " amp "})
	if err != nil {
		t.Fatalf("PrepareAgent() error = %v", err)
	}
	if agent.Name != "Amp" || agent.Command != "amp" {
		t.Fatalf("agent = %#v, want trimmed fields", agent)
	}
}

func TestAppendFolderAgentDedupesBySlug(t *testing.T) {
	t.Parallel()

	cfg := Config{Folders: []Folder{{Name: "API", Namespace: "api", Agents: []Agent{{Name: "Codex", Command: "codex"}}}}}
	if err := AppendFolderAgent(&cfg, 0, Agent{Name: " codex ", Command: " codex --fast "}); err != nil {
		t.Fatalf("AppendFolderAgent() error = %v", err)
	}
	if len(cfg.Folders[0].Agents) != 1 {
		t.Fatalf("agents = %#v, want duplicate agent ignored", cfg.Folders[0].Agents)
	}
}

func TestAppendCommand(t *testing.T) {
	t.Parallel()

	cfg := Config{Folders: []Folder{{Name: "API", Namespace: "api", Commands: []Command{{Name: "start", Command: "make start"}}}}}
	if err := AppendCommand(&cfg, 0, Command{Name: " Dev ", Command: " make dev "}); err != nil {
		t.Fatalf("AppendCommand() error = %v", err)
	}
	if got := cfg.Folders[0].Commands[1]; got.Name != "Dev" || got.Command != "make dev" {
		t.Fatalf("command = %#v, want trimmed appended command", got)
	}

	err := AppendCommand(&cfg, 0, Command{Name: " dev ", Command: "make another"})
	if err == nil || err.Error() != "command name already exists" {
		t.Fatalf("duplicate AppendCommand() error = %v, want command name already exists", err)
	}
}
