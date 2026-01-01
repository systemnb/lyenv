package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"time"

	"gopkg.in/yaml.v3"
)

// lyenv version (can override with -ldflags)
const lyenvVersion = "0.1.0"

func usage() {
	fmt.Fprintf(os.Stderr, `lyenv - Directory-based isolated environment manager (MVP)
Usage:
  lyenv create <DIR>     Create a new lyenv environment directory
  lyenv init <DIR>       Initialize and verify an existing lyenv environment
  lyenv activate         Print shell snippet to activate the current lyenv
  lyenv config set <KEY> <VALUE> [--type=string|int|float|bool|json]
                                     Set a configuration value (dot path) with optional type enforcement
  lyenv config get <KEY>             Get a configuration value (dot path)
  lyenv config dump [<KEY>] <FILE>   Dump full config or a specific key to a file (YAML)
  lyenv config load <FILE> [--merge=override|append|keep]
                                     Load and merge a YAML file into lyenv.yaml with a merge strategy
  lyenv config importjson <FILE> <JSON_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]
                                     Import a value from a JSON file (dot path) into lyenv.yaml
									 lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]
                                     Import a value from a YAML file (dot path) into lyenv.yaml
  lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]
                                     Import a value from a YAML file (dot path) into lyenv.yaml

`)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "create":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Error: create requires exactly 1 argument <DIR>")
			os.Exit(2)
		}
		dir := strings.TrimSpace(args[1])
		if dir == "" {
			fmt.Fprintln(os.Stderr, "Error: <DIR> must not be empty")
			os.Exit(2)
		}
		if err := cmdCreate(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Create failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Environment created successfully: %s\n", dir)

	case "init":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "Error: init requires exactly 1 argument <DIR>")
			os.Exit(2)
		}
		dir := strings.TrimSpace(args[1])
		if dir == "" {
			fmt.Fprintln(os.Stderr, "Error: <DIR> must not be empty")
			os.Exit(2)
		}
		if err := cmdInit(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Init failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Environment initialized successfully.")

	case "activate":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Error: activate takes no arguments")
			os.Exit(2)
		}
		if err := cmdActivate(); err != nil {
			fmt.Fprintf(os.Stderr, "Activate failed: %v\n", err)
			os.Exit(1)
		}
		// NOTE: activate prints shell snippet to stdout; user should eval it.
	case "config":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: missing subcommand for config (set|get|dump|load|importjson)")
			os.Exit(2)
		}
		sub := args[1]
		switch sub {

		case "set":
			// lyenv config set <KEY> <VALUE> [--type=...]
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config set <KEY> <VALUE> [--type=string|int|float|bool|json]")
				os.Exit(2)
			}
			key := strings.TrimSpace(args[2])
			value := args[3]
			flags := parseFlags(args[4:]) // map[string]string
			typeOpt := flags["type"]
			if err := configSetWithType(".", "lyenv.yaml", key, value, typeOpt); err != nil {
				fmt.Fprintf(os.Stderr, "Config set failed: %v\n", err)
				os.Exit(1)
			}
			if typeOpt != "" {
				fmt.Printf("Config updated: %s=%s (type=%s)\n", key, value, typeOpt)
			} else {
				fmt.Printf("Config updated: %s=%s\n", key, value)
			}

		case "get":
			// lyenv config get <KEY>
			if len(args) != 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config get <KEY>")
				os.Exit(2)
			}
			key := strings.TrimSpace(args[2])
			out, err := configGet(".", "lyenv.yaml", key)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Config get failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(out)

		case "dump":
			// lyenv config dump <FILE>             -> dump all
			// lyenv config dump <KEY> <FILE>       -> dump specific key
			if len(args) == 3 {
				file := strings.TrimSpace(args[2])
				if err := configDump(".", "lyenv.yaml", "", file); err != nil {
					fmt.Fprintf(os.Stderr, "Config dump failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Config dumped to: %s\n", file)
			} else if len(args) == 4 {
				key := strings.TrimSpace(args[2])
				file := strings.TrimSpace(args[3])
				if err := configDump(".", "lyenv.yaml", key, file); err != nil {
					fmt.Fprintf(os.Stderr, "Config dump failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Config key dumped: %s -> %s\n", key, file)
			} else {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config dump [<KEY>] <FILE>")
				os.Exit(2)
			}

		case "load":
			// lyenv config load <FILE> [--merge=override|append|keep]
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config load <FILE> [--merge=override|append|keep]")
				os.Exit(2)
			}
			file := strings.TrimSpace(args[2])
			flags := parseFlags(args[3:])
			strategy := parseMergeStrategy(flags["merge"]) // default override
			if err := configLoadWithStrategy(".", "lyenv.yaml", file, strategy); err != nil {
				fmt.Fprintf(os.Stderr, "Config load failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config loaded and merged from: %s (strategy=%s)\n", file, strategy)

		case "importjson":
			// lyenv config importjson <FILE> <JSON_KEY> [--to=<CONFIG_KEY>] [--type=...] [--merge=...] [--input=1]
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config importjson <FILE> <JSON_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]")
				os.Exit(2)
			}
			jsonFile := strings.TrimSpace(args[2])
			jsonKey := strings.TrimSpace(args[3])
			flags := parseFlags(args[4:])

			destKey := flags["to"]
			if destKey == "" {
				destKey = jsonKey // default: same key path in config
			}
			typeOpt := flags["type"]
			strategy := parseMergeStrategy(flags["merge"])
			inputOn := flags["input"] == "1"

			if err := configImportJSON(".", "lyenv.yaml", jsonFile, jsonKey, destKey, typeOpt, strategy, inputOn); err != nil {
				fmt.Fprintf(os.Stderr, "Config importjson failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config updated from JSON: %s[%s] -> %s (type=%s, strategy=%s)\n",
				jsonFile, jsonKey, destKey, nonEmpty(typeOpt, "auto"), strategy)

		case "importyaml":
			// lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=...] [--merge=...] [--input=1]
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]")
				os.Exit(2)
			}
			yamlFile := strings.TrimSpace(args[2])
			yamlKey := strings.TrimSpace(args[3])
			flags := parseFlags(args[4:])

			destKey := flags["to"]
			if destKey == "" {
				destKey = yamlKey // default: same key path in config
			}
			typeOpt := flags["type"]
			strategy := parseMergeStrategy(flags["merge"])
			inputOn := flags["input"] == "1"

			if err := configImportYAML(".", "lyenv.yaml", yamlFile, yamlKey, destKey, typeOpt, strategy, inputOn); err != nil {
				fmt.Fprintf(os.Stderr, "Config importyaml failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config updated from YAML: %s[%s] -> %s (type=%s, strategy=%s)\n",
				yamlFile, yamlKey, destKey, nonEmpty(typeOpt, "auto"), strategy)

		default:
			fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", sub)
			os.Exit(2)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

// cmdCreate creates the directory structure for a lyenv environment.
func cmdCreate(dir string) error {
	absDir, errAbs := filepath.Abs(dir)
	if errAbs != nil {
		return fmt.Errorf("failed to resolve target path: %w", errAbs)
	}

	// Debug hint
	// fmt.Printf("Debug: resolved absolute path: %s\n", absDir)

	// Check if target exists
	fi, errStat := os.Stat(absDir)
	if errStat == nil {
		// Target exists
		if !fi.IsDir() {
			return fmt.Errorf("target exists but is not a directory: %s", absDir)
		}
		// Is a directory — check if already a lyenv dir
		if isLyenvDir(absDir) {
			fmt.Println("Note: target directory already looks like a lyenv environment. Skipped.")
			return nil
		}
	} else if !errors.Is(errStat, os.ErrNotExist) {
		// Stat failed for reasons other than "not exists"
		return fmt.Errorf("failed to stat target directory: %w", errStat)
	}

	// Create base structure (mkdir -p style)
	subdirs := []string{
		filepath.Join(absDir, ".lyenv"),
		filepath.Join(absDir, ".lyenv", "logs"),
		filepath.Join(absDir, ".lyenv", "registry"),
		filepath.Join(absDir, "bin"),
		filepath.Join(absDir, "cache"),
		filepath.Join(absDir, "plugins"),
		filepath.Join(absDir, "workspace"),
	}
	for _, d := range subdirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Write metadata
	if err := writeFileIfNotExists(filepath.Join(absDir, ".lyenv", "version"), lyenvVersion+"\n", 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/version: %w", err)
	}
	stateJSON := `{
  "created_at": "` + time.Now().Local().String() + `",
  "components": [],
  "notes": ""
}
`
	if err := writeFileIfNotExists(filepath.Join(absDir, ".lyenv", "state.json"), stateJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/state.json: %w", err)
	}

	// Default environment configuration
	defaultCfg := `env:
  name: "default"
  platform: "auto"        # auto-detect system platform
path:
  bin: "./bin"
  cache: "./cache"
  workspace: "./workspace"
plugins:
  installed: []
config:
  use_container: false
  pkg_manager: "auto"     # auto-detect package manager
`
	cfgPath := filepath.Join(absDir, "lyenv.yaml")
	if err := writeFileIfNotExists(cfgPath, defaultCfg, 0o644); err != nil {
		return fmt.Errorf("failed to write lyenv.yaml: %w", err)
	}

	return nil
}

// cmdActivate prints a snippet to activate the lyenv environment.
func cmdInit(dir string) error {
	absDir, errAbs := filepath.Abs(dir)
	if errAbs != nil {
		return fmt.Errorf("failed to resolve target path: %w", errAbs)
	}

	fmt.Println("Checking environment...")

	// Must exist and be a directory
	fi, errStat := os.Stat(absDir)
	if errStat != nil {
		return fmt.Errorf("target directory not found: %s", absDir)
	}
	if !fi.IsDir() {
		return fmt.Errorf("target exists but is not a directory: %s", absDir)
	}

	// Create missing structure (idempotent)
	subdirs := []string{
		filepath.Join(absDir, ".lyenv"),
		filepath.Join(absDir, ".lyenv", "logs"),
		filepath.Join(absDir, ".lyenv", "registry"),
		filepath.Join(absDir, "bin"),
		filepath.Join(absDir, "cache"),
		filepath.Join(absDir, "plugins"),
		filepath.Join(absDir, "workspace"),
	}
	for _, d := range subdirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Ensure metadata files exist
	if err := writeFileIfNotExists(filepath.Join(absDir, ".lyenv", "version"), lyenvVersion+"\n", 0o644); err != nil {
		return fmt.Errorf("failed to write .lyenv/version: %w", err)
	}
	// Merge or create state.json with initialized_at (simple append if missing)
	statePath := filepath.Join(absDir, ".lyenv", "state.json")
	_ = ensureInitializedAt(statePath)

	// Ensure main config exists
	defaultCfg := `env:
  name: "default"
  platform: "auto"        # auto-detect system platform
path:
  bin: "./bin"
  cache: "./cache"
  workspace: "./workspace"
plugins:
  installed: []
config:
  use_container: false
  pkg_manager: "auto"     # auto-detect package manager
`
	if err := writeFileIfNotExists(filepath.Join(absDir, "lyenv.yaml"), defaultCfg, 0o644); err != nil {
		return fmt.Errorf("failed to write lyenv.yaml: %w", err)
	}

	fmt.Println("OK: structure verified")
	return nil
}

// ensureInitializedAt writes/updates initialized_at in state.json (best-effort).
func ensureInitializedAt(statePath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	// If not exists, create a minimal state.json with initialized_at
	_, err := os.Stat(statePath)
	if errors.Is(err, os.ErrNotExist) {
		content := `{
  "created_at": "` + now + `",
  "initialized_at": "` + now + `",
  "components": [],
  "notes": ""
}
`
		return os.WriteFile(statePath, []byte(content), 0o644)
	}
	// If exists, append a simple marker file next to it to avoid full JSON merge complexity in MVP
	marker := statePath + ".initialized"
	return os.WriteFile(marker, []byte(now+"\n"), 0o644)
}

// cmdActivate prints a snippet to activate the lyenv environment.
func cmdActivate() error {
	// Use current working directory as LYENV_HOME
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	bin := filepath.Join(cwd, "bin")

	// Print shell snippet (bash/zsh compatible). User should: eval "$(lyenv activate)"
	// Ensures PATH and prompt prefix "(lyenv) ".
	fmt.Printf("export LYENV_HOME=%q\n", cwd)
	fmt.Printf("export PATH=%q:$PATH\n", bin)
	fmt.Println(`export LYENV_ACTIVE=1`)
	// Avoid double prefix by wrapping PS1 update in a guard (simple check)
	// Note: We cannot evaluate conditions here; rely on shell evaluation.
	// The snippet will set PS1 to "(lyenv) " + current PS1.
	fmt.Println(`if [ -z "${LYENV_PROMPT_APPLIED+x}" ]; then`)
	fmt.Println(`  export LYENV_PROMPT_APPLIED=1`)
	fmt.Println(`  export PS1="(lyenv) ${PS1}"`)
	fmt.Println(`fi`)

	return nil
}

// isLyenvDir checks if the directory already looks like a lyenv environment.
func isLyenvDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".lyenv", "version")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "lyenv.yaml")); err == nil {
		return true
	}
	return false
}

