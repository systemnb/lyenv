# lyenv.sh - tiny helper for stdio JSON response in pure shell

LYENV_RESP_STATUS="ok"
LYENV_RESP_LOGS=""
LYENV_RESP_ARTS=""
LYENV_RESP_MUT_GLOBAL="{}"
LYENV_RESP_MUT_PLUGIN="{}"

lyenv_log() {
  local msg="$1"
  if [ -z "$LYENV_RESP_LOGS" ]; then
    LYENV_RESP_LOGS=$(printf '[%s]' "\"$(printf '%s' "$msg" | sed 's/\"/\\\"/g')\"")
  else
    LYENV_RESP_LOGS=$(printf '%s' "$LYENV_RESP_LOGS" | sed 's/]$//')
    LYENV_RESP_LOGS=$(printf '%s,"%s"]' "$LYENV_RESP_LOGS" "$(printf '%s' "$msg" | sed 's/\"/\\\"/g')")
  fi
}

lyenv_emit_artifact() {
  local p="$1"
  if [ -z "$LYENV_RESP_ARTS" ]; then
    LYENV_RESP_ARTS=$(printf '["%s"]' "$p")
  else
    LYENV_RESP_ARTS=$(printf '%s' "$LYENV_RESP_ARTS" | sed 's/]$//')
    LYENV_RESP_ARTS=$(printf '%s,"%s"]' "$LYENV_RESP_ARTS" "$p")
  fi
}

# naive dot path set for plugin mutations (string value only)
lyenv_plugin_write_config() {
  local key="$1"; shift
  local value="$1"; shift
  LYENV_RESP_MUT_PLUGIN=$(printf '{"%s": "%s"}' "$key" "$value")
}

lyenv_respond_ok() {
  printf '{"status":"%s","logs":%s,"artifacts":%s,"mutations":{"global":%s,"plugin":%s}}\n' \
    "$LYENV_RESP_STATUS" \
    "${LYENV_RESP_LOGS:-[]}" \
    "${LYENV_RESP_ARTS:-[]}" \
    "${LYENV_RESP_MUT_GLOBAL:-{}}" \
    "${LYENV_RESP_MUT_PLUGIN:-{}}"
}

lyenv_respond_error() {
  local message="${1:-error}"
  printf '{"status":"error","message":"%s"}\n' "$(printf '%s' "$message" | sed 's/\"/\\\"/g')"
}
