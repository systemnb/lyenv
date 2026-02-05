// internal/platform/plan_stub.go
//go:build !windows && !linux && !android

package platform

import "fmt"

// DecidePlan is only compiled on unsupported platforms as a fallback stub.
func DecidePlan(prefix, name string) (InstallPlan, error) {
	return InstallPlan{}, fmt.Errorf("unsupported platform: no DecidePlan implementation for this GOOS")
}