// writeFileIfNotExists writes a file only if it does not already exist.
func writeFileIfNotExists(path, content string, perm os.FileMode) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil // skip existing
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check file %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(content), perm)
}

// ---------- Flag Parsing & Utilities ----------

func parseFlags(args []string) map[string]string {
	flags := make(map[string]string)
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			continue
		}
		a = strings.TrimPrefix(a, "--")
		if i := strings.IndexByte(a, '='); i >= 0 {
			k := a[:i]
			v := a[i+1:]
			flags[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		} else {
			// flags without value -> set to "1"
			flags[strings.ToLower(strings.TrimSpace(a))] = "1"
		}
	}
	return flags
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// ---------- YAML IO ----------

func loadYAML(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]interface{})
	}
	return m, nil
}

func saveYAML(path string, m map[string]interface{}) error {
	out, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// ---------- Dot Path Access ----------

func getByPath(m map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var cur interface{} = m
	for _, p := range parts {
		obj, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, exists := obj[p]
		if !exists {
			return nil, false
		}
		cur = val
	}
	return cur, true
}

func setByPath(m map[string]interface{}, path string, val interface{}) {
	parts := strings.Split(path, ".")
	cur := m
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = val
			return
		}
		next, ok := cur[p].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			cur[p] = next
		}
		cur = next
	}
}

