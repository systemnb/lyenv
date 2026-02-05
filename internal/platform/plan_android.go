//go:build android

package platform

import (
	"errors"
	"os"
	"path/filepath"
)

func DecidePlan(prefix, name string) (InstallPlan, error) {
	// Termux: use $PREFIX/bin
	target := prefix
	if target == "" {
		p := os.Getenv("PREFIX")
		if p == "" {
			return InstallPlan{}, errors.New("PREFIX is empty; are you in Termux?")
		}
		target = filepath.Join(p, "bin")
	}
	return InstallPlan{
		TargetDir: target,
		BinName:   name,
		FileMode:  0o755,
		Notes:     nil,
	}, nil
}
