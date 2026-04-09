package configfile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/SarthakJariwala/grove/internal/config"
)

func Load(path string) (config.Config, error) {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return config.Config{}, err
	}

	var cfg config.Config
	if _, err := toml.DecodeFile(resolvedPath, &cfg); err != nil {
		return config.Config{}, fmt.Errorf("decode config %q: %w", resolvedPath, err)
	}

	if err := cfg.Normalize(filepath.Dir(resolvedPath)); err != nil {
		return config.Config{}, err
	}

	return cfg, nil
}

func Save(path string, cfg config.Config) error {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config directory %q: %w", dir, err)
	}

	file, err := os.CreateTemp(dir, filepath.Base(resolvedPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config for %q: %w", resolvedPath, err)
	}
	tempPath := file.Name()
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
		_ = os.Remove(tempPath)
	}()

	if err := toml.NewEncoder(file).Encode(cfg); err != nil {
		return fmt.Errorf("encode config %q: %w", resolvedPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temp config for %q: %w", resolvedPath, err)
	}
	closed = true
	if err := os.Rename(tempPath, resolvedPath); err != nil {
		return fmt.Errorf("replace config %q: %w", resolvedPath, err)
	}

	return nil
}

func resolveConfigPath(path string) (string, error) {
	resolvedPath := path
	for i := 0; i < 32; i++ {
		info, err := os.Lstat(resolvedPath)
		if err != nil {
			if os.IsNotExist(err) {
				return resolvedPath, nil
			}
			return "", fmt.Errorf("stat config %q: %w", resolvedPath, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return resolvedPath, nil
		}

		target, err := os.Readlink(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("read symlink config %q: %w", resolvedPath, err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(resolvedPath), target)
		}
		resolvedPath = filepath.Clean(target)
	}

	return "", fmt.Errorf("resolve symlink config %q: too many symlinks", path)
}

func AppendFolder(path string, f config.Folder) error {
	var cfg config.Config
	if _, err := os.Stat(path); err == nil {
		loaded, loadErr := Load(path)
		if loadErr != nil {
			return loadErr
		}
		cfg = loaded
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check config %q: %w", path, err)
	}

	cfg.Folders = append(cfg.Folders, f)
	return Save(path, cfg)
}

func EnsureTemplate(path string) error {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return fmt.Errorf("create config directory %q: %w", filepath.Dir(resolvedPath), err)
	}

	const tmpl = `# Example grove config
#
# editor_command = "code ."
#
# [[agent]]
# name = "Codex"
# command = "codex"
#
# [[folder]]
# name = "Main API"
# path = "/Users/you/dev/main-api"
#
# [[folder.agent]]
# name = "Amp"
# command = "amp"
#
# [[folder.command]]
# name = "start"
# command = "make start"
#
# editor_command = "zed ."
`

	file, err := os.OpenFile(resolvedPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("create config template %q: %w", resolvedPath, err)
	}
	if _, err := file.WriteString(tmpl); err != nil {
		_ = file.Close()
		_ = os.Remove(resolvedPath)
		return fmt.Errorf("write config template %q: %w", resolvedPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(resolvedPath)
		return fmt.Errorf("close config template %q: %w", resolvedPath, err)
	}
	return nil
}