// ---------- Type Parsing ----------

func parseWithType(raw string, typeOpt string) (interface{}, error) {
	switch strings.ToLower(strings.TrimSpace(typeOpt)) {
	case "":
		return parseScalar(raw), nil
	case "string":
		return raw, nil
	case "int":
		i, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int: %v", err)
		}
		return i, nil
	case "float":
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float: %v", err)
		}
		return f, nil
	case "bool":
		l := strings.ToLower(strings.TrimSpace(raw))
		if l == "true" || l == "1" || l == "yes" {
			return true, nil
		}
		if l == "false" || l == "0" || l == "no" {
			return false, nil
		}
		return nil, fmt.Errorf("invalid bool: %q (accepted: true/false/1/0/yes/no)", raw)
	case "json":
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, fmt.Errorf("invalid JSON: %v", err)
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type: %s", typeOpt)
	}
}

func parseScalar(s string) interface{} {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// ---------- Merge Strategy ----------

type MergeStrategy string

const (
	MergeOverride MergeStrategy = "override" // overlay replaces base
	MergeAppend   MergeStrategy = "append"   // maps: deep-merge; arrays: concatenate; scalars: overlay replaces
	MergeKeep     MergeStrategy = "keep"     // keep base; overlay fills only missing keys
)

func parseMergeStrategy(s string) MergeStrategy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "append":
		return MergeAppend
	case "keep":
		return MergeKeep
	case "override", "":
		return MergeOverride
	default:
		return MergeOverride
	}
}

