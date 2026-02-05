# -*- coding: utf-8 -*-
"""
lyenv_sdk.py - Minimal Python SDK for lyenv stdio plugins.
"""
import sys
import json
from typing import Any, Dict, Optional

_REQUEST: Dict[str, Any] = {}
_RESPONSE: Dict[str, Any] = {
    "status": "ok",
    "logs": [],
    "artifacts": [],
    "mutations": {
        "global": {},
        "plugin": {},
    }
}

def read_request() -> Dict[str, Any]:
    line = sys.stdin.readline()
    if not line:
        raise RuntimeError("lyenv_sdk: empty stdin")
    global _REQUEST
    _REQUEST = json.loads(line)
    return _REQUEST

def _ensure_request_loaded():
    if not _REQUEST:
        raise RuntimeError("lyenv_sdk: call read_request() first")

def log(msg: str):
    _RESPONSE["logs"].append(str(msg))

def emit_artifact(path: str):
    _RESPONSE["artifacts"].append(str(path))

def _set_by_path(m: Dict[str, Any], dotted: str, val: Any):
    cur = m
    parts = dotted.split(".")
    for i, p in enumerate(parts):
        if i == len(parts) - 1:
            cur[p] = val
        else:
            cur = cur.setdefault(p, {})

def plugin_write_config(key: str, value: Any, scope: str = "plugin", merge: Optional[str] = None):
    _ensure_request_loaded()
    ms = _RESPONSE["mutations"]
    target = ms["plugin"] if scope == "plugin" else ms["global"]
    _set_by_path(target, key, value)

def respond_ok(message: str = ""):
    if message:
        _RESPONSE["message"] = message
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\n")
    sys.stdout.flush()

def respond_error(message: str):
    _RESPONSE["status"] = "error"
    _RESPONSE["message"] = message
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\n")
    sys.stdout.flush()
    sys.exit(1)
