// src/canvas/exporterStdio.ts
// Export as lyenv stdio multi-step plugin.
//
// - Data passing via plugin local config using flow_sdk.py + lyenv_sdk.py mutations.
// - Start node: maps CLI args -> Start outputs -> plugin config
// - Normal nodes: read inputs (wired) from config -> run underlying program -> write outputs to config
// - End node: reads its inputs -> respond_ok() with message (final output)
// - Execution order: linear path Start -> ... -> End (exactly 1 outgoing edge per step)
// - This exporter injects SDK files from public/sdks:
//   scripts/lyenv_sdk.py, scripts/lyenv_sdk.sh, scripts/lyenv_sdk.js, scripts/flow_sdk.py
// - Also injects scripts/flow_wiring.json and runner scripts.
//
// All comments in English.

import yaml from 'js-yaml'
import type { RFNode, RFEdge, RFNodeData } from './graph'

export type CommandStep = {
  executor: 'stdio'
  program: string
  workdir?: string
  use_stdio?: boolean
}
export type CommandSpec = {
  name: string
  summary?: string
  steps: CommandStep[]
}
export type Manifest = {
  name: string
  version: string
  expose: string[]
  config?: { local_file?: string }
  commands: CommandSpec[]
}

type FilesOut = { path: string; content: string }[]

const sanitize = (s: string) => (s || '').replace(/[^a-zA-Z0-9_.-]/g, '_')
const token = (s: string) =>
  (s || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._-]/g, '-')
    .replace(/-+/g, '-') || 'task'

function extByLang(lang?: string) {
  switch ((lang || '').toLowerCase()) {
    case 'python':
      return 'py'
    case 'javascript':
      return 'js'
    case 'bash':
      return 'sh'
    case 'lua':
      return 'lua'
    case 'go':
      return 'go'
    default:
      return 'txt'
  }
}

function findStartEnd(nodes: RFNode[], scope: Set<string>) {
  const inScope = nodes.filter((n) => scope.has(n.id))
  const start = inScope.find((n) => (n.data as any)?.kind === 'start')
  const end = inScope.find((n) => (n.data as any)?.kind === 'end')
  return { start, end }
}

function buildAdj(edges: RFEdge[], scope: Set<string>) {
  const out = new Map<string, RFEdge[]>()
  for (const e of edges) {
    if (!e.source || !e.target) continue
    if (!scope.has(e.source) || !scope.has(e.target)) continue
    out.set(e.source, [...(out.get(e.source) || []), e])
  }
  return { out }
}

/**
 * Extract a deterministic execution order using topological sorting (DAG).
 * This enables fan-out / fan-in wiring without requiring "exactly 1 outgoing edge".
 *
 * Rules:
 * - Graph must be acyclic within the scope
 * - Start will be placed first, End will be placed last
 * - For determinism, nodes with the same indegree are ordered by (position.y, position.x, id)
 */
function extractTopoOrder(nodes: RFNode[], edges: RFEdge[], scope: Set<string>): string[] {
  const { start, end } = findStartEnd(nodes, scope)
  if (!start || !end) throw new Error('Export requires Start and End nodes inside the group.')

  const inScope = nodes.filter(n => scope.has(n.id))
  const ids = inScope.map(n => n.id)

  const nodeById = new Map(inScope.map(n => [n.id, n] as const))

  // adjacency + indegree
  const out = new Map<string, Set<string>>()
  const indeg = new Map<string, number>()
  for (const id of ids) {
    out.set(id, new Set())
    indeg.set(id, 0)
  }

  for (const e of edges) {
    if (!e.source || !e.target) continue
    if (!scope.has(e.source) || !scope.has(e.target)) continue
    if (e.source === e.target) continue
    // De-dup parallel edges
    const s = e.source
    const t = e.target
    const set = out.get(s)!
    if (!set.has(t)) {
      set.add(t)
      indeg.set(t, (indeg.get(t) || 0) + 1)
    }
  }

  const sortKey = (id: string) => {
    const n = nodeById.get(id)
    const x = (n as any)?.position?.x ?? 0
    const y = (n as any)?.position?.y ?? 0
    return { x, y, id }
  }

  const cmp = (a: string, b: string) => {
    // Keep Start earliest whenever possible
    if (a === start.id) return -1
    if (b === start.id) return 1

    const ka = sortKey(a), kb = sortKey(b)

    // Left-to-right first
    if (ka.x !== kb.x) return ka.x - kb.x
    // Then top-to-bottom
    if (ka.y !== kb.y) return ka.y - kb.y

    // Stable tie-breaker
    return ka.id.localeCompare(kb.id)
  }


  // init queue (indegree 0)
  const queue: string[] = []
  for (const [id, d] of indeg.entries()) {
    if (d === 0) queue.push(id)
  }
  queue.sort(cmp)

  // Kahn
  const order: string[] = []
  while (queue.length) {
    const cur = queue.shift()!
    order.push(cur)

    for (const nxt of out.get(cur) || []) {
      indeg.set(nxt, (indeg.get(nxt) || 0) - 1)
      if (indeg.get(nxt) === 0) {
        queue.push(nxt)
        queue.sort(cmp)
      }
    }
  }

  // cycle detection
  if (order.length !== ids.length) {
    throw new Error('Cycle detected in workflow graph. Export requires an acyclic workflow (DAG).')
  }

  // Force Start first / End last (keep relative order for others)
  const withoutStartEnd = order.filter(x => x !== start.id && x !== end.id)
  return [start.id, ...withoutStartEnd, end.id]
}