func mergeMapWithStrategy(base, overlay map[string]interface{}, strategy MergeStrategy) map[string]interface{} {
	// copy base to avoid mutating input
	if base == nil {
		base = make(map[string]interface{})
	}
	if overlay == nil {
		return base
	}

	for k, ov := range overlay {
		bv, exists := base[k]
		if !exists {
			// not exist -> always set
			base[k] = ov
			continue
		}

		bm, bIsMap := bv.(map[string]interface{})
		om, oIsMap := ov.(map[string]interface{})
		ba, bIsArr := toIfaceSlice(bv)
		oa, oIsArr := toIfaceSlice(ov)

		switch strategy {
		case MergeKeep:
			// keep base; only add missing keys (already handled above)
			continue

		case MergeAppend:
			if bIsMap && oIsMap {
				base[k] = mergeMapWithStrategy(bm, om, strategy)
			} else if bIsArr && oIsArr {
				base[k] = append(ba, oa...) // concatenate arrays
			} else {
				// scalar or mismatched types -> overlay replaces
				base[k] = ov
			}

		case MergeOverride:
			if bIsMap && oIsMap {
				// Deep override for maps
				base[k] = mergeMapWithStrategy(bm, om, strategy)
			} else {
				// replace
				base[k] = ov
			}
		}
	}
	return base
}

