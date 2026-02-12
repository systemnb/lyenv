// src/canvas/exporterCli.ts
// Pure shell pipeline exporter: one bash step per group.
// Data is passed via CLI args between nodes; each node's stdout is its output.
// Keep comments in English.

import yaml from 'js-yaml'
import type { RFNode, RFEdge, RFNodeData } from './graph'
import {
  childrenOfGroup, activeIdsInScope, topoOrder,
  incomingByInputOrder, upstreamOutputIndex, portCounts, isSource,
} from './graph'

export type CommandStep = { executor: 'shell'; program: string; args?: string[]; workdir?: string; env?: Record<string, string> }
export type CommandSpec = { name: string; summary?: string; steps?: CommandStep[] }
export type Manifest = { name: string; version: string; expose: string[]; commands: CommandSpec[] }

const DEFAULT_EXT: Record<string, string> = { python: 'py', javascript: 'js', bash: 'sh', lua: 'lua', go: 'go' }
const sanitize = (s: string) => (s || '').replace(/[^a-zA-Z0-9_.-]/g, '_')
const token = (s: string) => (s || '').trim().toLowerCase().replace(/[^a-z0-9._-]/g, '-').replace(/-+/g,'-') || 'task'

// ---- Injection manifest types (loaded from /public) ----
type InjectionWhen = {
  always?: boolean
  ifAnyLanguage?: string[] // e.g. ["python","bash"]
}

type InjectionFile = {
  target: string          // path inside zip
  source: string          // URL path to fetch from (public/)
  when?: InjectionWhen
}

type InjectionManifest = {
  version: number
  files: InjectionFile[]
}

/** Extract languages used by code nodes in the current graph */
function collectLanguages(nodes: RFNode[]): Set<string> {
  const set = new Set<string>()
  for (const n of nodes) {
    const d = (n.data || {}) as any
    const kind = d.kind || 'code'
    if (kind === 'code') {
      const lang = (d.config?.language || 'python') as string
      set.add(String(lang).toLowerCase())
    }
    if (kind === 'command') {
      // You may treat command nodes as 'shell' if needed
      set.add('shell')
    }
  }
  return set
}

/** Decide whether to inject a file based on manifest conditions */
function shouldInject(f: InjectionFile, langs: Set<string>): boolean {
  const w = f.when || {}
  if (w.always) return true
  if (w.ifAnyLanguage && w.ifAnyLanguage.length) {
    return w.ifAnyLanguage.some(x => langs.has(String(x).toLowerCase()))
  }
  return false
}

/** Load injection manifest and inject extra files into zip */
async function injectExtraFiles(
  nodes: RFNode[],
  filesOut: { path: string; content: string }[],
  manifestUrl: string = '/lyenv-injections.json'
) {
  try {
    const resp = await fetch(manifestUrl, { cache: 'no-store' })
    if (!resp.ok) return
    const m = (await resp.json()) as InjectionManifest
    if (!m || !Array.isArray(m.files)) return

    const langs = collectLanguages(nodes)

    for (const f of m.files) {
      if (!f?.target || !f?.source) continue
      if (!shouldInject(f, langs)) continue

      const r = await fetch(f.source, { cache: 'no-store' })
      if (!r.ok) continue
      const content = await r.text()

      // Avoid duplicates: if already exists, skip
      if (filesOut.some(x => x.path === f.target)) continue
      filesOut.push({ path: f.target, content })
    }
  } catch {
    // ignore injection errors to not block export
  }
}


