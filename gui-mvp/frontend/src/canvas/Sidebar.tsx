// src/canvas/Sidebar.tsx
// Scrollable + collapsible + searchable palette loaded from file. Comments in English.

import React, { useMemo, useState } from 'react'
import { Button, Divider } from 'antd'

export type PaletteCategory = 'Shell' | 'Config' | 'General'
export type PaletteKey = string
export type PalettePortDef = { name: string; dtype?: string }
export type PaletteTemplate =
  | { kind: 'command'; command: string; args?: string[]; cwd?: string; ports?: { inputs?: PalettePortDef[]; outputs?: PalettePortDef[] } }
  | { kind: 'code'; language?: 'python'|'javascript'|'bash'|'lua'|'go'; source?: string; ports?: { inputs?: PalettePortDef[]; outputs?: PalettePortDef[] } }
export type PaletteItem = { key: PaletteKey; label: string; color: string; category: PaletteCategory; description?: string; template: PaletteTemplate }

type Props = {
  palette: PaletteItem[]
  groups: { id: string; name: string; count: number }[]
  onFocusGroup: (id: string) => void
  onExportGroup: (id: string) => void
  onUndo: () => void
  onRedo: () => void
  onSave: () => void
  onAutoLayout: () => void
  onOpenSettings: () => void
  onCreateFromPalette: (key: PaletteKey) => void
}

export default function Sidebar({
  palette, groups, onFocusGroup, onExportGroup, onUndo, onRedo, onSave, onAutoLayout, onOpenSettings, onCreateFromPalette,
}: Props) {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState<Record<PaletteCategory, boolean>>({ Shell: true, Config: true, General: true })

  const categories = useMemo(() => {
    const set = new Set<PaletteCategory>()
    palette.forEach(p => set.add(p.category))
    const order: PaletteCategory[] = ['Shell','Config','General']
    return order.filter(o => set.has(o)).concat(Array.from(set).filter(s => !order.includes(s)))
  }, [palette])

  const filteredBy = (cat: PaletteCategory) =>
    palette.filter(p =>
      p.category === cat &&
      (query.trim().length === 0 || p.label.toLowerCase().includes(query.trim().toLowerCase()))
    )

  const onDragStart = (ev: React.DragEvent, item: PaletteItem) => {
    ev.dataTransfer.setData('application/reactflow', JSON.stringify(item))
    ev.dataTransfer.effectAllowed = 'move'
  }

  return (
    <aside
      style={{
        width: 300, height: '100%', minHeight: 0,
        borderRight: '1px solid #e5e7eb', background: '#fafafa',
        display: 'flex', flexDirection: 'column',
      }}
    >
      {/* header */}
      <div style={{ padding: 12, display:'flex', alignItems:'center', justifyContent:'space-between' }}>
        <div style={{ fontWeight: 800 }}>Tools</div>
        <Button size="small" onClick={onOpenSettings}>Settings</Button>
      </div>
      <Divider style={{ margin:'0 0 8px 0' }} />

      {/* search */}
      <div style={{ padding: '0 12px 8px 12px' }}>
        <input
          placeholder="Search common modules..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          style={{ width:'100%', height:32, padding:'4px 8px', border:'1px solid #e5e7eb', borderRadius:6, background:'#fff' }}
        />
      </div>

      {/* scroll area */}
      <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '0 12px 12px 12px' }}>
        {categories.map(cat => {
          const items = filteredBy(cat)
          const isOpen = !!open[cat]
          return (
            <section key={cat} style={{ marginBottom: 12 }}>
              <button
                onClick={() => setOpen(prev => ({ ...prev, [cat]: !prev[cat] }))}
                style={{ width:'100%', textAlign:'left', background:'transparent', border:'none', cursor:'pointer', padding:'6px 4px', display:'flex', alignItems:'center', gap:8 }}
                title={isOpen ? 'Collapse' : 'Expand'}
              >
                <span style={{ display:'inline-block', transform: isOpen ? 'rotate(90deg)' : 'rotate(0deg)', transition:'transform 120ms ease', color:'#6b7280' }}>▸</span>
                <span style={{ fontWeight: 700 }}>{cat}</span>
              </button>

              {isOpen && (
                <ul style={{ listStyle:'none', margin: 8, marginTop: 4, padding:0, display:'flex', flexDirection:'column', gap:8 }}>
                  {items.length === 0 && <li style={{ color:'#6b7280' }}>No matches</li>}
                  {items.map(p => (
                    <li key={p.key}>
                      <div
                        draggable
                        onDragStart={(ev) => onDragStart(ev, p)}
                        onDoubleClick={() => onCreateFromPalette(p.key)}
                        title="Drag to canvas or double-click to create"
                        style={{ background:'#fff', border:'1px solid #e5e7eb', borderRadius:8, padding:'10px 10px', cursor:'grab', display:'flex', alignItems:'flex-start', gap:10 }}
                      >
                        <span style={{ width: 14, height:14, borderRadius:7, background:p.color, display:'inline-block', marginTop:3 }} />
                        <div style={{ flex:1 }}>
                          <div style={{ fontWeight:600 }}>{p.label}</div>
                          {p.description && <div style={{ color:'#6b7280', fontSize:12 }}>{p.description}</div>}
                        </div>
                        <Button size="small" type="link" onClick={() => onCreateFromPalette(p.key)}>Add</Button>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          )
        })}

        {/* groups */}
        <section>
          <div style={{ fontWeight: 700, margin:'8px 0 8px' }}>Groups</div>
          <div style={{ display:'flex', flexDirection:'column', gap:6 }}>
            {groups.length === 0 && <div style={{ color:'#6b7280' }}>No groups</div>}
            {groups.map(g => (
              <div key={g.id} style={{ display:'flex', alignItems:'center', justifyContent:'space-between' }}>
                <div>
                  <strong>{g.name || g.id}</strong>
                  <span style={{ marginLeft:6, color:'#6b7280' }}>({g.count})</span>
                </div>
                <div style={{ display:'flex', gap:8 }}>
                  <Button size="small" onClick={() => onFocusGroup(g.id)}>Focus</Button>
                  <Button size="small" onClick={() => onExportGroup(g.id)}>Export</Button>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* quick actions */}
        <section>
          <div style={{ fontWeight: 700, margin:'12px 0 8px' }}>Quick Actions</div>
          <div style={{ display:'flex', gap:8, flexWrap:'wrap' }}>
            <Button onClick={onUndo}>Undo</Button>
            <Button onClick={onRedo}>Redo</Button>
            <Button onClick={onSave}>Save</Button>
            <Button onClick={onAutoLayout}>Auto Layout</Button>
          </div>
        </section>
      </div>
    </aside>
  )
}
