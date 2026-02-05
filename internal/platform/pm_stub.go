// internal/platform/pm_stub.go
//go:build !windows && !linux && !android

package platform

// DetectPM stub for unsupported platforms.
func DetectPM() (string, bool) { return "", false }
