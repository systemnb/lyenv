package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lyenv/internal/cli"
	"lyenv/internal/config"
	"lyenv/internal/env"
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
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin add <PATH> [--name=<INSTALL_NAME>]")
				os.Exit(2)
			}
			// Find first non-flag as PATH
			var path string
			var flagArgs []string
			for _, a := range args[2:] {
				if strings.HasPrefix(a, "--") {
					flagArgs = append(flagArgs, a)
				} else if path == "" {
					path = a
				} else {
					// Extra positional tokens are not expected for 'add'; treat as error
					fmt.Fprintln(os.Stderr, "Error: too many positional arguments for 'plugin add'")
					os.Exit(2)
				}
			}
			if path == "" {
				fmt.Fprintln(os.Stderr, "Error: <PATH> must not be empty")
				os.Exit(2)
			}
			flags := config.ParseFlags(flagArgs)
			overrideName := flags["name"]

			if err := plugin.PluginAddLocal(".", path, overrideName); err != nil {
				fmt.Fprintf(os.Stderr, "Plugin add failed: %v\n", err)
				os.Exit(1)
			}

		case "install":
			if len(args) < 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin install <NAME|PATH> [--name=<INSTALL_NAME>] [--repo=<org/repo>] [--ref=<branch|tag|commit>] [--source=<url>] [--proxy=<url>]")
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
				fmt.Fprintln(os.Stderr, "Error: <NAME|PATH> must not be empty")
				os.Exit(2)
			}
			if err := plugin.PluginAdd(".", nameOrPath, source, repo, ref, proxy, overrideName); err != nil {
				fmt.Fprintf(os.Stderr, "Plugin install failed: %v\n", err)
				os.Exit(1)
			}

		case "info":
			if len(args) != 3 {
				fmt.Fprintln(os.Stderr, "Error: usage: lyenv plugin info <NAME>")
				os.Exit(2)
			}
			name := strings.TrimSpace(args[2])
			dir := filepath.Join(".", "plugins", name)
			man, err := plugin.LoadManifest(dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Plugin info failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Name: %s\nVersion: %s\n", man.Name, man.Version)
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
		if i := indexOf(args[3:], "--"); i >= 0 {
			rawFlags = args[3 : 3+i]
			passArgs = args[3+i+1:]
		} else {
			rawFlags = args[3:]
		}
		flags := config.ParseFlags(rawFlags)
		strategy := config.ParseMergeStrategy(flags["merge"])

		if err := plugin.RunPluginCommand(".", pl, cmd, passArgs, strategy); err != nil {
			fmt.Fprintf(os.Stderr, "Run failed: %v\n", err)
			os.Exit(1)
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
