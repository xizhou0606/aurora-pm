import { createHash } from 'node:crypto'
import type { ViewNode } from '@mobilewright/protocol'
import type { MobilePlatform, NativeVisualElement } from './types'

const MAX_ELEMENTS = 1500

function boundedText(value: unknown, maximum: number) {
  return typeof value === 'string' && value.trim() ? value.trim().slice(0, maximum) : null
}

function roleOf(type: string) {
  const value = type.toLowerCase()
  if (value.includes('button')) return 'button'
  if (value.includes('textfield') || value.includes('edittext') || value.includes('searchfield')) return 'textfield'
  if (value.includes('image')) return 'image'
  if (value.includes('switch') || value.includes('toggle')) return 'switch'
  if (value.includes('checkbox')) return 'checkbox'
  if (value.includes('slider') || value.includes('seekbar')) return 'slider'
  if (value.includes('tab')) return 'tab'
  if (value.includes('link')) return 'link'
  if (value.includes('cell')) return 'listitem'
  if (value.includes('text')) return 'text'
  return null
}

function fingerprint(platform: MobilePlatform, appID: string, path: string, node: ViewNode) {
  const identity = node.identifier ?? node.resourceId ?? `${node.type}:${path}`
  return createHash('sha256').update(`${platform}|${appID}|${identity}`).digest('hex').slice(0, 32)
}

export function flattenViewTree(roots: ViewNode[], input: { platform: MobilePlatform; appID: string; width: number; height: number; coordinateScale?: number }) {
  const elements: NativeVisualElement[] = []
  let visibleCount = 0
  const scale = input.coordinateScale && input.coordinateScale > 0 ? input.coordinateScale : 1
  const visit = (node: ViewNode, path: string, parentID: string | null) => {
    const raw = { x: node.bounds.x / scale, y: node.bounds.y / scale, width: node.bounds.width / scale, height: node.bounds.height / scale }
    const include = node.isVisible && raw.width > 0 && raw.height > 0
    let nextParentID = parentID
    if (include) {
      visibleCount += 1
      if (elements.length < MAX_ELEMENTS) {
        const id = `native-${elements.length + 1}`
        const clipped = raw.x < 0 || raw.y < 0 || raw.x + raw.width > input.width || raw.y + raw.height > input.height
        const left = Math.max(0, raw.x); const top = Math.max(0, raw.y)
        const right = Math.min(input.width, raw.x + raw.width); const bottom = Math.min(input.height, raw.y + raw.height)
        elements.push({
          id, parent_id: parentID, native_type: node.type.slice(0, 120), role: roleOf(node.type),
          label: boundedText(node.label, 500), text: boundedText(node.text, 500), value: boundedText(node.value, 500),
          identifier: boundedText(node.identifier, 300), resource_id: boundedText(node.resourceId, 300),
          bounds: { x: left, y: top, width: Math.max(0, right - left), height: Math.max(0, bottom - top) },
          state: { visible: node.isVisible, enabled: node.isEnabled, selected: node.isSelected ?? null, focused: node.isFocused ?? null, checked: node.isChecked ?? null },
          clipped, structural_fingerprint: fingerprint(input.platform, input.appID, path, node),
        })
        nextParentID = id
      }
    }
    node.children.forEach((child, index) => visit(child, `${path}/${child.type}[${index}]`, nextParentID))
  }
  roots.forEach((root, index) => visit(root, `${root.type}[${index}]`, null))
  return { elements, truncated: visibleCount > MAX_ELEMENTS, visibleCount }
}
