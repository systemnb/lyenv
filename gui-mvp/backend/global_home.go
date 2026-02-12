package main

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveGlobalHome() string {
	if v := strings.TrimSpace(os.Getenv("LYENV_GLOBAL_HOME")); v != "" {
		_ = os.MkdirAll(v, 0o755)
		return v
	}
	hd, _ := os.UserHomeDir()
	p := filepath.Join(hd, ".lyenv")
	_ = os.MkdirAll(p, 0o755)
	return p
}
