package ui

import (
	"os/exec"

	"github.com/SarthakJariwala/grove/internal/tmux"
)

type sessionManager interface {
	LoadSnapshot() (tmux.SessionSnapshot, error)
	NewSession(name, cwd string) error
	NewSessionWithCommand(name, cwd, command string) error
	SendKeys(target, command string) error
	RenameSession(oldName, newName string) error
	KillSession(name string) error
	CapturePane(target string) (string, error)
	AttachCommand(name string) *exec.Cmd
}
