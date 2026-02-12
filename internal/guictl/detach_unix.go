//go:build !windows

package guictl

import (
	"os/exec"
	"syscall"
)

// applyDetach detaches the child process from the current session on Unix-like systems.
func applyDetach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
