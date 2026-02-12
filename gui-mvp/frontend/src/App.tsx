import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Layout, Button, Space, Typography, theme, Select, message, Modal, Input, Tooltip } from 'antd'
import {
  PlusOutlined, BranchesOutlined, SaveOutlined, FolderOpenOutlined,
  DeleteOutlined, FullscreenOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  PlayCircleOutlined, ReloadOutlined, DownOutlined, UpOutlined
} from '@ant-design/icons'
import JSZip from 'jszip'
import yaml from 'js-yaml'

import Canvas, { CanvasHandle } from './canvas/Canvas'
import type { RFNode, RFEdge } from './canvas/graph'
import { buildManifestAndFilesStdio } from './canvas/exporterStdio'

const { Header, Sider, Content } = Layout
const { Text } = Typography

type EnvInfo = { name: string; path: string; from?: string }

function b64FromUint8(u8: Uint8Array) {
  // Encode Uint8Array to base64 (browser safe)
  let s = ''
  for (let i = 0; i < u8.length; i++) s += String.fromCharCode(u8[i])
  return btoa(s)
}

function parseArgs(raw: string): string[] {
  const t = raw.trim()
  if (!t) return []
  // Minimal: split by whitespace; you can enhance to support quotes later
  return t.split(/\s+/g)
}

function tokenCmdName(s: string) {
  return (s || '').trim().toLowerCase()
    .replace(/[^a-z0-9._-]/g, '-')
    .replace(/-+/g, '-') || 'task'
}

