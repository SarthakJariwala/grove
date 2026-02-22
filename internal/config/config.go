package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Folders []Folder `toml:"folder"`
}

type Folder struct {
	Name           string `toml:"name"`
	Path           string `toml:"path"`
	DefaultCommand string `toml:"default_command"`
	Namespace      string
}

func Load(path string) (Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	if err := cfg.normalize(filepath.Dir(path)); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) normalize(baseDir string) error {
	seen := map[string]struct{}{}
	for i := range c.Folders {
		folder := &c.Folders[i]
		folder.Name = strings.TrimSpace(folder.Name)
		folder.Path = strings.TrimSpace(folder.Path)
		folder.DefaultCommand = strings.TrimSpace(folder.DefaultCommand)

		if folder.Name == "" {
			return fmt.Errorf("folder[%d] name is required", i)
		}
		if folder.Path == "" {
			return fmt.Errorf("folder[%d] path is required", i)
		}

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
		if _, exists := seen[namespace]; exists {
			return fmt.Errorf("folder %q conflicts with another folder namespace %q", folder.Name, namespace)
		}
		seen[namespace] = struct{}{}
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

func AppendFolder(path string, f Folder) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open config for append: %w", err)
	}
	defer file.Close()

	block := fmt.Sprintf("\n[[folder]]\nname = %q\npath = %q\n", f.Name, f.Path)
	if f.DefaultCommand != "" {
		block += fmt.Sprintf("default_command = %q\n", f.DefaultCommand)
	}

	if _, err := file.WriteString(block); err != nil {
		return fmt.Errorf("write folder block: %w", err)
	}
	return nil
}

func EnsureTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	const tmpl = `# Example grove config
#
# [[folder]]
# name = "Main API"
# path = "/Users/you/dev/main-api"
# default_command = "bin/dev"
`

	return os.WriteFile(path, []byte(tmpl), 0o644)
}
