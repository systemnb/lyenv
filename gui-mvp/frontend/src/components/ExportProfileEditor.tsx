// src/components/ExportProfileEditor.tsx
// Export profile editor (Drawer). Comments in English.

import React, { useEffect, useMemo, useState } from 'react'
import { Drawer, Button, Input, Select, Divider, message } from 'antd'
import Editor from '@monaco-editor/react'
import yaml from 'js-yaml'

type ExportProfile = {
    pluginName: string
    version: string
    expose: string[]
    localConfigFile: string
    configYaml: string
    readmeTitle?: string
    readmeBody?: string
}

type Props = {
    open: boolean
    value: ExportProfile
    onCancel: () => void
    onSave: (v: ExportProfile) => void
    onSaveAndExport?: (v: ExportProfile) => void
}

function normalizeExpose(arr: string[], fallback: string) {
    const cleaned = (arr || []).map(s => (s || '').trim()).filter(Boolean)
    return cleaned.length ? cleaned : [fallback]
}

export default function ExportProfileEditor({ open, value, onCancel, onSave, onSaveAndExport }: Props) {
    const [draft, setDraft] = useState<ExportProfile>(value)

    useEffect(() => { setDraft(value) }, [value, open])

    const canSave = useMemo(() => {
        return (draft.pluginName || '').trim().length > 0 && (draft.version || '').trim().length > 0
    }, [draft])

    const doSave = (alsoExport: boolean) => {
        const pluginName = (draft.pluginName || '').trim()
        const version = (draft.version || '').trim()
        if (!pluginName) { message.error('Plugin name is required'); return }
        if (!version) { message.error('Version is required'); return }

        const expose = normalizeExpose(draft.expose, pluginName)
        const localConfigFile = (draft.localConfigFile || './config.yaml').trim() || './config.yaml'
        const configYaml = draft.configYaml || ''

        const next: ExportProfile = {
            ...draft,
            pluginName,
            version,
            expose,
            localConfigFile,
            configYaml,
        }

        try {
            yaml.load(configYaml) // just validate
          } catch (e:any) {
            message.error('config.yaml is invalid YAML: ' + (e?.message || ''))
            return
          }

        onSave(next)
        message.success('Export settings saved')
        if (alsoExport && onSaveAndExport) onSaveAndExport(next)
    }

    return (
        <Drawer
            title="Export Settings"
            placement="right"
            width={640}
            open={open}
            onClose={onCancel}
            destroyOnClose
            extra={
                <div style={{ display: 'flex', gap: 8 }}>
                    <Button onClick={onCancel}>Close</Button>
                    <Button type="primary" disabled={!canSave} onClick={() => doSave(false)}>Save</Button>
                    {onSaveAndExport && (
                        <Button type="primary" disabled={!canSave} onClick={() => doSave(true)}>Save & Export</Button>
                    )}
                </div>
            }
        >
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                <div>
                    <div style={{ fontWeight: 600, marginBottom: 6 }}>Plugin Name</div>
                    <Input
                        value={draft.pluginName}
                        onChange={(e) => setDraft(s => ({ ...s, pluginName: e.target.value }))}
                        placeholder="myplugin"
                    />
                </div>

                <div>
                    <div style={{ fontWeight: 600, marginBottom: 6 }}>Version</div>
                    <Input
                        value={draft.version}
                        onChange={(e) => setDraft(s => ({ ...s, version: e.target.value }))}
                        placeholder="0.1.0"
                    />
                </div>

                <div>
                    <div style={{ fontWeight: 600, marginBottom: 6 }}>Expose (Shims)</div>
                    <Select
                        mode="tags"
                        style={{ width: '100%' }}
                        value={draft.expose}
                        onChange={(v) => setDraft(s => ({ ...s, expose: v as string[] }))}
                        tokenSeparators={[',', ' ']}
                        placeholder="myplugin (press Enter to add more)"
                    />
                    <div style={{ color: '#6b7280', fontSize: 12, marginTop: 4 }}>
                        These are the CLI shims users can run (expose list in manifest).
                    </div>
                </div>

                <div>
                    <div style={{ fontWeight: 600, marginBottom: 6 }}>manifest.config.local_file</div>
                    <Input
                        value={draft.localConfigFile}
                        onChange={(e) => setDraft(s => ({ ...s, localConfigFile: e.target.value }))}
                        placeholder="./config.yaml"
                    />
                </div>

                <Divider />

                <div>
                    <div style={{ fontWeight: 600, marginBottom: 6 }}>config.yaml (Plugin default config)</div>

                    <div style={{ border: '1px solid #e5e7eb', borderRadius: 8, overflow: 'hidden' }}>
                        <Editor
                            height="260px"
                            defaultLanguage="yaml"
                            value={draft.configYaml}
                            onChange={(v) => setDraft(s => ({ ...s, configYaml: v ?? '' }))}
                            options={{
                                minimap: { enabled: false },
                                fontSize: 13,
                                tabSize: 2,
                                insertSpaces: true,
                                wordWrap: 'on',
                                scrollBeyondLastLine: false,
                                automaticLayout: true, // ✅ auto resize with drawer
                                renderLineHighlight: 'all',
                            }}
                        />
                    </div>
                    <div style={{ color: '#6b7280', fontSize: 12, marginTop: 6 }}>
                        Tip: YAML is indentation-sensitive. Prefer 2-space indentation.
                    </div>


                    <Divider />

                    <div>
                        <div style={{ fontWeight: 600, marginBottom: 6 }}>README Title (optional)</div>
                        <Input
                            value={draft.readmeTitle || ''}
                            onChange={(e) => setDraft(s => ({ ...s, readmeTitle: e.target.value }))}
                            placeholder="My Plugin"
                        />
                    </div>

                    <div>
                        <div style={{ fontWeight: 600, marginBottom: 6 }}>README Body (optional)</div>
                        <Input.TextArea
                            value={draft.readmeBody || ''}
                            onChange={(e) => setDraft(s => ({ ...s, readmeBody: e.target.value }))}
                            rows={6}
                            placeholder="Describe your plugin..."
                        />
                    </div>
                </div>
            </div>
        </Drawer>
    )
}
