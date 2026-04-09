package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/SarthakJariwala/grove/internal/config"
	"github.com/SarthakJariwala/grove/internal/tmux"
)

type managedSessionKind int

const (
	managedUnknown managedSessionKind = iota
	managedAgent
	managedTerminal
	managedCommand
)

type managedSessionID struct {
	kind  managedSessionKind
	slug  string
	index int
}

func agentSessionName(folder config.Folder, slug string, index int) string {
	return fmt.Sprintf("%s/agent-%s-%d", folder.Namespace, sanitizeLeaf(slug), index)
}

func terminalSessionName(folder config.Folder, index int) string {
	return fmt.Sprintf("%s/term-%d", folder.Namespace, index)
}

func commandSessionName(folder config.Folder, slug string) string {
	return fmt.Sprintf("%s/cmd-%s", folder.Namespace, sanitizeLeaf(slug))
}

func parseManagedSession(namespace, fullName string) (managedSessionID, bool) {
	prefix := namespace + "/"
	if !strings.HasPrefix(fullName, prefix) {
		return managedSessionID{}, false
	}
	leaf := strings.TrimPrefix(fullName, prefix)

	if slug, index, ok := parseIndexedLeaf(leaf, "agent-"); ok {
		if slug == "" || sanitizeLeaf(slug) != slug || index <= 0 {
			return managedSessionID{}, false
		}
		return managedSessionID{kind: managedAgent, slug: slug, index: index}, true
	}
	if strings.HasPrefix(leaf, "cmd-") {
		slug := strings.TrimPrefix(leaf, "cmd-")
		if slug == "" || sanitizeLeaf(slug) != slug {
			return managedSessionID{}, false
		}
		return managedSessionID{kind: managedCommand, slug: slug}, true
	}
	if strings.HasPrefix(leaf, "term-") {
		rawIndex := strings.TrimPrefix(leaf, "term-")
		if rawIndex == "" || strings.Contains(rawIndex, "-") || strings.HasPrefix(rawIndex, "+") {
			return managedSessionID{}, false
		}
		index, err := strconv.Atoi(rawIndex)
		if err != nil || index <= 0 {
			return managedSessionID{}, false
		}
		return managedSessionID{kind: managedTerminal, index: index}, true
	}

	return managedSessionID{}, false
}

func parseIndexedLeaf(leaf, prefix string) (string, int, bool) {
	if !strings.HasPrefix(leaf, prefix) {
		return "", 0, false
	}
	remainder := strings.TrimPrefix(leaf, prefix)
	lastDash := strings.LastIndex(remainder, "-")
	if lastDash == -1 {
		index, err := strconv.Atoi(remainder)
		return "", index, err == nil
	}

	rawIndex := remainder[lastDash+1:]
	if rawIndex == "" || strings.Contains(rawIndex, "-") || strings.HasPrefix(rawIndex, "+") {
		return "", 0, false
	}

	index, err := strconv.Atoi(rawIndex)
	if err != nil {
		return "", 0, false
	}
	return remainder[:lastDash], index, true
}

func nextTerminalIndex(folder config.Folder, sessions []tmux.Session) int {
	maxIndex := 0
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if ok && id.kind == managedTerminal && id.index > maxIndex {
			maxIndex = id.index
		}
	}
	return maxIndex + 1
}

func nextAgentIndex(folder config.Folder, agentName string, sessions []tmux.Session) int {
	slug := sanitizeLeaf(agentName)
	maxIndex := 0
	for _, session := range sessions {
		id, ok := parseManagedSession(folder.Namespace, session.Name)
		if ok && id.kind == managedAgent && id.slug == slug && id.index > maxIndex {
			maxIndex = id.index
		}
	}
	return maxIndex + 1
}
