package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lyenv/internal/config"
	runtime2 "lyenv/internal/runtime"
)

// MergeStrategy is an alias to config.MergeStrategy for convenience.
type MergeStrategy = config.MergeStrategy

// ResolvePluginDir resolves a plugin directory by either its physical install
// name (directory name under plugins/) or by its manifest logical name.
// It returns the plugin dir path, the resolved install name, and error if any.
func ResolvePluginDir(envDir, name string) (pluginDir string, installName string, err error) {
	pluginsDir := filepath.Join(envDir, "plugins")

	// 1) Try physical install name first.
	candidate := filepath.Join(pluginsDir, name)
	if _, statErr := os.Stat(candidate); statErr == nil {
		return candidate, name, nil
	}

	// 2) Try registry by install name.
	if rec, recErr := GetByInstallName(envDir, name); recErr == nil {
		dir := filepath.Join(pluginsDir, rec.InstallName)
		if _, statErr2 := os.Stat(dir); statErr2 == nil {
			return dir, rec.InstallName, nil
		}
	}

	// 3) Fallback: try registry by manifest logical name.
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

// RunPluginCommand executes a plugin command (single or multi-step), handling:
// - stdio and shell executors
// - structured mutations (global/plugin config)
// - global timeout and fail-fast/keep-going policy
// - per-dispatch JSONL logging and global dispatch log
//
// Arguments:
//   - ctx: context with optional deadline for global timeout
//   - envDir: lyenv environment directory (current working env root)
//   - pluginName: install name or manifest logical name
//   - command: command name within manifest (or entry fallback)
//   - passArgs: positional args to pass to the command (stdio: in request.args; shell: appended to line)
//   - strategy: merge strategy for applying mutations to global config
//   - keepGoing: if true, multi-step continues on error; otherwise fail-fast
func RunPluginCommand(ctx context.Context, envDir, pluginName, command string, passArgs []string, strategy MergeStrategy, keepGoing bool) error {
	_, err := RunPluginCommandWithRecord(ctx, envDir, pluginName, command, passArgs, strategy, keepGoing)
	return err
}

// RunPluginCommandWithRecord executes a plugin command and always writes a dispatch record.
// It returns the dispatch record (even when error happens after plugin resolved).
func RunPluginCommandWithRecord(ctx context.Context, envDir, pluginName, command string, passArgs []string, strategy MergeStrategy, keepGoing bool) (*DispatchRecord, error) {
	// Resolve plugin directory and install name.
	pluginDir, resolvedInstall, err := ResolvePluginDir(envDir, pluginName)
	if err != nil {
		return nil, err
	}

	// Load manifest.
	man, err := LoadManifest(pluginDir)
	if err != nil {
		return nil, err
	}

	// Prepare dispatch id + log file path early (so record always has them).
	dispatchID := newDispatchID()
	pluginLogFile := logPath(pluginDir, command)

	// Build record now; we'll update status/duration at the end.
	rec := &DispatchRecord{
		ID:            dispatchID,
		Plugin:        resolvedInstall,
		Command:       command,
		Args:          passArgs,
		Status:        "error",
		PluginLogFile: pluginLogFile,
		LogFile:       pluginLogFile,
	}

	start := time.Now()
	defer func() {
		rec.DurationMS = time.Since(start).Milliseconds()
		if rec.TS == "" {
			rec.TS = time.Now().UTC().Format(time.RFC3339)
		}

		// 1) Copy plugin log to env global dispatch log path (stable even after cleanup)
		dst := filepath.Join(envDir, ".lyenv", "logs", "dispatch", rec.ID+".log")
		if err := copyFile(rec.PluginLogFile, dst); err == nil {
			// Store relative path for portability
			rel := filepath.Join(".lyenv", "logs", "dispatch", rec.ID+".log")
			rec.LogFile = rel
		} else {
			// If copy failed, keep original (best-effort)
			rec.LogFile = rec.PluginLogFile
		}

		// 2) Always write dispatch record
		writeDispatchLog(envDir, *rec)
	}()

	// Load global config (always YAML).
	globalCfg, err := config.LoadYAML(filepath.Join(envDir, "lyenv.yaml"))
	if err != nil {
		return rec, fmt.Errorf("failed to read global config: %w", err)
	}

	// Load plugin local config if present.
	pluginCfg := map[string]interface{}{}
	if strings.TrimSpace(man.Config.LocalFile) != "" {
		lp := filepath.Join(pluginDir, man.Config.LocalFile)
		if _, err := os.Stat(lp); err == nil {
			if pluginCfg, err = config.LoadAny(lp); err != nil {
				return rec, fmt.Errorf("failed to read plugin config: %w", err)
			}
		}
	}

	// Build stdio request payload.
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
		"dispatch_id":    dispatchID, // NEW: useful for plugins/log correlation
	}

	// Prepare plugin log file (per-dispatch JSONL).
	if err := os.MkdirAll(filepath.Dir(pluginLogFile), 0o755); err != nil {
		return rec, err
	}
	lf, err := os.OpenFile(pluginLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return rec, fmt.Errorf("cannot open log file: %w", err)
	}
	defer lf.Close()
	w := bufio.NewWriter(lf)
	defer w.Flush()

	// Console hint for resolution.
	if isJSONMode() {
		// fmt.Fprintf(os.Stderr, "Plugin resolved: name=%s install=%s dir=%s\n", pluginName, resolvedInstall, pluginDir)
		writeLogLine(w, map[string]interface{}{
			"level":   "info",
			"message": fmt.Sprintf("Plugin resolved: name=%s install=%s dir=%s\n", pluginName, resolvedInstall, pluginDir),
		})
	} else {
		// fmt.Printf("Plugin resolved: name=%s install=%s dir=%s\n", pluginName, resolvedInstall, pluginDir)
		writeLogLine(w, map[string]interface{}{
			"level":   "info",
			"message": fmt.Sprintf("Plugin resolved: name=%s install=%s dir=%s\n", pluginName, resolvedInstall, pluginDir),
		})
	}

	// Start dispatch log line in plugin JSONL
	writeLogLine(w, map[string]interface{}{
		"level":       "info",
		"message":     "dispatch start",
		"dispatch_id": dispatchID,
		"action":      command,
		"args":        passArgs,
		"timeout":     ctxTimeoutSeconds(ctx),
		"keepGoing":   keepGoing,
	})

	// Find command spec by name; fallback to Entry when commands not defined.
	var spec *CommandSpec
	for i := range man.Commands {
		if man.Commands[i].Name == command {
			spec = &man.Commands[i]
			break
		}
	}
	if spec == nil && strings.TrimSpace(man.Entry.Path) != "" {
		spec = &CommandSpec{
			Name:       command,
			Executor:   man.Entry.Type,
			Program:    man.Entry.Path,
			Args:       append(man.Entry.Args, passArgs...),
			Workdir:    "",
			Env:        map[string]string{},
			UseStdio:   strings.EqualFold(man.Entry.Type, "stdio"),
			LogCapture: true,
		}
	}
	if spec == nil {
		writeLogLine(w, map[string]interface{}{
			"level":       "error",
			"message":     "command not found",
			"dispatch_id": dispatchID,
			"action":      command,
		})
		return rec, fmt.Errorf("command not found: %s", command)
	}

	// Early cancellation check.
	select {
	case <-ctx.Done():
		writeLogLine(w, map[string]interface{}{
			"level":       "error",
			"message":     "dispatch canceled before start",
			"dispatch_id": dispatchID,
			"error":       ctx.Err().Error(),
		})
		return rec, fmt.Errorf("canceled: %w", ctx.Err())
	default:
	}

	var exitCode int
	var resp map[string]interface{}

	// Multi-step execution.
	if len(spec.Steps) > 0 {
		writeLogLine(w, map[string]interface{}{
			"level":       "info",
			"message":     "multi-step command start",
			"dispatch_id": dispatchID,
			"steps":       len(spec.Steps),
		})

		for idx, st := range spec.Steps {
			stepContinue := st.ContinueOnError || keepGoing

			// Check context at step boundary.
			if err := ctx.Err(); err != nil {
				writeLogLine(w, map[string]interface{}{
					"level":       "error",
					"message":     "step skipped due to context error",
					"dispatch_id": dispatchID,
					"step_index":  idx,
					"error":       err.Error(),
				})
				return rec, fmt.Errorf("canceled or timeout: %w", err)
			}

			stepStart := time.Now()
			writeLogLine(w, map[string]interface{}{
				"level":             "info",
				"message":           "step start",
				"dispatch_id":       dispatchID,
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
					Args:     st.Args,
					Workdir:  st.Workdir,
					Env:      st.Env,
					UseStdio: true,
				}
				resp, exitCode = spawnStdio(ctx, tmp, pluginDir, req, w)

			default:
				writeLogLine(w, map[string]interface{}{
					"level":       "error",
					"message":     "unsupported executor in step",
					"dispatch_id": dispatchID,
					"step_index":  idx,
					"executor":    st.Executor,
				})
				return rec, fmt.Errorf("unsupported executor: %s", st.Executor)
			}

			// Apply stdio mutations
			if resp != nil {
				if msg, ok := resp["message"].(string); ok && strings.TrimSpace(msg) != "" {
					// Keep latest stdio message as result (End step should override)
					rec.Result = msg
				}
				if status, _ := resp["status"].(string); status != "ok" {
					writeLogLine(w, map[string]interface{}{
						"level":       "error",
						"message":     "step stdio error",
						"dispatch_id": dispatchID,
						"step_index":  idx,
						"error":       fmt.Sprintf("%v", resp["message"]),
					})
					if !stepContinue {
						return rec, fmt.Errorf("plugin error: %v", resp["message"])
					}
				}
				applyMutations(envDir, pluginDir, man, globalCfg, pluginCfg, strategy, resp)
				echoStdioSideEffects(resp)
			}

			stepDur := time.Since(stepStart).Milliseconds()
			writeLogLine(w, map[string]interface{}{
				"level":       "info",
				"message":     "step end",
				"dispatch_id": dispatchID,
				"step_index":  idx,
				"duration_ms": stepDur,
				"exit_code":   exitCode,
			})

			if exitCode != 0 && !stepContinue {
				return rec, fmt.Errorf("plugin step exit code: %d", exitCode)
			}
		}

		// Multi-step done
		writeLogLine(w, map[string]interface{}{
			"level":       "info",
			"message":     "multi-step command end",
			"dispatch_id": dispatchID,
		})
		writeLogLine(w, map[string]interface{}{
			"level":       "info",
			"message":     fmt.Sprintf("Plugin log: %s", pluginLogFile),
			"dispatch_id": dispatchID,
		})

		rec.Status = "ok"
		return rec, nil
	}

	// Single-command execution.
	switch strings.ToLower(spec.Executor) {
	case "stdio":
		resp, exitCode = spawnStdio(ctx, spec, pluginDir, req, w)

	case "shell":
		exitCode = runShell(ctx, spec, pluginDir, passArgs, w)

	default:
		writeLogLine(w, map[string]interface{}{
			"level":       "error",
			"message":     "unsupported executor",
			"dispatch_id": dispatchID,
			"executor":    spec.Executor,
		})
		return rec, fmt.Errorf("unsupported executor: %s", spec.Executor)
	}

	if resp != nil {
		if msg, ok := resp["message"].(string); ok && strings.TrimSpace(msg) != "" {
			rec.Result = msg
		}
		if status, _ := resp["status"].(string); status != "ok" {
			return rec, fmt.Errorf("plugin error: %v", resp["message"])
		}
		applyMutations(envDir, pluginDir, man, globalCfg, pluginCfg, strategy, resp)
		echoStdioSideEffects(resp)
	}

	// End of dispatch
	writeLogLine(w, map[string]interface{}{
		"level":       "info",
		"message":     "dispatch end",
		"dispatch_id": dispatchID,
		"exit_code":   exitCode,
	})

	if isJSONMode() {
		fmt.Fprintf(os.Stderr, "Plugin log: %s\n", pluginLogFile)
	} else {
		fmt.Printf("Plugin log: %s\n", pluginLogFile)
	}

	if exitCode != 0 {
		return rec, fmt.Errorf("plugin command exit code: %d", exitCode)
	}

	rec.Status = "ok"
	return rec, nil
}

