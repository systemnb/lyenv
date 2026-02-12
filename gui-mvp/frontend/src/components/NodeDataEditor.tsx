// gui-mvp/frontend/src/components/NodeDataEditor.tsx
import React, { useEffect, useMemo } from 'react'
import { Modal, Form, Input, Select, Tabs, Button, Space } from 'antd'
import Editor, { type OnMount } from '@monaco-editor/react'
import type { FunctionalData, NodeKind, Port } from '../types'

type Props = {
  open: boolean
  initial?: FunctionalData
  onCancel: () => void
  onApply: (patch: FunctionalData) => void
}

const LANGS = ['python','javascript','bash','lua','go'] as const

/** Generate a stable random id for a port */
function uid(prefix: 'in'|'out') {
  return `${prefix}-${Math.random().toString(36).slice(2, 8)}`
}

/** Normalize ports: ensure id/name/dtype exist and ids are stable */
function normalizePorts(patch: FunctionalData): FunctionalData {
  const out: FunctionalData = { ...patch }
  const inputs  = (out.ports?.inputs  ?? []).map((p, i) => ({
    id: p.id && String(p.id).trim() ? p.id : uid('in'),
    name: p.name && String(p.name).trim() ? p.name : `in${i}`,
    dtype: p.dtype && String(p.dtype).trim() ? p.dtype : 'any',
  }))
  const outputs = (out.ports?.outputs ?? []).map((p, i) => ({
    id: p.id && String(p.id).trim() ? p.id : uid('out'),
    name: p.name && String(p.name).trim() ? p.name : `out${i}`,
    dtype: p.dtype && String(p.dtype).trim() ? p.dtype : 'any',
  }))
  out.ports = { inputs, outputs }
  return out
}

