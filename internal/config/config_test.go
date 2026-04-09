package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "simple", in: "Main API", want: "main-api"},
		{name: "trims and lowers", in: "  Hello_World  ", want: "hello-world"},
		{name: "collapses separators", in: "a---b___c", want: "a-b-c"},
		{name: "empty when no alnum", in: "---", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Slug(tt.in)
			if got != tt.want {
				t.Fatalf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := ExpandHome("~"), home; got != want {
		t.Fatalf("ExpandHome(~) = %q, want %q", got, want)
	}

	if got, want := ExpandHome("~/projects/app"), filepath.Join(home, "projects", "app"); got != want {
		t.Fatalf("ExpandHome(~/projects/app) = %q, want %q", got, want)
	}

	if got, want := ExpandHome("/abs/path"), "/abs/path"; got != want {
		t.Fatalf("ExpandHome(/abs/path) = %q, want %q", got, want)
	}
}

func TestConfigNormalize(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	cfg := Config{
		EditorCommand: "  code .  ",
		Folders: []Folder{
			{
				Name:          " Main API ",
				Path:          " ./api ",
				EditorCommand: "  zed .  ",
			},
		},
	}

	if err := cfg.Normalize(base); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if cfg.EditorCommand != "code ." {
		t.Fatalf("EditorCommand = %q, want %q", cfg.EditorCommand, "code .")
	}

	f := cfg.Folders[0]
	if f.Name != "Main API" {
		t.Fatalf("folder.Name = %q, want %q", f.Name, "Main API")
	}
	if f.EditorCommand != "zed ." {
		t.Fatalf("folder.EditorCommand = %q, want %q", f.EditorCommand, "zed .")
	}
	if f.Namespace != "main-api" {
		t.Fatalf("folder.Namespace = %q, want %q", f.Namespace, "main-api")
	}

	wantPath, err := filepath.Abs(filepath.Join(base, "./api"))
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	if f.Path != wantPath {
		t.Fatalf("folder.Path = %q, want %q", f.Path, wantPath)
	}
}

func TestConfigNormalizeErrors(t *testing.T) {
	t.Parallel()

	base := t.TempDir()

	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing folder name",
			cfg:     Config{Folders: []Folder{{Path: "./x"}}},
			wantErr: "name is required",
		},
		{
			name:    "missing folder path",
			cfg:     Config{Folders: []Folder{{Name: "x"}}},
			wantErr: "path is required",
		},
		{
			name:    "conflicting namespace",
			cfg:     Config{Folders: []Folder{{Name: "My App", Path: "./a"}, {Name: "my-app", Path: "./b"}}},
			wantErr: "conflicts with",
		},
		{
			name:    "empty namespace",
			cfg:     Config{Folders: []Folder{{Name: "---", Path: "./a"}}},
			wantErr: "produced empty namespace",
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

func TestConfigNormalizeTrimsAgentsAndCommands(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cfg := Config{
		EditorCommand: " code . ",
		Agents:        []Agent{{Name: " Codex ", Command: " codex "}},
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
			name:    "global agent whitespace name",
			cfg:     Config{Agents: []Agent{{Name: "   ", Command: "codex"}}},
			wantErr: "agent[0] name is required",
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
			name: "folder agent whitespace command",
			cfg: Config{Folders: []Folder{{
				Name:   "API",
				Path:   "./api",
				Agents: []Agent{{Name: "Amp", Command: "   "}},
			}}},
			wantErr: "folder[0] agent[0] command is required",
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
		{
			name: "folder command whitespace name",
			cfg: Config{Folders: []Folder{{
				Name:     "API",
				Path:     "./api",
				Commands: []Command{{Name: "   ", Command: "make start"}},
			}}},
			wantErr: "folder[0] command[0] name is required",
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
