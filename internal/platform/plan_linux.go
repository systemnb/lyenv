//go:build linux

package platform

import (
	"errors"
	"os"
	"path/filepath"
)

func DecidePlan(prefix, name string) (InstallPlan, error) {
	var notes []string
	target := ""
	if prefix != "" {
		target = prefix
	} else {
		home, _ := os.UserHomeDir()
		candidates := []string{}
		if home != "" {
			candidates = append(candidates, filepath.Join(home, ".local", "bin"))
		}
		candidates = append(candidates, "/usr/local/bin")

		for _, c := range candidates {
			if err := EnsureDir(c, 0o755); err == nil {
				test := filepath.Join(c, ".lyenv_write_test")
				if err := os.WriteFile(test, []byte("ok"), 0o644); err == nil {
					_ = os.Remove(test)
					target = c
					break
				}
			}
		}
		if target == "" && home != "" {
			target = filepath.Join(home, ".local", "bin")
			notes = append(notes, "Creating ~/.local/bin as fallback (no root privileges).")
		}
	}
	if target == "" {
		return InstallPlan{}, errors.New("cannot determine a writable install directory")
	}
	return InstallPlan{
		TargetDir: target,
		BinName:   name,
		FileMode:  0o755,
		Notes:     notes,
	}, nil
}