func newDispatchID() string {
	// English comments: stable enough without extra deps.
	// Format: d_<unixnano>_<pid>
	return fmt.Sprintf("d_%d_%d", time.Now().UnixNano(), os.Getpid())
}

// ctxTimeoutSeconds extracts remaining seconds until context deadline, best-effort.
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

func parseLastJSONObject(stdout string) (map[string]interface{}, error) {
	s := strings.TrimSpace(stdout)
	if s == "" {
		return nil, fmt.Errorf("empty stdout (EOF)")
	}

	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		ln := strings.TrimSpace(lines[i])
		if ln == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(ln), &obj); err == nil {
			return obj, nil
		}
	}
	return nil, fmt.Errorf("no valid JSON object found in stdout")
}

func trimForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

// spawnStdio starts a stdio-capable program.
// - Workdir resolution: prefer spec.Workdir (abs or plugin-relative), else <plugin>/scripts if exists, else <plugin>.
// - Program resolution: abs => as-is; relative with path sep => join with <plugin>; bare name => check in workdir first.
// - Shebang detection: if shebang exists, explicitly launch the interpreter with script path.
// - Windows fallback: wrap .py/.js/.sh with interpreter (py/python/node/bash).
// - Sends JSON request to stdin; decodes JSON response from stdout.
func spawnStdio(ctx context.Context, spec *CommandSpec, pluginDir string, req map[string]interface{}, w *bufio.Writer) (map[string]interface{}, int) {
	// Ensure absolute plugin directory.
	absPluginDir, err := filepath.Abs(pluginDir)
	if err != nil {
		writeLogLine(w, map[string]interface{}{"level": "error", "message": "abs pluginDir failed", "error": err.Error()})
		return map[string]interface{}{"status": "error", "message": err.Error()}, 1
	}

	// Resolve effective workdir.
	resolvedWorkdir := absPluginDir
	if strings.TrimSpace(spec.Workdir) != "" {
		if filepath.IsAbs(spec.Workdir) {
			resolvedWorkdir = spec.Workdir
		} else {
			resolvedWorkdir = filepath.Join(absPluginDir, spec.Workdir)
		}
	} else {
		scriptsDir := filepath.Join(absPluginDir, "scripts")
		if st, err := os.Stat(scriptsDir); err == nil && st.IsDir() {
			resolvedWorkdir = scriptsDir
		}
	}

	// Resolve entry program.
	prog := strings.TrimSpace(spec.Program)
	if prog == "" {
		return map[string]interface{}{"status": "error", "message": "stdio executor requires 'program'"}, 1
	}

	// Normalize path separators (important for Windows when manifest uses ./scripts/x.py).
	prog = filepath.FromSlash(prog)

	entry := prog
	if filepath.IsAbs(prog) {
		// use as-is
	} else if strings.Contains(prog, "\\") || strings.Contains(prog, "/") {
		// treat as plugin-relative path
		entry = filepath.Join(absPluginDir, prog)
	} else {
		// bare program name => prefer file in workdir if present
		candidate := filepath.Join(resolvedWorkdir, prog)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			entry = candidate
		}
	}

	args := append([]string{}, spec.Args...) // manifest-defined args only

	// Detect shebang and build command accordingly.
	useInterpreter := false
	interp := ""
	scriptAbs := ""
	interpExtra := []string{} // interpreter flags, e.g. ["-3"] for py launcher

	// Small helpers (kept inside function to avoid changing other files).
	isWindowsAppsStub := func(p string) bool {
		low := strings.ToLower(p)
		return strings.Contains(low, `\microsoft\windowsapps\`)
	}

	findPythonOnWindows := func() (string, []string, bool) {
		// Prefer py launcher (best on Windows).
		if p, e := exec.LookPath("py"); e == nil && !isWindowsAppsStub(p) {
			return p, []string{"-3"}, true
		}
		// Prefer real python, reject WindowsApps stub.
		if p, e := exec.LookPath("python"); e == nil && !isWindowsAppsStub(p) {
			return p, nil, true
		}
		if p, e := exec.LookPath("python3"); e == nil && !isWindowsAppsStub(p) {
			return p, nil, true
		}
		return "", nil, false
	}

	// Shebang detection (works well on Unix; Windows not reliable, but keep it).
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
						// Example: #!/usr/bin/env python3
						if abs, err := exec.LookPath(fields[1]); err == nil {
							interp = abs
							useInterpreter = true
							scriptAbs = entry
						}
					} else if filepath.IsAbs(fields[0]) {
						// Absolute interpreter path
						interp = fields[0]
						useInterpreter = true
						scriptAbs = entry
					}
				}
			}
		}
	}

	// --- Windows fallback: wrap scripts by extension ---
	if runtime.GOOS == "windows" && !useInterpreter {
		low := strings.ToLower(entry)

		switch {
		case strings.HasSuffix(low, ".py"):
			p, extra, ok := findPythonOnWindows()
			if !ok {
				msg := "python not found in PATH (or only WindowsApps stub). Install Python or disable App Execution Aliases."
				writeLogLine(w, map[string]interface{}{"level": "error", "message": msg, "entry": entry})
				return map[string]interface{}{"status": "error", "message": msg}, 1
			}
			interp = p
			interpExtra = extra // e.g. ["-3"] when using py
			useInterpreter = true
			scriptAbs = entry

		case strings.HasSuffix(low, ".js"):
			if p, e := exec.LookPath("node"); e == nil && !isWindowsAppsStub(p) {
				interp = p
				useInterpreter = true
				scriptAbs = entry
			} else {
				msg := "node not found in PATH (required for .js stdio scripts on Windows)"
				writeLogLine(w, map[string]interface{}{"level": "error", "message": msg, "entry": entry})
				return map[string]interface{}{"status": "error", "message": msg}, 1
			}

		case strings.HasSuffix(low, ".sh"):
			// Optional: if user has Git-Bash/WSL bash in PATH.
			if p, e := exec.LookPath("bash"); e == nil {
				interp = p
				useInterpreter = true
				scriptAbs = entry
			} else {
				msg := "bash not found in PATH (required for .sh scripts on Windows)"
				writeLogLine(w, map[string]interface{}{"level": "error", "message": msg, "entry": entry})
				return map[string]interface{}{"status": "error", "message": msg}, 1
			}
		}
	}

	// Non-Windows fallback: if no shebang, try by extension (keeps scripts runnable).
	if !useInterpreter {
		ext := strings.ToLower(filepath.Ext(entry))
		switch ext {
		case ".py":
			if abs, err := exec.LookPath("python3"); err == nil {
				interp = abs
			} else if abs, err := exec.LookPath("python"); err == nil {
				interp = abs
			}
		case ".js":
			if abs, err := exec.LookPath("node"); err == nil {
				interp = abs
			}
		case ".sh":
			if abs, err := exec.LookPath("bash"); err == nil {
				interp = abs
			} else if abs, err := exec.LookPath("sh"); err == nil {
				interp = abs
			}
		}
		if interp != "" {
			useInterpreter = true
			scriptAbs = entry
		}
	}

	// Build cmd
	var cmd *exec.Cmd
	if useInterpreter && interp != "" && scriptAbs != "" {
		// interp <interpExtra...> <scriptAbs> <args...>
		fullArgs := append([]string{}, interpExtra...)
		fullArgs = append(fullArgs, scriptAbs)
		fullArgs = append(fullArgs, args...)
		cmd = exec.CommandContext(ctx, interp, fullArgs...)
	} else {
		cmd = exec.CommandContext(ctx, entry, args...)
	}

	cmd.Dir = resolvedWorkdir
	cmd.Env = withExtraEnv(os.Environ(), spec.Env)

	// Capture stdout/stderr
	// Capture stdout (keep buffered: stdout is reserved for final JSON response)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	// Stream stderr to terminal in real-time, and also keep a copy in errBuf for logs/errors
	var errBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &errBuf)

	// IMPORTANT: send request JSON to stdin
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		writeLogLine(w, map[string]interface{}{
			"level":   "error",
			"message": "stdin pipe failed",
			"error":   err.Error(),
			"entry":   entry,
		})
		return map[string]interface{}{"status": "error", "message": err.Error()}, 1
	}

	go func() {
		enc := json.NewEncoder(stdinPipe)
		_ = enc.Encode(req)
		_ = stdinPipe.Close()
	}()

	// Debug: write spawn context to plugin JSONL log.
	writeLogLine(w, map[string]interface{}{
		"level":   "debug",
		"message": "spawn stdio",
		"entry":   entry,
		"cmd":     cmd.Path,
		"argv":    cmd.Args,
		"workdir": cmd.Dir,
	})

	// Run
	runErr := cmd.Run()

	stdoutStr := outBuf.String()
	stderrStr := errBuf.String()

	// Log stderr if non-empty
	if strings.TrimSpace(stderrStr) != "" {
		writeLogLine(w, map[string]interface{}{
			"level":   "error",
			"message": "stdio stderr",
			"entry":   entry,
			"stderr":  trimForLog(stderrStr, 4000),
		})
	}

	// If process failed, try to parse stdout as a stdio response (often contains useful JSON error).
	if runErr != nil {
		if strings.TrimSpace(stdoutStr) != "" {
			if resp2, err2 := parseLastJSONObject(stdoutStr); err2 == nil {
				writeLogLine(w, map[string]interface{}{
					"level":   "error",
					"message": "stdio returned error response",
					"entry":   entry,
					"error":   runErr.Error(),
					"stdout":  trimForLog(stdoutStr, 2000),
					"stderr":  trimForLog(stderrStr, 2000),
				})
				return resp2, exitCode(runErr)
			}
		}

		writeLogLine(w, map[string]interface{}{
			"level":   "error",
			"message": "stdio process failed",
			"entry":   entry,
			"error":   runErr.Error(),
			"stderr":  trimForLog(stderrStr, 2000),
			"stdout":  trimForLog(stdoutStr, 2000),
		})
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("stdio process failed: %v", runErr),
		}, exitCode(runErr)
	}

	// Parse stdout as JSON response (last JSON object).
	resp, parseErr := parseLastJSONObject(stdoutStr)
	if parseErr != nil {
		writeLogLine(w, map[string]interface{}{
			"level":   "error",
			"message": "resp decode failed",
			"error":   parseErr.Error(),
			"stdout":  trimForLog(stdoutStr, 4000),
			"stderr":  trimForLog(stderrStr, 4000),
		})
		return map[string]interface{}{
			"status":  "error",
			"message": parseErr.Error(),
		}, 1
	}

	return resp, 0
}

// isBareCommand reports whether program path is a single token without any path separator.
func isBareCommand(p string) bool {
	return !filepath.IsAbs(p) && !strings.ContainsRune(p, os.PathSeparator)
}

// runShell executes a shell command line. It currently uses "bash -c" on Unix-like
// systems and will fail on Windows if bash is not present. If you want full
// cross-platform support, replace the implementation to call an OS-aware utility
// (e.g., internal/runtime.BuildShellCommandCtx).
func runShell(ctx context.Context, spec *CommandSpec, pluginDir string, passArgs []string, w *bufio.Writer) int {
	line := strings.TrimSpace(spec.Program)
	if line == "" && len(spec.Args) > 0 {
		line = strings.Join(spec.Args, " ")
	}
	if len(passArgs) > 0 {
		line = strings.TrimSpace(line + " " + strings.Join(passArgs, " "))
	}

	// Convert env map to list: KEY=VAL
	var extra []string
	for k, v := range spec.Env {
		extra = append(extra, fmt.Sprintf("%s=%s", k, v))
	}

	workdir := dirOr(pluginDir, spec.Workdir)

	// Use OS-aware shell execution (Windows/cmd, Unix/bash/sh)
	cmd := runtime2.BuildShellCommandCtx(ctx, workdir, extra, line)
	cmd.Stdout = newLogWriter(w, "stdout")
	cmd.Stderr = newLogWriter(w, "stderr")

	return exitCode(cmd.Run())
}

// logPath returns a dated JSONL log file path inside the plugin logs directory.
func logPath(pluginDir, action string) string {
	d := filepath.Join(pluginDir, "logs", time.Now().UTC().Format("2006-01-02"))
	_ = os.MkdirAll(d, 0o755)
	ts := time.Now().UTC().Format("20060102T150405Z")
	return filepath.Join(d, fmt.Sprintf("%s-%s.log", action, ts))
}

// logWriter writes newline-delimited JSON log entries with a "level" and "message".
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

// writeLogLine marshals a map to JSON and appends it as a line to the buffer.
func writeLogLine(w *bufio.Writer, kv map[string]interface{}) {
	if _, ok := kv["ts"]; !ok {
		kv["ts"] = time.Now().UTC().Format(time.RFC3339)
	}
	b, _ := json.Marshal(kv)
	w.Write(b)
	w.WriteString("\n")
	w.Flush()
}

// exitCode extracts an exit code from error or returns 0 on nil, 1 otherwise.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

// dirOr returns an absolute working directory by prioritizing spec.workdir.
// If workdir is empty, the pluginDir is used. If workdir is relative, it is
// joined with pluginDir.
func dirOr(pluginDir, workdir string) string {
	if strings.TrimSpace(workdir) == "" {
		return pluginDir
	}
	if filepath.IsAbs(workdir) {
		return workdir
	}
	return filepath.Join(pluginDir, workdir)
}

// withExtraEnv appends extra environment variables to the base environment.
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

// applyMutations merges stdio mutations back into global and plugin configs.
// - global mutations obey the provided merge strategy
// - plugin local config always uses override for simplicity (MVP)
func applyMutations(envDir, pluginDir string, man *PluginManifest,
	globalCfg map[string]interface{}, pluginCfg map[string]interface{},
	strategy MergeStrategy, resp map[string]interface{}) {

	muts, ok := resp["mutations"].(map[string]interface{})
	if !ok {
		return
	}

	// Global config mutations with selected strategy.
	if g, ok := muts["global"].(map[string]interface{}); ok {
		merged := config.MergeMapWithStrategy(globalCfg, g, strategy)
		_ = config.SaveYAML(filepath.Join(envDir, "lyenv.yaml"), merged)
		// fmt.Printf("Global config updated (strategy=%s).\n", strategy)
	}

	// Plugin local config mutations (override semantics).
	if p, ok := muts["plugin"].(map[string]interface{}); ok && strings.TrimSpace(man.Config.LocalFile) != "" {
		merged := config.MergeMapWithStrategy(pluginCfg, p, config.MergeOverride)
		_ = config.SaveAny(filepath.Join(pluginDir, man.Config.LocalFile), merged)
		// fmt.Println("Plugin local config updated.")
	}
}

// echoStdioSideEffects prints stdio logs and artifacts to stdout for visibility.
func echoStdioSideEffects(resp map[string]interface{}) {
	out := os.Stdout
	if isJSONMode() {
		out = os.Stderr
	}

	// NEW: print message if present
	if msg, ok := resp["message"].(string); ok && strings.TrimSpace(msg) != "" && !isJSONMode() {
		fmt.Fprintln(out, msg)
	}

	if logs, ok := resp["logs"].([]interface{}); ok {
		for _, l := range logs {
			fmt.Fprintln(out, fmt.Sprint(l))
		}
	}
	if arts, ok := resp["artifacts"].([]interface{}); ok {
		for _, a := range arts {
			fmt.Fprintln(out, "Artifact:", fmt.Sprint(a))
		}
	}
}

func isJSONMode() bool {
	return strings.TrimSpace(os.Getenv("LYENV_JSON")) == "1"
}

func ensureDir(p string) {
	_ = os.MkdirAll(p, 0o755)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	ensureDir(filepath.Dir(dst))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
