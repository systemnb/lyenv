# -*- coding: utf-8 -*-
"""
lyenv_sdk.py - Python SDK for lyenv stdio plugins.

Features:
- Robust request reading (supports single-line JSON and multi-line JSON)
- Read config (global/plugin) with dotted-path access
- Read paths/system/action/args/dispatch_id
- Emit logs / artifacts
- Write mutations (global/plugin) with dotted-path
- respond_ok / respond_error with safety guards

Compatibility:
- plugin_write_config supports legacy calls and new kwargs (scope/merge/...)
"""

import sys
import json
from typing import Any, Dict, Optional, List

_REQUEST: Dict[str, Any] = {}
_RESPONDED: bool = False

_RESPONSE: Dict[str, Any] = {
    "status": "ok",
    "logs": [],
    "artifacts": [],
    "mutations": {
        "global": {},
        "plugin": {},
    }
}

# ---------------------------
# Request helpers
# ---------------------------

def read_request() -> Dict[str, Any]:
    """
    Read request JSON from stdin.
    lyenv typically sends one-line JSON, but we support multi-line for robustness.
    """
    global _REQUEST
    raw = sys.stdin.read()
    if not raw or not raw.strip():
        raise RuntimeError("lyenv_sdk: empty stdin")
    _REQUEST = json.loads(raw)
    return _REQUEST

def _ensure_request_loaded() -> None:
    if not _REQUEST:
        raise RuntimeError("lyenv_sdk: call read_request() first")

def request() -> Dict[str, Any]:
    """Return current request (must call read_request() first)."""
    _ensure_request_loaded()
    return _REQUEST

def action() -> str:
    _ensure_request_loaded()
    return str(_REQUEST.get("action") or "")

def args() -> List[str]:
    _ensure_request_loaded()
    a = _REQUEST.get("args") or []
    return [str(x) for x in a]

def dispatch_id() -> str:
    _ensure_request_loaded()
    return str(_REQUEST.get("dispatch_id") or _REQUEST.get("dispatchId") or "")

def paths() -> Dict[str, str]:
    _ensure_request_loaded()
    p = _REQUEST.get("paths") or {}
    return {str(k): str(v) for k, v in p.items()}

def system() -> Dict[str, str]:
    _ensure_request_loaded()
    s = _REQUEST.get("system") or {}
    return {str(k): str(v) for k, v in s.items()}

def get_path(name: str, default: str = "") -> str:
    p = paths()
    return str(p.get(name, default))

# ---------------------------
# Config helpers
# ---------------------------

def _get_by_path(obj: Any, dotted: str, default: Any = None) -> Any:
    """
    Dotted path getter: a.b.c
    Supports dict nesting only (sufficient for lyenv configs).
    """
    if dotted is None or dotted == "":
        return obj
    cur = obj
    for part in dotted.split("."):
        if isinstance(cur, dict) and part in cur:
            cur = cur[part]
        else:
            return default
    return cur

def config_scope(scope: str) -> Dict[str, Any]:
    """
    scope: 'global' or 'plugin'
    """
    _ensure_request_loaded()
    cfg = (_REQUEST.get("config") or {})
    if scope == "plugin":
        return cfg.get("plugin") or {}
    return cfg.get("global") or {}

def config_get(key: str, default: Any = None, scope: str = "plugin") -> Any:
    """
    Read config by dotted key from req.config.<scope>.
    Example: config_get("driver.name", scope="plugin")
    """
    base = config_scope(scope)
    return _get_by_path(base, key, default)

def config_global(key: str, default: Any = None) -> Any:
    return config_get(key, default, scope="global")

def config_plugin(key: str, default: Any = None) -> Any:
    return config_get(key, default, scope="plugin")

# Optional aliases (nice to have)
def config(key: str, default: Any = None) -> Any:
    return config_plugin(key, default)

def plugin_config(key: str, default: Any = None) -> Any:
    return config_plugin(key, default)

def global_config(key: str, default: Any = None) -> Any:
    return config_global(key, default)

# ---------------------------
# Response helpers (logs/artifacts/mutations)
# ---------------------------

def log(msg: Any) -> None:
    _RESPONSE["logs"].append(str(msg))

def emit_artifact(path: Any) -> None:
    _RESPONSE["artifacts"].append(str(path))

def _set_by_path(m: Dict[str, Any], dotted: str, val: Any) -> None:
    cur = m
    parts = dotted.split(".") if dotted else []
    if not parts:
        raise ValueError("lyenv_sdk: empty dotted key")
    for i, p in enumerate(parts):
        if i == len(parts) - 1:
            cur[p] = val
        else:
            nxt = cur.get(p)
            if not isinstance(nxt, dict):
                nxt = {}
                cur[p] = nxt
            cur = nxt

def mutate(key: str, value: Any, scope: str = "plugin") -> None:
    """
    Write mutation by dotted key to response.mutations.<scope>.
    scope: 'plugin' or 'global'
    """
    ms = _RESPONSE["mutations"]
    target = ms["plugin"] if scope == "plugin" else ms["global"]
    _set_by_path(target, key, value)

# ✅ FIXED: Backward/forward compatible signature
def plugin_write_config(
    key: str,
    value: Any,
    scope: str = "plugin",
    merge: Optional[str] = None,
    **kwargs: Any
) -> None:
    """
    Backward/forward compatible config mutation writer.

    - scope: "plugin" or "global"
    - merge: reserved (merge strategy handled by core)
    - kwargs: ignored for forward compatibility
    """
    _ensure_request_loaded()
    mutate(key, value, scope=scope)

def global_write_config(key: str, value: Any) -> None:
    mutate(key, value, scope="global")

# Optional nicer aliases
def plugin_mutate(key: str, value: Any) -> None:
    mutate(key, value, scope="plugin")

def global_mutate(key: str, value: Any) -> None:
    mutate(key, value, scope="global")

# ---------------------------
# Respond helpers
# ---------------------------

def _ensure_not_responded() -> None:
    global _RESPONDED
    if _RESPONDED:
        raise RuntimeError("lyenv_sdk: respond_* called more than once")
    _RESPONDED = True

def respond_ok(message: str = "", extra: Optional[Dict[str, Any]] = None) -> None:
    """
    Send success response JSON to stdout.
    message: final output message (End node typically uses this)
    extra: optional extra fields merged into response root
    """
    _ensure_not_responded()
    if message is not None and str(message).strip() != "":
        _RESPONSE["message"] = str(message)
    if extra:
        for k, v in extra.items():
            _RESPONSE[k] = v
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\n")
    sys.stdout.flush()

def respond_error(message: str, code: int = 1, extra: Optional[Dict[str, Any]] = None) -> None:
    """
    Send error response JSON to stdout and exit.
    """
    _ensure_not_responded()
    _RESPONSE["status"] = "error"
    _RESPONSE["message"] = str(message)
    if extra:
        for k, v in extra.items():
            _RESPONSE[k] = v
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\n")
    sys.stdout.flush()
    raise SystemExit(code)