/**
 * Build wiring map: dstNodeId -> dstInputPortName -> { node: srcNodeId, port: srcOutputPortName }
 * Only edges within scope are considered.
 */
function buildWiring(nodes: RFNode[], edges: RFEdge[], scope: Set<string>) {
  const nodeById = new Map(nodes.map((n) => [n.id, n] as const))
  const wiring: Record<string, Record<string, { node: string; port: string }>> = {}

  for (const e of edges) {
    if (!e.source || !e.target) continue
    if (!scope.has(e.source) || !scope.has(e.target)) continue
    const src = nodeById.get(e.source)!
    const dst = nodeById.get(e.target)!
    const sPorts = (src.data as RFNodeData)?.ports?.outputs || []
    const tPorts = (dst.data as RFNodeData)?.ports?.inputs || []
    const sInfo = sPorts.find((p) => p.id === e.sourceHandle) || sPorts[0]
    const tInfo = tPorts.find((p) => p.id === e.targetHandle) || tPorts[0]
    if (!sInfo || !tInfo) continue

    wiring[dst.id] = wiring[dst.id] || {}
    wiring[dst.id][tInfo.name || tInfo.id] = { node: src.id, port: sInfo.name || sInfo.id }
  }
  return wiring
}

// -------------------------
// Public SDK loader (Vite public/)
// -------------------------

const _sdkCache = new Map<string, string>()

function joinBaseUrl(base: string, rel: string) {
  // base often like "/" or "/subpath/"
  const b = base.endsWith('/') ? base : base + '/'
  const r = rel.startsWith('/') ? rel.slice(1) : rel
  return b + r
}

async function loadPublicText(relPath: string, fallback: string): Promise<string> {
  const base = (import.meta as any).env?.BASE_URL ?? '/'
  const url = joinBaseUrl(String(base), relPath)

  if (_sdkCache.has(url)) return _sdkCache.get(url) as string

  try {
    const res = await fetch(url, { cache: 'no-store' })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const text = await res.text()
    if (!text || !text.trim()) throw new Error('empty content')
    _sdkCache.set(url, text)
    return text
  } catch {
    // Offline / missing file: keep export functional
    _sdkCache.set(url, fallback)
    return fallback
  }
}

// -------------------------
// Fallback SDKs (minimal; real sources should live in public/sdks/*)
// -------------------------