func toIfaceSlice(v interface{}) ([]interface{}, bool) {
	switch a := v.(type) {
	case []interface{}:
		return a, true
	default:
		return nil, false
	}
}

// ---------- Config Operations ----------

func configSetWithType(envDir, cfgFile, key, rawValue, typeOpt string) error {
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := loadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	val, err := parseWithType(rawValue, typeOpt)
	if err != nil {
		return err
	}
	setByPath(m, key, val)
	if err := saveYAML(cfgPath, m); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

func configGet(envDir, cfgFile, key string) (string, error) {
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := loadYAML(cfgPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config: %w", err)
	}
	val, ok := getByPath(m, key)
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	switch v := val.(type) {
	case string:
		return v + "\n", nil
	case bool:
		if v {
			return "true\n", nil
		}
		return "false\n", nil
	case int, int64, float64, float32:
		return fmt.Sprintf("%v\n", v), nil
	default:
		out, err := yaml.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to serialize value: %w", err)
		}
		return string(out), nil
	}
}

func configDump(envDir, cfgFile, key, outFile string) error {
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := loadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	var toWrite interface{} = m
	if key != "" {
		val, ok := getByPath(m, key)
		if !ok {
			return fmt.Errorf("key not found: %s", key)
		}
		toWrite = val
	}
	out, err := yaml.Marshal(toWrite)
	if err != nil {
		return fmt.Errorf("failed to serialize YAML: %w", err)
	}
	if err := os.WriteFile(outFile, out, 0o644); err != nil {
		return fmt.Errorf("failed to write dump file: %w", err)
	}
	return nil
}

func configLoadWithStrategy(envDir, cfgFile, srcFile string, strategy MergeStrategy) error {
	cfgPath := filepath.Join(envDir, cfgFile)
	base, err := loadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read base config: %w", err)
	}
	overlay, err := loadYAML(srcFile)
	if err != nil {
		return fmt.Errorf("failed to read source config: %w", err)
	}

	merged := mergeMapWithStrategy(base, overlay, strategy)
	if err := saveYAML(cfgPath, merged); err != nil {
		return fmt.Errorf("failed to write merged config: %w", err)
	}
	return nil
}

// Import a value from JSON file and write into lyenv.yaml, supporting type and merge strategy.
func configImportJSON(envDir, cfgFile, jsonFile, jsonKey, destKey, typeOpt string, strategy MergeStrategy, inputOn bool) error {
	// Load JSON
	jf, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}
	var jm interface{}
	if err := json.Unmarshal(jf, &jm); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	// Extract value by dot path
	jmap, ok := jm.(map[string]interface{})
	var jval interface{}
	var found bool
	if ok {
		jval, found = getByPath(jmap, jsonKey)
	}
	if !found || jval == nil {
		if inputOn {
			// Prompt user for a value (raw text)
			prompt := fmt.Sprintf("Enter value for JSON key '%s' (type=%s, or 'json'):", jsonKey, nonEmpty(typeOpt, "auto"))
			in, err := promptLine(prompt)
			if err != nil {
				return fmt.Errorf("input failed: %v", err)
			}
			// Parse with type
			parsed, err := parseWithType(in, typeOpt)
			if err != nil {
				return err
			}
			jval = parsed
		} else {
			return fmt.Errorf("JSON key not found or empty: %s", jsonKey)
		}
	}

	// Coerce type if requested
	if typeOpt != "" {
		// If original is a complex object and typeOpt != json, we still allow coercion for scalars
		raw := toJSONStringIfNeeded(jval)
		parsed, err := parseWithType(raw, typeOpt)
		if err != nil {
			return err
		}
		jval = parsed
	}

	// Load YAML config
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := loadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// If destination key exists and both sides are composite, apply merge strategy.
	existing, exists := getByPath(m, destKey)
	if exists {
		// Merge behavior:
		em, eIsMap := existing.(map[string]interface{})
		jm2, jIsMap := jval.(map[string]interface{})
		ea, eIsArr := toIfaceSlice(existing)
		ja, jIsArr := toIfaceSlice(jval)

		if eIsMap && jIsMap {
			merged := mergeMapWithStrategy(em, jm2, strategy)
			setByPath(m, destKey, merged)
		} else if eIsArr && jIsArr {
			switch strategy {
			case MergeAppend:
				setByPath(m, destKey, append(ea, ja...))
			case MergeKeep:
				// keep base
				// no-op
			case MergeOverride:
				setByPath(m, destKey, jval)
			}
		} else {
			// scalar or type mismatch -> follow strategy
			switch strategy {
			case MergeKeep:
				// keep existing
			case MergeOverride, MergeAppend:
				// treat append as override for scalars/mismatch
				setByPath(m, destKey, jval)
			}
		}
	} else {
		// Not exists -> set
		setByPath(m, destKey, jval)
	}

	if err := saveYAML(cfgPath, m); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// Import a value from YAML file and write into lyenv.yaml, supporting type and merge strategy.
