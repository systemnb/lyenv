# -*- coding: utf-8 -*-
"""
flow_sdk.py - Flow helper SDK for lyenv GUI-exported stdio workflows.

Goals:
- Make node authoring "business-only": inputs -> outputs
- Hide wiring/config plumbing behind simple helpers
- Store outputs in plugin config at:
    flow.outputs.<node_id>.<port> = "<string>"
- Read upstream values using wiring map:
    wiring[dst_node_id][dst_input_port] = { "node": src_node_id, "port": src_output_port }

Requires:
- lyenv_sdk.py injected in the same scripts folder
  (plugin_write_config supports scope="plugin")
"""

import json
from typing import Any, Dict, List, Optional, Tuple

from lyenv_sdk import plugin_write_config, config_plugin, log


# ---------------------------
# Config storage convention
# ---------------------------

def _flow_key(node_id: str, port: str) -> str:
    return f"flow.outputs.{node_id}.{port}"


def set_output(node_id: str, port: str, value: Any) -> None:
    """Write one output to plugin mutations (flow.outputs...)."""
    plugin_write_config(_flow_key(node_id, port), "" if value is None else str(value), scope="plugin")


def set_outputs(node_id: str, outputs: Dict[str, Any]) -> None:
    """Write multiple outputs {port:value}."""
    for k, v in (outputs or {}).items():
        set_output(node_id, k, v)


def get_output(node_id: str, port: str, default: str = "") -> str:
    """Read one output from plugin config (merged into request)."""
    v = config_plugin(_flow_key(node_id, port), default)
    return "" if v is None else str(v)


# ---------------------------
# Wiring helpers
# ---------------------------

def load_wiring(path: str) -> Dict[str, Any]:
    """Load wiring JSON: dstNodeId -> dstInputPort -> {node, port}."""
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f) or {}


def resolve_ref(wiring: Dict[str, Any], dst_node_id: str, dst_input_port: str) -> Optional[Tuple[str, str]]:
    """
    Resolve (src_node_id, src_port) for a given dst input port.
    Returns None if not wired.
    """
    m = (wiring or {}).get(dst_node_id) or {}
    ref = m.get(dst_input_port)
    if isinstance(ref, dict):
        src_node = str(ref.get("node") or "")
        src_port = str(ref.get("port") or "")
        if src_node and src_port:
            return (src_node, src_port)
    return None


def get_inputs(req: Dict[str, Any], wiring: Dict[str, Any], node_id: str, input_ports: List[str], default: str = "") -> List[str]:
    """
    Return inputs for node_id in the exact order of input_ports.
    Each value comes from upstream output according to wiring.
    """
    argv: List[str] = []
    for name in input_ports:
        ref = resolve_ref(wiring, node_id, name)
        if ref:
            src_node, src_port = ref
            argv.append(get_output(src_node, src_port, default))
        else:
            argv.append(default)
    return argv


def get_input(req: Dict[str, Any], wiring: Dict[str, Any], node_id: str, port_name: str, default: str = "") -> str:
    """Convenience: read one input port value."""
    vals = get_inputs(req, wiring, node_id, [port_name], default=default)
    return vals[0] if vals else default


# ---------------------------
# Debug utilities
# ---------------------------

def debug_dump_wiring(wiring: Dict[str, Any], node_id: Optional[str] = None) -> None:
    """
    Log wiring map (whole or per node) for debugging in GUI console.
    """
    if node_id:
        log({ "wiring": { node_id: (wiring or {}).get(node_id, {}) } })
    else:
        log({ "wiring": wiring or {} })


def debug_dump_io(req: Dict[str, Any], wiring: Dict[str, Any], node_id: str, input_ports: List[str], output_ports: Optional[List[str]] = None) -> None:
    """
    Log resolved inputs and (optionally) current stored outputs.
    Useful for debugging dataflow quickly.
    """
    ins = get_inputs(req, wiring, node_id, input_ports, default="")
    log({ "node": node_id, "inputs": dict(zip(input_ports, ins)) })

    if output_ports:
        outs = { p: get_output(node_id, p, "") for p in output_ports }
        log({ "node": node_id, "outputs": outs })
