#!/usr/bin/env bash
# lyenv_sdk.sh - Bash SDK for lyenv stdio plugins (enhanced)
# - Requires jq (recommended). If jq missing, fallback to python/python3 for JSON parsing.

set -euo pipefail

LY_REQ_JSON=""
LY_RESP_STATUS="ok"
LY_RESP_MESSAGE=""
LY_RESP_LOGS=()
LY_RESP_ARTIFACTS=()
LY_MUT_GLOBAL="{}"
LY_MUT_PLUGIN="{}"
LY_RESPONDED="0"

ly_read_request() {
  LY_REQ_JSON="$(cat)"
  if [[ -z "${LY_REQ_JSON//[[:space:]]/}" ]]; then
    echo "lyenv_sdk: empty stdin" >&2
    return 1
  fi
}

_has_jq() { command -v jq >/dev/null 2>&1; }
_has_py() { command -v python >/dev/null 2>&1 || command -v python3 >/dev/null 2>&1; }

_json_get() {
  local json="$1"
  local expr="$2"
  if _has_jq; then
    echo "$json" | jq -r "$expr"
  elif _has_py; then
    local pybin="python"
    command -v python >/dev/null 2>&1 || pybin="python3"
    "$pybin" - <<'PY' "$json" "$expr"
import sys, json
j = json.loads(sys.argv[1])
expr = sys.argv[2].strip()
# supports ".a.b.c" only
if expr.startswith("."): expr = expr[1:]
cur = j
if expr:
  for p in expr.split("."):
    if isinstance(cur, dict) and p in cur:
      cur = cur[p]
    else:
      cur = ""
      break
print(cur if cur is not None else "")
PY
  else
    echo ""
  fi
}

# Request helpers
ly_action() { _json_get "$LY_REQ_JSON" '.action'; }
ly_args_json() { _json_get "$LY_REQ_JSON" '.args | @json'; }
ly_dispatch_id() { _json_get "$LY_REQ_JSON" '.dispatch_id'; }
ly_path() { local k="$1"; _json_get "$LY_REQ_JSON" ".paths.${k}"; }

# Config getters
ly_config_get() {
  local scope="$1"   # plugin/global
  local dotted="$2"  # a.b.c
  local jqpath=".config.${scope}"
  local sub=""
  IFS='.' read -r -a parts <<< "$dotted"
  for p in "${parts[@]}"; do sub="${sub}.${p}"; done
  _json_get "$LY_REQ_JSON" "${jqpath}${sub}"
}

# Response helpers
ly_log() { LY_RESP_LOGS+=("$*"); }
# Real-time log (stderr). Also append to response logs for the final JSON (optional).
ly_log_stream() {
  local msg="$*"
  # stderr to show immediately in CLI
  printf '%s\n' "$msg" >&2
  # ensure flush (printf is unbuffered for terminals; keep explicit behavior)
  # also keep it for final response logs
  LYENV_RESP_LOGS+=("$msg")
}

ly_emit_artifact() { LY_RESP_ARTIFACTS+=("$*"); }

# Mutations (jq preferred)
ly_mutate_set() {
  local scope="$1"  # plugin/global
  local dotted="$2"
  local value="$3"

  local target="$LY_MUT_PLUGIN"
  [[ "$scope" == "global" ]] && target="$LY_MUT_GLOBAL"

  if _has_jq; then
    local jqassign="."
    IFS='.' read -r -a parts <<< "$dotted"
    for p in "${parts[@]}"; do jqassign="${jqassign}.${p}"; done
    target="$(echo "$target" | jq --arg v "$value" "${jqassign}=\$v")"
  elif _has_py; then
    local pybin="python"
    command -v python >/dev/null 2>&1 || pybin="python3"
    target="$("$pybin" - <<'PY' "$target" "$dotted" "$value"
import sys, json
obj = json.loads(sys.argv[1]); dotted = sys.argv[2]; val = sys.argv[3]
cur = obj
parts = dotted.split(".")
for i,p in enumerate(parts):
  if i == len(parts)-1: cur[p] = val
  else:
    if p not in cur or not isinstance(cur[p], dict): cur[p] = {}
    cur = cur[p]
print(json.dumps(obj, ensure_ascii=False))
PY
)"
  else
    :
  fi

  if [[ "$scope" == "global" ]]; then
    LY_MUT_GLOBAL="$target"
  else
    LY_MUT_PLUGIN="$target"
  fi
}

# Backward-compatible alias name
ly_plugin_write_config() {
  local key="$1"
  local value="$2"
  local scope="${3:-plugin}"  # plugin/global
  ly_mutate_set "$scope" "$key" "$value"
}

ly_ensure_not_responded() {
  if [[ "$LY_RESPONDED" == "1" ]]; then
    echo "lyenv_sdk: respond_* called more than once" >&2
    return 1
  fi
  LY_RESPONDED="1"
}

ly_respond_ok() {
  local msg="${1:-}"
  ly_ensure_not_responded || return 1
  LY_RESP_STATUS="ok"
  LY_RESP_MESSAGE="$msg"

  if _has_jq; then
    local logs_json; logs_json="$(printf '%s\n' "${LY_RESP_LOGS[@]:-}" | jq -R . | jq -s .)"
    local art_json;  art_json="$(printf '%s\n' "${LY_RESP_ARTIFACTS[@]:-}" | jq -R . | jq -s .)"

    jq -n \
      --arg status "$LY_RESP_STATUS" \
      --arg message "$LY_RESP_MESSAGE" \
      --argjson logs "$logs_json" \
      --argjson artifacts "$art_json" \
      --argjson g "$LY_MUT_GLOBAL" \
      --argjson p "$LY_MUT_PLUGIN" \
      '{status:$status, message:$message, logs:$logs, artifacts:$artifacts, mutations:{global:$g, plugin:$p}}'
  else
    printf '{"status":"%s","message":"%s","logs":[],"artifacts":[],"mutations":{"global":%s,"plugin":%s}}\n' \
      "$LY_RESP_STATUS" "$LY_RESP_MESSAGE" "$LY_MUT_GLOBAL" "$LY_MUT_PLUGIN"
  fi
}

ly_respond_error() {
  local msg="$1"
  ly_ensure_not_responded || return 1
  LY_RESP_STATUS="error"
  LY_RESP_MESSAGE="$msg"
  ly_respond_ok "$msg"
  exit 1
}
