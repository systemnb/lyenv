package plugin

import (
	"bufio"
	"bytes"
	"context"
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

// RunPluginCommand executes a plugin command (single or multi-step) with logging and config mutations.
// It accepts either the install name (preferred) or the manifest logical name as `pluginName`.
// RunPluginCommand executes a plugin command (single or multi-step) with logging, config mutations,
// global timeout and fail-fast/keep-going policy.
// - ctx: global context; cancellation or deadline applies to all steps.
// - keepGoing: when true, multi-step execution continues on errors; when false (fail-fast), it stops at first failure.
func RunPluginCommand(ctx context.Context, envDir, pluginName, command string, passArgs []string, strategy MergeStrategy, keepGoing bool) error {
	// Resolve plugin directory (install name or manifest logical name)
	pluginDir, resolvedInstall, err := ResolvePluginDir(envDir, pluginName)
	if err != nil {
		return err
	}

	// Load manifest
	man, err := LoadManifest(pluginDir)
	if err != nil {
		return err
	}

	// Load global config (lyenv.yaml, always YAML)
	globalCfg, err := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read global config: %w", err)
	}

	// Load plugin local config (YAML or JSON by extension)
	pluginCfg := map[string]interface{}{}
	if strings.TrimSpace(man.Config.LocalFile) != "" {
		lp := filepath.Join(pluginDir, man.Config.LocalFile)
		if _, err := os.Stat(lp); err == nil {
			if pluginCfg, err = config.LoadAny(lp); err != nil {
				return fmt.Errorf("failed to read plugin config: %w", err)
			}
		}
	}

	// Prepare request JSON for stdio steps or single stdio run
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

	// Create plugin log file
	logFile := logPath(pluginDir, command)
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return err
	}
	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open log file: %w", err)
	}
	defer lf.Close()
	w := bufio.NewWriter(lf)

	// Console hint for resolution
	fmt.Printf("Plugin resolved: name=%s install=%s dir=%s\n", pluginName, resolvedInstall, pluginDir)

	// Start dispatch logging
	writeLogLine(w, map[string]interface{}{
		"level":     "info",
		"message":   "dispatch start",
		"action":    command,
		"args":      passArgs,
		"timeout":   ctxTimeoutSeconds(ctx), // for insight
		"keepGoing": keepGoing,
	})
	start := time.Now()

	// Find matching command spec
	var spec *CommandSpec
	for i := range man.Commands {
		if man.Commands[i].Name == command {
			spec = &man.Commands[i]
			break
		}
	}
	// Fallback to entry when commands not explicitly defined
	if spec == nil && strings.TrimSpace(man.Entry.Path) != "" {
		spec = &CommandSpec{
			Name:       command,
			Executor:   man.Entry.Type,
			Program:    man.Entry.Path,
			Args:       append(man.Entry.Args, passArgs...), // pass args into stdio program if needed
			Workdir:    "",
			Env:        map[string]string{},
			UseStdio:   strings.EqualFold(man.Entry.Type, "stdio"),
			LogCapture: true,
		}
	}
	if spec == nil {
		writeLogLine(w, map[string]interface{}{"level": "error", "message": "command not found", "action": command})
		return fmt.Errorf("command not found: %s", command)
	}

	var exitCode int
	var resp map[string]interface{}

	// Handle global context cancellation early
	select {
	case <-ctx.Done():
		writeLogLine(w, map[string]interface{}{"level": "error", "message": "dispatch canceled before start", "error": ctx.Err().Error()})
		return fmt.Errorf("canceled: %w", ctx.Err())
	default:
	}

	// Multi-step execution
	if len(spec.Steps) > 0 {
		writeLogLine(w, map[string]interface{}{
			"level":   "info",
			"message": "multi-step command start",
			"steps":   len(spec.Steps),
		})
		for idx, st := range spec.Steps {
			// Apply global keepGoing override
			stepContinue := st.ContinueOnError || keepGoing

			// Check context per step
			if err := ctx.Err(); err != nil {
				writeLogLine(w, map[string]interface{}{
					"level":      "error",
					"message":    "step skipped due to context error",
					"step_index": idx,
					"error":      err.Error(),
				})
				return fmt.Errorf("canceled or timeout: %w", err)
			}

			stepStart := time.Now()
			writeLogLine(w, map[string]interface{}{
				"level":             "info",
				"message":           "step start",
				"step_index":        idx,
				"executor":          st.Executor,
				"program":           st.Program,
				"args":              st.Args,
				"continue_on_error": stepContinue,
			})

			resp = nil
			exitCode = 0

			switch strings.ToLower(st.Executor) {
			case "shell":
				tmp := &CommandSpec{
					Executor: "shell",
					Program:  st.Program,
					Args:     st.Args,
					Workdir:  st.Workdir,
					Env:      st.Env,
				}
				exitCode = runShell(ctx, tmp, pluginDir, []string{}, w)

			case "stdio":
				tmp := &CommandSpec{
					Executor: "stdio",
					Program:  st.Program,
					Args:     append(st.Args, passArgs...), // pass args if needed
					Workdir:  st.Workdir,
					Env:      st.Env,
					UseStdio: true,
				}
				resp, exitCode = spawnStdio(ctx, tmp, pluginDir, req, w)

			default:
				writeLogLine(w, map[string]interface{}{
					"level":      "error",
					"message":    "unsupported executor in step",
					"step_index": idx,
					"executor":   st.Executor,
				})
				return fmt.Errorf("unsupported executor: %s", st.Executor)
			}

			// Apply mutations when stdio
			if resp != nil {
				if status, _ := resp["status"].(string); status != "ok" {
					writeLogLine(w, map[string]interface{}{
						"level":      "error",
						"message":    "step stdio error",
						"step_index": idx,
						"error":      fmt.Sprintf("%v", resp["message"]),
					})
					if !stepContinue {
						return fmt.Errorf("plugin error: %v", resp["message"])
					}
				}
				if muts, ok := resp["mutations"].(map[string]interface{}); ok {
					if g, ok := muts["global"].(map[string]interface{}); ok {
						merged := config.MergeMapWithStrategy(globalCfg, g, strategy)
						if err := config.SaveYAML(filepath.Join(envDir, "lyenv.yaml"), merged); err != nil {
							return fmt.Errorf("failed to write global config: %w", err)
						}
						fmt.Printf("Global config updated (strategy=%s).\n", strategy)
					}
					if p, ok := muts["plugin"].(map[string]interface{}); ok && strings.TrimSpace(man.Config.LocalFile) != "" {
						merged := config.MergeMapWithStrategy(pluginCfg, p, config.MergeOverride)
						if err := config.SaveAny(filepath.Join(pluginDir, man.Config.LocalFile), merged); err != nil {
							return fmt.Errorf("failed to write plugin config: %w", err)
						}
						fmt.Println("Plugin local config updated.")
					}
				}
				// Optional: echo stdio logs/artifacts
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

			stepDur := time.Since(stepStart).Milliseconds()
			writeLogLine(w, map[string]interface{}{
				"level":       "info",
				"message":     "step end",
				"step_index":  idx,
				"duration_ms": stepDur,
				"exit_code":   exitCode,
			})
			if exitCode != 0 && !stepContinue {
				return fmt.Errorf("plugin step exit code: %d", exitCode)
			}
		}

		// Done
		writeLogLine(w, map[string]interface{}{"level": "info", "message": "multi-step command end"})
		dur := time.Since(start).Milliseconds()
		writeDispatchLog(envDir, DispatchRecord{
			Plugin:     resolvedInstall,
			Command:    command,
			Args:       passArgs,
			Status:     "ok",
			LogFile:    logFile,
			DurationMS: dur,
		})
		fmt.Printf("Plugin log: %s\n", logFile)
		return nil
	}

	// Single command execution (legacy path)
	switch strings.ToLower(spec.Executor) {
	case "stdio":
		// Pass args to stdio program if needed, and also via req["args"]
		spec.Args = append(spec.Args, passArgs...)
		resp, exitCode = spawnStdio(ctx, spec, pluginDir, req, w)

	case "shell":
		exitCode = runShell(ctx, spec, pluginDir, passArgs, w)

	default:
		writeLogLine(w, map[string]interface{}{"level": "error", "message": "unsupported executor", "executor": spec.Executor})
		return fmt.Errorf("unsupported executor: %s", spec.Executor)
	}

	// Apply mutations for single stdio run
	if resp != nil {
		if status, _ := resp["status"].(string); status != "ok" {
			return fmt.Errorf("plugin error: %v", resp["message"])
		}
		if muts, ok := resp["mutations"].(map[string]interface{}); ok {
			if g, ok := muts["global"].(map[string]interface{}); ok {
				merged := config.MergeMapWithStrategy(globalCfg, g, strategy)
				if err := config.SaveYAML(filepath.Join(envDir, "lyenv.yaml"), merged); err != nil {
					return fmt.Errorf("failed to write merged global config: %w", err)
				}
				fmt.Printf("Global config updated (strategy=%s).\n", strategy)
			}
			if p, ok := muts["plugin"].(map[string]interface{}); ok && strings.TrimSpace(man.Config.LocalFile) != "" {
				merged := config.MergeMapWithStrategy(pluginCfg, p, config.MergeOverride)
				if err := config.SaveAny(filepath.Join(pluginDir, man.Config.LocalFile), merged); err != nil {
					return fmt.Errorf("failed to write plugin config: %w", err)
				}
				fmt.Println("Plugin local config updated.")
			}
		}
		// Echo logs/artifacts
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

	// End of dispatch
	dur := time.Since(start).Milliseconds()
	writeLogLine(w, map[string]interface{}{
		"level":       "info",
		"message":     "dispatch end",
		"duration_ms": dur,
		"exit_code":   exitCode,
	})
	status := "ok"
	if exitCode != 0 {
		status = "error"
	}
	writeDispatchLog(envDir, DispatchRecord{
		Plugin:     resolvedInstall,
		Command:    command,
		Args:       passArgs,
		Status:     status,
		LogFile:    logFile,
		DurationMS: dur,
	})

	fmt.Printf("Plugin log: %s\n", logFile)
	if exitCode != 0 {
		return fmt.Errorf("plugin command exit code: %d", exitCode)
	}
	return nil
}

// ctxTimeoutSeconds renders remaining timeout for logging (best-effort).
func ctxTimeoutSeconds(ctx context.Context) int64 {
	d, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	rem := time.Until(d)
	if rem <= 0 {
		return 0
	}
	return int64(rem.Seconds())
}

// ---- executors ----

func spawnStdio(ctx context.Context, spec *CommandSpec, pluginDir string, req map[string]interface{}, w *bufio.Writer) (map[string]interface{}, int) {
	// Decide entry path:
	entry := spec.Program
	if filepath.IsAbs(spec.Program) {
		// absolute path, keep as-is
	} else if strings.ContainsRune(spec.Program, os.PathSeparator) {
		// plugin-relative path
		entry = filepath.Join(pluginDir, spec.Program)
	} else {
		// bare command name (e.g., "python3"), keep as-is
	}

	args := append(spec.Args, []string{}...)

	// If entry is a plugin-relative script, try to parse shebang and launch via interpreter when applicable.
	useInterpreter := false
	interp := ""
	scriptPath := ""

	if strings.ContainsRune(spec.Program, os.PathSeparator) && !filepath.IsAbs(spec.Program) {
		if st, err := os.Stat(entry); err == nil && !st.IsDir() {
			f, err := os.Open(entry)
			if err == nil {
				defer f.Close()
				r := bufio.NewReader(f)
				line, _ := r.ReadString('\n')
				if strings.HasPrefix(line, "#!") {
					fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "#!"))
					if len(fields) >= 1 {
						if filepath.Base(fields[0]) == "env" && len(fields) >= 2 {
							if abs, err := exec.LookPath(fields[1]); err == nil {
								interp = abs
								useInterpreter = true
								scriptPath = entry
							}
						} else if filepath.IsAbs(fields[0]) {
							interp = fields[0]
							useInterpreter = true
							scriptPath = entry
						}
					}
				}
			}
		}
	}

	var cmd *exec.Cmd
	if useInterpreter && interp != "" && scriptPath != "" {
		fullArgs := append(args, scriptPath)
		cmd = exec.CommandContext(ctx, interp, fullArgs...)
	} else {
		cmd = exec.CommandContext(ctx, entry, args...)
	}

	cmd.Dir = dirOr(pluginDir, spec.Workdir)
	cmd.Env = withExtraEnv(os.Environ(), spec.Env)
	cmd.Stderr = newLogWriter(w, "stderr")

	writeLogLine(w, map[string]interface{}{
		"level":   "debug",
		"message": "spawn stdio",
		"entry":   entry,
		"interp":  interp,
		"args":    args,
		"workdir": cmd.Dir,
		"envPATH": os.Getenv("PATH"),
	})

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

func runShell(ctx context.Context, spec *CommandSpec, pluginDir string, passArgs []string, w *bufio.Writer) int {
	line := strings.TrimSpace(spec.Program)
	if line == "" && len(spec.Args) > 0 {
		line = strings.Join(spec.Args, " ")
	}
	if len(passArgs) > 0 {
		line = strings.TrimSpace(line + " " + strings.Join(passArgs, " "))
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", line)
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
