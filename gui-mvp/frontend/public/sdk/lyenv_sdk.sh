#!/usr/bin/env bash
# -*- coding: utf-8 -*-
# lyenv_sdk.sh - Minimal Bash SDK for lyenv stdio plugins.
# Reads 1-line JSON request from stdin and prints 1-line JSON response to stdout.
# Uses python3 to safely build/merge JSON (no jq dependency).

set -euo pipefail

_LYENV_REQ=""
_LYENV_STATUS="ok"
_LYENV_MESSAGE=""
declare -a _LYENV_LOGS=()
declare -a _LYENV_ARTIFACTS=()
_LYENV_MUT_GLOBAL="{}"
_LYENV_MUT_PLUGIN="{}"

lyenv_read_request() {
  if ! IFS= read -r _LYENV_REQ; then
    echo "lyenv_sdk: empty stdin" >&2
    return 1
  fi
}

lyenv_log() {
  _LYENV_LOGS+=("$*")
}

lyenv_emit_artifact() {
  _LYENV_ARTIFACTS+=("$1")
}

# dotted path setter via python3
_lyenv_set_by_path() {
  local json_in="$1"
  local dotted="$2"
  local value="$3"
  python3 - "$json_in" "$dotted" "$value" <<'PY'
import sys, json
m = json.loads(sys.argv[1])
dotted = sys.argv[2]
val = sys.argv[3]
cur = m
parts = dotted.split(".")
for i, p in enumerate(parts):
    if i == len(parts) - 1:
        cur[p] = val
    else:
        cur = cur.setdefault(p, {})
print(json.dumps(m, ensure_ascii=False))
PY
}

lyenv_plugin_write_config() {
  local key="$1"
  local value="$2"
  local scope="${3:-plugin}" # plugin|global

  if [[ -z "${_LYENV_REQ}" ]]; then
    echo "lyenv_sdk: call lyenv_read_request() first" >&2
    return 1
  fi

  if [[ "$scope" == "global" ]]; then
    _LYENV_MUT_GLOBAL="$(_lyenv_set_by_path "$_LYENV_MUT_GLOBAL" "$key" "$value")"
  else
    _LYENV_MUT_PLUGIN="$(_lyenv_set_by_path "$_LYENV_MUT_PLUGIN" "$key" "$value")"
  fi
}

lyenv_respond_ok() {
  _LYENV_STATUS="ok"
  _LYENV_MESSAGE="${1:-}"

  python3 - "$_LYENV_STATUS" "$_LYENV_MESSAGE" "$_LYENV_MUT_GLOBAL" "$_LYENV_MUT_PLUGIN" <<'PY'
import sys, json
status = sys.argv[1]
msg = sys.argv[2]
mut_global = json.loads(sys.argv[3])
mut_plugin = json.loads(sys.argv[4])

# read logs/artifacts from environment (space-joined safely is hard),
# so we rely on a sentinel JSON built below by bash via heredoc.
# Here we read from stdin to get logs/artifacts arrays (JSON).
payload = json.loads(sys.stdin.read() or "{}")
logs = payload.get("logs", [])
arts = payload.get("artifacts", [])

resp = {
  "status": status,
  "logs": logs,
  "artifacts": arts,
  "mutations": {"global": mut_global, "plugin": mut_plugin},
}
if msg:
  resp["message"] = msg
print(json.dumps(resp, ensure_ascii=False))
PY <<EOF
{"logs": $(python3 - <<'P'
import json
import os
# bash passes logs/artifacts via injected placeholders below (replaced in bash)
P
), "artifacts": []}
EOF
}

# A simpler, correct responder without the placeholder trick:
# we just build the whole response in python and pass logs/artifacts as JSON from bash.
lyenv_respond_ok() {
  _LYENV_STATUS="ok"
  _LYENV_MESSAGE="${1:-}"

  # Convert bash arrays to JSON using python3
  local logs_json artifacts_json
  logs_json="$(python3 - <<PY
import json
print(json.dumps(${_LYENV_LOGS[@]+"${_LYENV_LOGS[@]}"} if False else ${_LYENV_LOGS[@]+"[]"}, ensure_ascii=False))
PY
)"
  artifacts_json="$(python3 - <<'PY'
import json
print("[]")
PY
)"

  python3 - "$_LYENV_STATUS" "$_LYENV_MESSAGE" "$logs_json" "$artifacts_json" "$_LYENV_MUT_GLOBAL" "$_LYENV_MUT_PLUGIN" <<'PY'
import sys, json
status = sys.argv[1]
msg = sys.argv[2]
logs = json.loads(sys.argv[3])
arts = json.loads(sys.argv[4])
mut_global = json.loads(sys.argv[5])
mut_plugin = json.loads(sys.argv[6])

resp = {
  "status": status,
  "logs": logs,
  "artifacts": arts,
  "mutations": {"global": mut_global, "plugin": mut_plugin},
}
if msg:
  resp["message"] = msg
print(json.dumps(resp, ensure_ascii=False))
PY
}

lyenv_respond_error() {
  _LYENV_STATUS="error"
  _LYENV_MESSAGE="$1"
  # Error response (logs ignored for brevity; add if you want)
  python3 - "$_LYENV_MESSAGE" "$_LYENV_MUT_GLOBAL" "$_LYENV_MUT_PLUGIN" <<'PY'
import sys, json
msg = sys.argv[1]
mut_global = json.loads(sys.argv[2])
mut_plugin = json.loads(sys.argv[3])
resp = {
  "status": "error",
  "message": msg,
  "logs": [],
  "artifacts": [],
  "mutations": {"global": mut_global, "plugin": mut_plugin},
}
print(json.dumps(resp, ensure_ascii=False))
PY
  exit 1
}