// Note: The YAML root must be a mapping for dot path traversal; list root is not supported in MVP.
func configImportYAML(envDir, cfgFile, yamlFile, yamlKey, destKey, typeOpt string, strategy MergeStrategy, inputOn bool) error {
	// Load YAML (as map)
	ymap, err := loadYAML(yamlFile)
	if err != nil {
		return fmt.Errorf("failed to read YAML file: %w", err)
	}

	// Extract value by dot path
	yval, found := getByPath(ymap, yamlKey)
	if !found || yval == nil {
		if inputOn {
			prompt := fmt.Sprintf("Enter value for YAML key '%s' (type=%s, or 'json'):", yamlKey, nonEmpty(typeOpt, "auto"))
			in, err := promptLine(prompt)
			if err != nil {
				return fmt.Errorf("input failed: %v", err)
			}
			parsed, err := parseWithType(in, typeOpt)
			if err != nil {
				return err
			}
			yval = parsed
		} else {
			return fmt.Errorf("YAML key not found or empty: %s", yamlKey)
		}
	}

	// Coerce type if requested
	if typeOpt != "" {
		raw := toJSONStringIfNeeded(yval)
		parsed, err := parseWithType(raw, typeOpt)
		if err != nil {
			return err
		}
		yval = parsed
	}

	// Load lyenv YAML config
	cfgPath := filepath.Join(envDir, cfgFile)
	m, err := loadYAML(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Merge into destination key according to strategy
	existing, exists := getByPath(m, destKey)
	if exists {
		em, eIsMap := existing.(map[string]interface{})
		ym, yIsMap := yval.(map[string]interface{})
		ea, eIsArr := toIfaceSlice(existing)
		ya, yIsArr := toIfaceSlice(yval)

		if eIsMap && yIsMap {
			merged := mergeMapWithStrategy(em, ym, strategy)
			setByPath(m, destKey, merged)
		} else if eIsArr && yIsArr {
			switch strategy {
			case MergeAppend:
				setByPath(m, destKey, append(ea, ya...))
			case MergeKeep:
				// keep base
			case MergeOverride:
				setByPath(m, destKey, yval)
			}
		} else {
			// scalar or mismatched types
			switch strategy {
			case MergeKeep:
				// keep existing
			case MergeOverride, MergeAppend:
				setByPath(m, destKey, yval)
			}
		}
	} else {
		// Not exists -> set
		setByPath(m, destKey, yval)
	}

	if err := saveYAML(cfgPath, m); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// ---------- Small helpers ----------

func toJSONStringIfNeeded(v interface{}) string {
	switch vv := v.(type) {
	case string:
		return vv
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func promptLine(prompt string) (string, error) {
	fmt.Println(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		// In case of EOF without newline, still accept the buffer
		if errors.Is(err, os.ErrClosed) {
			return strings.TrimSpace(line), nil
		}
		if line != "" {
			return strings.TrimSpace(line), nil
		}
		return "", err
	}
	return strings.TrimSpace(line), nil
}
