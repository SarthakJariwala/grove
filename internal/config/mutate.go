package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PrepareFolderName(name string, existing []Folder) (Folder, error) {
	folder := Folder{Name: strings.TrimSpace(name)}
	if folder.Name == "" {
		return Folder{}, fmt.Errorf("folder name is required")
	}

	folder.Namespace = Slug(folder.Name)
	if folder.Namespace == "" {
		return Folder{}, fmt.Errorf("folder name produced empty namespace")
	}

	for _, existingFolder := range existing {
		namespace := existingFolder.Namespace
		if namespace == "" {
			namespace = Slug(existingFolder.Name)
		}
		if namespace == folder.Namespace {
			return Folder{}, fmt.Errorf("namespace %q already exists", folder.Namespace)
		}
	}

	return folder, nil
}

func PrepareFolderPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("folder path is required")
	}

	absPath, err := filepath.Abs(ExpandHome(path))
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("path not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}

	return absPath, nil
}

func PrepareAgent(agent Agent) (Agent, error) {
	if err := normalizeAgent(&agent, "agent"); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func PrepareCommand(command Command) (Command, error) {
	command.Name = strings.TrimSpace(command.Name)
	command.Command = strings.TrimSpace(command.Command)
	if command.Name == "" {
		return Command{}, fmt.Errorf("command name is required")
	}
	if command.Command == "" {
		return Command{}, fmt.Errorf("command is required")
	}
	return command, nil
}

func AppendFolder(cfg *Config, folder Folder) error {
	prepared, err := PrepareFolderName(folder.Name, cfg.Folders)
	if err != nil {
		return err
	}
	prepared.Path = strings.TrimSpace(folder.Path)
	if prepared.Path == "" {
		return fmt.Errorf("folder path is required")
	}
	prepared.EditorCommand = strings.TrimSpace(folder.EditorCommand)
	prepared.Agents = append([]Agent(nil), folder.Agents...)
	prepared.Commands = append([]Command(nil), folder.Commands...)
	cfg.Folders = append(cfg.Folders, prepared)
	return nil
}

func AppendFolderAgent(cfg *Config, folderIndex int, agent Agent) error {
	if folderIndex < 0 || folderIndex >= len(cfg.Folders) {
		return fmt.Errorf("select a folder")
	}

	prepared, err := PrepareAgent(agent)
	if err != nil {
		return err
	}

	key := Slug(prepared.Name)
	for _, existing := range cfg.Folders[folderIndex].Agents {
		if Slug(existing.Name) == key {
			return nil
		}
	}

	agents := append([]Agent(nil), cfg.Folders[folderIndex].Agents...)
	cfg.Folders[folderIndex].Agents = append(agents, prepared)
	return nil
}

func CommandNameExists(folder Folder, name string) bool {
	key := Slug(name)
	if key == "" {
		return false
	}

	for _, existing := range folder.Commands {
		if Slug(existing.Name) == key {
			return true
		}
	}

	return false
}

func AppendCommand(cfg *Config, folderIndex int, command Command) error {
	if folderIndex < 0 || folderIndex >= len(cfg.Folders) {
		return fmt.Errorf("select a folder")
	}

	prepared, err := PrepareCommand(command)
	if err != nil {
		return err
	}
	if CommandNameExists(cfg.Folders[folderIndex], prepared.Name) {
		return fmt.Errorf("command name already exists")
	}

	commands := append([]Command(nil), cfg.Folders[folderIndex].Commands...)
	cfg.Folders[folderIndex].Commands = append(commands, prepared)
	return nil
}
