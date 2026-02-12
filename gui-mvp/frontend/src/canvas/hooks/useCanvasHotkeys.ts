// src/canvas/hooks/useCanvasHotkeys.ts
import { useEffect } from 'react'

/** Utility: detect whether current focus is a text/editing input */
export function isTextInput(el: Element | null) {
  if (!el) return false
  const tag = el.tagName?.toLowerCase?.() || ''
  return (
    tag === 'input' ||
    tag === 'textarea' ||
    (el as HTMLElement).isContentEditable ||
    (el as HTMLElement).classList?.contains?.('monaco-editor') ||
    !!(el as HTMLElement).closest?.('.monaco-editor')
  )
}

export type HotkeyHandlers = {
  save?: () => void
  deleteSelection?: () => void
  group?: () => void
  ungroup?: () => void
  undo?: () => void
  redo?: () => void
  // optional extra actions
  fitView?: () => void
  autoLayout?: () => void
}

type Options = {
  isBlocked?: boolean  // e.g. a modal is open → disable hotkeys
  handlers: HotkeyHandlers
}

/** Register global canvas hotkeys (Ctrl/Cmd combos, Delete, etc.) */
export default function useCanvasHotkeys(opts: Options) {
  const { isBlocked, handlers } = opts

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (isBlocked) return
      if (isTextInput(document.activeElement)) return

      const mod = e.ctrlKey || e.metaKey
      const k = e.key.toLowerCase()

      // Delete selection
      if (e.key === 'Delete') {
        if (handlers.deleteSelection) { e.preventDefault(); handlers.deleteSelection() }
        return
      }

      // Save
      if (mod && k === 's') {
        if (handlers.save) { e.preventDefault(); handlers.save() }
        return
      }

      // Group / Ungroup
      if (mod && k === 'g' && !e.shiftKey) {
        if (handlers.group) { e.preventDefault(); handlers.group() }
        return
      }
      if (mod && k === 'g' && e.shiftKey) {
        if (handlers.ungroup) { e.preventDefault(); handlers.ungroup() }
        return
      }

      // Undo / Redo
      if (mod && k === 'z' && !e.shiftKey) {
        if (handlers.undo) { e.preventDefault(); handlers.undo() }
        return
      }
      if ((mod && k === 'y') || (mod && k === 'z' && e.shiftKey)) {
        if (handlers.redo) { e.preventDefault(); handlers.redo() }
        return
      }

      // Optional: fit view / auto layout (uncomment if desired)
      if (mod && k === 'f') { handlers.fitView?.(); e.preventDefault(); return }
      if (mod && k === 'l') { handlers.autoLayout?.(); e.preventDefault(); return }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [isBlocked, handlers])
}
