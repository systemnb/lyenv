//go:build windows

package platform

import (
	"os"
	"path/filepath"
)

func DecidePlan(prefix, name string) (InstallPlan, error) {
	target := ""
	if prefix != "" {
		target = prefix
	} else {
		// Prefer %LOCALAPPDATA%\lyenv\bin
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			// fallback to user profile
			if home, _ := os.UserHomeDir(); home != "" {
				base = filepath.Join(home, "AppData", "Local")
			}
		}
		target = filepath.Join(base, "lyenv", "bin")
	}
	return InstallPlan{
		TargetDir: target,
		BinName:   name,  // DecorateName 会在 oscommon.go 根据 GOOS 加 .exe
		FileMode:  0o755, //.mode ignored on Windows but keep consistent
		Notes:     nil,
	}, nil
}
