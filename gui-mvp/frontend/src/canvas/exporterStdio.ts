// src/canvas/exporterStdio.ts
// Export as lyenv stdio multi-step plugin.
// - Data passing via plugin local config using lyenv_sdk.py mutations.
// - Start node: maps CLI args -> Start outputs -> plugin config
// - Normal nodes: read inputs (wired) from config -> run underlying program -> write outputs to config
// - End node: reads its inputs -> respond_ok() with message (final output)
// - Execution order: linear path Start -> ... -> End (exactly 1 outgoing edge per step)
// - This exporter injects scripts/lyenv_sdk.py, scripts/flow_sdk.py, scripts/flow_wiring.json, and runner scripts.
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
const token = (s: string) => (s || '').trim().toLowerCase().replace(/[^a-z0-9._-]/g, '-').replace(/-+/g, '-') || 'task'

function extByLang(lang?: string) {
  switch ((lang || '').toLowerCase()) {
    case 'python': return 'py'
    case 'javascript': return 'js'
    case 'bash': return 'sh'
    case 'lua': return 'lua'
    case 'go': return 'go'
    default: return 'txt'
  }
}

function findStartEnd(nodes: RFNode[], scope: Set<string>) {
  const inScope = nodes.filter(n => scope.has(n.id))
  const start = inScope.find(n => (n.data as any)?.kind === 'start')
  const end = inScope.find(n => (n.data as any)?.kind === 'end')
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
 * Extract a linear control order defined by Start -> ... -> End.
 * Requirements:
 * - Start and End exist in scope.
 * - Each node on the control path (except End) has exactly 1 outgoing edge within scope.
 * - End has 0 outgoing edges within scope.
 */
function extractLinearOrder(nodes: RFNode[], edges: RFEdge[], scope: Set<string>): string[] {
  const { start, end } = findStartEnd(nodes, scope)
  if (!start || !end) throw new Error('Export requires Start and End nodes inside the group.')

  const { out } = buildAdj(edges, scope)
  const visited = new Set<string>()
  const order: string[] = []

  let cur = start.id
  while (true) {
    if (visited.has(cur)) throw new Error('Cycle detected in Start->End path.')
    visited.add(cur)
    order.push(cur)
    if (cur === end.id) break

    const outs = out.get(cur) || []
    if (outs.length !== 1) {
      throw new Error(`Control node ${cur} must have exactly 1 outgoing edge in Start->End order (found ${outs.length}).`)
    }
    cur = outs[0].target as string
  }
  return order
}

/**
 * Build wiring map: dstNodeId -> dstInputPortName -> { node: srcNodeId, port: srcOutputPortName }
 * Only edges within scope are considered.
 */
function buildWiring(nodes: RFNode[], edges: RFEdge[], scope: Set<string>) {
  const nodeById = new Map(nodes.map(n => [n.id, n] as const))
  const wiring: Record<string, Record<string, { node: string; port: string }>> = {}

  for (const e of edges) {
    if (!e.source || !e.target) continue
    if (!scope.has(e.source) || !scope.has(e.target)) continue
    const src = nodeById.get(e.source)!
    const dst = nodeById.get(e.target)!
    const sPorts = ((src.data as RFNodeData)?.ports?.outputs || [])
    const tPorts = ((dst.data as RFNodeData)?.ports?.inputs || [])
    const sInfo = sPorts.find(p => p.id === e.sourceHandle) || sPorts[0]
    const tInfo = tPorts.find(p => p.id === e.targetHandle) || tPorts[0]
    if (!sInfo || !tInfo) continue

    wiring[dst.id] = wiring[dst.id] || {}
    wiring[dst.id][tInfo.name || tInfo.id] = { node: src.id, port: sInfo.name || sInfo.id }
  }
  return wiring
}

/** Inject user's lyenv_sdk.py (Python stdio SDK) */
function makeLyenvSdkPy(): string {
  return `# -*- coding: utf-8 -*-
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
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\\n")
    sys.stdout.flush()

def respond_error(message: str):
    _RESPONSE["status"] = "error"
    _RESPONSE["message"] = message
    sys.stdout.write(json.dumps(_RESPONSE, ensure_ascii=False) + "\\n")
    sys.stdout.flush()
    sys.exit(1)
`
}

/**
 * Flow SDK built on top of lyenv_sdk.py.
 * Storage convention:
 *   plugin config path: flow.outputs.<node_id>.<port_name> = "<string>"
 * Read convention:
 *   request contains merged plugin config; we try:
 *     req["config"]["plugin"]  (preferred)
 *     req["plugin_config"]
 *     req["plugin"]
 */
function makeFlowSdkPy(): string {
  return `# -*- coding: utf-8 -*-
\"\"\"flow_sdk.py - Flow helper using lyenv_sdk plugin mutations.

Conventions:
  - Write outputs to plugin config:
      flow.outputs.<node_id>.<port> = "<string>"
  - Read inputs from plugin config based on wiring map.

Start node:
  - is_source=True, inputs are CLI args mapped by Start output port order.
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

/** Runner for Start node: map CLI args -> Start output ports -> config */
function makeStartRunnerPy(startId: string, outPorts: string[]): string {
  const outJson = JSON.stringify(outPorts)
  return `# -*- coding: utf-8 -*-
# start_runner.py - stdio runner for Start node (CLI args -> config)
from lyenv_sdk import read_request, respond_ok, respond_error
from flow_sdk import write_outputs

START_ID = ${JSON.stringify(startId)}
OUT_PORTS = ${outJson}

def main():
    try:
        req = read_request()
        args = [str(x) for x in (req.get("args") or [])]
        # Map args by Start output port order
        write_outputs(START_ID, OUT_PORTS, args)
        respond_ok("ok")
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
# end_runner.py - stdio runner for End node (config -> response)
from lyenv_sdk import read_request, respond_ok, respond_error, log
from flow_sdk import load_wiring, build_inputs

END_ID = ${JSON.stringify(endId)}
IN_PORTS = ${inJson}

def main():
    try:
        req = read_request()
        wiring = load_wiring("./scripts/flow_wiring.json")
        vals = build_inputs(req, wiring, END_ID, IN_PORTS)
        # Compose a human-friendly message; also log structured values
        msg = " ".join(vals).strip()
        log({ "end_inputs": dict(zip(IN_PORTS, vals)) })
        respond_ok(msg)
    except Exception as e:
        respond_error(str(e))

if __name__ == "__main__":
    main()
`
}

/** Runner for normal node: read inputs from config -> run underlying program -> write outputs -> respond_ok */
function makeNodeRunnerPy(nodeId: string, label: string, inputPorts: string[], outputPorts: string[], program: string, fixedArgs: string[]): string {
  return `# -*- coding: utf-8 -*-
# runner_${sanitize(nodeId)}.py - stdio runner for node "${label}"
import subprocess
from typing import List
from lyenv_sdk import read_request, log, respond_ok, respond_error
from flow_sdk import load_wiring, build_inputs, write_outputs

NODE_ID = ${JSON.stringify(nodeId)}
INPUT_PORTS = ${JSON.stringify(inputPorts)}
OUTPUT_PORTS = ${JSON.stringify(outputPorts)}
PROGRAM = ${JSON.stringify(program)}
FIXED_ARGS = ${JSON.stringify(fixedArgs)}

def split_outputs(s: str, out_count: int) -> List[str]:
    s = (s or "").strip()
    if out_count <= 1:
        return [s]
    return s.split()

def main():
    try:
        req = read_request()
        wiring = load_wiring("./scripts/flow_wiring.json")
        argv = build_inputs(req, wiring, NODE_ID, INPUT_PORTS)

        cmd = [PROGRAM] + list(FIXED_ARGS) + argv
        p = subprocess.run(cmd, capture_output=True, text=True)

        if p.stderr:
            # keep stderr in logs for debugging
            log(p.stderr.strip())

        if p.returncode != 0:
            respond_error(f"node failed: {NODE_ID}")
            return

        outs = split_outputs(p.stdout, len(OUTPUT_PORTS))
        write_outputs(NODE_ID, OUTPUT_PORTS, outs)
        respond_ok("ok")
    except Exception as e:
        respond_error(str(e))

if __name__ == "__main__":
    main()
`
}

/** Main export (async for future injections; currently pure build) */
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
  files.push({ path: 'config.yaml', content: '# default config for the plugin\nflow:\n  outputs: {}\n' })
  files.push({ path: 'scripts/lyenv_sdk.py', content: makeLyenvSdkPy() })
  files.push({ path: 'scripts/flow_sdk.py', content: makeFlowSdkPy() })

  const nodeById = new Map(allNodes.map(n => [n.id, n] as const))

  // Build scopes: per group or whole canvas
  const groups = allNodes.filter(n => n.type === 'group')
  const scopes: Array<{ cmdName: string; scope: Set<string> }> = []

  if (!groups.length) {
    const ids = allNodes.filter(n => n.type !== 'group').map(n => n.id)
    scopes.push({ cmdName: 'run', scope: new Set(ids) })
  } else {
    for (const g of groups) {
      const cmdName = token(((g.data as any)?.label) || g.id)
      const kids = allNodes.filter(n => (n as any).parentId === g.id && n.type !== 'group').map(n => n.id)
      if (!kids.length) continue
      scopes.push({ cmdName, scope: new Set(kids) })
    }
  }

  for (const s of scopes) {
    const order = extractLinearOrder(allNodes, allEdges, s.scope)
    const wiring = buildWiring(allNodes, allEdges, s.scope)
    files.push({ path: 'scripts/flow_wiring.json', content: JSON.stringify(wiring, null, 2) + '\n' })

    const { start, end } = findStartEnd(allNodes, s.scope)
    if (!start || !end) throw new Error('Start/End missing in scope.')

    const startOutPorts = ((start.data as RFNodeData)?.ports?.outputs || []).map(p => p.name || p.id)
    const endInPorts = ((end.data as RFNodeData)?.ports?.inputs || []).map(p => p.name || p.id)

    // Inject start/end runners
    files.push({ path: 'scripts/start_runner.py', content: makeStartRunnerPy(start.id, startOutPorts) })
    files.push({ path: 'scripts/end_runner.py', content: makeEndRunnerPy(end.id, endInPorts) })

    const steps: CommandStep[] = []
    steps.push({ executor: 'stdio', program: './scripts/start_runner.py', workdir: '.', use_stdio: true })

    for (const nid of order) {
      const n = nodeById.get(nid)!
      const d = (n.data || {}) as RFNodeData
      const kind = (d.kind || 'code') as string

      if (kind === 'start' || kind === 'end') continue

      const inputPorts = (d.ports?.inputs || []).map(p => p.name || p.id)
      const outputPorts = (d.ports?.outputs || []).map(p => p.name || p.id)

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
      files.push({ path: runnerPath, content: makeNodeRunnerPy(nid, String(d.label || nid), inputPorts, outputPorts, program, fixedArgs) })
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
