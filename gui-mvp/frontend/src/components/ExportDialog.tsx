// gui-mvp/frontend/src/components/ExportDialog.tsx
import React, { useEffect } from 'react'
import { Modal, Form, Input, Space } from 'antd'

export type ExportConfig = {
  pluginName: string
  shimName: string
  branchNames: Record<string, string> // sinkId -> commandName
}

type Props = {
  open: boolean
  sinks: string[]
  defaultPluginName?: string
  defaultShimName?: string
  onCancel: () => void
  onOk: (cfg: ExportConfig) => void
}

/**
 * Export dialog:
 * - let developer set pluginName / shimName
 * - when parallel branches exist (multiple sinks), set per-branch command names.
 */
export default function ExportDialog({
  open,
  sinks,
  onCancel,
  onOk,
  defaultPluginName = 'demo',
  defaultShimName = 'demo',
}: Props) {
  const [form] = Form.useForm<ExportConfig>()

  useEffect(() => {
    if (open) {
      const branchNames: Record<string, string> = {}
      for (const sid of sinks) branchNames[sid] = `run_${sid.toLowerCase()}`
      form.setFieldsValue({
        pluginName: defaultPluginName,
        shimName: defaultShimName,
        branchNames,
      })
    } else {
      form.resetFields()
    }
  }, [open, sinks, form, defaultPluginName, defaultShimName])

  const submit = async () => {
    const v = await form.validateFields()
    onOk(v)
  }

  return (
    <Modal
      open={open}
      onCancel={onCancel}
      onOk={submit}
      title="Export LyEnv Plugin"
      destroyOnClose
    >
      <Form layout="vertical" form={form}>
        <Form.Item
          label="Plugin Name"
          name="pluginName"
          rules={[{ required: true, message: 'Required' }]}
        >
          <Input placeholder="e.g. my-plugin" />
        </Form.Item>

        <Form.Item
          label="Shim Name"
          name="shimName"
          rules={[{ required: true, message: 'Required' }]}
        >
          <Input placeholder="e.g. myshim" />
        </Form.Item>

        {sinks.length > 0 && (
          <>
            <div style={{ fontWeight: 600, marginTop: 8 }}>Parallel Branch Commands</div>
            <Space direction="vertical" style={{ width: '100%', marginTop: 8 }}>
              {sinks.map((sid) => (
                <Form.Item
                  key={sid}
                  label={`Command name for sink "${sid}"`}
                  name={['branchNames', sid]}
                  rules={[{ required: true, message: 'Required' }]}
                >
                  <Input placeholder={`run_${sid.toLowerCase()}`} />
                </Form.Item>
              ))}
            </Space>
          </>
        )}
      </Form>
    </Modal>
  )
}
