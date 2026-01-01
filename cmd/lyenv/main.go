package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"lyenv/internal/cli"
	"lyenv/internal/config"
	"lyenv/internal/env"
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

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}
