import type { JSONContent } from '@tiptap/core'
import { zipSync } from 'fflate'

import { serializeRichTextMarkdown } from '@/shared/ui/file-media/rich-text-markdown'
import type { RichTextDocumentLabels } from '@/shared/ui/file-media/rich-text-types'
import { parseRichTextBitableReference, richTextBitablePath } from '@/shared/ui'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { DOCUMENT_IMAGE_MIME_BY_EXTENSION, DOCUMENT_MARKDOWN_LIMITS as limits, matchesDocumentImageMime } from './markdownBundle'

export type DocumentMarkdownExportFormat = 'markdown' | 'zip'
export type DocumentMarkdownExportWarning = 'external_images' | 'simplified_formatting'
export type DocumentMarkdownExportErrorCode =
  | 'invalid_document' | 'unsupported_content' | 'unsafe_source' | 'requires_bundle'
  | 'asset_failed' | 'image_unsupported' | 'markdown_too_large' | 'archive_too_many_files'
  | 'archive_uncompressed_too_large' | 'archive_too_large'

export class DocumentMarkdownExportError extends Error {
  constructor(readonly code: DocumentMarkdownExportErrorCode) {
    super(code)
    this.name = 'DocumentMarkdownExportError'
  }
}

type ExportInput = {
  documentId: number
  title: string
  content: string
  format: DocumentMarkdownExportFormat
  labels: RichTextDocumentLabels
  // Must read through the current document's authenticated asset endpoint, never the stored src.
  loadAsset: (assetId: number) => Promise<Blob>
}

const nodeTypes = new Set([
  'doc', 'paragraph', 'text', 'heading', 'bulletList', 'orderedList', 'listItem',
  'codeBlock', 'blockquote', 'hardBreak', 'horizontalRule', 'image', 'taskList',
  'taskItem', 'table', 'tableRow', 'tableCell', 'tableHeader',
])
const markTypes = new Set(['bold', 'italic', 'strike', 'code', 'link'])
const encoder = new TextEncoder()
const privateAssetPath = /^\/api\/v1\/platform\/documents\/([1-9]\d*)\/assets\/([1-9]\d*)\/content$/

function fail(code: DocumentMarkdownExportErrorCode): never {
  throw new DocumentMarkdownExportError(code)
}