function escapeArg(a: string): string {
  if (!a) return "''"
  return `'${a.replace(/'/g, `'\\''`)}'`
}

function generateBashForGroup(nodes: RFNode[], edges: RFEdge[], scope: Set<string>):
  { bash: string; codeFiles: { path: string; content: string }[] } {

  const nodeById = new Map(nodes.map(n => [n.id, n] as const))
  const active = activeIdsInScope(nodes, edges, scope)
  const order = topoOrder(nodes, edges, active)

  const codeFiles: { path: string; content: string }[] = []
  const lines: string[] = []
  lines.push('#!/usr/bin/env bash')
  lines.push('set -euo pipefail')
  lines.push('shopt -s lastpipe')

  // collect CLI args
  lines.push('ARGS=("$@")')
  lines.push('CUR=0')

  lines.push('')
  lines.push('# topological execution')
  for (const id of order) {
    const n = nodeById.get(id)!
    const d = (n.data || {}) as RFNodeData
    const kind = (d.kind || 'code') as string
    const { in: inCnt, out: outCnt } = portCounts(n)
    const label = (d.label || id)

    const inEdges = incomingByInputOrder(n, edges, nodeById)
    const argvPieces: string[] = []

    if (isSource(id, edges, active)) {
      if (inCnt > 0) {
        lines.push(`# ${label}: take ${inCnt} arg(s) from ARGS starting CUR=` + '${CUR}' + ' → argv')
        lines.push('argv=()')
        lines.push(`for i in $(seq 1 ${inCnt}); do argv+=("` + '${ARGS[$CUR]}' + `"); CUR=$((CUR+1)); done`)
      } else {
        lines.push(`# ${label}: no inputs; empty argv`)
        lines.push('argv=()')
      }
    } else {
      lines.push(`# ${label}: build argv from upstream outputs`)
      lines.push('argv=()')
      for (const e of inEdges) {
        const up = nodeById.get(e.source!)!
        const upOutIdx = upstreamOutputIndex(e, nodeById)
        const baseVar = sanitize(up.id)
        if (portCounts(up).out <= 1) argvPieces.push('"$' + `${baseVar}_out"`)
        else argvPieces.push('"$' + `${baseVar}_out_${upOutIdx}"`)
      }
      if (argvPieces.length) lines.push(`argv+=(${argvPieces.join(' ')})`)
    }

    let cmdStr = ''
    if (kind === 'command') {
      const cmd = (d.config as any)?.command || ''
      const args = (d.config as any)?.args || []
      const parts = [cmd, ...Array.from(args || [])].filter(Boolean).map(escapeArg)
      cmdStr = parts.join(' ') + ' ' + '"${argv[@]}"'
    } else {
      const lang = (d.config as any)?.language || 'python'
      let source = (d.config as any)?.source || ''
      if (!String(source).trim()) {
        if (lang === 'python') source = 'from sys import argv\nprint(" ".join(argv[1:]))\n'
        else if (lang === 'javascript') source = 'console.log(process.argv.slice(2).join(" "));\n'
        else source = 'printf "%s\\n" "$*"\n'
      }
      const ext = DEFAULT_EXT[String(lang).toLowerCase()] || 'sh'
      const rel = `scripts/${sanitize(n.id)}.${ext}`
      codeFiles.push({ path: rel, content: source })
      cmdStr = `./${rel} ` + '"${argv[@]}"'
    }

    const base = sanitize(n.id)
    if (outCnt <= 1) {
      lines.push(`# ${label}: capture single output`)
      lines.push(`${base}_out="$(${cmdStr})"`)
    } else {
      lines.push(`# ${label}: capture ${outCnt} outputs (space-split)`)
      const vars = Array.from({ length: outCnt }, (_, i) => `${base}_out_${i}`).join(' ')
      lines.push(`read -r ${vars} <<< "$(${cmdStr})"`)
    }
    lines.push('')
  }

  return { bash: lines.join('\n'), codeFiles }
}

export type BuildResult = { manifest: Manifest; files: { path: string; content: string }[] }

export async function buildManifestAndFiles(allNodes: RFNode[], allEdges: RFEdge[], pluginName: string, shim: string): Promise<BuildResult> {
  const manifest: Manifest = { name: pluginName, version: '0.1.0', expose: [token(shim)], commands: [] }
  const files: { path: string; content: string }[] = []

  const groups = allNodes.filter(n => n.type === 'group')
  if (!groups.length) {
    const scope = new Set(allNodes.filter(n => n.type !== 'group').map(n => n.id))
    const { bash, codeFiles } = generateBashForGroup(allNodes, allEdges, scope)
    manifest.commands.push({ name: 'run', summary: 'Auto-generated pipeline', steps: [{ executor:'shell', program:'bash', args:['-lc', bash] }] })
    files.push(...codeFiles)
  } else {
    for (const g of groups) {
      const childs = childrenOfGroup(allNodes, g.id)
      if (!childs.length) continue
      const scope = new Set(childs)
      const { bash, codeFiles } = generateBashForGroup(allNodes, allEdges, scope)
      const cmdName = token((g.data as any)?.label || g.id)
      manifest.commands.push({ name: cmdName, summary: `Group: ${cmdName}`, steps: [{ executor:'shell', program:'bash', args:['-lc', bash] }] })
      files.push(...codeFiles)
    }
  }
  files.push({ path: 'README.md', content: `# ${pluginName}\nGenerated by Canvas exporter (shell pipeline).\n` })
  await injectExtraFiles(allNodes, files)
  return { manifest, files }
}

export function dumpManifestYAML(manifest: Manifest): string {
  return yaml.dump(manifest)
}