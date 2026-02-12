package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func findLyenvBin() (string, error) {
	// Prefer sibling of current executable (dist layout)
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "lyenv")); err == nil {
			return filepath.Join(dir, "lyenv"), nil
		}
		if _, err := os.Stat(filepath.Join(dir, "lyenv.exe")); err == nil {
			return filepath.Join(dir, "lyenv.exe"), nil
		}
	}
	return exec.LookPath("lyenv")
}

func runLyenv(envPath, plugin, command string, args []string) (*DispatchRecord, error) {
    ly, err := findLyenvBin()
    if err != nil {
        return nil, fmt.Errorf("lyenv not found: %w", err)
    }
    if plugin == "" || command == "" {
        return nil, fmt.Errorf("plugin/command required")
    }

    cmdArgs := []string{"run", plugin, command, "--json"}
    cmdArgs = append(cmdArgs, args...)

    cmd := exec.Command(ly, cmdArgs...)
    cmd.Dir = envPath

    var out bytes.Buffer
    var er bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &er

    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("lyenv run failed: %v stderr=%s", err, er.String())
    }

    // stdout is JSON
    var rec DispatchRecord
    if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &rec); err != nil {
        return nil, fmt.Errorf("invalid lyenv --json output: %v, raw=%s", err, out.String())
    }
    return &rec, nil
}

