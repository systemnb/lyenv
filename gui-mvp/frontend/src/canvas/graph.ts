// src/canvas/graph.ts
// Graph helpers: scope, topological order, port index lookup, wiring order.
// Keep comments in English.

import type { Edge, Node } from '@xyflow/react'
import type { FunctionalData } from '../types'

export type RFNodeData = FunctionalData
export type RFNode = Node<RFNodeData, 'editable' | 'group'>
export type RFEdge = Edge

/** Get children node ids inside a group (v12: parentId) */
export function childrenOfGroup(all: RFNode[], groupId: string): string[] {
  return all.filter(n => (n as any).parentId === groupId && n.type !== 'group').map(n => n.id)
}

/** Build active subgraph (nodes that appear in edges within scope) */
export function activeIdsInScope(nodes: RFNode[], edges: RFEdge[], scope: Set<string>): Set<string> {
  const active = new Set<string>()
  edges.forEach(e => {
    if (e.source && e.target && scope.has(e.source) && scope.has(e.target)) {
      active.add(e.source); active.add(e.target)
    }
  })
  if (active.size === 0 && scope.size === 1) active.add(Array.from(scope)[0]!)
  return active
}

/** Topo sort (Kahn) within active scope */
export function topoOrder(nodes: RFNode[], edges: RFEdge[], active: Set<string>): string[] {
  const indeg: Record<string, number> = {}
  active.forEach(id => indeg[id] = 0)
  edges.forEach(e => {
    if (e.source && e.target && active.has(e.source) && active.has(e.target)) {
      indeg[e.target] = (indeg[e.target] ?? 0) + 1
    }
  })
  const adj = new Map<string, string[]>()
  edges.forEach(e => {
    if (e.source && e.target && active.has(e.source) && active.has(e.target)) {
      const list = adj.get(e.source) || []
      list.push(e.target)
      adj.set(e.source, list)
    }
  })
  const q: string[] = []
  active.forEach(id => { if ((indeg[id] ?? 0) === 0) q.push(id) })
  const out: string[] = []
  while (q.length) {
    const v = q.shift()!
    out.push(v)
    for (const w of (adj.get(v) || [])) {
      indeg[w]--
      if (indeg[w] === 0) q.push(w)
    }
  }
  return out.length === active.size ? out : Array.from(active)
}

/** Port index lookup by handle id */
export function outputIndexOf(node: RFNode, handleId?: string | null): number {
  const outs = (node.data as RFNodeData)?.ports?.outputs || []
  if (!handleId) return 0
  const idx = outs.findIndex(p => p.id === handleId)
  return idx >= 0 ? idx : 0
}
export function inputIndexOf(node: RFNode, handleId?: string | null): number {
  const ins = (node.data as RFNodeData)?.ports?.inputs || []
  if (!handleId) return 0
  const idx = ins.findIndex(p => p.id === handleId)
  return idx >= 0 ? idx : 0
}

/** Incoming edges sorted by target input port index */
export function incomingByInputOrder(node: RFNode, edges: RFEdge[], nodeById: Map<string, RFNode>): RFEdge[] {
  const inEdges = edges.filter(e => e.target === node.id)
  const withIdx = inEdges.map(e => ({ e, idx: inputIndexOf(node, e.targetHandle) }))
  withIdx.sort((a, b) => a.idx - b.idx)
  return withIdx.map(x => x.e)
}

/** Outgoing edges to get upstream output index per edge */
export function upstreamOutputIndex(e: RFEdge, nodeById: Map<string, RFNode>): number {
  const up = nodeById.get(e.source!)!
  return outputIndexOf(up, e.sourceHandle)
}

/** Port counts */
export function portCounts(node: RFNode): { in: number; out: number } {
  const d = (node.data || {}) as RFNodeData
  return { in: d.ports?.inputs?.length || 0, out: d.ports?.outputs?.length || 0 }
}

/** Is source node (no incoming edge in active scope) */
export function isSource(nodeId: string, edges: RFEdge[], active: Set<string>): boolean {
  return !edges.some(e => e.target === nodeId && active.has(e.source!))
}
