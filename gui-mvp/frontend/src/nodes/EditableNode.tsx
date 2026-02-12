import React, { memo, useEffect, useMemo } from 'react'
import {
  Handle,
  Position,
  NodeResizer,
  useUpdateNodeInternals,
  type NodeProps,
  type Node,
} from '@xyflow/react'
import type { FunctionalData } from '../types'

const MIN_W = 140
const MIN_H = 80
const DEFAULT_TEXT = '#ffffff'

function regularPolygonPoints(sides: number): string {
  const cx = 50, cy = 50, r = 45
  const pts: string[] = []
  for (let i = 0; i < sides; i++) {
    const a = -Math.PI / 2 + (2 * Math.PI * i) / sides
    pts.push(`${cx + r * Math.cos(a)},${cy + r * Math.sin(a)}`)
  }
  return pts.join(' ')
}

function EditableNodeImpl(props: NodeProps<Node<FunctionalData, 'editable'>>) {
  const { id, data, selected } = props

  const label   = data?.label    ?? 'Node'
  const color   = data?.color    ?? '#4f46e5'
  const shape   = data?.shape    ?? 'rect'
  const rotate  = Number.isFinite(data?.rotation) ? (data!.rotation as number) : 0

  const portsIn  = data?.ports?.inputs  ?? []
  const portsOut = data?.ports?.outputs ?? []

  const updateInternals = useUpdateNodeInternals()
  useEffect(() => { updateInternals(id) }, [id, portsIn.length, portsOut.length, updateInternals])

  const trianglePts = useMemo(() => regularPolygonPoints(3), [])
  const pentagonPts = useMemo(() => regularPolygonPoints(5), [])
  const hexagonPts  = useMemo(() => regularPolygonPoints(6), [])

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        userSelect: 'none',
        position: 'relative',
        boxShadow: selected
          ? '0 0 0 3px rgba(79,70,229,.65), 0 10px 18px rgba(0,0,0,.18)'
          : '0 1px 3px rgba(0,0,0,.14)',
        borderRadius: shape === 'round' ? 16 : 8,
      }}
    >
      <NodeResizer
        isVisible={!!selected}
        minWidth={MIN_W}
        minHeight={MIN_H}
        keepAspectRatio={shape === 'circle'}
        handleStyle={{ width: 10, height: 10 }}
        lineStyle={{ borderColor: 'rgba(255,255,255,.75)' }}
      />

      <div
        style={{
          position: 'absolute',
          inset: 0,
          transform: `rotate(${rotate}deg)`,
          transformOrigin: '50% 50%',
          display: 'grid',
          placeItems: 'center',
        }}
      >
        {shape === 'rect' || shape === 'round' ? (
          <div style={{ width: '100%', height: '100%', background: color, borderRadius: shape === 'round' ? 16 : 8 }} />
        ) : shape === 'circle' ? (
          <svg viewBox="0 0 100 100" width="100%" height="100%"><circle cx="50" cy="50" r="45" fill={color} /></svg>
        ) : shape === 'triangle' ? (
          <svg viewBox="0 0 100 100" width="100%" height="100%"><polygon points={trianglePts} fill={color} /></svg>
        ) : shape === 'pentagon' ? (
          <svg viewBox="0 0 100 100" width="100%" height="100%"><polygon points={pentagonPts} fill={color} /></svg>
        ) : (
          <svg viewBox="0 0 100 100" width="100%" height="100%"><polygon points={hexagonPts} fill={color} /></svg>
        )}

        <div
          style={{
            position: 'absolute', inset: 0, display: 'grid', placeItems: 'center',
            color: DEFAULT_TEXT, fontWeight: 700, letterSpacing: 0.2, fontSize: 14, textShadow: '0 1px 1px rgba(0,0,0,.2)',
            pointerEvents: 'none',
          }}
        >
          {label}
        </div>
      </div>

      {portsIn.map((p, i) => (
        <Handle
          key={p.id || `in-${i}`}
          id={p.id || `in-${i}`}
          type="target"
          position={Position.Left}
          style={{ top: `${((i + 1) * 100) / (portsIn.length + 1)}%`, transform: 'translateY(-50%)', background: '#fff' }}
        />
      ))}

      {portsOut.map((p, i) => (
        <Handle
          key={p.id || `out-${i}`}
          id={p.id || `out-${i}`}
          type="source"
          position={Position.Right}
          style={{ top: `${((i + 1) * 100) / (portsOut.length + 1)}%`, transform: 'translateY(-50%)', background: '#fff' }}
        />
      ))}
    </div>
  )
}

export default memo(EditableNodeImpl)
