// src/canvas/hooks/useHistoryHotkeys.ts
// History stack (undo/redo) + global hotkeys for the canvas.
// Design:
// - History is based on provided snapshots (nodes/edges) to avoid RF timing issues.
// - UI-driven changes (drag end / add/remove edges) can still push from RF state.
// - Programmatic operations (create/delete/edit-data/group/ungroup) MUST call commitSnapshotState(nextNodes,nextEdges).
// - Prevent snapback by suspending pushes during undo/redo.
// - Hotkeys DO NOT auto-commit; handlers decide whether to commit.
//
// Comments in English.

import { useCallback, useEffect, useRef } from 'react'
import type {
  ReactFlowInstance,
  Node as RFNode,
  Edge as RFEdge,
  NodeChange,
  EdgeChange,
} from '@xyflow/react'

function deepClone<T>(v: T): T {
  // structuredClone is supported in modern browsers
  // Fallback to JSON clone for plain data objects
  try {
    // @ts-ignore
    if (typeof structuredClone === 'function') return structuredClone(v)
  } catch {}
  return JSON.parse(JSON.stringify(v))
}

function isTextInput(el: Element | null) {
  if (!el) return false
  const tag = el.tagName?.toLowerCase?.() || ''
  return (
    tag === 'input' ||
    tag === 'textarea' ||
    (el as HTMLElement).isContentEditable ||
    (el as HTMLElement).classList?.contains?.('monaco-editor') ||
    !!(el as HTMLElement).closest?.('.monaco-editor')
  )
}

function shouldCommitNodes<N extends RFNode>(changes: NodeChange<N>[]): boolean {
  for (const ch of changes) {
    if (ch.type === 'position') {
      const dragging = (ch as any).dragging
      if (dragging === false) return true // drag end
    }
    if (ch.type === 'add' || ch.type === 'remove') return true
    // ignore select/dimensions to avoid extra steps
  }
  return false
}

function shouldCommitEdges<E extends RFEdge>(changes: EdgeChange<E>[]): boolean {
  for (const ch of changes) {
    if (ch.type === 'add' || ch.type === 'remove' || (ch as any).type === 'replace') return true
  }
  return false
}

export type HistoryHotkeysOptions<N extends RFNode = RFNode, E extends RFEdge = RFEdge> = {
  rf: ReactFlowInstance<N, E>
  setNodes: (updater: any) => void
  setEdges: (updater: any) => void
  isBlocked?: boolean
  onNodesChangeOrig: (changes: NodeChange<N>[]) => void
  onEdgesChangeOrig: (changes: EdgeChange<E>[]) => void
  // hotkey handlers
  save?: () => void
  deleteSelection?: () => void
  group?: () => void
  ungroup?: () => void
  selectAll?: () => void
}

export default function useHistoryHotkeys<N extends RFNode = RFNode, E extends RFEdge = RFEdge>(
  opts: HistoryHotkeysOptions<N, E>
) {
  const {
    rf, setNodes, setEdges,
    isBlocked,
    onNodesChangeOrig, onEdgesChangeOrig,
    save, deleteSelection, group, ungroup,selectAll
  } = opts

  const historyRef = useRef<Array<{ nodes: N[]; edges: E[] }>>([])
  const redoRef = useRef<Array<{ nodes: N[]; edges: E[] }>>([])

  const suspendRef = useRef(false)

  /** Push snapshot from current RF state (only for user-driven commits) */
  const pushFromRF = useCallback(() => {
    if (suspendRef.current) return
    historyRef.current.push({
      nodes: deepClone(rf.getNodes() as N[]),
      edges: deepClone(rf.getEdges() as E[]),
    })
    if (historyRef.current.length > 200) historyRef.current.shift()
    redoRef.current = []
  }, [rf])

  /** Push snapshot from provided state (programmatic commits) */
  const commitSnapshotState = useCallback((nodes: N[], edges: E[]) => {
    if (suspendRef.current) return
    historyRef.current.push({
      nodes: deepClone(nodes),
      edges: deepClone(edges),
    })
    if (historyRef.current.length > 200) historyRef.current.shift()
    redoRef.current = []
  }, [])

  // initial snapshot once
  useEffect(() => {
    pushFromRF()
  }, [pushFromRF])

  const undo = useCallback(() => {
    const h = historyRef.current
    if (h.length <= 1) return

    suspendRef.current = true

    const cur = h.pop()!
    redoRef.current.push(cur)
    const prev = h[h.length - 1]
    setNodes(prev.nodes as any)
    setEdges(prev.edges as any)

    queueMicrotask(() => { suspendRef.current = false })
  }, [setNodes, setEdges])

  const redo = useCallback(() => {
    const r = redoRef.current
    if (!r.length) return

    suspendRef.current = true

    const next = r.pop()!
    historyRef.current.push(next)
    setNodes(next.nodes as any)
    setEdges(next.edges as any)

    queueMicrotask(() => { suspendRef.current = false })
  }, [setNodes, setEdges])

  const onNodesChangeWithHistory = useCallback((changes: NodeChange<N>[]) => {
    onNodesChangeOrig(changes)
    if (suspendRef.current) return
    if (shouldCommitNodes(changes)) {
      setTimeout(pushFromRF, 0)
    }
  }, [onNodesChangeOrig, pushFromRF])

  const onEdgesChangeWithHistory = useCallback((changes: EdgeChange<E>[]) => {
    onEdgesChangeOrig(changes)
    if (suspendRef.current) return
    if (shouldCommitEdges(changes)) {
      setTimeout(pushFromRF, 0)
    }
  }, [onEdgesChangeOrig, pushFromRF])

  // Hotkeys: just call handlers. Do not auto-commit here.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (isBlocked) return
      if (isTextInput(document.activeElement)) return

      const mod = e.ctrlKey || e.metaKey
      const k = e.key.toLowerCase()

      if (e.key === 'Delete') {
        if (deleteSelection) { e.preventDefault(); deleteSelection() }
        return
      }
      if (mod && k === 's') {
        if (save) { e.preventDefault(); save() }
        return
      }
      if (mod && k === 'g' && !e.shiftKey) {
        if (group) { e.preventDefault(); group() }
        return
      }
      if (mod && k === 'g' && e.shiftKey) {
        if (ungroup) { e.preventDefault(); ungroup() }
        return
      }
      if (mod && k === 'z' && !e.shiftKey) { e.preventDefault(); undo(); return }
      if ((mod && k === 'y') || (mod && k === 'z' && e.shiftKey)) { e.preventDefault(); redo(); return }
      if (mod && k == 'a' && !e.shiftKey) {
        if (selectAll) {e.preventDefault(); selectAll()}
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [isBlocked, save, deleteSelection, group, ungroup, undo, redo])

  return {
    onNodesChangeWithHistory,
    onEdgesChangeWithHistory,
    undo,
    redo,
    commitSnapshotState, // <-- KEY for node-data edits
  }
}