function fallbackLyenvSdkPy(): string {
  // Minimal but robust: read full stdin, config_get, mutate, respond
  return `# -*- coding: utf-8 -*-
"""
lyenv_sdk.py - Fallback Python SDK (minimal).
Prefer using public/sdks/lyenv_sdk.py.
"""
import sys, json
from typing import Any, Dict, Optional, List

_REQUEST: Dict[str, Any] = {}
_RESPONDED = False
_RESPONSE: Dict[str, Any] = {
  "status":"ok","logs":[],"artifacts":[],"mutations":{"global":{},"plugin":{}}
}

def read_request() -> Dict[str, Any]:
    raw = sys.stdin.read()
    if not raw or not raw.strip():
        raise RuntimeError("lyenv_sdk: empty stdin")
    global _REQUEST
    _REQUEST = json.loads(raw)
    return _REQUEST

def _ensure():
    if not _REQUEST:
        raise RuntimeError("lyenv_sdk: call read_request() first")

def _get_by_path(obj: Any, dotted: str, default: Any=None) -> Any:
    if not dotted:
        return obj
    cur = obj
    for p in dotted.split("."):
        if isinstance(cur, dict) and p in cur:
            cur = cur[p]
        else:
            return default
    return cur

def config_get(key: str, default: Any=None, scope: str="plugin") -> Any:
    _ensure()
    cfg = _REQUEST.get("config") or {}
    base = cfg.get(scope) or {}
    return _get_by_path(base, key, default)

def log(msg: Any) -> None:
    _RESPONSE["logs"].append(str(msg))

def emit_artifact(path: Any) -> None:
    _RESPONSE["artifacts"].append(str(path))

def _set_by_path(m: Dict[str, Any], dotted: str, val: Any) -> None:
    cur = m
    parts = dotted.split(".")
    for i,p in enumerate(parts):
        if i == len(parts)-1:
            cur[p] = val
        else:
            nxt = cur.get(p)
            if not isinstance(nxt, dict):
                nxt = {}
                cur[p] = nxt
            cur = nxt

def plugin_write_config(key: str, value: Any, scope: str="plugin", merge: Optional[str]=None) -> None:
    _ensure()
    target = _RESPONSE["mutations"]["plugin"] if scope=="plugin" else _RESPONSE["mutations"]["global"]
    _set_by_path(target, key, value)

def respond_ok(message: str="") -> None:
    global _RESPONDED
    if _RESPONDED:
        raise RuntimeError("lyenv_sdk: respond called twice")
    _RESPONDED = True
    if message and str(message).strip():
        _RESPONSE["message"] = str(message)
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\\n")
    sys.stdout.flush()

def respond_error(message: str) -> None:
    global _RESPONDED
    if _RESPONDED:
        raise RuntimeError("lyenv_sdk: respond called twice")
    _RESPONDED = True
    _RESPONSE["status"] = "error"
    _RESPONSE["message"] = str(message)
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\\n")
    sys.stdout.flush()
    raise SystemExit(1)
`
}

function fallbackFlowSdkPy(): string {
  return `# -*- coding: utf-8 -*-
\"\"\"flow_sdk.py - Fallback flow helper.
Prefer using public/sdks/flow_sdk.py.
\"\"\"
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
`
}

function fallbackLyenvSdkSh(): string {
  // Minimal stub: respond ok/error with empty logs/mutations.
  // Prefer using public/sdks/lyenv_sdk.sh.
  return `#!/usr/bin/env bash
set -euo pipefail
# Fallback lyenv_sdk.sh (minimal). Prefer public/sdks/lyenv_sdk.sh.

LYENV_REQ_JSON=""
ly_read_request(){ LYENV_REQ_JSON="$(cat)"; [[ -n "\${LYENV_REQ_JSON//[[:space:]]/}" ]] || { echo "lyenv_sdk: empty stdin" >&2; return 1; }; }
ly_log(){ :; }
ly_emit_artifact(){ :; }
ly_mutate_set(){ :; }
ly_respond_ok(){ printf '{"status":"ok","logs":[],"artifacts":[],"mutations":{"global":{},"plugin":{}}}\\n'; }
ly_respond_error(){ printf '{"status":"error","message":"%s","logs":[],"artifacts":[],"mutations":{"global":{},"plugin":{}}}\\n' "\${1:-error}"; exit 1; }
`
}

function fallbackLyenvSdkJs(): string {
  // Minimal stub: read stdin JSON and respond.
  // Prefer using public/sdks/lyenv_sdk.js.
  return `// Fallback lyenv_sdk.js (minimal). Prefer public/sdks/lyenv_sdk.js.
let REQUEST=null;
const RESPONSE={status:"ok",logs:[],artifacts:[],mutations:{global:{},plugin:{}}};
function readAllStdin(){return new Promise(r=>{let d="";process.stdin.setEncoding("utf8");process.stdin.on("data",c=>d+=c);process.stdin.on("end",()=>r(d));});}
async function read_request(){const raw=await readAllStdin(); if(!raw||!raw.trim()) throw new Error("lyenv_sdk: empty stdin"); REQUEST=JSON.parse(raw); return REQUEST;}
function respond_ok(message=""){ if(message&&message.trim()) RESPONSE.message=message; process.stdout.write(JSON.stringify(RESPONSE)+"\\n"); }
function respond_error(message="error"){ RESPONSE.status="error"; RESPONSE.message=String(message); process.stdout.write(JSON.stringify(RESPONSE)+"\\n"); process.exit(1); }
module.exports={read_request,respond_ok,respond_error};
`
}

