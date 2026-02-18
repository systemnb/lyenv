package cli

import (
	"fmt"
	"os"
)

func Usage() {
	fmt.Fprintf(os.Stderr, `lyenv - Directory-based isolated environment manager

Usage:
  lyenv install [--bindir=<DIR>] [--gui=<PATH>] [--init-gui=0|1]
                                     Install lyenv and lyenv-gui to user bin dir (default: ~/.lyenv/bin)
                                     Initializes ~/.lyenv/gui/config.yaml and ~/.lyenv/gui/logs (default: init-gui=1)

  lyenv uninstall [--bindir=<DIR>] [--purge-gui=0|1]
                                     Remove lyenv and lyenv-gui from bin dir (default: ~/.lyenv/bin)
                                     Keeps ~/.lyenv/gui by default; purge-gui=1 to delete it

  lyenv create <DIR>                 Create a new lyenv environment directory with default config and structure
  lyenv init <DIR>                   Verify and repair an existing lyenv environment (idempotent)
  lyenv activate [--shell=bash|zsh|powershell|pwsh|cmd]
                                    Print a shell snippet to activate the current lyenv.
                                    Linux/macOS (bash/zsh): eval "$(lyenv activate)"
                                    Windows PowerShell:     lyenv activate | Invoke-Expression
                                    Windows CMD:           for /f "delims=" \%i in ('lyenv activate --shell=cmd') do %i

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
  
  lyenv run <PLUGIN> <COMMAND> [--merge=override|append|keep] [--timeout=<sec>] [--fail-fast|--keep-going] [...args]
                                     Run a plugin command (single or multi-step). 'stdio' returns mutations; 'shell' prints logs.
  lyenv gui start [--addr=127.0.0.1:18888] [--open]
                                     Start GUI server in background (global mode). Requires 'lyenv-gui' binary.
                                     Global files:
                                       - PID:   ~/.lyenv/gui/gui.pid
                                       - Logs:  ~/.lyenv/gui/logs/gui.log
                                       - Config:~/.lyenv/gui/config.yaml

  lyenv gui stop                      Stop GUI server (best-effort using pid file).
  lyenv gui status                    Show GUI server status (running/stopped).
  lyenv gui open [--addr=127.0.0.1:18888]
                                     Open GUI in default browser.
                                     On Android/Termux: uses 'termux-open-url' if available.

  lyenv gui add <DIR> [--name=<ENV_NAME>] [--create=1]
                                     Register a directory as a GUI-available environment.
                                     - If <DIR> is NOT a lyenv env yet, it will auto 'create' + 'init' when --create=1.
                                     - When --create is omitted, it defaults to 1 (auto create/init) for convenience.
                                     - Registered envs are stored in global GUI config: ~/.lyenv/gui/config.yaml (envs.pinned).

  lyenv gui list [--json] [--all]
                                     List GUI-registered environments.
                                     - Default: show only valid envs (existing dirs).
                                     - With --all: also show missing entries (stale items in config).

  lyenv gui remove <NAME|PATH>
                                     Remove one registered env by name or absolute/relative path.

  lyenv gui prune
                                     Remove all missing (deleted) env entries from GUI registry config (cleanup stale list).


Examples:
  lyenv create android-env
  lyenv init android-env
  eval "$(lyenv activate)"

  # Install plugin by name via plugin center
  lyenv plugin install tester --name=testtools
  tctl run

  # lyenv flags must be prefixed with --lyenv-
  gcc --lyenv-timeout=10 --lyenv-merge=keep --input a.c --out a

Notes:
  - 'stdio' steps return structured JSON (status/logs/artifacts/mutations).
  - Mutations are merged into lyenv.yaml and plugin local config (YAML/JSON by extension).
  - Logs are recorded as JSON Lines under plugins/<INSTALL_NAME>/logs/YYYY-MM-DD/.
`)
}
