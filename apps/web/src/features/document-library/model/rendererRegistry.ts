import type { ComponentType } from 'react'

export type DocumentResourceDescriptor = {
  resourceId: string
  typeKey: string
  payloadRef: string
}

export type DocumentResourceRendererProps = {
  resource: DocumentResourceDescriptor
  readOnly: boolean
}

export type DocumentResourceRendererAdapter = {
  typeKey: string
  Renderer: ComponentType<DocumentResourceRendererProps>
}

export type DocumentResourceRendererRegistry = {
  resolve: (typeKey: string) => DocumentResourceRendererAdapter | undefined
  typeKeys: readonly string[]
}

function validTypeKey(value: string): boolean {
  return /^[a-z][a-z0-9_]{0,63}$/.test(value)
}

export function createDocumentResourceRendererRegistry(
  adapters: readonly DocumentResourceRendererAdapter[],
): DocumentResourceRendererRegistry {
  const entries = new Map<string, DocumentResourceRendererAdapter>()
  for (const adapter of adapters) {
    if (!validTypeKey(adapter.typeKey) || entries.has(adapter.typeKey)) {
      throw new Error('Invalid or duplicate document resource renderer type')
    }
    entries.set(adapter.typeKey, adapter)
  }
  return {
    resolve: (typeKey) => entries.get(typeKey),
    typeKeys: [...entries.keys()],
  }
}

export function createDefaultDocumentResourceRendererRegistry(input: {
  RichTextRenderer: DocumentResourceRendererAdapter['Renderer']
  BitableRenderer: DocumentResourceRendererAdapter['Renderer']
}): DocumentResourceRendererRegistry {
  return createDocumentResourceRendererRegistry([
    { typeKey: 'rich_text', Renderer: input.RichTextRenderer },
    { typeKey: 'bitable', Renderer: input.BitableRenderer },
  ])
}
