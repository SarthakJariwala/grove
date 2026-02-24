package configfile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/SarthakJariwala/grove/internal/config"
)

func Load(path string) (config.Config, error) {
	var cfg config.Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return config.Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	if err := cfg.Normalize(filepath.Dir(path)); err != nil {
		return config.Config{}, err
	}

	return cfg, nil
}

func AppendFolder(path string, f config.Folder) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
		return fmt.Errorf("check config %q: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory %q: %w", filepath.Dir(path), err)
	}

	const tmpl = `# Example grove config
#
# [[folder]]
# name = "Main API"
# path = "/Users/you/dev/main-api"
# default_command = "bin/dev"
`

	if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
		return fmt.Errorf("write config template %q: %w", path, err)
	}
	return nil
}