// -------------------------
// Runner templates
// -------------------------

/** Runner for Start node: map CLI args -> Start output ports -> flow.outputs */
function makeStartRunnerPy(startId: string, outPorts: string[]): string {
  const outJson = JSON.stringify(outPorts)
  return `# -*- coding: utf-8 -*-
# start_runner.py - stdio runner for Start node (CLI args -> flow outputs)
from lyenv_sdk import read_request, respond_ok, respond_error, log
from flow_sdk import set_outputs

START_ID = ${JSON.stringify(startId)}
OUT_PORTS = ${outJson}

def main():
    try:
        req = read_request()
        args = [str(x) for x in (req.get("args") or [])]

        # Map args by port order; missing values become ""
        vals = { OUT_PORTS[i]: (args[i] if i < len(args) else "") for i in range(len(OUT_PORTS)) }

        log({ "start_args": args, "start_outputs": vals })
        set_outputs(START_ID, vals)

        # Keep empty message to reduce console noise
        respond_ok("")
    except Exception as e:
        respond_error(str(e))

if __name__ == "__main__":
    main()
`
}


/** Runner for End node: read inputs -> respond_ok (final output) */
function makeEndRunnerPy(endId: string, inPorts: string[]): string {
  const inJson = JSON.stringify(inPorts)
  return `# -*- coding: utf-8 -*-
# end_runner.py - stdio runner for End node (flow outputs -> final response)
from lyenv_sdk import read_request, respond_ok, respond_error, log
from flow_sdk import load_wiring, get_inputs

END_ID = ${JSON.stringify(endId)}
IN_PORTS = ${inJson}

def main():
    try:
        req = read_request()
        wiring = load_wiring("./scripts/flow_wiring.json")
        vals = get_inputs(req, wiring, END_ID, IN_PORTS, default="")

        msg = " ".join(vals).strip()
        log({ "end_inputs": dict(zip(IN_PORTS, vals)) })
        respond_ok(msg)
    except Exception as e:
        respond_error(str(e))

if __name__ == "__main__":
    main()
`
}

/** Runner for normal node (HYBRID):
 * - Always resolves inputs via flow_sdk (dataflow)
 * - Always passes req JSON to child stdin (enables lyenv_sdk.read_request in child)
 * - Child can output:
 *   A) Plain stdout (text) or JSON array -> mapped to output ports
 *   B) Full stdio JSON response (status/logs/artifacts/mutations/outputs/message) -> merged into runner response
 */
