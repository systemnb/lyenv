# -*- coding: utf-8 -*-
"""flow_sdk.py - Flow helper using lyenv_sdk plugin mutations.

Conventions:
  - Write outputs to plugin config:
      flow.outputs.<node_id>.<port> = "<string>"
  - Read inputs from plugin config based on wiring map.

Start node:
  - is_source=True, inputs are CLI args mapped by Start output port order.
"""

import json
from typing import Any, Dict, List
from lyenv_sdk import plugin_write_config

def _get_plugin_cfg(req: Dict[str, Any]) -> Dict[str, Any]:
    cfg = {}
    if isinstance(req.get("config"), dict):
        cfg = req["config"].get("plugin") or {}
    if not cfg and isinstance(req.get("plugin_config"), dict):
        cfg = req.get("plugin_config") or {}
    if not cfg and isinstance(req.get("plugin"), dict):
        cfg = req.get("plugin") or {}
    return cfg if isinstance(cfg, dict) else {}

def _get_by_path(m: Dict[str, Any], dotted: str) -> Any:
    cur: Any = m
    for p in dotted.split("."):
        if not isinstance(cur, dict) or p not in cur:
            return None
        cur = cur[p]
    return cur

def set_output(node_id: str, port: str, value: Any):
    plugin_write_config(f"flow.outputs.{node_id}.{port}", "" if value is None else str(value), scope="plugin")

def get_output(cfg: Dict[str, Any], node_id: str, port: str) -> str:
    v = _get_by_path(cfg, f"flow.outputs.{node_id}.{port}")
    return "" if v is None else str(v)

def load_wiring(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f) or {}

def build_inputs(req: Dict[str, Any], wiring: Dict[str, Any], node_id: str, input_ports: List[str]) -> List[str]:
    cfg = _get_plugin_cfg(req)
    mapping: Dict[str, Any] = (wiring.get(node_id) or {})
    argv: List[str] = []
    for name in input_ports:
        ref = mapping.get(name)
        if ref and isinstance(ref, dict):
            argv.append(get_output(cfg, ref.get("node",""), ref.get("port","")))
        else:
            argv.append("")
    return argv

def write_outputs(node_id: str, output_ports: List[str], values: List[str]):
    for i, p in enumerate(output_ports):
        set_output(node_id, p, values[i] if i < len(values) else "")