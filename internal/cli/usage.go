package cli

import (
	"fmt"
	"os"
)

func Usage() {
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
                                     lyenv plugin install <NAME|PATH> [--repo=<org/repo>] [--ref=<branch|tag|commit>] [--source=<url>] [--proxy=<url>]
                                     Install a plugin from local path or remote source
  lyenv plugin add <PATH>            Install a local plugin from a directory (manifest: YAML or JSON)
  lyenv plugin list                  List installed plugins
  lyenv plugin info <NAME>           Show plugin manifest details
  lyenv plugin remove <NAME>         Uninstall a plugin
  lyenv plugin install <NAME|PATH> [--repo=<org/repo>] [--ref=<branch|tag|commit>] [--source=<url>] [--proxy=<url>]
                                     Install a plugin from local path or remote source (manifest: YAML or JSON)

  lyenv run <PLUGIN> <COMMAND> [--merge=override|append|keep] [-- ...args]
                                     Run a plugin command (shell or stdio as declared in manifest)

`)
}
