// src/canvas/hooks/useGroupActions.ts
// Group/Ungroup actions for React Flow (v12 uses parentId). Comments in English.

import { useCallback } from 'react'
import type { ReactFlowInstance, Node as RFNode } from '@xyflow/react'

export type GroupActionsOptions<N extends RFNode = RFNode> = {
  rf: ReactFlowInstance<N, any>
  selectedNodeIds: string[]
  setNodes: (updater: (prev: N[]) => N[] | N[]) => void
  defaultNodeSize: { width: number; height: number }
}

/** Provide groupSelection / ungroupSelection actions */
export default function useGroupActions<N extends RFNode = RFNode>(opts: GroupActionsOptions<N>) {
  const { rf, selectedNodeIds, setNodes, defaultNodeSize } = opts

  const groupSelection = useCallback(() => {
    const all = rf.getNodes() as any[]
    const sel = all.filter(n => selectedNodeIds.includes(n.id) && n.type !== 'group')
    if (!sel.length) return

    const sized = sel.map((n) => {
      const w = Number((n.style as any)?.width || defaultNodeSize.width)
      const h = Number((n.style as any)?.height || defaultNodeSize.height)
      return { n, x: n.position.x, y: n.position.y, w, h }
    })
    const minX = Math.min(...sized.map(s => s.x)), minY = Math.min(...sized.map(s => s.y))
    const maxX = Math.max(...sized.map(s => s.x + s.w)), maxY = Math.max(...sized.map(s => s.y + s.h))
    const pad = 24
    const gx = minX - pad, gy = minY - pad
    const gw = (maxX - minX) + pad * 2, gh = (maxY - minY) + pad * 2
    const gid = `G_${Date.now()}_${Math.random().toString(36).slice(2,6)}`

    const groupNode = {
      id: gid, type: 'group', position: { x: gx, y: gy }, style: { width: gw, height: gh }, data: { label: 'group' }
    }
    const children = sel.map((n: any) => ({
      ...n, position: { x: n.position.x - gx, y: n.position.y - gy }, parentId: gid, extent: 'parent' as const
    }))

    setNodes((prev: any[]) => {
      const rest = prev.filter(p => !selectedNodeIds.includes(p.id))
      return [...rest, groupNode, ...children]
    })
  }, [rf, selectedNodeIds, setNodes, defaultNodeSize.width, defaultNodeSize.height])

  const ungroupSelection = useCallback(() => {
    const all = rf.getNodes() as any[]
    const groups = all.filter(n => selectedNodeIds.includes(n.id) && n.type === 'group')
    if (!groups.length) return
    const g = groups[0]!
    const gx = g.position.x, gy = g.position.y
    setNodes((prev: any[]) => {
      const children = prev.filter((n: any) => n.parentId === g.id)
      const moved = children.map((c: any) => ({
        ...c, position: { x: c.position.x + gx, y: c.position.y + gy }, parentId: undefined, extent: undefined,
      }))
      const rest = prev.filter((n: any) => n.id !== g.id && n.parentId !== g.id)
      return [...rest, ...moved]
    })
  }, [rf, selectedNodeIds, setNodes])

  return { groupSelection, ungroupSelection }
}
