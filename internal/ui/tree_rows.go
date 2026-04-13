package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type sectionKind int

const (
	sectionNone sectionKind = iota
	sectionAgents
	sectionTerminals
	sectionCommands
)

func buildTreeRows(cfg config.Config, sessions map[int][]tmux.Session, sessionByName map[string]tmux.Session) []treeRow {
	rows := make([]treeRow, 0)
	for folderIndex, folder := range cfg.Folders {
		rows = append(rows, treeRow{typeOf: rowFolder, folderIndex: folderIndex, displayName: folder.Name})
		rows = append(rows, buildAgentRows(folderIndex, folder, sessions[folderIndex])...)
		rows = append(rows, buildTerminalRows(folderIndex, folder, sessions[folderIndex])...)
		rows = append(rows, buildCommandRows(folderIndex, folder, sessionByName)...)
	}
	return rows
}

func buildCommandRows(folderIndex int, folder config.Folder, sessionByName map[string]tmux.Session) []treeRow {
	rows := make([]treeRow, 0, len(folder.Commands))
	for _, command := range folder.Commands {
		sessionName := commandSessionName(folder, command.Name)
		session, ok := sessionByName[sessionName]
		status := "stopped"
		attached := false
		windows := 0
		if ok {
			attached = session.Attached
			windows = session.Windows
			if commandSessionRunning(session) {
				status = "running"
			}
		}
		rows = append(rows, treeRow{
			typeOf:         rowCommand,
			section:        sectionCommands,
			folderIndex:    folderIndex,
			sessionName:    sessionName,
			displayName:    command.Name,
			commandText:    command.Command,
			status:         status,
			attached:       attached,
			windows:        windows,
			currentCommand: session.CurrentCommand,
			paneTitle:      session.PaneTitle,
			currentPath:    session.CurrentPath,
			lastActivity:   session.LastActivity,
			hasAlerts:      session.HasAlerts,
			alertsBell:     session.AlertsBell,
			alertsActivity: session.AlertsActivity,
			alertsSilence:  session.AlertsSilence,
		})
	}
	return rows
}

func buildAgentRows(folderIndex int, folder config.Folder, sessions []tmux.Session) []treeRow {
	rows := make([]treeRow, 0)
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if !ok || id.kind != managedAgent {
			continue
		}
		rows = append(rows, sessionTreeRow(folderIndex, sectionAgents, rowAgentInstance, session, fmt.Sprintf("%s #%d", titleSlug(id.slug), id.index)))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].sessionName < rows[j].sessionName })
	return rows
}

func buildTerminalRows(folderIndex int, folder config.Folder, sessions []tmux.Session) []treeRow {
	rows := make([]treeRow, 0)
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if ok && id.kind == managedCommand {
			continue
		}
		if ok && id.kind == managedAgent {
			continue
		}

		displayName := strings.TrimPrefix(session.Name, folder.Namespace+"/")
		if ok && id.kind == managedTerminal {
			displayName = fmt.Sprintf("Terminal #%d", id.index)
		}
		rows = append(rows, sessionTreeRow(folderIndex, sectionTerminals, rowTerminalInstance, session, displayName))
	}
	sort.Slice(rows, func(i, j int) bool {
		iManaged := strings.HasPrefix(rows[i].sessionName, folder.Namespace+"/term-")
		jManaged := strings.HasPrefix(rows[j].sessionName, folder.Namespace+"/term-")
		if iManaged != jManaged {
			return iManaged
		}
		return rows[i].sessionName < rows[j].sessionName
	})
	return rows
}

func sessionTreeRow(folderIndex int, section sectionKind, typeOf rowType, session tmux.Session, displayName string) treeRow {
	status := "detached"
	if session.Attached {
		status = "attached"
	}
	return treeRow{
		typeOf:         typeOf,
		section:        section,
		folderIndex:    folderIndex,
		sessionName:    session.Name,
		displayName:    displayName,
		status:         status,
		attached:       session.Attached,
		windows:        session.Windows,
		hasAlerts:      session.HasAlerts,
		alertsBell:     session.AlertsBell,
		alertsActivity: session.AlertsActivity,
		alertsSilence:  session.AlertsSilence,
		currentCommand: session.CurrentCommand,
		paneTitle:      session.PaneTitle,
		currentPath:    session.CurrentPath,
		lastActivity:   session.LastActivity,
	}
}

func commandSessionRunning(session tmux.Session) bool {
	command := strings.TrimSpace(session.CurrentCommand)
	return command != "" && !isShellCommand(command)
}

func titleSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}