export default function NodeDataEditor({ open, initial, onCancel, onApply }: Props) {
  const [form] = Form.useForm<FunctionalData>()

  useEffect(() => {
    if (open) {
      form.setFieldsValue({
        // Basic fields
        label: initial?.label ?? undefined,
        color: initial?.color ?? '#4f46e5',
        kind: initial?.kind ?? 'code',
        // Config and ports
        config: initial?.config ?? { language:'python', source:'# write code here\n' },
        ports: initial?.ports ?? {
          inputs:  [{ id: uid('in'),  name:'in',  dtype:'any'}],
          outputs: [{ id: uid('out'), name:'out', dtype:'any'}],
        }
      } as any)
    } else {
      form.resetFields()
    }
  }, [open, initial, form])

  /** Submit handler: normalize ports and emit patch */
  const submit = async () => {
    const v = await form.validateFields()
    const patch = normalizePorts(v)
    onApply(patch)
  }

  /** Watchers to keep Monaco editors in sync with Form */
  const kind: NodeKind = Form.useWatch('kind', form) as NodeKind
  const codeLang = useMemo(() => {
    if (kind !== 'code') return undefined
    const cfg = form.getFieldValue('config') as any
    return (cfg?.language as typeof LANGS[number]) || 'python'
  }, [kind, form])
  const sourceValue: string = Form.useWatch(['config','source'], form) as string

  const monoMount: OnMount = (editor) => {
    editor.updateOptions({ minimap: { enabled: false } })
  }

  return (
    <Modal open={open} onCancel={onCancel} onOk={submit} title="Edit Node Data" width={860} destroyOnClose>
      <Form form={form} layout="vertical">
        <Tabs
          items={[
            {
              key:'general', label:'General',
              children: (
                <Space direction="vertical" style={{ width:'100%' }}>
                  {/* Node label */}
                  <Form.Item name="label" label="Label">
                    <Input placeholder="display name (optional)"/>
                  </Form.Item>

                  {/* Node color presets (quick pick) */}
                  <div style={{ marginTop: 8 }}>
                    <div style={{ marginBottom:4, fontWeight:600 }}>Node color</div>
                    <div style={{ display:'flex', gap:6, flexWrap:'wrap' }}>
                      {['#4f46e5','#14b8a6','#f59e0b','#60a5fa','#ef4444','#22c55e'].map(c => (
                        <button
                          key={c}
                          onClick={()=> form.setFieldValue(['color'], c)}
                          style={{
                            width:24, height:24, borderRadius:12, border: '1px solid #e5e7eb',
                            background: c, cursor:'pointer'
                          }}
                          title={c}
                          type="button"
                        />
                      ))}
                      <input
                        type="color"
                        value={form.getFieldValue(['color']) || '#4f46e5'}
                        onChange={(e)=> form.setFieldValue(['color'], e.target.value)}
                        title="Custom"
                      />
                    </div>
                  </div>

                  {/* Node kind */}
                  <Form.Item name="kind" label="Node Kind" rules={[{ required:true }]} style={{ marginTop: 8 }}>
                    <Select
                      options={[
                        { value:'condition', label:'Condition' },
                        { value:'command',   label:'Command' },
                        { value:'code',      label:'Code' },
                        { value:'custom',    label:'Custom' },
                      ]}
                    />
                  </Form.Item>
                </Space>
              )
            },
            {
              key:'ports', label:'Ports',
              children: (
                <Space direction="vertical" style={{ width:'100%' }}>
                  <Form.List name={['ports','inputs']}>
                    {(fields, { add, remove }) => (
                      <div>
                        <div style={{ fontWeight:600, marginBottom:6 }}>Inputs</div>
                        {fields.map((f) => (
                          <Space key={f.key} style={{ display:'flex', marginBottom:8 }} align="baseline">
                            {/* name */}
                            <Form.Item {...f} name={[f.name,'name']} rules={[{ required:true }]} style={{ minWidth: 160 }}>
                              <Input placeholder="name"/>
                            </Form.Item>
                            {/* dtype */}
                            <Form.Item {...f} name={[f.name,'dtype']} rules={[{ required:true }]} style={{ minWidth: 120 }}>
                              <Input placeholder="dtype (e.g. any, text, number)"/>
                            </Form.Item>
                            <Button danger onClick={() => remove(f.name)}>Remove</Button>
                          </Space>
                        ))}
                        <Button onClick={() => add({ id: uid('in'), name:`in${fields.length}`, dtype:'any' } as Port)}>+ Add input</Button>
                      </div>
                    )}
                  </Form.List>

                  <Form.List name={['ports','outputs']}>
                    {(fields, { add, remove }) => (
                      <div style={{ marginTop:12 }}>
                        <div style={{ fontWeight:600, marginBottom:6 }}>Outputs</div>
                        {fields.map((f) => (
                          <Space key={f.key} style={{ display:'flex', marginBottom:8 }} align="baseline">
                            {/* name */}
                            <Form.Item {...f} name={[f.name,'name']} rules={[{ required:true }]} style={{ minWidth: 160 }}>
                              <Input placeholder="name"/>
                            </Form.Item>
                            {/* dtype */}
                            <Form.Item {...f} name={[f.name,'dtype']} rules={[{ required:true }]} style={{ minWidth: 120 }}>
                              <Input placeholder="dtype (e.g. any, text, number)"/>
                            </Form.Item>
                            <Button danger onClick={() => remove(f.name)}>Remove</Button>
                          </Space>
                        ))}
                        <Button onClick={() => add({ id: uid('out'), name:`out${fields.length}`, dtype:'any' } as Port)}>+ Add output</Button>
                      </div>
                    )}
                  </Form.List>
                </Space>
              )
            },
            {
              key:'config', label:'Config',
              children: (
                <>
                  {kind === 'condition' && (
                    <Space direction="vertical" style={{ width:'100%' }}>
                      <Form.Item name={['config','language']} label="Expr Language" initialValue="js">
                        <Select options={[{value:'js',label:'JavaScript'},{value:'jsonata',label:'JSONata'}]} />
                      </Form.Item>
                      <Form.Item name={['config','expression']} label="Expression" rules={[{required:true}]}>
                        <Input.TextArea rows={4} placeholder="e.g. Number(input.a) > Number(input.b)"/>
                      </Form.Item>
                    </Space>
                  )}

                  {kind === 'command' && (
                    <Space direction="vertical" style={{ width:'100%' }}>
                      <Form.Item name={['config','command']} label="Command" rules={[{required:true}]}>
                        <Input placeholder="e.g. python"/>
                      </Form.Item>
                      <Form.Item name={['config','args']} label="Args (comma split)">
                        <Input placeholder="main.py,--flag,--count=3"/>
                      </Form.Item>
                      <Form.Item name={['config','cwd']} label="Working Directory">
                        <Input placeholder="."/>
                      </Form.Item>
                    </Space>
                  )}

                  {kind === 'code' && (
                    <Space direction="vertical" style={{ width:'100%' }}>
                      <Form.Item name={['config','language']} label="Language" initialValue="python">
                        <Select options={Array.from(LANGS).map(v=>({value:v,label:v}))} />
                      </Form.Item>
                      <Form.Item
                        name={['config','source']}
                        label="Source"
                        rules={[{required:true}]}
                      >
                        <Editor
                          height="300px"
                          defaultLanguage={codeLang || 'python'}
                          value={sourceValue || ''}
                          onChange={(val) => form.setFieldValue(['config','source'], val ?? '')}
                          onMount={monoMount}
                          options={{ fontSize:13, minimap:{enabled:false} }}
                        />
                      </Form.Item>
                      <Form.Item name={['config','entry']} label="Entry (optional)">
                        <Input placeholder="e.g. main.handler"/>
                      </Form.Item>
                    </Space>
                  )}

                  {kind === 'custom' && (
                    <Form.Item name="config" label="Custom JSON" rules={[{required:true}]}>
                      <Editor
                        height="300px"
                        defaultLanguage="json"
                        onMount={monoMount}
                        value={JSON.stringify(initial?.config ?? {}, null, 2)}
                        onChange={(val) => {
                          try {
                            const parsed = JSON.parse(val || '{}')
                            form.setFieldValue('config', parsed)
                          } catch {
                            // ignore invalid JSON while typing
                          }
                        }}
                        options={{ fontSize:13, minimap:{enabled:false} }}
                      />
                    </Form.Item>
                  )}
                </>
              )
            }
          ]}
        />
      </Form>
    </Modal>
  )
}
