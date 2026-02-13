#!/usr/bin/env bash
# flow_sdk.sh - Flow helper for lyenv stdio plugins (Bash)
#
# Requires:
# - source ./scripts/lyenv_sdk.sh first
# - jq recommended (lyenv_sdk.sh has jq/python fallback)
#
# Conventions:
# - Outputs stored in plugin config:
#   flow.outputs.<node_id>.<port> = "<string>"
# - Wiring JSON:
#   wiring[dstNodeId][dstInputPort] = {node: srcNodeId, port: srcOutputPort}

set -euo pipefail

FLOW_WIRING_JSON=""

flow_load_wiring() {
  local path="$1"
  FLOW_WIRING_JSON="$(cat "$path")"
  if [[ -z "${FLOW_WIRING_JSON//[[:space:]]/}" ]]; then
    FLOW_WIRING_JSON="{}"
  fi
}

_flow_key() {
  local node="$1"
  local port="$2"
  echo "flow.outputs.${node}.${port}"
}

flow_resolve_ref_node() {
  local dst="$1"
  local inport="$2"
  # returns src node id or empty
  _json_get "$FLOW_WIRING_JSON" ".${dst}.${inport}.node"
}

flow_resolve_ref_port() {
  local dst="$1"
  local inport="$2"
  _json_get "$FLOW_WIRING_JSON" ".${dst}.${inport}.port"
}

flow_get_output() {
  local node="$1"
  local port="$2"
  local def="${3:-}"
  # read from request merged plugin config
  local key="$(_flow_key "$node" "$port")"
  local v
  v="$(ly_config_get plugin "$key")"
  [[ -z "${v}" ]] && echo "$def" || echo "$v"
}

flow_set_output() {
  local node="$1"
  local port="$2"
  local value="${3:-}"
  local key="$(_flow_key "$node" "$port")"
  # write plugin mutation
  ly_plugin_write_config "$key" "$value" "plugin"
}

flow_get_input() {
  local node="$1"
  local inport="$2"
  local def="${3:-}"
  local srcNode srcPort
  srcNode="$(flow_resolve_ref_node "$node" "$inport")"
  srcPort="$(flow_resolve_ref_port "$node" "$inport")"
  if [[ -z "${srcNode}" || -z "${srcPort}" ]]; then
    echo "$def"
    return 0
  fi
  flow_get_output "$srcNode" "$srcPort" "$def"
}

flow_debug_dump_wiring() {
  local node="${1:-}"
  if [[ -n "$node" ]]; then
    ly_log "{\"wiring\": {\"$node\": $(echo "$FLOW_WIRING_JSON" | jq -c ".${node} // {}" 2>/dev/null || echo "{}")}}"
  else
    ly_log "{\"wiring\": ${FLOW_WIRING_JSON}}"
  fi
}
