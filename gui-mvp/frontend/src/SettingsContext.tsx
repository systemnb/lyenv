// SettingsContext.tsx
import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'

type ThemeMode = 'light' | 'system' | 'custom'
type GridVariant = 'dots' | 'lines'

type Settings = {
  themeMode: ThemeMode
  canvasBg: string         // e.g. '#ffffff'
  gridVariant: GridVariant // 'dots' or 'lines'
  gridColor: string        // e.g. '#dddddd'
  nodeDefaultColor: string // e.g. '#4f46e5'
  autoSave: boolean
}

const DEFAULT: Settings = {
  themeMode: 'light',
  canvasBg: '#ffffff',
  gridVariant: 'dots',
  gridColor: '#dddddd',
  nodeDefaultColor: '#4f46e5',
  autoSave: true,
}

const KEY = 'rf-settings'

const SettingsContext = createContext<{
  settings: Settings
  setSettings: (patch: Partial<Settings>) => void
}>({ settings: DEFAULT, setSettings: () => {} })

export const SettingsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [settings, setSettingsState] = useState<Settings>(() => {
    try {
      const raw = localStorage.getItem(KEY)
      return raw ? { ...DEFAULT, ...JSON.parse(raw) } : DEFAULT
    } catch { return DEFAULT }
  })

  const setSettings = (patch: Partial<Settings>) => {
    setSettingsState(prev => {
      const next = { ...prev, ...patch }
      localStorage.setItem(KEY, JSON.stringify(next))
      // reflect CSS variables globally (for custom theme)
      document.documentElement.style.setProperty('--canvas-bg', next.canvasBg)
      document.documentElement.style.setProperty('--grid-color', next.gridColor)
      document.documentElement.style.setProperty('--node-default-color', next.nodeDefaultColor)
      return next
    })
  }

  useEffect(() => {
    // initialize CSS variables
    document.documentElement.style.setProperty('--canvas-bg', settings.canvasBg)
    document.documentElement.style.setProperty('--grid-color', settings.gridColor)
    document.documentElement.style.setProperty('--node-default-color', settings.nodeDefaultColor)
  }, [])

  const value = useMemo(() => ({ settings, setSettings }), [settings])
  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>
}

export const useSettings = () => useContext(SettingsContext)
