import React, { useMemo } from 'react'
import { Menu } from 'antd'
import type { MenuProps } from 'antd'

/** A leaf node with an action. */
export type MenuLeaf = {
  key: string
  label: string
  action: () => void
}

/** A parent node with children (no direct action). */
export type MenuParent = {
  key: string
  label: string
  children: MenuNode[]
}

export type MenuNode = MenuLeaf | MenuParent

type Props = {
  visible: boolean
  x: number
  y: number
  items: MenuNode[]
  onClose: () => void
}

/** Convert our MenuNode[] to AntD items, and build a key->action map. */
function useAntdItemsAndActions(spec: MenuNode[]) {
  const { items, actionMap } = useMemo(() => {
    const map = new Map<string, () => void>()

    const convert = (n: MenuNode): Required<MenuProps>['items'][number] => {
      if ('children' in n) {
        return { key: n.key, label: n.label, children: n.children.map(convert) }
      }
      map.set(n.key, n.action)
      return { key: n.key, label: n.label }
    }

    return { items: spec.map(convert), actionMap: map }
  }, [spec])

  return { items, actionMap }
}

/** Absolute-positioned context menu using AntD Menu.
 *  - No per-item onClick, to comply with Menu items typing.
 *  - All click handling is centralized in Menu.onClick.
 */
export default function ContextMenu({ visible, x, y, items: spec, onClose }: Props) {
  const { items, actionMap } = useAntdItemsAndActions(spec)
  if (!visible) return null

  const onClick: MenuProps['onClick'] = ({ key }) => {
    const fn = actionMap.get(String(key))
    if (fn) fn()
    onClose()
  }

  return (
    <div
      onClick={onClose}
      onContextMenu={(e) => e.preventDefault()}
      style={{
        position: 'fixed',
        left: x,
        top: y,
        zIndex: 1000,
        background: '#fff',
        border: '1px solid #e5e7eb',
        borderRadius: 8,
        boxShadow: '0 8px 24px rgba(0,0,0,.12)',
        overflow: 'hidden',
      }}
    >
      <Menu selectable={false} items={items} onClick={onClick} style={{ minWidth: 240 }} />
    </div>
  )
}
