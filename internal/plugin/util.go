package plugin

import (
	"io/fs"
	"lyenv/internal/config"
	"os"
	"path/filepath"
	"strings"
)

func getMirrorPrefix(envDir string) string {
	cfg, err := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(config.GetString(cfg, "config.network.mirror_prefix"))
	if p == "" {
		return ""
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

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
	// Apply mirror prefix from config (only for github urls if you want)
	url = applyMirrorEnv(url)
	return downloadToFile(url, outPath, proxy)
}
