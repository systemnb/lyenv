package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // TODO: tighten later
}

// register in main.go: mux.HandleFunc("/ws/logs", wsLogsHandler(cfgPath))
func wsLogsHandler(cfgPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envPath := strings.TrimSpace(r.URL.Query().Get("envPath"))
		dispatchID := strings.TrimSpace(r.URL.Query().Get("dispatchId"))
		if envPath == "" || dispatchID == "" {
			http.Error(w, "missing envPath/dispatchId", http.StatusBadRequest)
			return
		}

		// allow-list check
		cfg, _, _ := loadGuiConfig(cfgPath)
		envs, _ := discoverEnvs(cfg)
		allowed := false
		for _, e := range envs {
			if e.Path == envPath {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, "envPath not allowed", http.StatusForbidden)
			return
		}

		// resolve dispatch record => log file
		rec, err := findDispatchByID(envPath, dispatchID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		logFileAbs := filepath.Join(envPath, rec.LogFile)

		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send an initial metadata message
		_ = conn.WriteJSON(map[string]any{
			"type":     "meta",
			"dispatch": rec,
			"logFile":  logFileAbs,
		})

		// Tail file: send existing last N lines first (optional)
		sendLastLines(conn, logFileAbs, 200)

		// Then stream new lines
		tailFile(conn, logFileAbs)
	}
}

func findDispatchByID(envPath, id string) (*DispatchRecord, error) {
	p := filepath.Join(envPath, ".lyenv", "logs", "dispatch.log")
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	var lastMatch *DispatchRecord
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec DispatchRecord
		if err := json.Unmarshal([]byte(line), &rec); err == nil {
			if rec.ID == id {
				// keep last match (should be unique anyway)
				tmp := rec
				lastMatch = &tmp
			}
		}
	}
	if lastMatch == nil {
		return nil, os.ErrNotExist
	}
	return lastMatch, nil
}

func sendLastLines(conn *websocket.Conn, file string, n int) {
	// Best-effort: if not exist, ignore.
	b, err := os.ReadFile(file)
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r\n")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		_ = conn.WriteJSON(map[string]any{
			"type": "line",
			"line": ln,
		})
	}
}

func tailFile(conn *websocket.Conn, file string) {
	var offset int64 = 0

	for {
		// detect client close
		if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(2*time.Second)); err != nil {
			return
		}

		fi, err := os.Stat(file)
		if err != nil {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		// If file truncated, reset offset
		if fi.Size() < offset {
			offset = 0
		}

		f, err := os.Open(file)
		if err != nil {
			time.Sleep(250 * time.Millisecond)
			continue
		}

		_, _ = f.Seek(offset, 0)
		r := bufio.NewReader(f)

		for {
			line, err := r.ReadString('\n')
			if len(line) > 0 {
				offset += int64(len(line))
				line = strings.TrimRight(line, "\r\n")
				if strings.TrimSpace(line) != "" {
					_ = conn.WriteJSON(map[string]any{
						"type": "line",
						"line": line,
					})
				}
			}
			if err != nil {
				break
			}
		}
		_ = f.Close()
		time.Sleep(200 * time.Millisecond)
	}
}
