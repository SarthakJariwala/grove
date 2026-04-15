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

func TestEnsureTemplateCreatesTargetThroughDanglingSymlink(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "real", "nested", "config.toml")
	linkPath := filepath.Join(tmp, "config.toml")

	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if err := EnsureTemplate(linkPath); err != nil {
		t.Fatalf("EnsureTemplate() error = %v", err)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config path mode = %v, want symlink", info.Mode())
	}

	b, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(b), "Example grove config") {
		t.Fatalf("template content missing expected header: %q", string(b))
	}
}

func TestAppendFolder(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")

	if err := Save(cfgPath, config.Config{
		Agents: []config.Agent{{Name: "Codex", Command: "codex"}},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	f := config.Folder{
		Name:          " Main API ",
		Path:          "/tmp/main-api",
		EditorCommand: " zed . ",
	}

	if err := AppendFolder(cfgPath, f); err != nil {
		t.Fatalf("AppendFolder() error = %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Agents) != 1 || loaded.Agents[0].Name != "Codex" {
		t.Fatalf("loaded.Agents = %#v, want preserved global agents", loaded.Agents)
	}
	if len(loaded.Folders) != 1 || loaded.Folders[0].Name != "Main API" {
		t.Fatalf("loaded.Folders = %#v, want appended folder", loaded.Folders)
	}
	if loaded.Folders[0].EditorCommand != "zed ." {
		t.Fatalf("loaded.Folders[0].EditorCommand = %q, want %q", loaded.Folders[0].EditorCommand, "zed .")
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
		"[[agent]]",
		"name = \"Codex\"",
		"command = \"codex\"",
		"",
		"[[folder]]",
		"name = \"Main API\"",
		"path = \"./projects\"",
		"",
		"[[folder.agent]]",
		"name = \"Amp\"",
		"command = \"amp\"",
		"",
		"[[folder.command]]",
		"name = \"start\"",
		"command = \"make start\"",
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
	if len(cfg.Agents) != 1 || cfg.Agents[0].Command != "codex" {
		t.Fatalf("cfg.Agents = %#v, want one global agent", cfg.Agents)
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
	if len(f.Agents) != 1 || f.Agents[0].Command != "amp" {
		t.Fatalf("folder.Agents = %#v, want one folder agent", f.Agents)
	}
	if len(f.Commands) != 1 || f.Commands[0].Command != "make start" {
		t.Fatalf("folder.Commands = %#v, want one folder command", f.Commands)
	}
}

func TestSaveRoundTripsNestedAgentsAndCommands(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	want := config.Config{
		EditorCommand: "code .",
		Agents:        []config.Agent{{Name: "Codex", Command: "codex"}},
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

	if len(got.Folders) != 1 {
		t.Fatalf("len(got.Folders) = %d, want 1", len(got.Folders))
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

func TestSaveNormalizesBeforeWrite(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	projects := filepath.Join(tmp, "projects")
	if err := os.MkdirAll(projects, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	cfgPath := filepath.Join(tmp, "config.toml")
	if err := Save(cfgPath, config.Config{Folders: []config.Folder{{
		Name:          " API ",
		Path:          " ./projects ",
		EditorCommand: " zed . ",
	}}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(b)
	if !strings.Contains(content, `name = "API"`) {
		t.Fatalf("content = %q, want trimmed folder name", content)
	}
	if !strings.Contains(content, `editor_command = "zed ."`) {
		t.Fatalf("content = %q, want trimmed editor command", content)
	}
	if !strings.Contains(content, `path = "`+projects+`"`) {
		t.Fatalf("content = %q, want normalized absolute path", content)
	}
}

func TestSavePreservesSymlinkedConfigPath(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "real-config.toml")
	linkPath := filepath.Join(tmp, "config.toml")

	if err := os.WriteFile(targetPath, []byte("editor_command = \"code .\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if err := Save(linkPath, config.Config{
		Agents: []config.Agent{{Name: "Codex", Command: "codex"}},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config path mode = %v, want symlink", info.Mode())
	}

	loaded, err := Load(linkPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Agents) != 1 || loaded.Agents[0].Command != "codex" {
		t.Fatalf("loaded.Agents = %#v, want updated target contents", loaded.Agents)
	}
}

func TestSavePreservesDanglingSymlinkedConfigPath(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "real-config.toml")
	linkPath := filepath.Join(tmp, "config.toml")

	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if err := Save(linkPath, config.Config{
		Agents: []config.Agent{{Name: "Codex", Command: "codex"}},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat() error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config path mode = %v, want symlink", info.Mode())
	}

	loaded, err := Load(linkPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Agents) != 1 || loaded.Agents[0].Command != "codex" {
		t.Fatalf("loaded.Agents = %#v, want created target contents", loaded.Agents)
	}
}

func TestLoadNormalizesRelativePathsFromSymlinkTarget(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	realDir := filepath.Join(tmp, "real")
	linkDir := filepath.Join(tmp, "link")
	projectsDir := filepath.Join(realDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	realPath := filepath.Join(realDir, "config.toml")
	linkPath := filepath.Join(linkDir, "config.toml")
	content := strings.Join([]string{
		"[[folder]]",
		"name = \"Main API\"",
		"path = \"./projects\"",
	}, "\n")
	if err := os.WriteFile(realPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	loaded, err := Load(linkPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantPath, err := filepath.Abs(projectsDir)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	if got := loaded.Folders[0].Path; got != wantPath {
		t.Fatalf("folder.Path = %q, want %q", got, wantPath)
	}
}
