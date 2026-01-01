package plugin

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lyenv/internal/config"
)

type MergeStrategy = config.MergeStrategy

func ResolvePluginDir(envDir, name string) (pluginDir string, installName string, err error) {
	pluginsDir := filepath.Join(envDir, "plugins")

	// 1) Try as install name (physical)
	candidate := filepath.Join(pluginsDir, name)
	if _, statErr := os.Stat(candidate); statErr == nil {
		return candidate, name, nil
	}

	// 2) Fallback to registry: find record by install name
	if rec, recErr := GetByInstallName(envDir, name); recErr == nil {
		dir := filepath.Join(pluginsDir, rec.InstallName)
		if _, statErr2 := os.Stat(dir); statErr2 == nil {
			return dir, rec.InstallName, nil
		}
	}

	// 3) Fallback to registry: find record by manifest logical name
	if r, loadErr := LoadRegistry(envDir); loadErr == nil {
		for _, p := range r.Plugins {
			if p.Name == name {
				dir := filepath.Join(pluginsDir, p.InstallName)
				if _, statErr3 := os.Stat(dir); statErr3 == nil {
					return dir, p.InstallName, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("plugin directory not found for: %s", name)
}

func RunPluginCommand(envDir, pluginName, command string, passArgs []string, strategy MergeStrategy) error {
	pluginDir, resolvedInstall, err := ResolvePluginDir(envDir, pluginName)
	if err != nil {
		return err
	}
	man, err := LoadManifest(pluginDir)
	if err != nil {
		return err
	}

	globalCfg, err := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read global config: %w", err)
	}
	pluginCfg := map[string]interface{}{}
	if man.Config.LocalFile != "" {
		lp := filepath.Join(pluginDir, man.Config.LocalFile)
		if _, err := os.Stat(lp); err == nil {
			if pluginCfg, err = config.LoadAny(lp); err != nil {
				return fmt.Errorf("failed to read plugin config: %w", err)
			}
		}
	}

	var spec *CommandSpec
	for i := range man.Commands {
		if man.Commands[i].Name == command {
			spec = &man.Commands[i]
			break
		}
	}
	if spec == nil && man.Entry.Path != "" {
		spec = &CommandSpec{
			Name:       command,
			Executor:   man.Entry.Type,
			Program:    man.Entry.Path,
			Args:       man.Entry.Args,
			UseStdio:   strings.EqualFold(man.Entry.Type, "stdio"),
			LogCapture: true,
		}
	}
	if spec == nil {
		return fmt.Errorf("command not found: %s", command)
	}

	req := map[string]interface{}{
		"action": command,
		"args":   passArgs,
		"paths": map[string]string{
			"home":       envDir,
			"bin":        filepath.Join(envDir, "bin"),
			"workspace":  filepath.Join(envDir, "workspace"),
			"plugin_dir": pluginDir,
		},
		"system": map[string]string{
			"os":   runtime.GOOS,
			"arch": runtime.GOARCH,
		},
		"config": map[string]interface{}{
			"global": globalCfg,
			"plugin": pluginCfg,
		},
		"merge_strategy": string(strategy),
		"started_at":     time.Now().UTC().Format(time.RFC3339),
	}

	logFile := logPath(pluginDir, command)
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	fmt.Printf("Plugin resolved: name=%s install=%s dir=%s\n", pluginName, resolvedInstall, pluginDir)
	writeLogLine(w, map[string]interface{}{
		"level":   "info",
		"action":  command,
		"message": "dispatch start",
		"args":    passArgs,
	})

	start := time.Now()
	var resp map[string]interface{}
	var exitCode int

	switch strings.ToLower(spec.Executor) {
	case "stdio":
		resp, exitCode = spawnStdio(spec, pluginDir, req, w)
	case "shell":
		exitCode = runShell(spec, pluginDir, passArgs, w)
	default:
		writeLogLine(w, map[string]interface{}{"level": "error", "message": "unsupported executor", "executor": spec.Executor})
		return fmt.Errorf("unsupported executor: %s", spec.Executor)
	}

	dur := time.Since(start).Milliseconds()
	writeLogLine(w, map[string]interface{}{
		"level":       "info",
		"action":      command,
		"message":     "dispatch end",
		"duration_ms": dur,
		"exit_code":   exitCode,
	})

	if resp != nil {
		if status, _ := resp["status"].(string); status != "ok" {
			return fmt.Errorf("plugin error: %v", resp["message"])
		}
		if muts, ok := resp["mutations"].(map[string]interface{}); ok {
			if g, ok := muts["global"].(map[string]interface{}); ok {
				merged := config.MergeMapWithStrategy(globalCfg, g, strategy)
				if err := config.SaveYAML(filepath.Join(envDir, "lyenv.yaml"), merged); err != nil {
					return fmt.Errorf("failed to write global config: %w", err)
				}
				fmt.Printf("Global config updated (strategy=%s).\n", strategy)
			}
			if p, ok := muts["plugin"].(map[string]interface{}); ok && man.Config.LocalFile != "" {
				merged := config.MergeMapWithStrategy(pluginCfg, p, config.MergeOverride)
				if err := config.SaveAny(filepath.Join(pluginDir, man.Config.LocalFile), merged); err != nil {
					return fmt.Errorf("failed to write plugin config: %w", err)
				}
				fmt.Println("Plugin local config updated.")
			}
		}
		if logs, ok := resp["logs"].([]interface{}); ok {
			for _, l := range logs {
				fmt.Println(fmt.Sprint(l))
			}
		}
		if arts, ok := resp["artifacts"].([]interface{}); ok {
			for _, a := range arts {
				fmt.Printf("Artifact: %s\n", fmt.Sprint(a))
			}
		}
	}

	status := "ok"
	if exitCode != 0 {
		status = "error"
	}
	writeDispatchLog(envDir, DispatchRecord{
		Plugin:     pluginName,
		Command:    command,
		Args:       passArgs,
		Status:     status,
		LogFile:    logFile,
		DurationMS: dur,
	})
	fmt.Printf("Plugin log: %s\n", logFile)

	return nil
}

// ---- executors ----

func spawnStdio(spec *CommandSpec, pluginDir string, req map[string]interface{}, w *bufio.Writer) (map[string]interface{}, int) {
	// Decide entry path:
	// - If spec.Program is absolute ("/..."), use as-is.
	// - If spec.Program has NO path separator, treat as a system command (e.g., "python3") and use as-is.
	// - Otherwise, treat as plugin-relative path and join with pluginDir.
	entry := spec.Program
	if filepath.IsAbs(spec.Program) {
		// absolute path, keep as-is
	} else if strings.ContainsRune(spec.Program, os.PathSeparator) {
		// plugin-relative path
		entry = filepath.Join(pluginDir, spec.Program)
	} else {
		// bare command name (e.g., "python3"), keep as-is
	}

	// Build the command with args (stdio protocol expects JSON on stdin)
	args := append(spec.Args, []string{}...)
	cmd := exec.Command(entry, args...)
	writeLogLine(w, map[string]interface{}{"level": "debug", "message": "spawn stdio", "entry": entry, "args": args, "workdir": cmd.Dir})
	cmd.Dir = dirOr(pluginDir, spec.Workdir)
	cmd.Env = withExtraEnv(os.Environ(), spec.Env)
	cmd.Stderr = newLogWriter(w, "stderr")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		writeLogLine(w, map[string]interface{}{"level": "error", "message": "start failed", "error": err.Error()})
		return map[string]interface{}{"status": "error", "message": err.Error()}, exitCode(err)
	}
	enc := json.NewEncoder(stdin)
	_ = enc.Encode(req)
	_ = stdin.Close()

	var resp map[string]interface{}
	dec := json.NewDecoder(stdout)
	if err := dec.Decode(&resp); err != nil {
		writeLogLine(w, map[string]interface{}{"level": "error", "message": "resp decode failed", "error": err.Error()})
		return map[string]interface{}{"status": "error", "message": err.Error()}, exitCode(err)
	}
	err := cmd.Wait()
	return resp, exitCode(err)
}

func runShell(spec *CommandSpec, pluginDir string, passArgs []string, w *bufio.Writer) int {
	line := strings.TrimSpace(spec.Program)
	if line == "" && len(spec.Args) > 0 {
		line = strings.Join(spec.Args, " ")
	}
	if len(passArgs) > 0 {
		line = strings.TrimSpace(line + " " + strings.Join(passArgs, " "))
	}
	cmd := exec.Command("bash", "-c", line)
	cmd.Dir = dirOr(pluginDir, spec.Workdir)
	cmd.Env = withExtraEnv(os.Environ(), spec.Env)
	cmd.Stdout = newLogWriter(w, "stdout")
	cmd.Stderr = newLogWriter(w, "stderr")
	return exitCode(cmd.Run())
}

// ---- logging & helpers ----

func logPath(pluginDir, action string) string {
	d := filepath.Join(pluginDir, "logs", time.Now().UTC().Format("2006-01-02"))
	_ = os.MkdirAll(d, 0o755)
	ts := time.Now().UTC().Format("20060102T150405Z")
	return filepath.Join(d, fmt.Sprintf("%s-%s.log", action, ts))
}

type logWriter struct {
	w     *bufio.Writer
	level string
	buf   bytes.Buffer
}

func newLogWriter(w *bufio.Writer, level string) *logWriter {
	return &logWriter{w: w, level: level}
}

func (lw *logWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			writeLogLine(lw.w, map[string]interface{}{
				"level":   lw.level,
				"message": lw.buf.String(),
			})
			lw.buf.Reset()
		} else {
			lw.buf.WriteByte(b)
		}
	}
	return len(p), nil
}

func writeLogLine(w *bufio.Writer, kv map[string]interface{}) {
	if _, ok := kv["ts"]; !ok {
		kv["ts"] = time.Now().UTC().Format(time.RFC3339)
	}
	b, _ := json.Marshal(kv)
	w.Write(b)
	w.WriteString("\n")
	w.Flush()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

func dirOr(pluginDir, workdir string) string {
	if strings.TrimSpace(workdir) == "" {
		return pluginDir
	}
	if filepath.IsAbs(workdir) {
		return workdir
	}
	return filepath.Join(pluginDir, workdir)
}

func withExtraEnv(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	out := append([]string{}, base...)
	for k, v := range extra {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
