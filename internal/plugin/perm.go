package plugin

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// NormalizePluginPermissions ensures directories are 0755,
// regular files are 0644, and files with shebang are 0755.func NormalizePluginPermissions(root string) error {
func NormalizePluginPermissions(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(path, 0o755)
		}
		_ = os.Chmod(path, 0o644)
		f, err := os.Open(path)
		if err == nil {
			defer f.Close()
			r := bufio.NewReader(f)
			line, _ := r.ReadString('\n')
			if strings.HasPrefix(line, "#!") {
				_ = os.Chmod(path, 0o755)
			}
		}
		return nil
	})
}

func EnsureLogsDir(root string) error {
	return os.MkdirAll(filepath.Join(root, "logs"), 0o755)
}
