#!/usr/bin/env bash
# lyenv_sdk.sh - Bash SDK for lyenv stdio plugins (minimal, with config read/write).
# Requires: jq (recommended). If jq missing, will fallback to python for JSON parsing if available.

set -euo pipefail

LYENV_REQ_JSON=""
LYENV_RESP_STATUS="ok"
LYENV_RESP_MESSAGE=""
LYENV_RESP_LOGS=()
LYENV_RESP_ARTIFACTS=()
LYENV_MUT_GLOBAL="{}"
LYENV_MUT_PLUGIN="{}"

ly_read_request() {
  # Read all stdin
  LYENV_REQ_JSON="$(cat)"
  if [[ -z "${LYENV_REQ_JSON//[[:space:]]/}" ]]; then
    echo "lyenv_sdk: empty stdin" >&2
    return 1
  fi
}

# --- JSON helpers ---

_has_jq() { command -v jq >/dev/null 2>&1; }
_has_py() { command -v python >/dev/null 2>&1 || command -v python3 >/dev/null 2>&1; }

_json_get() {
  local json="$1"
  local jqexpr="$2"
  if _has_jq; then
    echo "$json" | jq -r "$jqexpr"
  elif _has_py; then
    local pybin="python"
    command -v python >/dev/null 2>&1 || pybin="python3"
    "$pybin" - <<'PY' "$json" "$jqexpr"
import sys, json
j = json.loads(sys.argv[1])
expr = sys.argv[2]
# very small subset: ".a.b.c" only
path = expr.strip()
if path.startswith("."):
    path = path[1:]
cur = j
if path:
    for p in path.split("."):
        if isinstance(cur, dict) and p in cur:
            cur = cur[p]
        else:
            cur = ""
            break
print(cur if cur is not None else "")
PY
  else
    echo ""  # no parser available
  fi
}

# request getters
ly_action() { _json_get "$LYENV_REQ_JSON" '.action'; }
ly_args_json() { _json_get "$LYENV_REQ_JSON" '.args | @json'; }
ly_path() {
  local key="$1"
  _json_get "$LYENV_REQ_JSON" ".paths.${key}"
}

# config getters: scope=plugin/global and dotted path like driver.name
ly_config_get() {
  local scope="$1"    # plugin/global
  local dotted="$2"   # a.b.c
  local jqpath=".config.${scope}"
  # convert dotted to jq: .a.b.c
  local sub=""
  IFS='.' read -r -a parts <<< "$dotted"
  for p in "${parts[@]}"; do
    sub="${sub}.${p}"
  done
  _json_get "$LYENV_REQ_JSON" "${jqpath}${sub}"
}

# response helpers
ly_log() { LYENV_RESP_LOGS+=("$*"); }
ly_emit_artifact() { LYENV_RESP_ARTIFACTS+=("$*"); }

# mutations: write dotted into JSON (uses jq if possible; python fallback otherwise)
ly_mutate_set() {
  local scope="$1"  # plugin/global
  local dotted="$2"
  local value="$3"

  local target=""
  if [[ "$scope" == "global" ]]; then
    target="$LYENV_MUT_GLOBAL"
  else
    target="$LYENV_MUT_PLUGIN"
  fi

  if _has_jq; then
    # Build jq assignment like .a.b.c = "value"
    local jqassign="."
    IFS='.' read -r -a parts <<< "$dotted"
    for p in "${parts[@]}"; do
      jqassign="${jqassign}${p:+.${p}}"
    done
    target="$(echo "$target" | jq --arg v "$value" "${jqassign}=\$v")"
  elif _has_py; then
    local pybin="python"
    command -v python >/dev/null 2>&1 || pybin="python3"
    target="$("$pybin" - <<'PY' "$target" "$dotted" "$value"
import sys, json
obj = json.loads(sys.argv[1])
dotted = sys.argv[2]
val = sys.argv[3]
cur = obj
parts = dotted.split(".")
for i,p in enumerate(parts):
    if i == len(parts)-1:
        cur[p] = val
    else:
        if p not in cur or not isinstance(cur[p], dict):
            cur[p] = {}
        cur = cur[p]
print(json.dumps(obj, ensure_ascii=False))
PY
)"
  else
    # no parser: cannot mutate
    :
  fi

  if [[ "$scope" == "global" ]]; then
    LYENV_MUT_GLOBAL="$target"
  else
    LYENV_MUT_PLUGIN="$target"
  fi
}

ly_respond_ok() {
  local msg="${1:-}"
  LYENV_RESP_STATUS="ok"
  LYENV_RESP_MESSAGE="$msg"

  # build JSON response (use jq if possible)
  if _has_jq; then
    local logs_json
    logs_json="$(printf '%s\n' "${LYENV_RESP_LOGS[@]:-}" | jq -R . | jq -s .)"
    local art_json
    art_json="$(printf '%s\n' "${LYENV_RESP_ARTIFACTS[@]:-}" | jq -R . | jq -s .)"
    jq -n \
      --arg status "$LYENV_RESP_STATUS" \
      --arg message "$LYENV_RESP_MESSAGE" \
      --argjson logs "$logs_json" \
      --argjson artifacts "$art_json" \
      --argjson g "$LYENV_MUT_GLOBAL" \
      --argjson p "$LYENV_MUT_PLUGIN" \
      '{status:$status,message:$message,logs:$logs,artifacts:$artifacts,mutations:{global:$g,plugin:$p}}'
  else
    # fallback: minimal output
    printf '{"status":"%s","message":"%s","logs":[],"artifacts":[],"mutations":{"global":%s,"plugin":%s}}\n' \
      "$LYENV_RESP_STATUS" "$LYENV_RESP_MESSAGE" "$LYENV_MUT_GLOBAL" "$LYENV_MUT_PLUGIN"
  fi
}

ly_respond_error() {
  local msg="$1"
  LYENV_RESP_STATUS="error"
  LYENV_RESP_MESSAGE="$msg"
  ly_respond_ok "$msg"  # reuse builder
  exit 1
}
