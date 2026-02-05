// internal/platform/pm_linux.go
//go:build linux

package platform

import "os/exec"

// DetectPM finds a preferred package manager on Linux.
func DetectPM() (string, bool) {
	candidates := []string{"apt", "apt-get", "dnf", "pacman", "zypper"}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c, true
		}
	}
	return "", false
}
