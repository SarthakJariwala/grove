package tmuxconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDefaultCreatesXDGConfigWhenMissing(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	created, path, err := EnsureDefault()
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if !created {
		t.Fatalf("EnsureDefault() created = %v, want true", created)
	}

	wantPath := filepath.Join(xdg, "tmux", "tmux.conf")
	if path != wantPath {
		t.Fatalf("EnsureDefault() path = %q, want %q", path, wantPath)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("written tmux.conf is empty")
	}
}

func TestEnsureDefaultSkipsWhenLegacyExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

	legacyPath := filepath.Join(home, ".tmux.conf")
	if err := os.WriteFile(legacyPath, []byte("set -g mouse on\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	created, path, err := EnsureDefault()
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if created {
		t.Fatalf("EnsureDefault() created = %v, want false", created)
	}
	if path != "" {
		t.Fatalf("EnsureDefault() path = %q, want empty", path)
	}
}

func TestEnsureDefaultSkipsWhenXDGExists(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	xdgPath := filepath.Join(xdg, "tmux", "tmux.conf")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(xdgPath, []byte("set -g history-limit 10000\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	created, path, err := EnsureDefault()
	if err != nil {
		t.Fatalf("EnsureDefault() error = %v", err)
	}
	if created {
		t.Fatalf("EnsureDefault() created = %v, want false", created)
	}
	if path != "" {
		t.Fatalf("EnsureDefault() path = %q, want empty", path)
	}
}