export default function App() {
  const { token } = theme.useToken()
  const canvasRef = useRef<CanvasHandle>(null)

  const [collapsed, setCollapsed] = useState(false)

  // env list
  const [envs, setEnvs] = useState<EnvInfo[]>([])
  const [envPath, setEnvPath] = useState<string | null>(null)

  // dispatch + console
  const [dispatchId, setDispatchId] = useState<string | null>(null)
  const [consoleOpen, setConsoleOpen] = useState(true)
  const [logs, setLogs] = useState<string[]>([])
  const logsBoxRef = useRef<HTMLDivElement | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  // Run flow modals
  const [pickOpen, setPickOpen] = useState(false)
  const [argsOpen, setArgsOpen] = useState(false)
  const [cmdOptions, setCmdOptions] = useState<Array<{ label: string; value: string }>>([])
  const [pickedCmd, setPickedCmd] = useState<string>('')
  const [argsText, setArgsText] = useState<string>('')

  const envOptions = useMemo(
    () => envs.map(e => ({ label: `${e.name} — ${e.path}`, value: e.path })),
    [envs]
  )

  const loadEnvs = async () => {
    try {
      const res = await fetch('/api/envs', { cache: 'no-store' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      const list: EnvInfo[] = Array.isArray(data.envs) ? data.envs : []
      setEnvs(list)
      if (!envPath && list.length) setEnvPath(list[0].path)
      if (list.length === 0) {
        message.warning('No GUI env registered. Use: lyenv gui add <DIR>')
      }
    } catch (e: any) {
      console.error(e)
      message.error(e?.message || 'Load envs failed')
    }
  }

  useEffect(() => { loadEnvs() }, []) // initial load

  // Cleanup websocket on unmount
  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [])

  // Auto scroll console to bottom
  useEffect(() => {
    if (!consoleOpen) return
    const el = logsBoxRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [logs, consoleOpen])

  // Canvas actions
  const addNode = () => canvasRef.current?.addNodeAtCenter()
  const autoLayout = () => canvasRef.current?.autoLayout()
  const saveFlow = () => canvasRef.current?.saveToLocal()
  const loadFlow = () => canvasRef.current?.loadFromLocal()
  const delSel = () => canvasRef.current?.deleteSelection()
  const fitView = () => canvasRef.current?.fitView()

  /** Click Run: pick which group command to run */
  const onRunClick = async () => {
    if (!envPath) {
      message.warning('Select an environment first.')
      return
    }
    const g = canvasRef.current?.getGraph?.()
    if (!g) {
      message.error('Canvas not ready (missing getGraph())')
      return
    }
    const nodes = g.nodes as RFNode[]
    const groups = nodes.filter(n => n.type === 'group')

    const options: Array<{ label: string; value: string }> = []
    if (groups.length === 0) {
      options.push({ label: 'Whole canvas', value: 'run' })
    } else {
      for (const gr of groups) {
        const label = (gr.data as any)?.label || gr.id
        options.push({ label: String(label), value: tokenCmdName(String(label)) })
      }
    }

    setCmdOptions(options)
    setPickedCmd(options[0]?.value || 'run')
    setPickOpen(true)
  }

  const onPickOk = () => {
    if (!pickedCmd) {
      message.warning('Pick a workflow/group first')
      return
    }
    setPickOpen(false)
    setArgsText('')
    setArgsOpen(true)
  }

  /** Build plugin zip -> call backend install+run -> connect ws */
  const onArgsOk = async () => {
    if (!envPath) return
    setArgsOpen(false)

    const g = canvasRef.current?.getGraph?.()
    if (!g) return
    const nodes = g.nodes as RFNode[]
    const edges = g.edges as RFEdge[]

    // reset console state
    setLogs([])
    setDispatchId(null)
    setConsoleOpen(true)

    // close old ws
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }

    try {
      const installName = `gui-temp-${Date.now()}`
      const shim = installName

      // exporter creates all group commands; we will run the chosen one
      const { manifest, files } = await buildManifestAndFilesStdio(nodes, edges, installName, shim)

      // zip in browser
      const zip = new JSZip()
      zip.file('manifest.yaml', yaml.dump(manifest))
      for (const f of files) zip.file(f.path, f.content)
      const u8 = await zip.generateAsync({ type: 'uint8array' })
      const zipB64 = b64FromUint8(u8)

      const res = await fetch('/api/flow/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          envPath,
          zipB64,
          installName,
          command: pickedCmd,
          args: parseArgs(argsText),
          cleanup: true, // remove plugin after run
        })
      })
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `HTTP ${res.status}`)
      }
      const data = await res.json()
      const id = String(data.dispatchId || '')
      const result = data?.dispatch?.result
      if (result) setLogs(prev => [...prev, `[result] ${result}`])
      if (!id) throw new Error('missing dispatchId')
      setDispatchId(id)

      // connect ws to tail logs
      const wsUrl =
        `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}` +
        `/ws/logs?envPath=${encodeURIComponent(envPath)}&dispatchId=${encodeURIComponent(id)}`
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data)
          if (msg.type === 'line') setLogs(prev => [...prev, String(msg.line)])
          else setLogs(prev => [...prev, String(ev.data)])
        } catch {
          setLogs(prev => [...prev, String(ev.data)])
        }
      }
      ws.onerror = () => setLogs(prev => [...prev, '[ws error]'])
      ws.onclose = () => setLogs(prev => [...prev, '[ws closed]'])
    } catch (e: any) {
      console.error(e)
      message.error(e?.message || 'Run failed')
      setLogs(prev => [...prev, `[error] ${e?.message || 'Run failed'}`])
    }
  }

  return (
    <Layout style={{ height: '100vh', overflow: 'hidden' }}>
      <Header style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        background: '#fff', borderBottom: `1px solid ${token.colorBorderSecondary}`,
        height: 56, lineHeight: '56px'
      }}>
        <Space>
          <Button
            type="default"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(v => !v)}
          />

          <Select
            style={{ width: 420 }}
            value={envPath ?? undefined}
            placeholder="Select env (use: lyenv gui add <DIR>)"
            options={envOptions}
            onChange={(v) => setEnvPath(v)}
            status={envOptions.length === 0 ? 'warning' : undefined}
          />

          <Tooltip title="Refresh env list">
            <Button icon={<ReloadOutlined />} onClick={loadEnvs} />
          </Tooltip>

          <Button type="primary" icon={<PlayCircleOutlined />} onClick={onRunClick}>
            Run
          </Button>

          <Text type="secondary">Dispatch: {dispatchId ?? '-'}</Text>
        </Space>

        <Space>
          <Button icon={<FullscreenOutlined />} onClick={fitView}>Fit</Button>
        </Space>
      </Header>

      <Layout style={{ minHeight: 0 }}>
        <Sider
          theme="light"
          collapsible
          collapsed={collapsed}
          onCollapse={setCollapsed}
          collapsedWidth={0}
          trigger={null}
          style={{ borderRight: `1px solid ${token.colorBorderSecondary}`, overflow: 'hidden' }}
          width={240}
        >
          <div style={{ padding: 12 }}>
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Button block icon={<PlusOutlined />} onClick={addNode}>Add Node</Button>
              <Button block icon={<BranchesOutlined />} onClick={autoLayout}>Auto Layout</Button>
              <Button block icon={<SaveOutlined />} onClick={saveFlow}>Save</Button>
              <Button block icon={<FolderOpenOutlined />} onClick={loadFlow}>Load</Button>
              <Button block danger icon={<DeleteOutlined />} onClick={delSel}>Delete Selection</Button>
            </Space>
            <div style={{ color: token.colorTextTertiary, fontSize: 12, marginTop: 16 }}>
              Double-click canvas to create node. Right-click for shortcuts.
            </div>
          </div>
        </Sider>

        {/* IMPORTANT: Content is flex column with overflow hidden */}
        <Content style={{ display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden' }}>
          {/* Canvas area MUST be overflow hidden so Canvas can't push console out */}
          <div style={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
            <Canvas ref={canvasRef} />
          </div>

          {/* Console panel (always within viewport) */}
          <div style={{
            borderTop: `1px solid ${token.colorBorderSecondary}`,
            background: '#0b1020',
            color: '#c7d2fe',
            height: consoleOpen ? 260 : 40,
            transition: 'height 160ms ease',
            flex: '0 0 auto',
          }}>
            <div style={{
              height: 40,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '0 12px',
              background: '#0a0f1c',
            }}>
              <Space>
                <Text style={{ color: '#e0e7ff', fontWeight: 600 }}>Console</Text>
                <Text style={{ color: 'rgba(224,231,255,0.6)' }}>
                  {dispatchId ? `dispatch=${dispatchId}` : 'idle'}
                </Text>
              </Space>

              <Button
                size="small"
                type="text"
                style={{ color: '#c7d2fe' }}
                icon={consoleOpen ? <DownOutlined /> : <UpOutlined />}
                onClick={() => setConsoleOpen(v => !v)}
              >
                {consoleOpen ? 'Collapse' : 'Expand'}
              </Button>
            </div>

            {consoleOpen && (
              <div
                ref={logsBoxRef}
                style={{
                  height: 220,
                  overflow: 'auto',
                  padding: 10,
                  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
                  fontSize: 12,
                }}
              >
                {logs.length === 0 ? (
                  <div style={{ color: 'rgba(199,210,254,0.65)' }}>
                    No logs yet. Click Run to start.
                  </div>
                ) : (
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
                    {logs.join('\n')}
                  </pre>
                )}
              </div>
            )}
          </div>
        </Content>
      </Layout>

      {/* Modal 1: pick group/command */}
      <Modal
        open={pickOpen}
        title="Pick a workflow/group to run"
        okText="Next"
        onCancel={() => setPickOpen(false)}
        onOk={onPickOk}
        getContainer={() => document.body}
      >
        <Select
          style={{ width: '100%' }}
          options={cmdOptions}
          value={pickedCmd || undefined}
          onChange={setPickedCmd}
          placeholder="Choose a workflow command"
        />
        <div style={{ marginTop: 12, color: 'rgba(0,0,0,0.45)' }}>
          Each group becomes one command. If no groups, "run" means the whole canvas.
        </div>
      </Modal>

      {/* Modal 2: args */}
      <Modal
        open={argsOpen}
        title="Input arguments"
        okText="Run"
        onCancel={() => setArgsOpen(false)}
        onOk={onArgsOk}
        getContainer={() => document.body}
      >
        <Input
          placeholder='Args (space separated), e.g. "foo bar"'
          value={argsText}
          onChange={(e) => setArgsText(e.target.value)}
        />
      </Modal>
    </Layout>
  )
}