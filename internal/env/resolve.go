package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveEnvDir returns the environment root directory to operate on.
// Policy (per your choice #1):
// - Prefer LYENV_HOME if set and valid
// - Otherwise, allow current directory only if it looks like a lyenv env root
// - Else return error asking user to activate
func ResolveEnvDir() (string, error) {
	// 1) Prefer LYENV_HOME
	if v := strings.TrimSpace(os.Getenv("LYENV_HOME")); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", fmt.Errorf("invalid LYENV_HOME: %v", err)
		}
		if IsLyenvDir(abs) {
			return abs, nil
		}
		return "", fmt.Errorf("LYENV_HOME is set but not a lyenv environment: %s", abs)
	}

	// 2) Fallback: current directory if it is an env root
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot get current directory: %v", err)
	}
	if IsLyenvDir(cwd) {
		return cwd, nil
	}

	// 3) Enforced activate requirement
	return "", fmt.Errorf("no active environment. Please run: cd <ENV_DIR> && eval \"$(lyenv activate)\"")
}
