#!/usr/bin/env bash
# Demo stdio command: read request, write one-line mutations via SDK.
set -euo pipefail

# Ensure we can source sdk from current dir
# configure.sh and lyenv_sdk.sh should be in the same folder
source ./lyenv_sdk.sh

main() {
  lyenv_read_request

  # plugin local config
  lyenv_plugin_write_config "hello.enabled" "true" "plugin"

  # global config
  # Use UTC ISO8601
  ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  lyenv_plugin_write_config "demo.last_run_at" "$ts" "global"

  # simple log (we do not parse req in bash demo)
  lyenv_log "bash: configured at $ts"

  lyenv_respond_ok "ok"
}

main