function makeNodeRunnerPy(
  nodeId: string,
  label: string,
  inputPorts: string[],
  outputPorts: string[],
  program: string,
  fixedArgs: string[]
): string {
  const progLower = (program || '').toLowerCase()
  const isPython = progLower === 'python' || progLower === 'python3' || progLower === 'py'

  return `# -*- coding: utf-8 -*-
# runner_${sanitize(nodeId)}.py - stdio runner for node "${label}" (hybrid runtime)

import subprocess
import sys
import json
from typing import List, Any, Dict, Tuple

from lyenv_sdk import read_request, log, respond_ok, respond_error, mutate, emit_artifact
from flow_sdk import load_wiring, get_inputs, set_outputs, debug_dump_io

NODE_ID = ${JSON.stringify(nodeId)}
INPUT_PORTS = ${JSON.stringify(inputPorts)}
OUTPUT_PORTS = ${JSON.stringify(outputPorts)}
PROGRAM = ${isPython ? "sys.executable" : JSON.stringify(program)}
FIXED_ARGS = ${JSON.stringify(fixedArgs)}

# ---------------------------
# Helpers
# ---------------------------

def _as_text(x: Any) -> str:
    return "" if x is None else str(x)

def _parse_outputs_from_text(stdout_text: str, out_count: int) -> List[str]:
    """
    Text output parsing strategy:
    1) If stdout is JSON array: ["a","b",...], map by index (recommended)
    2) Else if out_count == 1: return raw text
    3) Else fallback: split() tokens (last resort; unsafe for spaces)
    """
    s = (stdout_text or "").strip()
    if out_count <= 0:
        return []
    if s == "":
        return [""] * out_count

    # Try JSON array first
    try:
        obj = json.loads(s)
        if isinstance(obj, list):
            arr = [_as_text(v) for v in obj]
            if len(arr) < out_count:
                arr = arr + [""] * (out_count - len(arr))
            return arr[:out_count]
    except Exception:
        pass

    if out_count == 1:
        return [s]

    parts = s.split()
    if len(parts) < out_count:
        parts = parts + [""] * (out_count - len(parts))
    return parts[:out_count]

def _looks_like_stdio_resp(obj: Any) -> bool:
    return isinstance(obj, dict) and ("status" in obj)

def _flatten_dict(prefix: str, obj: Any, out: List[Tuple[str, Any]]) -> None:
    """
    Flatten nested dict into dotted keys:
      {"a":{"b":1}} -> ("a.b", 1)
    Lists/scalars are treated as leaf values.
    """
    if isinstance(obj, dict):
        for k, v in obj.items():
            k = str(k)
            np = f"{prefix}.{k}" if prefix else k
            _flatten_dict(np, v, out)
    else:
        if prefix:
            out.append((prefix, obj))

def _merge_child_mutations(child_resp: Dict[str, Any]) -> None:
    muts = child_resp.get("mutations") or {}
    if not isinstance(muts, dict):
        return

    for scope_name in ["plugin", "global"]:
        scope_obj = muts.get(scope_name) or {}
        flat: List[Tuple[str, Any]] = []
        _flatten_dict("", scope_obj, flat)
        for k, v in flat:
            mutate(k, v, scope=scope_name)

def _merge_child_logs(child_resp: Dict[str, Any]) -> None:
    logs = child_resp.get("logs") or []
    if isinstance(logs, list):
        for x in logs:
            log(x)

def _merge_child_artifacts(child_resp: Dict[str, Any]) -> None:
    arts = child_resp.get("artifacts") or []
    if isinstance(arts, list):
        for x in arts:
            emit_artifact(x)

def _extract_child_outputs(child_resp: Dict[str, Any], out_count: int) -> List[str]:
    """
    Child outputs priority:
    1) child_resp.outputs (list) if present
    2) if out_count==1: child_resp.message
    3) fallback: empty list padded
    """
    outs = child_resp.get("outputs")
    if isinstance(outs, list):
        arr = [_as_text(v) for v in outs]
        if len(arr) < out_count:
            arr = arr + [""] * (out_count - len(arr))
        return arr[:out_count]

    if out_count == 1:
        msg = _as_text(child_resp.get("message") or "")
        return [msg]

    return [""] * out_count

def _should_pass_stdin() -> bool:
    """
    Pass req JSON to child stdin for script-like nodes.
    For most GUI code nodes, FIXED_ARGS[0] points to ./scripts/<node>.ext.
    If this returns False, child won't get stdin.
    """
    if len(FIXED_ARGS) >= 1 and isinstance(FIXED_ARGS[0], str):
        s0 = FIXED_ARGS[0]
        if s0.startswith("./scripts/") or s0.startswith("scripts/"):
            return True
    # Also pass stdin for python interpreter mode (safe)
    if PROGRAM == sys.executable:
        return True
    return False

# ---------------------------
# Main
# ---------------------------

def main():
    try:
        req = read_request()
        wiring = load_wiring("./scripts/flow_wiring.json")

        argv = get_inputs(req, wiring, NODE_ID, INPUT_PORTS, default="")
        # Great for GUI debugging: shows resolved inputs and current stored outputs (if any)
        debug_dump_io(req, wiring, NODE_ID, INPUT_PORTS, OUTPUT_PORTS)

        cmd = [PROGRAM] + list(FIXED_ARGS) + argv
        log({ "node": NODE_ID, "cmd": cmd })

        pass_stdin = _should_pass_stdin()
        req_json = json.dumps(req, ensure_ascii=False) + "\\n"

        try:
            if pass_stdin:
                p = subprocess.run(cmd, input=req_json, capture_output=True, text=True)
            else:
                p = subprocess.run(cmd, capture_output=True, text=True)
        except Exception as e:
            respond_error(f"node failed: {NODE_ID}: {e}")
            return

        # Always log stderr (truncate)
        if p.stderr:
            s = p.stderr.strip()
            if len(s) > 2000:
                s = s[:2000] + "...(truncated)"
            log({ "node": NODE_ID, "stderr": s })

        raw_out = (p.stdout or "").strip()

        # 1) Try interpret stdout as a stdio JSON response from child
        child_resp = None
        if raw_out:
            try:
                obj = json.loads(raw_out)
                if _looks_like_stdio_resp(obj):
                    child_resp = obj
            except Exception:
                child_resp = None

        if child_resp is not None:
            # If child provides stdio response, honor it
            st = str(child_resp.get("status") or "")
            if st != "ok":
                msg = _as_text(child_resp.get("message") or f"node failed: {NODE_ID}")
                respond_error(msg)
                return

            # Merge logs/artifacts/mutations from child into runner response
            _merge_child_logs(child_resp)
            _merge_child_artifacts(child_resp)
            _merge_child_mutations(child_resp)

            # Map outputs for downstream ports
            outs = _extract_child_outputs(child_resp, len(OUTPUT_PORTS))
            set_outputs(NODE_ID, dict(zip(OUTPUT_PORTS, outs)))
            log({ "node": NODE_ID, "outputs": dict(zip(OUTPUT_PORTS, outs)) })

            # keep empty message to reduce console noise
            respond_ok("")
            return

        # 2) Otherwise treat as normal process output
        if p.returncode != 0:
            # no child stdio response, fall back to returncode+stderr
            msg = (p.stderr or "").strip()
            if msg:
                msg = msg[:400]
            respond_error(f"node failed: {NODE_ID}: rc={p.returncode} {msg}")
            return

        outs = _parse_outputs_from_text(p.stdout, len(OUTPUT_PORTS))
        set_outputs(NODE_ID, dict(zip(OUTPUT_PORTS, outs)))
        log({ "node": NODE_ID, "outputs": dict(zip(OUTPUT_PORTS, outs)) })

        respond_ok("")
    except Exception as e:
        respond_error(str(e))

if __name__ == "__main__":
    main()
`
}



