// Node functional kinds (similar to ComfyUI categories)
export type NodeKind = 'condition' | 'command' | 'code' | 'custom'

// Logical port with a name and declared dtype (like ComfyUI typed sockets)
export type Port = { id: string; name: string; dtype: string }

// Per-kind config payloads
export type ConditionConfig = {
  language: 'js' | 'jsonata'
  expression: string
}

export type CommandConfig = {
  command: string
  args?: string[]
  cwd?: string
  env?: Record<string, string>
}

export type CodeConfig = {
  language: 'python' | 'javascript' | 'bash' | 'lua' | 'go'
  source: string      // full source text edited in Monaco
  entry?: string      // optional entry function / file
}

// catch-all custom
export type CustomConfig = Record<string, any>

// Unified config union
export type NodeConfig = ConditionConfig | CommandConfig | CodeConfig | CustomConfig

// Extended data shape stored on nodes
export type FunctionalData = {
  // appearance
  label?: string
  color?: string
  shape?: 'rect' | 'round' | 'circle' | 'triangle' | 'pentagon' | 'hexagon'
  rotation?: number

  // functional
  kind?: NodeKind
  config?: NodeConfig

  // I/O ports (unlimited)
  ports?: {
    inputs: Port[]
    outputs: Port[]
  }
}
