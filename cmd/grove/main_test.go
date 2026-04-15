package main

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := defaultConfigPath()
	want := filepath.Join(home, ".config", "grove", "config.toml")
	if got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPathUsesXDGConfigHome(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	got := defaultConfigPath()
	want := filepath.Join(xdg, "grove", "config.toml")
	if got != want {
		t.Fatalf("defaultConfigPath() = %q, want %q", got, want)
	}
}