function safeName(title: string, documentId: number): string {
  const candidate = title.normalize('NFC').replace(/[<>:"/\\|?*\u0000-\u001f\u007f]/g, '_')
    .replace(/^\.+|[. ]+$/g, '').trim()
  let name = ''
  for (const character of candidate) {
    if (encoder.encode(name + character).length > 180) break
    name += character
  }
  if (!name || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(name)) return `document-${documentId}`
  return name
}

function safeExternalSource(source: string, image: boolean): void {
  if (/[\u0000-\u0020\u007f]/.test(source)) fail('unsafe_source')
  if (!image && /^#[^\s]+$/.test(source)) return
  let url: URL
  try {
    url = !image && source && !source.startsWith('//') && !/^[a-z][a-z\d+.-]*:/i.test(source)
      ? new URL(source, 'https://document.invalid') : new URL(source)
  } catch { fail('unsafe_source') }
  if (!['https:', 'http:', ...(!image ? ['mailto:', 'tel:'] : [])].includes(url.protocol)
    || url.username || url.password) fail('unsafe_source')
  // A signed URL or URL-embedded credential is not a portable public resource.
  if ([...url.searchParams.keys()].some((key) => /token|signature|credential|authorization|secret|password|^x-amz-|^x-goog-|^q-sign-|^sig$|^key$|^auth$/i.test(key))) {
    fail('unsafe_source')
  }
  if (url.pathname.includes('/api/v1/platform/documents/') && url.pathname.includes('/assets/')) fail('unsafe_source')
}

function privateAssetReference(source: string, assetId: unknown): number | undefined {
  let match = privateAssetPath.exec(source)
  if (!match && /^https?:\/\//i.test(source)) {
    let url: URL
    try { url = new URL(source) } catch { fail('unsafe_source') }
    match = privateAssetPath.exec(url.pathname)
    if (match) {
      const rawPath = source.slice(source.indexOf('/', source.indexOf('://') + 3))
      if (url.username || url.password || /[?#@\s\\]/.test(source)
        || rawPath !== url.pathname || !Number.isSafeInteger(Number(assetId)) || Number(assetId) <= 0) {
        fail('unsafe_source')
      }
    }
  }
  if (!match) return undefined
  const id = Number(match[2])
  if (!Number.isSafeInteger(id) || (assetId !== null && assetId !== undefined && Number(assetId) !== id)) fail('unsafe_source')
  return id
}

function normalizeDocument(
  node: JSONContent,
  warnings: Set<DocumentMarkdownExportWarning>,
  assets: Map<number, JSONContent[]>,
  depth = 0,
): JSONContent | null {
  if (!node || typeof node !== 'object' || depth > 100 || typeof node.type !== 'string') fail('invalid_document')
  if (node.content !== undefined && !Array.isArray(node.content)) fail('invalid_document')
  if (node.marks !== undefined && !Array.isArray(node.marks)) fail('invalid_document')
  if (node.type === 'unsupportedBlock') fail('unsupported_content')
  if (node.type === 'bitable') {
    const reference = parseRichTextBitableReference(node.attrs)
    if (!reference) fail('invalid_document')
    warnings.add('simplified_formatting')
    return { type: 'paragraph', content: [{ type: 'text', text: DOCUMENTS_COPY.bitable.title, marks: [{ type: 'link', attrs: { href: richTextBitablePath(reference) } }] }] }
  }
  if (node.type === 'tableOfContents') {
    warnings.add('simplified_formatting')
    return null
  }
  if (node.type === 'mention') {
    warnings.add('simplified_formatting')
    return { type: 'text', text: `@${String(node.attrs?.label || node.attrs?.username || node.attrs?.id || '')}` }
  }
  const content = node.content?.map((child) => normalizeDocument(child, warnings, assets, depth + 1))
    .filter((child): child is JSONContent => child !== null)
  if (node.type === 'callout') {
    warnings.add('simplified_formatting')
    return { type: 'blockquote', content: [{ type: 'paragraph', content }] }
  }
  if (!nodeTypes.has(node.type)) fail('unsupported_content')
  if (node.type === 'text' && typeof node.text !== 'string') fail('invalid_document')
  const marks = node.marks?.filter((mark) => {
    if (!mark || typeof mark.type !== 'string') fail('invalid_document')
    if (!markTypes.has(mark.type)) { warnings.add('simplified_formatting'); return false }
    if (mark.type === 'link') safeExternalSource(String(mark.attrs?.href || ''), false)
    return true
  })
  if (node.attrs?.textAlign || node.attrs?.colwidth || node.attrs?.width || node.attrs?.height) warnings.add('simplified_formatting')
  if (['tableCell', 'tableHeader'].includes(node.type)
    && (Number(node.attrs?.colspan || 1) !== 1 || Number(node.attrs?.rowspan || 1) !== 1)) fail('unsupported_content')
  if (['tableCell', 'tableHeader'].includes(node.type)) {
    if (content?.some((child) => !['paragraph', 'image'].includes(child.type || ''))) fail('unsupported_content')
    // The editor stores pasted table images as block siblings of paragraphs.
    // TipTap's GFM table renderer renders each child's *children*, so a direct
    // atom would otherwise disappear. Wrap only for serialization, retaining
    // the same image object tracked for authenticated resource replacement.
    content?.forEach((child, index) => {
      if (child.type !== 'image') return
      content[index] = { type: 'paragraph', content: [child] }
      warnings.add('simplified_formatting')
    })
    if ((content?.length || 0) > 1) warnings.add('simplified_formatting')
  }
  if (node.type === 'table' && !content?.[0]?.content?.every((cell) => cell.type === 'tableHeader')) {
    warnings.add('simplified_formatting')
  }
  const result: JSONContent = { ...node, ...(content ? { content } : {}), ...(marks ? { marks } : {}) }
  if (node.type === 'image') {
    const source = String(node.attrs?.src || '').trim()
    const assetId = node.attrs?.assetId
    const id = privateAssetReference(source, assetId)
    if (id !== undefined) {
      // Copies retain the source document path but receive an explicit asset reference.
      // Ignore the stored doc ID AND host: only loadAsset against the current document
      // may authorize/read the resource, including absolute URLs persisted by paste.
      assets.set(id, [...(assets.get(id) || []), result])
    } else {
      if (assetId !== null && assetId !== undefined) fail('unsafe_source')
      safeExternalSource(source, true)
      warnings.add('external_images')
    }
  }
  return result
}

async function bundleAssets(assets: Map<number, JSONContent[]>, loadAsset: ExportInput['loadAsset']) {
  if (assets.size + 1 > limits.entries) fail('archive_too_many_files')
  const entries: Record<string, Uint8Array> = {}
  let bytesUsed = 0
  for (const [id, nodes] of assets) {
    let blob: Blob
    try { blob = await loadAsset(id) } catch { fail('asset_failed') }
    if (!blob.size || blob.size > limits.imageBytes) fail('image_unsupported')
    bytesUsed += blob.size
    if (bytesUsed > limits.uncompressedBytes) fail('archive_uncompressed_too_large')
    // Images are already compressed. ZIP uses STORE, so reject before allocating a
    // second archive-sized buffer when it cannot fit the import envelope anyway.
    if (bytesUsed > limits.archiveBytes) fail('archive_too_large')
    let bytes: Uint8Array
    try { bytes = new Uint8Array(await blob.arrayBuffer()) } catch { fail('asset_failed') }
    const mime = blob.type.split(';', 1)[0].trim().toLowerCase()
    const extension = Object.keys(DOCUMENT_IMAGE_MIME_BY_EXTENSION)
      .find((value) => DOCUMENT_IMAGE_MIME_BY_EXTENSION[value] === mime && matchesDocumentImageMime(bytes, mime))
    if (!extension) fail('image_unsupported')
    const path = `assets/${id}${extension}`
    entries[path] = bytes
    nodes.forEach((node) => { node.attrs = { ...node.attrs, src: path, assetId: null } })
  }
  return { entries, bytesUsed }
}

export async function buildDocumentMarkdownExport(input: ExportInput): Promise<{
  blob: Blob
  fileName: string
  warnings: DocumentMarkdownExportWarning[]
}> {
  if (!Number.isSafeInteger(input.documentId) || input.documentId <= 0 || !['markdown', 'zip'].includes(input.format)) fail('invalid_document')
  if (encoder.encode(input.content).length > limits.uncompressedBytes) fail('invalid_document')
  let raw: JSONContent
  try { raw = JSON.parse(input.content) as JSONContent } catch { fail('invalid_document') }
  if (raw?.type !== 'doc') fail('invalid_document')
  const warnings = new Set<DocumentMarkdownExportWarning>()
  const assets = new Map<number, JSONContent[]>()
  const document = normalizeDocument(raw, warnings, assets)
  if (!document) fail('invalid_document')
  if (input.format === 'markdown' && assets.size) fail('requires_bundle')
  const { entries, bytesUsed } = await bundleAssets(assets, input.loadAsset)
  let markdown: string
  try { markdown = serializeRichTextMarkdown(document, input.labels) } catch { fail('unsupported_content') }
  // A trailing newline also makes an empty document a valid importable Markdown file.
  const bytes = encoder.encode(`${markdown.trimEnd()}\n`)
  if (bytes.length > limits.markdownBytes) fail('markdown_too_large')
  const name = safeName(input.title, input.documentId)
  const result = { fileName: `${name}.md`, blob: new Blob([bytes], { type: 'text/markdown;charset=utf-8' }), warnings: [...warnings] }
  if (input.format === 'markdown') return result
  if (bytesUsed + bytes.length > limits.uncompressedBytes) fail('archive_uncompressed_too_large')
  entries[`${name}.md`] = bytes
  const archive = zipSync(entries, { level: 0 })
  if (archive.byteLength > limits.archiveBytes) fail('archive_too_large')
  return { ...result, fileName: `${name}.zip`, blob: new Blob([archive], { type: 'application/zip' }) }
}
