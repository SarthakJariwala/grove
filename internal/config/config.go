package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Agent struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
}

type Command struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
}

type Config struct {
	EditorCommand string   `toml:"editor_command"`
	Agents        []Agent  `toml:"agent"`
	Folders       []Folder `toml:"folder"`
}

type Folder struct {
	Name          string    `toml:"name"`
	Path          string    `toml:"path"`
	EditorCommand string    `toml:"editor_command"`
	Agents        []Agent   `toml:"agent"`
	Commands      []Command `toml:"command"`
	Namespace     string    `toml:"-"`
}

func (c *Config) Normalize(baseDir string) error {
	c.EditorCommand = strings.TrimSpace(c.EditorCommand)
	for i := range c.Agents {
		if err := normalizeAgent(&c.Agents[i], fmt.Sprintf("agent[%d]", i)); err != nil {
			return err
		}
	}

	seen := map[string]string{}
	for i := range c.Folders {
		folder := &c.Folders[i]
		folder.Name = strings.TrimSpace(folder.Name)
		folder.Path = strings.TrimSpace(folder.Path)
		folder.EditorCommand = strings.TrimSpace(folder.EditorCommand)

		if folder.Name == "" {
			return fmt.Errorf("folder[%d] name is required", i)
		}
		if folder.Path == "" {
			return fmt.Errorf("folder[%d] path is required", i)
		}

		for j := range folder.Agents {
			if err := normalizeAgent(&folder.Agents[j], fmt.Sprintf("folder[%d] agent[%d]", i, j)); err != nil {
				return err
			}
		}
		for j := range folder.Commands {
			if err := normalizeCommand(&folder.Commands[j], fmt.Sprintf("folder[%d] command[%d]", i, j)); err != nil {
				return err
			}
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

func normalizeAgent(agent *Agent, scope string) error {
	agent.Name = strings.TrimSpace(agent.Name)
	agent.Command = strings.TrimSpace(agent.Command)
	if agent.Name == "" {
		return fmt.Errorf("%s name is required", scope)
	}
	if agent.Command == "" {
		return fmt.Errorf("%s command is required", scope)
	}
	return nil
}

func normalizeCommand(command *Command, scope string) error {
	command.Name = strings.TrimSpace(command.Name)
	command.Command = strings.TrimSpace(command.Command)
	if command.Name == "" {
		return fmt.Errorf("%s name is required", scope)
	}
	if command.Command == "" {
		return fmt.Errorf("%s command is required", scope)
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
