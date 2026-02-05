// internal/platform/pm_windows.go
//go:build windows

package platform

import "os/exec"

// DetectPM finds a preferred package manager on Windows.
func DetectPM() (string, bool) {
	if _, err := exec.LookPath("winget"); err == nil {
		return "winget", true
	}
	if _, err := exec.LookPath("choco"); err == nil {
		return "choco", true
	}
	if _, err := exec.LookPath("scoop"); err == nil {
		return "scoop", true
	}
	return "", false
}
