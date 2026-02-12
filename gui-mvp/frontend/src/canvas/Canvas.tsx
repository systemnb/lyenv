// src/canvas/Canvas.tsx
// Canvas updated for stdio-flow export with Start/End control nodes (Option A).
// - Export uses exporterStdio (stdio multi-step, data via plugin config mutations).
// - Start node: outputs map CLI args -> flow.outputs.start.<port>
// - End node: reads its inputs and respond_ok(final message)
// - Execution order: Start -> ... -> End linear path
// - Hotkeys: Ctrl+S save, Ctrl+A select all (does not affect undo history)
// - Disable Space-to-pan while modal open to allow Monaco to type spaces.
//
// All comments in English.

import React, {
  useCallback, useEffect, useMemo, useRef, useState, forwardRef, useImperativeHandle,
} from 'react'
import {
  ReactFlow,
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  useReactFlow,
  useOnSelectionChange,
  ReactFlowProvider,
  type OnConnect,
  type NodeTypes,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { message, Drawer, Divider, Switch, Select } from 'antd'
import JSZip from 'jszip'
import yaml from 'js-yaml'

import EditableNode from '../nodes/EditableNode'
import ContextMenu, { type MenuNode } from '../components/ContextMenu'
import NodeEditor from '../components/NodeEditor'
import NodeDataEditor from '../components/NodeDataEditor'

import Sidebar, { type PaletteItem, type PaletteKey } from './Sidebar'
import type { RFNode, RFEdge, RFNodeData } from './graph'
import useGroupActions from './hooks/useGroupActions'
import useHistoryHotkeys from './hooks/useHistoryHotkeys'
import { buildManifestAndFilesStdio } from './exporterStdio'

export type CanvasHandle = {
  addNodeAtCenter: () => void
  autoLayout: () => void
  saveToLocal: () => void
  loadFromLocal: () => void
  deleteSelection: () => void
  fitView: () => void
  importFlowByPicker: () => void
  exportFlowAsJSON: () => void
  importLyenvByPicker: () => void
  exportLyenvAsZip: () => void
  getGraph: () => {nodes: any[], edges: any[]}
}

type RFNodeType = RFNode
type RFEdgeType = RFEdge

const LOCAL_KEY = 'rf-mvp-flow'
const AUTOSAVE_KEY = 'rf-mvp-flow-autosave'
const DEFAULT_NODE_STYLE: React.CSSProperties = { width: 220, height: 120 }

type GridVariant = 'dots' | 'lines'
type Settings = { canvasBg: string; gridVariant: GridVariant; gridColor: string; nodeDefaultColor: string; autoSave: boolean }

const SPECIALS: PaletteItem[] = [
  {
    key: '__start__',
    label: 'Start',
    color: '#111827',
    category: 'General',
    description: 'Special: CLI args → outputs (input manager)',
    template: { kind: 'code', language: 'python', source: '', ports: { inputs: [], outputs: [{ name: 'arg1', dtype: 'text' }] } }
  },
  {
    key: '__end__',
    label: 'End',
    color: '#111827',
    category: 'General',
    description: 'Special: inputs → final output (output manager)',
    template: { kind: 'code', language: 'python', source: '', ports: { inputs: [{ name: 'result', dtype: 'text' }], outputs: [] } }
  }
]

const Canvas = forwardRef<CanvasHandle>(function Canvas(_, ref) {
  return (
    <ReactFlowProvider>
      <CanvasInner ref={ref} />
    </ReactFlowProvider>
  )
})
export default Canvas

const CanvasInner = forwardRef<CanvasHandle>(function CanvasInner(_, ref) {
  const [nodes, setNodes, onNodesChange] = useNodesState<RFNodeType>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdgeType>([])
  const rf = useReactFlow<RFNodeType, RFEdgeType>()

  // refs for deterministic nextState commits
  const nodesRef = useRef<RFNodeType[]>([])
  const edgesRef = useRef<RFEdgeType[]>([])
  useEffect(() => { nodesRef.current = nodes }, [nodes])
  useEffect(() => { edgesRef.current = edges }, [edges])

  // selection
  const [selectedNodeIds, setSelectedNodeIds] = useState<string[]>([])
  const [selectedEdgeIds, setSelectedEdgeIds] = useState<string[]>([])
  useOnSelectionChange({
    onChange: useCallback(({ nodes, edges }) => {
      setSelectedNodeIds(nodes.map((n) => n.id))
      setSelectedEdgeIds(edges.map((e) => e.id))
    }, []),
  })

  // editors
  const [editorOpen, setEditorOpen] = useState(false)
  const [editorNodeId, setEditorNodeId] = useState<string | undefined>(undefined)
  const [dataEditorOpen, setDataEditorOpen] = useState(false)
  const [dataEditorNodeId, setDataEditorNodeId] = useState<string | undefined>(undefined)
  const isModalOpen = editorOpen || dataEditorOpen

  const editorInitial: RFNodeData | undefined = useMemo(() => {
    if (!editorNodeId) return undefined
    const n = rf.getNodes().find(n => n.id === editorNodeId)
    return n?.data as RFNodeData | undefined
  }, [editorNodeId, rf])
  const dataEditorInitial: RFNodeData | undefined = useMemo(() => {
    if (!dataEditorNodeId) return undefined
    const n = rf.getNodes().find(n => n.id === dataEditorNodeId)
    return n?.data as RFNodeData | undefined
  }, [dataEditorNodeId, rf])

  // file pickers
  const fileJsonRef = useRef<HTMLInputElement | null>(null)
  const fileZipRef  = useRef<HTMLInputElement | null>(null)

  // settings
  const [settings, setSettings] = useState<Settings>({
    canvasBg: '#ffffff',
    gridVariant: 'dots',
    gridColor: '#dddddd',
    nodeDefaultColor: '#4f46e5',
    autoSave: true,
  })
  const [settingsOpen, setSettingsOpen] = useState(false)

  // palette
  const [palette, setPalette] = useState<PaletteItem[]>([])
  useEffect(() => {
    let mounted = true
    ;(async () => {
      try {
        const resp = await fetch('/common-modules.json', { cache: 'no-store' })
        if (!resp.ok) throw new Error()
        const arr = await resp.json()
        if (mounted) setPalette(Array.isArray(arr) ? arr : [])
      } catch {
        setPalette([])
      }
    })()
    return () => { mounted = false }
  }, [])

  // pointer coords
  const pointerFlowPosRef = useRef<{ x: number; y: number } | null>(null)
  const contextMenuFlowPosRef = useRef<{ x: number; y: number } | null>(null)

  const screenToFlow = useCallback((clientX: number, clientY: number) => {
    const anyRf = rf as any
    if (typeof anyRf.screenToFlowPosition === 'function') return anyRf.screenToFlowPosition({ x: clientX, y: clientY })
    if (typeof anyRf.project === 'function') return anyRf.project({ x: clientX, y: clientY })
    return { x: clientX, y: clientY }
  }, [rf])

  /** group actions */
  const { groupSelection, ungroupSelection } = useGroupActions<RFNodeType>({
    rf,
    selectedNodeIds,
    setNodes,
    defaultNodeSize: { width: Number(DEFAULT_NODE_STYLE.width), height: Number(DEFAULT_NODE_STYLE.height) }
  })

  /** history + hotkeys (your stable version) */
  const { onNodesChangeWithHistory, onEdgesChangeWithHistory, undo, redo, commitSnapshotState } =
    useHistoryHotkeys<RFNodeType, RFEdgeType>({
      rf,
      setNodes,
      setEdges,
      isBlocked: isModalOpen,
      onNodesChangeOrig: onNodesChange as any,
      onEdgesChangeOrig: onEdgesChange as any,
      save: () => saveToLocal(),
      deleteSelection: () => deleteSelection(),
      // group/ungroup handled in Canvas to avoid circular refs
      group: () => {
        groupSelection(); requestAnimationFrame(() => commitSnapshotState(rf.getNodes() as any, rf.getEdges() as any))
      },
      ungroup: () => {
        ungroupSelection(); requestAnimationFrame(() => commitSnapshotState(rf.getNodes() as any, rf.getEdges() as any))
      },
      selectAll: () => {
        if (isModalOpen) return
        setNodes((prev) => prev.map(n => ({ ...n, selected: true })))
        setEdges((prev) => prev.map(ed => ({ ...ed, selected: true })))
      },
    })


  /** Create special Start/End nodes */
  const createSpecialNode = useCallback((key: string, pos: { x: number; y: number }) => {
    if (key === '__start__') {
      const id = `START_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`
      const node: RFNodeType = {
        id,
        type: 'editable',
        position: pos,
        style: { width: 180, height: 90 },
        data: {
          label: 'Start',
          color: '#111827',
          shape: 'rect',
          rotation: 0,
          kind: 'start',
          ports: { inputs: [], outputs: [{ id: `out-${id}-0`, name: 'arg1', dtype: 'text' }] },
          config: {},
        } as any,
      }
      const nextNodes = [...nodesRef.current, node]
      setNodes(nextNodes)
      commitSnapshotState(nextNodes as any, edgesRef.current as any)
      return true
    }
    if (key === '__end__') {
      const id = `END_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`
      const node: RFNodeType = {
        id,
        type: 'editable',
        position: pos,
        style: { width: 180, height: 90 },
        data: {
          label: 'End',
          color: '#111827',
          shape: 'rect',
          rotation: 0,
          kind: 'end',
          ports: { inputs: [{ id: `in-${id}-0`, name: 'result', dtype: 'text' }], outputs: [] },
          config: {},
        } as any,
      }
      const nextNodes = [...nodesRef.current, node]
      setNodes(nextNodes)
      commitSnapshotState(nextNodes as any, edgesRef.current as any)
      return true
    }
    return false
  }, [setNodes, commitSnapshotState])

  /** Create node from palette template (normal nodes) */
  const createNodeFromTemplate = useCallback((item: PaletteItem, pos: { x: number; y: number }) => {
    // Special nodes
    if (createSpecialNode(item.key, pos)) return

    const id = `N_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`
    const toPorts = (list: { name: string; dtype?: string }[] | undefined, prefix: 'in'|'out') =>
      (list || []).map((p, i) => ({ id: `${prefix}-${id}-${i}`, name: p.name || `${prefix}${i}`, dtype: p.dtype || 'any' }))

    let data: RFNodeData
    if (item.template.kind === 'command') {
      data = {
        label: item.label,
        color: item.color,
        shape: 'rect',
        rotation: 0,
        kind: 'command',
        ports: {
          inputs: toPorts(item.template.ports?.inputs, 'in'),
          outputs: toPorts(item.template.ports?.outputs, 'out'),
        },
        config: { command: item.template.command, args: item.template.args || [], cwd: item.template.cwd || '.' } as any,
      }
    } else {
      data = {
        label: item.label,
        color: item.color,
        shape: 'rect',
        rotation: 0,
        kind: 'code',
        ports: {
          inputs: toPorts(item.template.ports?.inputs, 'in'),
          outputs: toPorts(item.template.ports?.outputs, 'out'),
        },
        config: { language: item.template.language || 'python', source: item.template.source || '' },
      }
    }

    const node: RFNodeType = { id, type: 'editable', data, style: DEFAULT_NODE_STYLE, position: pos }
    const nextNodes = [...nodesRef.current, node]
    setNodes(nextNodes)
    commitSnapshotState(nextNodes as any, edgesRef.current as any)
  }, [createSpecialNode, setNodes, commitSnapshotState])

  /** connect */
  const onConnect: OnConnect = useCallback(
    (params) => setEdges((eds: RFEdgeType[]) => addEdge({ ...params, animated: true } as any, eds) as RFEdgeType[]),
    [setEdges]
  )
  const validateConnection = useCallback(() => true, [])

  /** delete selection */
  const deleteSelection = useCallback(() => {
    if (!selectedNodeIds.length && !selectedEdgeIds.length) return
    const nextNodes = nodesRef.current.filter(n => !selectedNodeIds.includes(n.id))
    const nextEdges = edgesRef.current.filter(e => !selectedEdgeIds.includes(e.id))
    setNodes(nextNodes)
    setEdges(nextEdges)
    commitSnapshotState(nextNodes as any, nextEdges as any)
  }, [selectedNodeIds, selectedEdgeIds, setNodes, setEdges, commitSnapshotState])

  /** autosave */
  useEffect(() => {
    if (!settings.autoSave) return
    const h = setTimeout(() => {
      const payload = JSON.stringify({ nodes: rf.getNodes(), edges: rf.getEdges() })
      localStorage.setItem(AUTOSAVE_KEY, payload)
    }, 150)
    return () => clearTimeout(h)
  }, [nodes, edges, rf, settings.autoSave])

  /** auto-load */
  useEffect(() => {
    const raw = localStorage.getItem(AUTOSAVE_KEY) || localStorage.getItem(LOCAL_KEY)
    if (!raw) return
    try {
      const parsed = JSON.parse(raw) as { nodes: RFNodeType[]; edges: RFEdgeType[] }
      const fixed = (parsed.nodes || []).map(n => {
        const hasWH = !!(n.style && (n.style as any).width && (n.style as any).height)
        return hasWH ? n : { ...n, style: { ...(n.style || {}), ...DEFAULT_NODE_STYLE } }
      })
      setNodes(fixed as RFNodeType[])
      setEdges((parsed.edges || []) as RFEdgeType[])
      requestAnimationFrame(() => rf.fitView({ padding: 0.2 }))
    } catch { /* ignore */ }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  /** basic ops */
  const autoLayout = useCallback(() => {
    const spacingX = 240, spacingY = 150
    const cols = Math.max(1, Math.round(Math.sqrt(nodesRef.current.length || 1)))
    const nextNodes = nodesRef.current.map((n, i) => {
      if (n.type === 'group') return n
      const col = i % cols, row = Math.floor(i / cols)
      return { ...n, position: { x: 80 + col * spacingX, y: 80 + row * spacingY } }
    })
    setNodes(nextNodes)
    commitSnapshotState(nextNodes as any, edgesRef.current as any)
    requestAnimationFrame(() => rf.fitView({ padding: 0.2 }))
  }, [setNodes, commitSnapshotState, rf])

  const saveToLocal = useCallback(() => {
    const payload = JSON.stringify({ nodes: rf.getNodes(), edges: rf.getEdges() })
    localStorage.setItem(LOCAL_KEY, payload)
    message.success('Saved')
  }, [rf])

  const loadFromLocal = useCallback(() => {
    const raw = localStorage.getItem(LOCAL_KEY)
    if (!raw) return
    try {
      const parsed = JSON.parse(raw) as { nodes: RFNodeType[]; edges: RFEdgeType[] }
      setNodes(parsed.nodes as RFNodeType[])
      setEdges(parsed.edges as RFEdgeType[])
      commitSnapshotState(parsed.nodes as any, parsed.edges as any)
      requestAnimationFrame(() => rf.fitView({ padding: 0.2 }))
    } catch { message.error('Load failed') }
  }, [setNodes, setEdges, commitSnapshotState, rf])

  const fitView = useCallback(() => rf.fitView({ padding: 0.1 }), [rf])

  /** export/import flow json */
  const exportFlowAsJSON = useCallback(() => {
    const json = { nodes: rf.getNodes(), edges: rf.getEdges() }
    const blob = new Blob([JSON.stringify(json, null, 2)], { type: 'application/json' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob); a.download = 'flow.json'; a.click()
    URL.revokeObjectURL(a.href)
  }, [rf])

  const importFlowFromFile = useCallback(async (file: File) => {
    try {
      const text = await file.text()
      const parsed = JSON.parse(text) as { nodes: RFNodeType[]; edges: RFEdgeType[] }
      setNodes(parsed.nodes as RFNodeType[])
      setEdges(parsed.edges as RFEdgeType[])
      commitSnapshotState(parsed.nodes as any, parsed.edges as any)
      requestAnimationFrame(() => rf.fitView({ padding: 0.2 }))
      message.success(`Imported ${file.name}`)
    } catch (err) { console.error(err); message.error('Import failed') }
  }, [setNodes, setEdges, commitSnapshotState, rf])

  /** Export LyEnv zip (stdio flow) */
  const exportLyenvAsZip = useCallback(async () => {
    try {
      const pluginName = prompt('LyEnv plugin name:', 'myplugin') || 'myplugin'
      const shim = prompt('Shim alias to expose:', 'myplugin') || 'myplugin'
      const { manifest, files } = await buildManifestAndFilesStdio(
        rf.getNodes() as RFNodeType[],
        rf.getEdges() as RFEdgeType[],
        pluginName,
        shim
      )
      const zip = new JSZip()
      zip.file('manifest.yaml', yaml.dump(manifest))
      for (const f of files) zip.file(f.path, f.content)
      const blob = await zip.generateAsync({ type: 'blob' })
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob); a.download = `${manifest.name}.lyenv.zip`; a.click()
      URL.revokeObjectURL(a.href)
      message.success('Exported LyEnv (stdio flow)')
    } catch (err: any) {
      console.error(err)
      message.error(err?.message || 'Export failed')
    }
  }, [rf])

  /** Context menu */
  const [menu, setMenu] = useState<{ visible: boolean; x: number; y: number; type: 'pane' | 'node'; nodeId?: string }>({
    visible: false, x: 0, y: 0, type: 'pane',
  })

  const paneMenu: MenuNode[] = useMemo(() => ([
    { key: 'add-here', label: 'Add node here', action: () => {
      const pos = contextMenuFlowPosRef.current ?? pointerFlowPosRef.current ?? screenToFlow(window.innerWidth/2, window.innerHeight/2)
      // default add a generic node
      createNodeFromTemplate({ ...SPECIALS[0], key:'__generic__', label:'Node', color: settings.nodeDefaultColor,
        template: { kind:'code', language:'python', source:'', ports:{ inputs:[{name:'in'}], outputs:[{name:'out'}] } } } as any, pos)
    }},
    { key: 'group', label: 'Group (Ctrl/Cmd+G)', action: () => { groupSelection(); requestAnimationFrame(() => commitSnapshotState(rf.getNodes() as any, rf.getEdges() as any)) } },
    { key: 'ungroup', label: 'Ungroup (Ctrl/Cmd+Shift+G)', action: () => { ungroupSelection(); requestAnimationFrame(() => commitSnapshotState(rf.getNodes() as any, rf.getEdges() as any)) } },
    { key: 'export-lyenv', label: 'Export LyEnv (.zip)', action: () => exportLyenvAsZip() },
    { key: 'export-flow', label: 'Export flow (.json)', action: () => exportFlowAsJSON() },
    { key: 'import-flow', label: 'Import flow (.json)…', action: () => fileJsonRef.current?.click() },
  ]), [createNodeFromTemplate, settings.nodeDefaultColor, groupSelection, ungroupSelection, commitSnapshotState, rf, exportLyenvAsZip, exportFlowAsJSON])

  const nodeMenu: MenuNode[] = useMemo(() => ([
    { key:'edit', label:'Edit…', action: () => { setEditorNodeId(menu.nodeId!); setEditorOpen(true) } },
    { key:'edit-data', label:'Edit Data…', action: () => { setDataEditorNodeId(menu.nodeId!); setDataEditorOpen(true) } },
    { key:'delete', label:'Delete', action: () => deleteSelection() },
  ]), [menu.nodeId, deleteSelection])

  // Group node UI
  const GroupNode = useCallback(({ data, selected }: any) => {
    const title = data?.label || 'Group'
    return (
      <div
        style={{
          width: '100%', height: '100%',
          background: 'rgba(30,30,30,0.04)',
          border: selected ? '2px solid #7dd3fc' : '1px dashed #cbd5e1',
          borderRadius: 8,
        }}
        title={title}
      />
    )
  }, [])

  const nodeTypes: NodeTypes = useMemo(() => ({
    editable: EditableNode,
    group: GroupNode as any,
  }), [GroupNode])

  // Fix: Monaco space issue by disabling Space pan while modal open
  const panActivation = useMemo(() => (isModalOpen ? null : 'Space'), [isModalOpen])

  // Expose methods
  useImperativeHandle(ref, () => ({
    addNodeAtCenter: () => {
      const pos = pointerFlowPosRef.current ?? screenToFlow(window.innerWidth/2, window.innerHeight/2)
      // create a generic node
      createNodeFromTemplate({ ...SPECIALS[0], key:'__generic__', label:'Node', color: settings.nodeDefaultColor,
        template: { kind:'code', language:'python', source:'', ports:{ inputs:[{name:'in'}], outputs:[{name:'out'}] } } } as any, pos)
    },
    autoLayout,
    saveToLocal,
    loadFromLocal,
    deleteSelection,
    fitView,
    importFlowByPicker: () => fileJsonRef.current?.click(),
    exportFlowAsJSON,
    importLyenvByPicker: () => fileZipRef.current?.click(),
    exportLyenvAsZip,
    getGraph: () => ({
      nodes: rf.getNodes(),
      edges: rf.getEdges(),
    }),
  }))

  return (
    <div style={{ height: '100%', width: '100%', display:'flex', flex: 1, minHeight: 0 }}>
      <Sidebar
        palette={[...SPECIALS, ...palette]}
        groups={(rf.getNodes() as RFNodeType[])
          .filter(n=>n.type==='group')
          .map(g=>({ id:g.id, name:(g.data as any)?.label || '', count:(rf.getNodes() as RFNodeType[]).filter(n => (n as any).parentId === g.id).length }))}
        onFocusGroup={() => {}}
        onExportGroup={() => {}}
        onUndo={undo}
        onRedo={redo}
        onSave={saveToLocal}
        onAutoLayout={autoLayout}
        onOpenSettings={() => setSettingsOpen(true)}
        onCreateFromPalette={(key: PaletteKey) => {
          const item = [...SPECIALS, ...palette].find(p => p.key === key)
          if (!item) return
          const pos = pointerFlowPosRef.current ?? screenToFlow(window.innerWidth/2, window.innerHeight/2)
          createNodeFromTemplate(item, pos)
        }}
      />

      <div
        style={{ flex:1, minWidth:0, height:'100%', background: settings.canvasBg }}
        onMouseMove={(ev) => {
          const pane = document.querySelector('.react-flow__pane') as HTMLElement | null
          const rect = pane?.getBoundingClientRect()
          if (rect && ev.clientX >= rect.left && ev.clientX <= rect.right && ev.clientY >= rect.top && ev.clientY <= rect.bottom) {
            pointerFlowPosRef.current = screenToFlow(ev.clientX, ev.clientY)
          } else {
            pointerFlowPosRef.current = null
          }
        }}
        onDragOver={(ev) => { ev.preventDefault(); ev.dataTransfer.dropEffect = 'move' }}
        onDrop={(ev) => {
          ev.preventDefault()
          try {
            const raw = ev.dataTransfer.getData('application/reactflow')
            if (!raw) return
            const item = JSON.parse(raw) as PaletteItem
            createNodeFromTemplate(item, screenToFlow(ev.clientX, ev.clientY))
          } catch {}
        }}
      >
        <input
          ref={fileJsonRef}
          type="file"
          accept=".json,application/json"
          style={{ display:'none' }}
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) importFlowFromFile(f)
            if (fileJsonRef.current) fileJsonRef.current.value = ''
          }}
        />
        <input
          ref={fileZipRef}
          type="file"
          accept=".zip,application/zip,application/x-zip-compressed"
          style={{ display:'none' }}
          onChange={() => {}}
        />

        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          onNodesChange={onNodesChangeWithHistory}
          onEdgesChange={onEdgesChangeWithHistory}
          onConnect={onConnect}
          isValidConnection={validateConnection}
          // Space fix:
          panActivationKeyCode={panActivation as any}
          disableKeyboardA11y={isModalOpen}
          onPaneClick={(event) => {
            const e = event as unknown as MouseEvent
            if ((e as any).detail === 2) {
              const pos = screenToFlow(e.clientX, e.clientY)
              createNodeFromTemplate({ ...SPECIALS[0], key:'__generic__', label:'Node', color: settings.nodeDefaultColor,
                template: { kind:'code', language:'python', source:'', ports:{ inputs:[{name:'in'}], outputs:[{name:'out'}] } } } as any, pos)
            }
          }}
          onPaneContextMenu={(event) => {
            event.preventDefault()
            const e = event as unknown as MouseEvent
            contextMenuFlowPosRef.current = screenToFlow(e.clientX, e.clientY)
            setMenu({ visible: true, x: e.clientX, y: e.clientY, type: 'pane' })
          }}
          onNodeContextMenu={(event, node) => {
            event.preventDefault()
            const e = event as unknown as MouseEvent
            pointerFlowPosRef.current = screenToFlow(e.clientX, e.clientY)
            setMenu({ visible: true, x: e.clientX, y: e.clientY, type: 'node', nodeId: node?.id })
          }}
          fitView
        >
          <MiniMap />
          <Controls />
          <Background
            variant={settings.gridVariant === 'dots' ? BackgroundVariant.Dots : BackgroundVariant.Lines}
            gap={16}
            size={1}
            color={settings.gridColor}
          />
        </ReactFlow>

        <ContextMenu
          visible={menu.visible}
          x={menu.x}
          y={menu.y}
          items={menu.type === 'pane' ? paneMenu : nodeMenu}
          onClose={() => {
            setMenu(m => ({ ...m, visible:false }))
            contextMenuFlowPosRef.current = null
          }}
        />

        <NodeEditor
          open={editorOpen}
          initial={editorInitial}
          onCancel={() => { setEditorOpen(false); setEditorNodeId(undefined) }}
          onApply={(patch) => {
            if (!editorNodeId) return
            const nextNodes = nodesRef.current.map(n => n.id === editorNodeId ? ({ ...n, data: { ...(n.data as any), ...(patch as any) } }) : n)
            setNodes(nextNodes)
            commitSnapshotState(nextNodes as any, edgesRef.current as any)
            setEditorOpen(false)
            setEditorNodeId(undefined)
          }}
        />
        <NodeDataEditor
          open={dataEditorOpen}
          initial={dataEditorInitial}
          onCancel={() => { setDataEditorOpen(false); setDataEditorNodeId(undefined) }}
          onApply={(patch) => {
            if (!dataEditorNodeId) return
            const nextNodes = nodesRef.current.map(n => n.id === dataEditorNodeId ? ({ ...n, data: { ...(n.data as any), ...(patch as any) } }) : n)
            setNodes(nextNodes)
            commitSnapshotState(nextNodes as any, edgesRef.current as any)
            setDataEditorOpen(false)
            setDataEditorNodeId(undefined)
          }}
        />
      </div>

      <Drawer title="Canvas Settings" placement="right" width={320} open={settingsOpen} onClose={() => setSettingsOpen(false)}>
        <div style={{ display:'flex', flexDirection:'column', gap:12 }}>
          <div>
            <div style={{ fontWeight:600, marginBottom:6 }}>Canvas background</div>
            <input type="color" value={settings.canvasBg} onChange={(e)=> setSettings(s => ({ ...s, canvasBg: e.target.value }))} />
          </div>
          <Divider />
          <div>
            <div style={{ fontWeight:600, marginBottom:6 }}>Grid</div>
            <div style={{ display:'flex', gap:8, alignItems:'center' }}>
              <Select
                value={settings.gridVariant}
                onChange={(v)=> setSettings(s => ({ ...s, gridVariant: v }))}
                options={[{value:'dots',label:'Dots'},{value:'lines',label:'Lines'}]}
                style={{ width: 120 }}
              />
              <input type="color" value={settings.gridColor} onChange={(e)=> setSettings(s => ({ ...s, gridColor: e.target.value }))} />
            </div>
          </div>
          <Divider />
          <div>
            <div style={{ fontWeight:600, marginBottom:6 }}>Auto save</div>
            <Switch checked={settings.autoSave} onChange={(v)=> setSettings(s => ({ ...s, autoSave: v }))} />
          </div>
        </div>
      </Drawer>
    </div>
  )
})