// -------------------------
// Export main
// -------------------------

/** Main export (async so we can fetch SDKs from public/) */
export async function buildManifestAndFilesStdio(
  allNodes: RFNode[],
  allEdges: RFEdge[],
  pluginName: string,
  shim: string
): Promise<{ manifest: Manifest; files: FilesOut }> {
  const manifest: Manifest = {
    name: pluginName,
    version: '0.1.0',
    expose: [token(shim)],
    config: { local_file: './config.yaml' },
    commands: [],
  }

  const files: FilesOut = []

  // Default plugin config
  files.push({
    path: 'config.yaml',
    content: '# default config for the plugin\nflow:\n  outputs: {}\n',
  })

  // Load SDKs from public/sdks (fallback to embedded stubs if missing)
  const sdkPy = await loadPublicText('sdks/lyenv_sdk.py', fallbackLyenvSdkPy())
  const sdkSh = await loadPublicText('sdks/lyenv_sdk.sh', fallbackLyenvSdkSh())
  const sdkJs = await loadPublicText('sdks/lyenv_sdk.js', fallbackLyenvSdkJs())
  const flowPy = await loadPublicText('sdks/flow_sdk.py', fallbackFlowSdkPy())
  const flowJs = await loadPublicText('sdks/flow_sdk.js', '/* fallback flow_sdk.js missing */\n')
  const flowSh = await loadPublicText('sdks/flow_sdk.sh', '#!/usr/bin/env bash\n# fallback flow_sdk.sh missing\n')

  files.push({ path: 'scripts/lyenv_sdk.py', content: sdkPy })
  files.push({ path: 'scripts/lyenv_sdk.sh', content: sdkSh })
  files.push({ path: 'scripts/lyenv_sdk.js', content: sdkJs })
  files.push({ path: 'scripts/flow_sdk.py', content: flowPy })
  files.push({ path: 'scripts/flow_sdk.js', content: flowJs })
  files.push({ path: 'scripts/flow_sdk.sh', content: flowSh })

  const nodeById = new Map(allNodes.map((n) => [n.id, n] as const))

  // Build scopes: per group or whole canvas
  const groups = allNodes.filter((n) => n.type === 'group')
  const scopes: Array<{ cmdName: string; scope: Set<string> }> = []

  if (!groups.length) {
    const ids = allNodes.filter((n) => n.type !== 'group').map((n) => n.id)
    scopes.push({ cmdName: 'run', scope: new Set(ids) })
  } else {
    for (const g of groups) {
      const cmdName = token(((g.data as any)?.label) || g.id)
      const kids = allNodes
        .filter((n) => (n as any).parentId === g.id && n.type !== 'group')
        .map((n) => n.id)
      if (!kids.length) continue
      scopes.push({ cmdName, scope: new Set(kids) })
    }
  }

  for (const s of scopes) {
    const order = extractTopoOrder(allNodes, allEdges, s.scope)
    const wiring = buildWiring(allNodes, allEdges, s.scope)

    // One wiring file per command scope (same path; overwritten each loop)
    files.push({ path: 'scripts/flow_wiring.json', content: JSON.stringify(wiring, null, 2) + '\n' })

    const { start, end } = findStartEnd(allNodes, s.scope)
    if (!start || !end) throw new Error('Start/End missing in scope.')

    const startOutPorts = ((start.data as RFNodeData)?.ports?.outputs || []).map((p) => p.name || p.id)
    const endInPorts = ((end.data as RFNodeData)?.ports?.inputs || []).map((p) => p.name || p.id)

    // Inject start/end runners (always same paths; overwritten each loop)
    files.push({ path: 'scripts/start_runner.py', content: makeStartRunnerPy(start.id, startOutPorts) })
    files.push({ path: 'scripts/end_runner.py', content: makeEndRunnerPy(end.id, endInPorts) })

    const steps: CommandStep[] = []
    steps.push({ executor: 'stdio', program: './scripts/start_runner.py', workdir: '.', use_stdio: true })

    for (const nid of order) {
      const n = nodeById.get(nid)!
      const d = (n.data || {}) as RFNodeData
      const kind = String((d as any).kind || 'code')

      if (kind === 'start' || kind === 'end') continue

      const inputPorts = (d.ports?.inputs || []).map((p) => p.name || p.id)
      const outputPorts = (d.ports?.outputs || []).map((p) => p.name || p.id)

      let program = ''
      let fixedArgs: string[] = []

      if (kind === 'command') {
        program = String((d.config as any)?.command || '')
        fixedArgs = Array.isArray((d.config as any)?.args) ? (d.config as any).args : []
        if (!program) throw new Error(`command node missing command: ${nid}`)
      } else {
        const lang = String((d.config as any)?.language || 'python')
        const ext = extByLang(lang)
        const codePath = `scripts/${sanitize(nid)}.${ext}`
        const src = String((d.config as any)?.source || '')
        files.push({ path: codePath, content: src || '' })

        if (lang.toLowerCase() === 'python') {
          program = 'python3'
          fixedArgs = [`./${codePath}`]
        } else if (lang.toLowerCase() === 'javascript') {
          program = 'node'
          fixedArgs = [`./${codePath}`]
        } else if (lang.toLowerCase() === 'bash') {
          program = 'bash'
          fixedArgs = [`./${codePath}`]
        } else {
          program = `./${codePath}`
          fixedArgs = []
        }
      }

      const runnerPath = `scripts/runner_${sanitize(nid)}.py`
      files.push({
        path: runnerPath,
        content: makeNodeRunnerPy(nid, String((d as any).label || nid), inputPorts, outputPorts, program, fixedArgs),
      })
      steps.push({ executor: 'stdio', program: `./${runnerPath}`, workdir: '.', use_stdio: true })
    }

    steps.push({ executor: 'stdio', program: './scripts/end_runner.py', workdir: '.', use_stdio: true })

    manifest.commands.push({
      name: s.cmdName,
      summary: `Flow command: ${s.cmdName}`,
      steps,
    })
  }

  // README
  files.push({ path: 'README.md', content: `# ${pluginName}\nGenerated by Canvas exporter (stdio flow).\n` })

  return { manifest, files }
}

export function dumpManifestYAML(manifest: Manifest): string {
  return yaml.dump(manifest)
}
