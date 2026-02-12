// Serve embedded Vite SPA with proper SPA fallback.
// - Embeds ./dist (same package dir) using //go:embed all:dist
// - Avoids shadowing the "fs" package (no local var named fs)
// - Uses http.ServeContent with bytes.NewReader (io.ReadSeeker)

package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Embed all files under dist/, including dot/underscore prefixed ones.
// See: https://pkg.go.dev/embed
//
//go:embed all:dist
var uiDist embed.FS

func main() {

	addr := flag.String("addr", "127.0.0.1:18888", "listen address")
	flag.Parse()

	globalHome := resolveGlobalHome()
	guiDir := filepath.Join(globalHome, "gui")
	cfgPath := filepath.Join(guiDir, "config.yaml")

	cfg, existed, err := loadGuiConfig(cfgPath)
	if err != nil {
		log.Fatal("load gui config:", err)
	}
	if !existed {
		_ = saveGuiConfig(cfgPath, cfg) // create default config
	}

	mux := http.NewServeMux()

	// Subtree to dist/ from the embedded FS
	distSub, err := fs.Sub(uiDist, "dist")
	if err != nil {
		log.Fatal("fs.Sub dist:", err)
	}

	// Static file server (assets, chunks, etc.)
	fileServer := http.FileServer(http.FS(distSub))

	// Serve hashed static assets quickly
	mux.Handle("/assets/", fileServer)

	// Root handler: try static file first, otherwise SPA fallback to index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

		// Try open the requested path from embedded FS
		if f, err := distSub.Open(clean); err == nil {
			defer f.Close()
			serveFileFromFS(w, r, f, clean)
			return
		}

		// If path is a directory, try its index.html
		if clean != "" {
			if f, err := distSub.Open(path.Join(clean, "index.html")); err == nil {
				defer f.Close()
				serveFileWithType(w, r, f, "index.html", "text/html; charset=utf-8")
				return
			}
		}

		// SPA fallback: dist/index.html
		index, err := distSub.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found in embedded dist", http.StatusInternalServerError)
			return
		}
		defer index.Close()
		serveFileWithType(w, r, index, "index.html", "text/html; charset=utf-8")
	})

	mux.HandleFunc("/api/envs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cfg, _, err := loadGuiConfig(cfgPath)
		if err != nil {
			http.Error(w, "load gui config failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// ONLY pinned, no scan (design intent)
		out := make([]EnvInfo, 0, len(cfg.Envs.Pinned))
		for _, p := range cfg.Envs.Pinned {
			if strings.TrimSpace(p.Path) == "" {
				continue
			}
			if isEnvDir(p.Path) {
				name := strings.TrimSpace(p.Name)
				if name == "" {
					name = filepath.Base(p.Path)
				}
				out = append(out, EnvInfo{Name: name, Path: p.Path, From: "pinned"})
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"envs": out})
	})

	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request JSON
		var req struct {
			EnvPath string   `json:"envPath"`
			Plugin  string   `json:"plugin"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		req.EnvPath = strings.TrimSpace(req.EnvPath)
		req.Plugin = strings.TrimSpace(req.Plugin)
		req.Command = strings.TrimSpace(req.Command)

		if req.EnvPath == "" || req.Plugin == "" || req.Command == "" {
			http.Error(w, "envPath/plugin/command required", http.StatusBadRequest)
			return
		}

		// Allow-list check: envPath must be in discovered env list
		cfg, _, err := loadGuiConfig(cfgPath)
		if err != nil {
			http.Error(w, "load gui config failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		envs, err := discoverEnvs(cfg)
		if err != nil {
			http.Error(w, "discover envs failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		allowed := false
		for _, e := range envs {
			if e.Path == req.EnvPath {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "envPath not allowed", http.StatusForbidden)
			return
		}

		// Run lyenv
		rec, err := runLyenv(req.EnvPath, req.Plugin, req.Command, req.Args)
		if err != nil {
			http.Error(w, "run failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		logAbs := filepath.Join(req.EnvPath, rec.LogFile)

		// Response
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"dispatchId": rec.ID,
			"dispatch":   rec,
			"logFile":    logAbs,
		})
	})

	mux.HandleFunc("/ws/logs", wsLogsHandler(cfgPath))

	mux.HandleFunc("/api/flow/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			EnvPath     string   `json:"envPath"`
			ZipB64      string   `json:"zipB64"`
			InstallName string   `json:"installName"`
			Command     string   `json:"command"`
			Args        []string `json:"args"`
			Cleanup     bool     `json:"cleanup"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		req.EnvPath = strings.TrimSpace(req.EnvPath)
		req.InstallName = strings.TrimSpace(req.InstallName)
		req.Command = strings.TrimSpace(req.Command)
		if req.EnvPath == "" || req.ZipB64 == "" || req.InstallName == "" || req.Command == "" {
			http.Error(w, "envPath/zipB64/installName/command required", http.StatusBadRequest)
			return
		}

		// allow-list: envPath must be in /api/envs list
		cfg, _, err := loadGuiConfig(cfgPath)
		if err != nil {
			http.Error(w, "load gui config failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		envs, err := discoverEnvs(cfg)
		if err != nil {
			http.Error(w, "discover envs failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		allowed := false
		for _, e := range envs {
			if e.Path == req.EnvPath {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "envPath not allowed (use: lyenv gui add <DIR>)", http.StatusForbidden)
			return
		}

		zipBytes, err := decodeZipB64(req.ZipB64)
		if err != nil {
			http.Error(w, "zip decode failed: "+err.Error(), http.StatusBadRequest)
			return
		}

		// install
		if err := installZipPlugin(req.EnvPath, req.InstallName, zipBytes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// cleanup on failure too
		if req.Cleanup {
			defer removeInstalled(req.EnvPath, req.InstallName)
		}

		// run
		rec, err := runInstalled(req.EnvPath, req.InstallName, req.Command, req.Args)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		logAbs := filepath.Join(req.EnvPath, rec.LogFile)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"dispatchId": rec.ID,
			"dispatch":   rec,
			"logFile":    logAbs,
		})
	})

	log.Printf("GUI listening on http://%s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// serveFileFromFS guesses Content-Type from extension and delegates to ServeContent.
func serveFileFromFS(w http.ResponseWriter, r *http.Request, f fs.File, name string) {
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" && strings.HasSuffix(name, "index.html") {
		ct = "text/html; charset=utf-8"
	}
	serveFileWithType(w, r, f, name, ct)
}

// serveFileWithType reads fs.File into memory and serves via http.ServeContent,
// which requires an io.ReadSeeker.
func serveFileWithType(w http.ResponseWriter, r *http.Request, f fs.File, name, contentType string) {
	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "read embedded file failed", http.StatusInternalServerError)
		return
	}
	if contentType == "" {
		// Best effort
		contentType = mime.TypeByExtension(filepath.Ext(name))
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
	}
	w.Header().Set("Content-Type", contentType)

	// Optional: set a stable modTime (here we use build/run time).
	modTime := time.Now()

	http.ServeContent(w, r, path.Base(name), modTime, bytes.NewReader(data))
}
