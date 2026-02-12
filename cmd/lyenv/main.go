package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lyenv/internal/cli"
	"lyenv/internal/config"
	"lyenv/internal/env"
	"lyenv/internal/guictl"
	"lyenv/internal/plugin"
	"lyenv/internal/version"
)

func usage() {
	cli.Usage()
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

	case "--version":
		fmt.Printf("lyenv %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildTime)
		return

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
		if err := env.CmdCreate(dir); err != nil {
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
		if err := env.CmdInit(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Init failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Environment initialized successfully.")

	case "activate":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Error: activate takes no arguments")
			os.Exit(2)
		}
		if err := env.CmdActivate(); err != nil {
			fmt.Fprintf(os.Stderr, "Activate failed: %v\n", err)
			os.Exit(1)
		}

	case "config":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: missing subcommand for config (set|get|dump|load|importjson|importyaml)")
			os.Exit(2)
		}
		sub := args[1]
		switch sub {
		case "set":
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config set <KEY> <VALUE> [--type=string|int|float|bool|json]")
				os.Exit(2)
			}
			key := strings.TrimSpace(args[2])
			value := args[3]
			flags := config.ParseFlags(args[4:])
			typeOpt := flags["type"]
			if err := config.ConfigSetWithType(".", "lyenv.yaml", key, value, typeOpt); err != nil {
				fmt.Fprintf(os.Stderr, "Config set failed: %v\n", err)
				os.Exit(1)
			}
			if typeOpt != "" {
				fmt.Printf("Config updated: %s=%s (type=%s)\n", key, value, typeOpt)
			} else {
				fmt.Printf("Config updated: %s=%s\n", key, value)
			}

		case "get":
			if len(args) != 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config get <KEY>")
				os.Exit(2)
			}
			key := strings.TrimSpace(args[2])
			out, err := config.ConfigGet(".", "lyenv.yaml", key)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Config get failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(out)

		case "dump":
			if len(args) == 3 {
				file := strings.TrimSpace(args[2])
				if err := config.ConfigDump(".", "lyenv.yaml", "", file); err != nil {
					fmt.Fprintf(os.Stderr, "Config dump failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Config dumped to: %s\n", file)
			} else if len(args) == 4 {
				key := strings.TrimSpace(args[2])
				file := strings.TrimSpace(args[3])
				if err := config.ConfigDump(".", "lyenv.yaml", key, file); err != nil {
					fmt.Fprintf(os.Stderr, "Config dump failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Config key dumped: %s -> %s\n", key, file)
			} else {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config dump [<KEY>] <FILE>")
				os.Exit(2)
			}

		case "load":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config load <FILE> [--merge=override|append|keep]")
				os.Exit(2)
			}
			file := strings.TrimSpace(args[2])
			flags := config.ParseFlags(args[3:])
			strategy := config.ParseMergeStrategy(flags["merge"])
			if err := config.ConfigLoadWithStrategy(".", "lyenv.yaml", file, strategy); err != nil {
				fmt.Fprintf(os.Stderr, "Config load failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config loaded and merged from: %s (strategy=%s)\n", file, strategy)

		case "importjson":
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config importjson <FILE> <JSON_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]")
				os.Exit(2)
			}
			jsonFile := strings.TrimSpace(args[2])
			jsonKey := strings.TrimSpace(args[3])
			flags := config.ParseFlags(args[4:])
			destKey := flags["to"]
			if destKey == "" {
				destKey = jsonKey
			}
			typeOpt := flags["type"]
			strategy := config.ParseMergeStrategy(flags["merge"])
			inputOn := flags["input"] == "1"
			if err := config.ConfigImportJSON(".", "lyenv.yaml", jsonFile, jsonKey, destKey, typeOpt, strategy, inputOn); err != nil {
				fmt.Fprintf(os.Stderr, "Config importjson failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config updated from JSON: %s[%s] -> %s (type=%s, strategy=%s)\n",
				jsonFile, jsonKey, destKey, config.NonEmpty(typeOpt, "auto"), strategy)

		case "importyaml":
			if len(args) < 4 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]")
				os.Exit(2)
			}
			yamlFile := strings.TrimSpace(args[2])
			yamlKey := strings.TrimSpace(args[3])
			flags := config.ParseFlags(args[4:])
			destKey := flags["to"]
			if destKey == "" {
				destKey = yamlKey
			}
			typeOpt := flags["type"]
			strategy := config.ParseMergeStrategy(flags["merge"])
			inputOn := flags["input"] == "1"
			if err := config.ConfigImportYAML(".", "lyenv.yaml", yamlFile, yamlKey, destKey, typeOpt, strategy, inputOn); err != nil {
				fmt.Fprintf(os.Stderr, "Config importyaml failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Config updated from YAML: %s[%s] -> %s (type=%s, strategy=%s)\n",
				yamlFile, yamlKey, destKey, config.NonEmpty(typeOpt, "auto"), strategy)

		default:
			fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", sub)
			os.Exit(2)
		}

	case "plugin":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: missing subcommand for plugin (install|list|info|remove)")
			os.Exit(2)
		}
		sub := args[1]
		switch sub {
		case "add":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin add <PATH|ZIP> [--name=<INSTALL_NAME>]")
				os.Exit(2)
			}
			// Find first non-flag as PATH or ZIP
			var pathOrZip string
			var flagArgs []string
			for _, a := range args[2:] {
				if strings.HasPrefix(a, "--") {
					flagArgs = append(flagArgs, a)
				} else if pathOrZip == "" {
					pathOrZip = a
				} else {
					fmt.Fprintln(os.Stderr, "Error: too many positional arguments for 'plugin add'")
					os.Exit(2)
				}
			}
			if pathOrZip == "" {
				fmt.Fprintln(os.Stderr, "Error: <PATH|ZIP> must not be empty")
				os.Exit(2)
			}
			flags := config.ParseFlags(flagArgs)
			overrideName := flags["name"]

			// NEW: support .zip directly
			if isZipFile(pathOrZip) {
				extractedRoot, cleanup, err := unzipToTempAndDetectRoot(pathOrZip)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ZIP extract failed: %v\n", err)
					os.Exit(1)
				}
				defer cleanup()
				if err := plugin.PluginAddLocal(".", extractedRoot, overrideName); err != nil {
					fmt.Fprintf(os.Stderr, "Plugin add (zip) failed: %v\n", err)
					os.Exit(1)
				}
			} else {
				if err := plugin.PluginAddLocal(".", pathOrZip, overrideName); err != nil {
					fmt.Fprintf(os.Stderr, "Plugin add failed: %v\n", err)
					os.Exit(1)
				}
			}

		case "install":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin install <NAME|PATH|ZIP> [--name=<INSTALL_NAME>] [--repo=<org/repo>] [--ref=<branch|tag|commit>] [--source=<url>] [--proxy=<url>]")
				os.Exit(2)
			}
			nameOrPath := strings.TrimSpace(args[2])
			flags := config.ParseFlags(args[3:])
			repo := flags["repo"]
			ref := flags["ref"]
			source := flags["source"]
			proxy := flags["proxy"]
			overrideName := flags["name"]

			if nameOrPath == "" {
				fmt.Fprintln(os.Stderr, "Error: <NAME|PATH|ZIP> must not be empty")
				os.Exit(2)
			}

			// NEW: local ZIP path install
			if isZipFile(nameOrPath) && fileExists(nameOrPath) {
				extractedRoot, cleanup, err := unzipToTempAndDetectRoot(nameOrPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "ZIP extract failed: %v\n", err)
					os.Exit(1)
				}
				defer cleanup()
				if err := plugin.PluginAddLocal(".", extractedRoot, overrideName); err != nil {
					fmt.Fprintf(os.Stderr, "Plugin install (zip) failed: %v\n", err)
					os.Exit(1)
				}
			} else {
				// existing logic (center by name, local path, repo/ref/source/proxy)
				if err := plugin.PluginAdd(".", nameOrPath, source, repo, ref, proxy, overrideName); err != nil {
					fmt.Fprintf(os.Stderr, "Plugin install failed: %v\n", err)
					os.Exit(1)
				}
			}

		case "info":
			if len(args) != 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin info <INSTALL_NAME|LOGICAL_NAME>")
				os.Exit(2)
			}
			input := strings.TrimSpace(args[2])
			dir, installName, err := plugin.ResolvePluginDir(".", input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Plugin info failed: %v\n", err)
				os.Exit(1)
			}
			man, err := plugin.LoadManifest(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Plugin info failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Install: %s\nName: %s\nVersion: %s\nDir: %s\n", installName, man.Name, man.Version, dir)
			if len(man.Commands) > 0 {
				fmt.Println("Commands:")
				for _, c := range man.Commands {
					fmt.Printf("  - %s: %s (executor=%s)\n", c.Name, c.Summary, c.Executor)
				}
			}
			if len(man.Expose) > 0 {
				fmt.Println("Exposed shims:")
				for _, s := range man.Expose {
					fmt.Printf("  - %s\n", s)
				}
			}

		case "remove":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin remove <INSTALL_NAME> [--force]")
				os.Exit(2)
			}
			installName := strings.TrimSpace(args[2])
			flags := config.ParseFlags(args[3:])
			force := flags["force"] == "1"
			if err := plugin.PluginRemove(".", installName, force); err != nil {
				fmt.Fprintf(os.Stderr, "Plugin remove failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Plugin removed: %s\n", installName)

		case "update":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin update <INSTALL_NAME> [--repo=<org/repo>] [--ref=<branch|tag|commit>] [--source=<url>] [--proxy=<url>]")
				os.Exit(2)
			}
			installName := strings.TrimSpace(args[2])
			flags := config.ParseFlags(args[3:])
			repo := flags["repo"]
			ref := flags["ref"]
			source := flags["source"]
			proxy := flags["proxy"]
			if err := plugin.PluginUpdate(".", installName, repo, ref, source, proxy); err != nil {
				fmt.Fprintf(os.Stderr, "Plugin update failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Plugin updated: %s\n", installName)

		case "list":
			flags := config.ParseFlags(args[2:])
			wantJSON := flags["json"] == "1"
			r, err := plugin.LoadRegistry(".")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Plugin list failed: %v\n", err)
				os.Exit(1)
			}
			if wantJSON {
				b, _ := json.MarshalIndent(r.Plugins, "", "  ")
				fmt.Println(string(b))
			} else {
				if len(r.Plugins) == 0 {
					fmt.Println("No plugins installed.")
				} else {
					for _, p := range r.Plugins {
						fmt.Printf("%s  %s  (%s)  install=%s  shims=%v\n",
							p.Name, p.Version, p.Source, p.InstallName, p.Shims)
					}
				}
			}

		case "search":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin search <KEYWORDS...>")
				os.Exit(2)
			}
			kws := args[2:]
			res, err := plugin.SearchCenterPlugins(".", kws)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Plugin search failed: %v\n", err)
				os.Exit(1)
			}
			if len(res) == 0 {
				fmt.Println("No matches.")
			} else {
				for _, line := range res {
					fmt.Println(line)
				}
			}

		case "center":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin center sync")
				os.Exit(2)
			}
			sub2 := args[2]
			switch sub2 {
			case "sync":
				p, err := plugin.CenterSync(".")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Center sync failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Center index cached: %s\n", p)
			default:
				fmt.Fprintf(os.Stderr, "Unknown plugin center subcommand: %s\n", sub2)
				os.Exit(2)
			}

		default:
			fmt.Fprintf(os.Stderr, "Unknown plugin subcommand: %s\n", sub)
			os.Exit(2)
		}

	case "run":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: usage: lyenv run <PLUGIN> <COMMAND> [--merge=override|append|keep] [-- ...args]")
			os.Exit(2)
		}
		pl := strings.TrimSpace(args[1])
		cmd := strings.TrimSpace(args[2])

		var rawFlags []string
		var passArgs []string

		for _, a := range args[3:] {
			if strings.HasPrefix(a, "--") {
				rawFlags = append(rawFlags, a)
			} else {
				passArgs = append(passArgs, a)
			}
		}

		flags := config.ParseFlags(rawFlags)
		strategy := config.ParseMergeStrategy(flags["merge"])

		// Parse timeout
		timeoutSec := int64(0)
		if v := strings.TrimSpace(flags["timeout"]); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
				timeoutSec = n
			}
		}

		// Parse fail-fast / keep-going
		keepGoing := flags["keep-going"] == "1"
		if flags["fail-fast"] == "1" {
			keepGoing = false
		}

		// Build context with timeout if provided
		ctx := context.Background()
		var cancel context.CancelFunc
		if timeoutSec > 0 {
			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer cancel()
		}

		wantJSON := flags["json"] == "1"

		if wantJSON {
			_ = os.Setenv("LYENV_JSON", "1")
		}

		// Call plugin runtime with options
		rec, err := plugin.RunPluginCommandWithRecord(ctx, ".", pl, cmd, passArgs, strategy, keepGoing)
		if wantJSON && rec != nil {
			b, _ := json.Marshal(rec)
			fmt.Println(string(b))
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Run failed: %v\n", err)
			os.Exit(1)
		}

	case "gui":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: missing subcommand for gui (start|stop|status|open)")
			os.Exit(2)
		}
		sub := args[1]
		flags := config.ParseFlags(args[2:])

		addr := strings.TrimSpace(flags["addr"])
		if addr == "" {
			addr = strings.TrimSpace(os.Getenv("LYENV_GUI_ADDR"))
		}
		if addr == "" {
			addr = "127.0.0.1:18888"
		}
		autoOpen := flags["open"] == "1"

		switch sub {
		case "start":
			if err := guictl.StartGlobal(addr); err != nil {
				fmt.Fprintf(os.Stderr, "GUI start failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("GUI started: http://%s\n", addr)
			if autoOpen {
				_ = guictl.OpenBrowser("http://" + addr)
			}
		case "stop":
			if err := guictl.StopGlobal(); err != nil {
				fmt.Fprintf(os.Stderr, "GUI stop failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("GUI stopped.")
		case "status":
			st, err := guictl.StatusGlobal()
			if err != nil {
				fmt.Fprintf(os.Stderr, "GUI status failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(st)
		case "open":
			_ = guictl.OpenBrowser("http://" + addr)

		case "add":
			// usage: lyenv gui add <DIR> [--name=xxx]
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv gui add <DIR> [--name=<ENV_NAME>]")
				os.Exit(2)
			}
			dir := strings.TrimSpace(args[2])
			name := strings.TrimSpace(flags["name"])
			createOn := true
			if flags["create"] == "0" {
				createOn = false
			}
			if err := guictl.GuiEnvAdd(dir, name, createOn); err != nil {
				fmt.Fprintf(os.Stderr, "GUI env add failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("OK")

		case "remove":
			// usage: lyenv gui remove <NAME|PATH>
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv gui remove <NAME|PATH>")
				os.Exit(2)
			}
			key := strings.TrimSpace(args[2])
			if err := guictl.GuiEnvRemove(key); err != nil {
				fmt.Fprintf(os.Stderr, "GUI env remove failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("OK")

		case "list":
			// usage: lyenv gui list [--json]
			wantJSON := flags["json"] == "1"
			lst, err := guictl.GuiEnvList()
			if err != nil {
				fmt.Fprintf(os.Stderr, "GUI env list failed: %v\n", err)
				os.Exit(1)
			}
			if wantJSON {
				b, _ := json.MarshalIndent(lst, "", "  ")
				fmt.Println(string(b))
			} else {
				if len(lst) == 0 {
					fmt.Println("No GUI environments registered. Use: lyenv gui add <DIR>")
				} else {
					for _, e := range lst {
						fmt.Printf("%s\t%s\n", e.Name, e.Path)
					}
				}
			}

		case "prune":
			n, err := guictl.GuiEnvPrune()
			if err != nil {
				fmt.Fprintf(os.Stderr, "GUI env prune failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Pruned %d missing entrie(s).\n", n)

		default:
			fmt.Fprintf(os.Stderr, "Unknown gui subcommand: %s\n", sub)
			os.Exit(2)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

func indexOf(arr []string, needle string) int {
	for i, a := range arr {
		if a == needle {
			return i
		}
	}
	return -1
}

// -----------------------------
// ZIP helpers (local install)
// -----------------------------

func isZipFile(p string) bool {
	lp := strings.ToLower(strings.TrimSpace(p))
	return strings.HasSuffix(lp, ".zip")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// unzipToTempAndDetectRoot extracts zip into ./cache/unzip-<ts>-<pid>,
// then returns the real root dir that contains manifest.yaml.
// If top-level contains a single subdir and manifest.yaml is inside it, we descend.
func unzipToTempAndDetectRoot(zipPath string) (root string, cleanup func(), err error) {
	// ensure cache dir
	cacheDir := filepath.Join(".", "cache")
	if e := os.MkdirAll(cacheDir, 0o755); e != nil {
		return "", func() {}, fmt.Errorf("mkdir cache: %w", e)
	}
	base := fmt.Sprintf("unzip-%d-%d", time.Now().Unix(), os.Getpid())
	dest := filepath.Join(cacheDir, base)
	if e := os.MkdirAll(dest, 0o755); e != nil {
		return "", func() {}, fmt.Errorf("mkdir temp: %w", e)
	}

	cleanup = func() {
		_ = os.RemoveAll(dest)
	}

	if e := unzipAll(zipPath, dest); e != nil {
		return "", cleanup, e
	}

	// Detect actual manifest root
	rootDir, e := detectManifestRoot(dest)
	if e != nil {
		return "", cleanup, e
	}
	return rootDir, cleanup, nil
}

func unzipAll(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if err := unzipEntry(f, dest); err != nil {
			return err
		}
	}
	return nil
}

func unzipEntry(f *zip.File, dest string) error {
	// protect against zip slip
	cleanName := filepath.Clean(f.Name)
	if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, ":") {
		return fmt.Errorf("illegal path in zip: %s", f.Name)
	}
	targetPath := filepath.Join(dest, cleanName)

	if f.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
	if err != nil {
		// fallback when mode not honored on some FS
		out, err = os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	return nil
}

func detectManifestRoot(dest string) (string, error) {
	// Case 1: manifest.yaml at dest
	if fileExists(filepath.Join(dest, "manifest.yaml")) {
		return dest, nil
	}
	// Case 2: single subdir that contains manifest.yaml
	entries, err := os.ReadDir(dest)
	if err != nil {
		return "", err
	}
	var subdirs []string
	for _, e := range entries {
		if e.IsDir() {
			subdirs = append(subdirs, filepath.Join(dest, e.Name()))
		}
	}
	if len(subdirs) == 1 && fileExists(filepath.Join(subdirs[0], "manifest.yaml")) {
		return subdirs[0], nil
	}
	// Case 3: scan shallow
	for _, sd := range subdirs {
		if fileExists(filepath.Join(sd, "manifest.yaml")) {
			return sd, nil
		}
	}
	return "", fmt.Errorf("manifest.yaml not found in %s", dest)
}
