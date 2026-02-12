package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type EnvInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	From string `json:"from"` // pinned|scan
}

func isEnvDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "lyenv.yaml")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, ".lyenv", "version")); err == nil {
		return true
	}
	return false
}

func discoverEnvs(cfg GuiConfig) ([]EnvInfo, error) {
	seen := map[string]EnvInfo{}

	// pinned
	for _, p := range cfg.Envs.Pinned {
		pp := strings.TrimSpace(p.Path)
		if pp == "" {
			continue
		}
		if abs, err := filepath.Abs(pp); err == nil {
			pp = abs
		}
		name := strings.TrimSpace(p.Name)
		if name == "" {
			name = filepath.Base(pp)
		}
		if isEnvDir(pp) {
			seen[pp] = EnvInfo{Name: name, Path: pp, From: "pinned"}
		}
	}

	// scan
	for _, s := range cfg.Envs.Scan {
		root := strings.TrimSpace(s.Root)
		if root == "" {
			continue
		}
		depth := s.Depth
		if depth <= 0 {
			depth = 3
		}

		// pattern is optional; we actually accept either lyenv.yaml or .lyenv/version
		_ = walkDepth(root, depth, func(dir string) {
			if isEnvDir(dir) {
				pp := dir
				if abs, err := filepath.Abs(pp); err == nil {
					pp = abs
				}
				if _, ok := seen[pp]; !ok {
					seen[pp] = EnvInfo{Name: filepath.Base(pp), Path: pp, From: "scan"}
				}
			}
		})
	}

	out := make([]EnvInfo, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func walkDepth(root string, depth int, visit func(dir string)) error {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}

	var rec func(string, int)
	rec = func(dir string, d int) {
		visit(dir)
		if d <= 0 {
			return
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if name == ".git" || name == "node_modules" {
				continue
			}
			rec(filepath.Join(dir, name), d-1)
		}
	}
	rec(root, depth)
	return nil
}
