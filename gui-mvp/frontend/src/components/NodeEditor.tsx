import React, { useEffect } from 'react'
import { Modal, Form, Input, Select, Slider, InputNumber, ColorPicker, Space } from 'antd'

export type Shape = 'rect' | 'round' | 'circle' | 'triangle' | 'pentagon' | 'hexagon'

export type EditableData = {
  label?: string
  color?: string
  shape?: Shape
  rotation?: number
  inputs?: number
  outputs?: number
}

type Props = {
  open: boolean
  initial?: EditableData
  onCancel: () => void
  onApply: (patch: EditableData) => void
}

/** AntD form-based node editor (rename, color, shape, rotation, ports). */
export default function NodeEditor({ open, initial, onCancel, onApply }: Props) {
  const [form] = Form.useForm<EditableData>()

  useEffect(() => {
    if (open) {
      form.setFieldsValue({
        label: initial?.label ?? 'Node',
        color: initial?.color ?? '#4f46e5',
        shape: initial?.shape ?? 'rect',
        rotation: initial?.rotation ?? 0,
        inputs: initial?.inputs ?? 1,
        outputs: initial?.outputs ?? 1,
      })
    } else {
      form.resetFields()
    }
  }, [open, initial, form])

  const submit = async () => {
    const v = await form.validateFields()
    onApply(v)
  }

  return (
    <Modal
      open={open}
      title="Edit Node"
      onCancel={onCancel}
      onOk={submit}
      okText="Apply"
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item name="label" label="Name" rules={[{ required: true, message: 'Required' }]}>
          <Input placeholder="Node name" />
        </Form.Item>

        <Form.Item label="Color" required>
          <Space>
            <Form.Item name="color" noStyle rules={[{ required: true }]}>
              {/* AntD v5 ColorPicker returns tinycolor object in onChange; pick hex string */}
              <ColorPicker
                showText
                onChange={(c) => form.setFieldValue('color', c.toHexString())}
              />
            </Form.Item>
            {/* fallback for older AntD (<5) or quick manual input */}
            <Form.Item name="color" noStyle shouldUpdate>
              {() => (
                <input
                  type="color"
                  value={form.getFieldValue('color')}
                  onChange={(e) => form.setFieldValue('color', e.target.value)}
                  style={{ width: 42, height: 32, border: 'none', background: 'transparent' }}
                />
              )}
            </Form.Item>
          </Space>
        </Form.Item>

        <Form.Item name="shape" label="Shape" initialValue="rect">
          <Select
            options={[
              { value: 'rect', label: 'Rectangle' },
              { value: 'round', label: 'Rounded' },
              { value: 'circle', label: 'Circle' },
              { value: 'triangle', label: 'Triangle' },
              { value: 'pentagon', label: 'Pentagon' },
              { value: 'hexagon', label: 'Hexagon' },
            ]}
          />
        </Form.Item>

        <Form.Item name="rotation" label="Rotation (deg)">
          <Slider min={-180} max={180} step={1} />
        </Form.Item>

        <Form.Item label="Ports (unlimited)">
          <Space size="large">
            <Form.Item name="inputs" label="Inputs" tooltip="Number of input handles" style={{ marginBottom: 0 }}>
              <InputNumber min={0} max={128} />
            </Form.Item>
            <Form.Item name="outputs" label="Outputs" tooltip="Number of output handles" style={{ marginBottom: 0 }}>
              <InputNumber min={0} max={128} />
            </Form.Item>
          </Space>
        </Form.Item>
      </Form>
    </Modal>
  )
}
