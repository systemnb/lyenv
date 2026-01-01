package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type DispatchRecord struct {
	TS         string   `json:"ts"`
	Plugin     string   `json:"plugin"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	Status     string   `json:"status"`
	LogFile    string   `json:"log_file"`
	DurationMS int64    `json:"duration_ms"`
}

func writeDispatchLog(envDir string, rec DispatchRecord) {
	logDir := filepath.Join(envDir, ".lyenv", "logs")
	_ = os.MkdirAll(logDir, 0o755)
	f, err := os.OpenFile(filepath.Join(logDir, "dispatch.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if rec.TS == "" {
		rec.TS = time.Now().UTC().Format(time.RFC3339)
	}
	_ = json.NewEncoder(f).Encode(rec)
}
