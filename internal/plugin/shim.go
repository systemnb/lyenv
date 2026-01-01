package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func CreateShims(envDir, installName string, expose []string) error {
	binDir := filepath.Join(envDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	for _, name := range expose {
		if runtime.GOOS == "windows" {
			if err := createCmdShim(binDir, name, installName); err != nil {
				return err
			}
			if err := createPsShim(binDir, name, installName); err != nil {
				return err
			}
		} else {
			if err := createUnixShim(binDir, name, installName); err != nil {
				return err
			}
		}
	}
	return nil
}

func createUnixShim(binDir, shimName, installName string) error {
	shimPath := filepath.Join(binDir, shimName)
	content := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
exec lyenv run %s "$@"
`, installName)
	return os.WriteFile(shimPath, []byte(content), 0o755)
}

func createCmdShim(binDir, shimName, installName string) error {
	shimPath := filepath.Join(binDir, shimName+".cmd")
	content := fmt.Sprintf(`@echo off
lyenv run %s %%*
`, installName)
	return os.WriteFile(shimPath, []byte(content), 0o644)
}

func createPsShim(binDir, shimName, installName string) error {
	shimPath := filepath.Join(binDir, shimName+".ps1")
	content := fmt.Sprintf(`#!/usr/bin/env pwsh
lyenv run %s $args
`, installName)
	return os.WriteFile(shimPath, []byte(content), 0o644)
}



func DeleteShims(envDir string, expose []string) error {
    binDir := filepath.Join(envDir, "bin")
    for _, name := range expose {
        paths := []string{
            filepath.Join(binDir, name),
            filepath.Join(binDir, name+".cmd"),
            filepath.Join(binDir, name+".ps1"),
        }
        for _, p := range paths {
            _ = os.Remove(p)
        }
    }
    return nil
}

