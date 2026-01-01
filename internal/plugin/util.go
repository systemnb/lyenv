package plugin

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func fetchURL(url, outPath, proxy string) error {
	var cmd *exec.Cmd
	if _, err := exec.LookPath("curl"); err == nil {
		args := []string{"-L", "-o", outPath, url}
		if proxy != "" {
			args = append([]string{"-x", proxy}, args...)
		}
		cmd = exec.Command("curl", args...)
	} else if _, err := exec.LookPath("wget"); err == nil {
		args := []string{"-O", outPath, url}
		cmd = exec.Command("wget", args...)
	} else {
		return fmt.Errorf("no downloader found (curl/wget)")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
