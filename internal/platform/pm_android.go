// internal/platform/pm_android.go
//go:build android

package platform

import "os/exec"

// DetectPM finds a preferred package manager on Android (Termux).
func DetectPM() (string, bool) {
	if _, err := exec.LookPath("pkg"); err == nil {
		return "pkg", true
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return "apt", true
	}
	return "", false
}
