package cli

import (
	"fmt"
	"os"
)

func Usage() {
	fmt.Fprintf(os.Stderr, `lyenv - Directory-based isolated environment manager
Usage:
  lyenv create <DIR>
  lyenv init <DIR>
  lyenv activate

  lyenv config set <KEY> <VALUE> [--type=string|int|float|bool|json]
  lyenv config get <KEY>
  lyenv config dump [<KEY>] <FILE>        (YAML or JSON by extension)
  lyenv config load <FILE> [--merge=override|append|keep]  (YAML or JSON source)
  lyenv config importjson <FILE> <JSON_KEY> [--to=<CONFIG_KEY>] [--type=...] [--merge=...] [--input=1]
  lyenv config importyaml <FILE> <YAML_KEY> [--to=<CONFIG_KEY>] [--type=...] [--merge=...] [--input=1]

  lyenv plugin add <PATH> [--name=<INSTALL_NAME>]   (manifest: YAML or JSON)
  lyenv plugin install <NAME|PATH> [--name=<INSTALL_NAME>] [--repo=<org/repo>] [--ref=<branch|tag|commit>] [--source=<url>] [--proxy=<url>]
  lyenv plugin update <INSTALL_NAME> [--repo=<org/repo>] [--ref=<branch|tag|commit>] [--source=<url>] [--proxy=<url>]
  lyenv plugin list [--json]
  lyenv plugin info <INSTALL_NAME>
  lyenv plugin remove <INSTALL_NAME> [--force]

  lyenv run <PLUGIN> <COMMAND> [--merge=override|append|keep] [-- ...args]
`)
}
