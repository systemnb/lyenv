package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DispatchRecord struct {
	ID            string   `json:"id"`
	TS            string   `json:"ts"`
	Plugin        string   `json:"plugin"`
	Command       string   `json:"command"`
	Args          []string `json:"args"`
	Status        string   `json:"status"`
	LogFile       string   `json:"log_file"`
	PluginLogFile string   `json:"plugin_log_file,omitempty"`
	Result        string   `json:"result,omitempty"`
	DurationMS    int64    `json:"duration_ms"`
}

func execLyenv(envPath string, args []string) (string, string, error) {
	ly, err := findLyenvBin()
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command(ly, args...)
	cmd.Dir = envPath

	var out bytes.Buffer
	var er bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &er

	err = cmd.Run()
	return out.String(), er.String(), err
}

func installZipPlugin(envPath, installName string, zipBytes []byte) error {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.zip", installName, time.Now().UnixNano()))
	if err := os.WriteFile(tmp, zipBytes, 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)

	// Use: lyenv plugin add <ZIP> --name=<installName>
	args := []string{"plugin", "add", tmp, "--name=" + installName}
	_, stderr, err := execLyenv(envPath, args)
	if err != nil {
		return fmt.Errorf("plugin add failed: %v stderr=%s", err, stderr)
	}
	return nil
}

func runInstalled(envPath, installName, command string, passArgs []string) (*DispatchRecord, error) {
	// Use: lyenv run <installName> <command> --json -- ...args
	// NOTE: your lyenv run currently treats any --xxx as flags and others as args;
	// we pass args as positional (no leading --).
	args := []string{"run", installName, command, "--json"}
	args = append(args, passArgs...)

	stdout, stderr, err := execLyenv(envPath, args)
	if err != nil {
		return nil, fmt.Errorf("run failed: %v stderr=%s", err, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	var rec DispatchRecord
	found := false
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" {
			continue
		}
		if json.Unmarshal([]byte(s), &rec) == nil && rec.ID != "" {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("invalid --json output: raw=%s", stdout)
	}
	return &rec, nil
}

func removeInstalled(envPath, installName string) {
	_, _, _ = execLyenv(envPath, []string{"plugin", "remove", installName, "--force"})
}

func decodeZipB64(zipB64 string) ([]byte, error) {
	zipB64 = strings.TrimSpace(zipB64)
	if zipB64 == "" {
		return nil, fmt.Errorf("zipB64 empty")
	}
	return base64.StdEncoding.DecodeString(zipB64)
}
