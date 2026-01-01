package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// lyenv version (can override with -ldflags)
const lyenvVersion = "0.1.0"

func usage() {
	fmt.Fprintf(os.Stderr, `lyenv - Directory-based isolated environment manager (MVP)
Usage:
  lyenv create <DIR>     Create a new lyenv environment directory
`)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "create":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Error: create requires exactly 1 argument <DIR>")
			os.Exit(2)
		}
		dir := strings.TrimSpace(args[1])
		if dir == "" {
			fmt.Fprintln(os.Stderr, "Error: <DIR> must not be empty")
			os.Exit(2)
		}
		if err := cmdCreate(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Create failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Environment created successfully: %s\n", dir)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

// cmdCreate creates the directory structure for a lyenv environment.
func cmdCreate(dir string) error {
	absDir, errAbs := filepath.Abs(dir)
	if errAbs != nil {
		return fmt.Errorf("failed to resolve target path: %w", errAbs)
	}

	// Debug hint
	// fmt.Printf("Debug: resolved absolute path: %s\n", absDir)

	// Check if target exists
	fi, errStat := os.Stat(absDir)
	if errStat == nil {
		// Target exists
		if !fi.IsDir() {
			return fmt.Errorf("target exists but is not a directory: %s", absDir)
		}
		// Is a directory — check if already a lyenv dir
		if isLyenvDir(absDir) {
			fmt.Println("Note: target directory already looks like a lyenv environment. Skipped.")
			return nil
		}
	} else if !errors.Is(errStat, os.ErrNotExist) {
		// Stat failed for reasons other than "not exists"
		return fmt.Errorf("failed to stat target directory: %w", errStat)
	}

	// Create base structure (mkdir -p style)
	subdirs := []string{
		filepath.Join(absDir, ".lyenv"),
		filepath.Join(absDir, ".lyenv", "logs"),
		filepath.Join(absDir, ".lyenv", "registry"),
		filepath.Join(absDir, "bin"),
		filepath.Join(absDir, "cache"),
		filepath.Join(absDir, "plugins"),
		filepath.Join(absDir, "workspace"),
	}
	for _, d := range subdirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Write metadata
	if err := writeFileIfNotExists(filepath.Join(absDir, ".lyenv", "version"), lyenvVersion+"\n", 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/version: %w", err)
	}
	stateJSON := `{
  "created_at": "` + time.Now().Local().String() + `",
  "components": [],
  "notes": ""
}
`
	if err := writeFileIfNotExists(filepath.Join(absDir, ".lyenv", "state.json"), stateJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/state.json: %w", err)
	}

	// Default environment configuration
	defaultCfg := `env:
  name: "default"
  platform: "auto"        # auto-detect system platform
path:
  bin: "./bin"
  cache: "./cache"
  workspace: "./workspace"
plugins:
  installed: []
config:
  use_container: false
  pkg_manager: "auto"     # auto-detect package manager
`
	cfgPath := filepath.Join(absDir, "lyenv.yaml")
	if err := writeFileIfNotExists(cfgPath, defaultCfg, 0o644); err != nil {
		return fmt.Errorf("failed to write lyenv.yaml: %w", err)
	}

	return nil
}

// isLyenvDir checks if the directory already looks like a lyenv environment.
func isLyenvDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".lyenv", "version")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "lyenv.yaml")); err == nil {
		return true
	}
	return false
}

// writeFileIfNotExists writes a file only if it does not already exist.
func writeFileIfNotExists(path, content string, perm os.FileMode) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil // skip existing
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check file %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(content), perm)
}
