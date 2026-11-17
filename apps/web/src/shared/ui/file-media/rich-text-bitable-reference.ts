export type RichTextBitableReference = {
  schemaVersion: 1
  resourceId: string
  tableId: string
  viewId: string
  height: number
}

const stableId = /^[A-Za-z0-9_-]{1,128}$/

/** Reference-only codec: rows, permissions, identities and credentials are never copied. */
export function parseRichTextBitableReference(value: unknown): RichTextBitableReference | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const source = value as Record<string, unknown>
  if (source.schemaVersion !== 1 || typeof source.resourceId !== 'string' || !stableId.test(source.resourceId)
    || typeof source.tableId !== 'string' || !stableId.test(source.tableId)
    || typeof source.viewId !== 'string' || !stableId.test(source.viewId)
    || typeof source.height !== 'number' || !Number.isSafeInteger(source.height) || source.height < 240 || source.height > 900) return null
  return { schemaVersion: 1, resourceId: source.resourceId, tableId: source.tableId, viewId: source.viewId, height: source.height }
}

export function richTextBitablePath(reference: RichTextBitableReference): string {
  return `/workspace/bitables/${encodeURIComponent(reference.resourceId)}?table=${encodeURIComponent(reference.tableId)}&view=${encodeURIComponent(reference.viewId)}`
}

export function containsRichTextBitable(value: string): boolean {
  try {
    const visit = (node: unknown): boolean => {
      if (!node || typeof node !== 'object') return false
      const content = node as { type?: unknown; attrs?: unknown; content?: unknown }
      return (content.type === 'bitable' && parseRichTextBitableReference(content.attrs) !== null)
        || (Array.isArray(content.content) && content.content.some(visit))
    }
    return visit(JSON.parse(value))
  } catch { return false }
}
