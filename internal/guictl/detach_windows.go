//go:build windows

package guictl

import "os/exec"

// applyDetach is a no-op on Windows.
func applyDetach(cmd *exec.Cmd) {}
