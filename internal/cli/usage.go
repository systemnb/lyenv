package cli

import (
	"fmt"
	"os"
)

func Usage() {
	fmt.Fprintf(os.Stderr, `lyenv - Directory-based isolated environment manager

Usage:
  lyenv create <DIR>                 Create a new lyenv environment directory with default config and structure
  lyenv init <DIR>                   Verify and repair an existing lyenv environment (idempotent)
  lyenv activate                     Print shell snippet to activate the current lyenv (bash/zsh); eval "$(lyenv activate)"

  lyenv config set <KEY> <VALUE> [--type=string|int|float|bool|json]
                                     Set a configuration value (dot path) with optional type enforcement
  lyenv config get <KEY>             Get a configuration value (dot path)
  lyenv config dump [<KEY>] <FILE>   Dump full config or a specific key to a file (YAML or JSON by extension)
  lyenv config load <FILE> [--merge=override|append|keep]
                                     Load and merge a YAML or JSON file into lyenv.yaml with a merge strategy
  lyenv config importjson <FILE> <JSON_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]
                                     Import a value from a JSON file (dot path) into lyenv.yaml
  lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=string|int|float|bool|json] [--merge=override|append|keep] [--input=1]
                                     Import a value from a YAML file (dot path) into lyenv.yaml

  lyenv plugin add <PATH> [--name=<INSTALL_NAME>]
                                     Install a local plugin from a directory (manifest: YAML or JSON) under a custom install name
  lyenv plugin install <NAME|PATH> [--name=<INSTALL_NAME>] [--repo=<org/repo>] [--ref=<branch|tag|commit|version>] [--source=<url>] [--proxy=<url>]
                                     Install a plugin from local path, remote repo, source archive, or by NAME via plugin center
  lyenv plugin update <INSTALL_NAME> [--repo=<org/repo>] [--ref=<branch|tag|commit|version>] [--source=<url>] [--proxy=<url>]
                                     Update an installed plugin in place (monorepo subpath or repo/source overrides)
  lyenv plugin list [--json]         List installed plugins (JSON for machine-readable output)
  lyenv plugin info <INSTALL_NAME|LOGICAL_NAME>
                                     Show plugin manifest details, resolved install directory and exposed shims
  lyenv plugin remove <INSTALL_NAME> [--force]
                                     Uninstall a plugin and remove related shims (best-effort with --force)
  lyenv plugin search <KEYWORDS...>    Search plugin center by name/description keywords
  lyenv plugin center sync            Cache plugin center index into .lyenv/registry/index.yaml|json
  lyenv plugin center sync            Cache plugin center index into .lyenv/registry/index.yaml|json
  lyenv plugin search <KEYWORDS...>   Search plugin center by name/description keywords
  
  lyenv run <PLUGIN> <COMMAND> [--merge=override|append|keep] [--timeout=<sec>] [--fail-fast|--keep-going] [-- ...args]
                                     Run a plugin command (single or multi-step). 'stdio' returns mutations; 'shell' prints logs.

Defaults written by 'lyenv create':
  - Structure: .lyenv/{logs,registry}, bin/, cache/, plugins/, workspace/
  - .lyenv/registry/installed.yaml: empty list (plugins: [])
  - lyenv.yaml:
      env: { name: "default", platform: "auto" }
      path: { bin: "./bin", cache: "./cache", workspace: "./workspace" }
      plugins:
        installed: []
        registry_url: "https://raw.githubusercontent.com/systemnb/lyenv-plugin-center/main/index.yaml"
        registry_format: "yaml"
        default_version_strategy: "latest"
      config:
        use_container: false
        pkg_manager: "auto"
        network: { proxy_url: "" }     # set proxy if needed

Examples:
  lyenv create android-env
  lyenv init android-env
  eval "$(lyenv activate)"

  # Install plugin by name via plugin center
  lyenv plugin install tester --name=testtools
  tctl run

  # Run with timeout and fail-fast policy
  lyenv run testtools slow --timeout=5 --fail-fast

Notes:
  - 'stdio' steps return structured JSON (status/logs/artifacts/mutations).
  - Mutations are merged into lyenv.yaml and plugin local config (YAML/JSON by extension).
  - Logs are recorded as JSON Lines under plugins/<INSTALL_NAME>/logs/YYYY-MM-DD/.
`)
}
