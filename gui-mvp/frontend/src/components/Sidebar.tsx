// Sidebar.tsx
import React from 'react'
import { useReactFlow } from '@xyflow/react'
import { useSettings } from '../SettingsContext'
import type { Node } from '@xyflow/react'
import { Button } from 'antd'

type PaletteItem = {
  key: 'csv' | 'filter' | 'summary'
  label: string
  color: string
}

const PALETTE: PaletteItem[] = [
  { key:'csv',    label:'CSV Loader', color:'#14b8a6' },
  { key:'filter', label:'Filter',     color:'#f59e0b' },
  { key:'summary',label:'Summary',    color:'#60a5fa' },
]

export const Sidebar: React.FC<{
  groups: { id: string; name: string; count: number }[]
  onFocusGroup: (id: string) => void
  onExportGroup: (id: string) => void
  onUndo: () => void
  onRedo: () => void
  onSave: () => void
  onAutoLayout: () => void
}> = ({ groups, onFocusGroup, onExportGroup, onUndo, onRedo, onSave, onAutoLayout }) => {
  const rf = useReactFlow()
  const { settings, setSettings } = useSettings()

  // Drag handlers for palette items
  const onDragStart = (ev: React.DragEvent, item: PaletteItem) => {
    ev.dataTransfer.setData('application/reactflow', JSON.stringify(item))
    ev.dataTransfer.effectAllowed = 'move'
  }

  return (
    <aside
      style={{
        width: 280, height: '100%',
        borderRight: '1px solid #e5e7eb',
        background: '#fafafa',
        display: 'flex', flexDirection: 'column',
        gap: 12, padding: 12,
      }}
    >
      {/* Common Nodes */}
      <section>
        <div style={{ fontWeight: 700, marginBottom: 8 }}>Common Nodes</div>
        <div style={{ display:'grid', gridTemplateColumns:'repeat(2, 1fr)', gap: 8 }}>
          {PALETTE.map(p => (
            <div
              key={p.key}
              draggable
              onDragStart={(ev) => onDragStart(ev, p)}
              title="Drag to canvas"
              style={{
                background:'#fff', border:'1px solid #e5e7eb', borderRadius:8,
                padding:'12px 10px', cursor:'grab',
                display:'flex', alignItems:'center', gap:8,
              }}
            >
              <span style={{
                width: 14, height:14, borderRadius:7, background:p.color, display:'inline-block'
              }} />
              <span>{p.label}</span>
            </div>
          ))}
        </div>
      </section>

      {/* Groups Overview */}
      <section>
        <div style={{ fontWeight: 700, marginBottom: 8 }}>Groups Overview</div>
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

      {/* Quick Actions */}
      <section>
        <div style={{ fontWeight: 700, marginBottom: 8 }}>Quick Actions</div>
        <div style={{ display:'flex', gap:8, flexWrap:'wrap' }}>
          <Button onClick={onUndo}>Undo</Button>
          <Button onClick={onRedo}>Redo</Button>
          <Button onClick={onSave}>Save</Button>
          <Button onClick={onAutoLayout}>Auto Layout</Button>
        </div>
      </section>

      {/* Settings */}
      <section>
        <div style={{ fontWeight: 700, marginBottom: 8 }}>Settings</div>
        {/* Theme mode */}
        <div style={{ marginBottom: 8 }}>
          <div style={{ marginBottom:4 }}>Theme</div>
          <div style={{ display:'flex', gap:8 }}>
            {(['light','system','custom'] as const).map(m => (
              <Button
                key={m}
                type={settings.themeMode === m ? 'primary' : 'default'}
                onClick={() => setSettings({ themeMode: m })}
              >
                {m}
              </Button>
            ))}
          </div>
        </div>
        {/* Canvas bg & grid */}
        <div style={{ marginBottom: 8 }}>
          <div style={{ marginBottom:4 }}>Canvas background</div>
          <input
            type="color"
            value={settings.canvasBg}
            onChange={(e)=> setSettings({ canvasBg: e.target.value })}
          />
        </div>
        <div style={{ marginBottom: 8 }}>
          <div style={{ marginBottom:4 }}>Grid</div>
          <div style={{ display:'flex', gap:8 }}>
            {(['dots','lines'] as const).map(v => (
              <Button
                key={v}
                type={settings.gridVariant === v ? 'primary' : 'default'}
                onClick={()=> setSettings({ gridVariant: v })}
              >
                {v}
              </Button>
            ))}
            <input
              type="color"
              value={settings.gridColor}
              onChange={(e)=> setSettings({ gridColor: e.target.value })}
              title="Grid color"
              style={{ marginLeft: 8 }}
            />
          </div>
        </div>
        {/* Node default color presets */}
        <div style={{ marginBottom: 8 }}>
          <div style={{ marginBottom:4 }}>Default node color</div>
          <div style={{ display:'flex', gap:6, flexWrap:'wrap' }}>
            {['#4f46e5','#14b8a6','#f59e0b','#60a5fa','#ef4444','#22c55e'].map(c => (
              <button
                key={c}
                onClick={()=> setSettings({ nodeDefaultColor: c })}
                style={{
                  width:24, height:24, borderRadius:12, border: settings.nodeDefaultColor===c? '2px solid #111':'1px solid #e5e7eb',
                  background: c, cursor:'pointer'
                }}
                title={c}
              />
            ))}
            <input
              type="color"
              value={settings.nodeDefaultColor}
              onChange={(e)=> setSettings({ nodeDefaultColor: e.target.value })}
              title="Custom color"
            />
          </div>
        </div>
        {/* Autosave toggle */}
        <div style={{ marginTop: 8 }}>
          <label style={{ display:'flex', alignItems:'center', gap:8 }}>
            <input
              type="checkbox"
              checked={settings.autoSave}
              onChange={(e)=> setSettings({ autoSave: e.target.checked })}
            />
            Auto save every change
          </label>
        </div>
      </section>
    </aside>
  )
}
