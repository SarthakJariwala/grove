package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	EditorCommand string   `toml:"editor_command"`
	Folders       []Folder `toml:"folder"`
}

type Folder struct {
	Name           string `toml:"name"`
	Path           string `toml:"path"`
	DefaultCommand string `toml:"default_command"`
	EditorCommand  string `toml:"editor_command"`
	Namespace      string `toml:"-"`
}

func (c *Config) Normalize(baseDir string) error {
	c.EditorCommand = strings.TrimSpace(c.EditorCommand)
	seen := map[string]string{}
	for i := range c.Folders {
		folder := &c.Folders[i]
		folder.Name = strings.TrimSpace(folder.Name)
		folder.Path = strings.TrimSpace(folder.Path)
		folder.DefaultCommand = strings.TrimSpace(folder.DefaultCommand)
		folder.EditorCommand = strings.TrimSpace(folder.EditorCommand)

		if folder.Name == "" {
			return fmt.Errorf("folder[%d] name is required", i)
		}
		if folder.Path == "" {
			return fmt.Errorf("folder[%d] path is required", i)
		}

		folder.Path = ExpandHome(folder.Path)

		if !filepath.IsAbs(folder.Path) {
			folder.Path = filepath.Join(baseDir, folder.Path)
		}

		absPath, err := filepath.Abs(folder.Path)
		if err != nil {
			return fmt.Errorf("resolve path for folder %q: %w", folder.Name, err)
		}
		folder.Path = absPath

		namespace := Slug(folder.Name)
		if namespace == "" {
			return fmt.Errorf("folder %q produced empty namespace", folder.Name)
		}
		if existing, exists := seen[namespace]; exists {
			return fmt.Errorf("folder %q conflicts with folder %q (both produce namespace %q)", folder.Name, existing, namespace)
		}
		seen[namespace] = folder.Name
		folder.Namespace = namespace
	}

	return nil
}

func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}

		if lastDash {
			continue
		}
		b.WriteByte('-')
		lastDash = true
	}

	out := strings.Trim(b.String(), "-")
	return out
}

func ExpandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
