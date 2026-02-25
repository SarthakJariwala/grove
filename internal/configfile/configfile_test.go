package configfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SarthakJariwala/grove/internal/config"
)

func TestEnsureTemplateCreatesFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "grove", "config.toml")

	if err := EnsureTemplate(cfgPath); err != nil {
		t.Fatalf("EnsureTemplate() error = %v", err)
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	content := string(b)
	if !strings.Contains(content, "Example grove config") {
		t.Fatalf("template content missing expected header: %q", content)
	}
}

func TestEnsureTemplateDoesNotOverwriteExisting(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")

	const existing = "editor_command = \"vim .\"\n"
	if err := os.WriteFile(cfgPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := EnsureTemplate(cfgPath); err != nil {
		t.Fatalf("EnsureTemplate() error = %v", err)
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if got := string(b); got != existing {
		t.Fatalf("content changed; got %q want %q", got, existing)
	}
}

func TestAppendFolder(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")

	const base = "editor_command = \"code .\"\n"
	if err := os.WriteFile(cfgPath, []byte(base), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	f := config.Folder{
		Name:           "Main API",
		Path:           "/tmp/main-api",
		DefaultCommand: "make dev",
		EditorCommand:  "zed .",
	}

	if err := AppendFolder(cfgPath, f); err != nil {
		t.Fatalf("AppendFolder() error = %v", err)
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := string(b)
	wantContains := []string{
		base,
		"[[folder]]",
		"name = \"Main API\"",
		"path = \"/tmp/main-api\"",
		"default_command = \"make dev\"",
		"editor_command = \"zed .\"",
	}
	for _, piece := range wantContains {
		if !strings.Contains(got, piece) {
			t.Fatalf("AppendFolder() output missing %q in %q", piece, got)
		}
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	projects := filepath.Join(tmp, "projects")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	cfgPath := filepath.Join(tmp, "config.toml")
	content := strings.Join([]string{
		"editor_command = \"code .\"",
		"",
		"[[folder]]",
		"name = \"Main API\"",
		"path = \"./projects\"",
		"default_command = \"make dev\"",
	}, "\n")

	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Folders) != 1 {
		t.Fatalf("len(Folders) = %d, want 1", len(cfg.Folders))
	}

	f := cfg.Folders[0]
	wantPath, err := filepath.Abs(filepath.Join(tmp, "projects"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	if f.Path != wantPath {
		t.Fatalf("folder.Path = %q, want %q", f.Path, wantPath)
	}
	if f.Namespace != "main-api" {
		t.Fatalf("folder.Namespace = %q, want %q", f.Namespace, "main-api")
	}
}
