import type { JSONContent } from '@tiptap/core'
import { unzipSync } from 'fflate'

import { parseRichTextMarkdown } from '@/shared/ui/file-media/rich-text-markdown'
import type { RichTextDocumentLabels } from '@/shared/ui/file-media/rich-text-types'
import { DOCUMENT_IMAGE_MIME_BY_EXTENSION, DOCUMENT_MARKDOWN_LIMITS, matchesDocumentImageMime } from './markdownBundle'

const { markdownBytes: MAX_MARKDOWN_BYTES, archiveBytes: MAX_ARCHIVE_BYTES,
  entries: MAX_ARCHIVE_ENTRIES, uncompressedBytes: MAX_ARCHIVE_UNCOMPRESSED_BYTES } = DOCUMENT_MARKDOWN_LIMITS

const importedNodeTypes = new Set([
  'doc', 'paragraph', 'text', 'heading', 'bulletList', 'orderedList', 'listItem',
  'codeBlock', 'blockquote', 'hardBreak', 'horizontalRule', 'image', 'taskList',
  'taskItem', 'table', 'tableRow', 'tableCell', 'tableHeader',
])

export type DocumentMarkdownImportErrorCode =
  | 'archive_too_large'
  | 'archive_invalid'
  | 'archive_unsafe_path'
  | 'archive_too_many_files'
  | 'archive_uncompressed_too_large'
  | 'markdown_missing'
  | 'markdown_multiple'
  | 'markdown_too_large'
  | 'markdown_invalid_encoding'
  | 'markdown_invalid'
  | 'image_missing'
  | 'image_unsupported'
  | 'image_unsafe_source'

export class DocumentMarkdownImportError extends Error {
  readonly code: DocumentMarkdownImportErrorCode

  constructor(code: DocumentMarkdownImportErrorCode) {
    super(code)
    this.name = 'DocumentMarkdownImportError'
    this.code = code
  }
}

export type DocumentMarkdownImportImage = {
  sources: readonly string[]
  archivePath: string
  file: File
}

export type DocumentMarkdownImportBundle = {
  title: string
  content: JSONContent
  images: readonly DocumentMarkdownImportImage[]
  externalImageCount: number
}

function extension(value: string): string {
  const match = /\.[^.\/]+$/.exec(value)
  return match?.[0].toLocaleLowerCase() ?? ''
}

function withoutExtension(value: string): string {
  return value.replace(/\.[^.\/]+$/, '')
}

function baseName(value: string): string {
  return value.split('/').at(-1) || value
}

function canonicalArchivePath(value: string): string {
  const normalized = value.normalize('NFC').replaceAll('\\', '/')
  if (!normalized || /[\u0000-\u001f\u007f]/.test(normalized) || normalized.startsWith('/') || /^[a-zA-Z]:\//.test(normalized)) {
    throw new DocumentMarkdownImportError('archive_unsafe_path')
  }
  const segments: string[] = []
  for (const segment of normalized.split('/')) {
    if (!segment || segment === '.') continue
    if (segment === '..') throw new DocumentMarkdownImportError('archive_unsafe_path')
    segments.push(segment)
  }
  if (!segments.length) throw new DocumentMarkdownImportError('archive_unsafe_path')
  return segments.join('/')
}

function archiveReferencePath(markdownPath: string, rawSource: string): string {
  const sourceWithoutFragment = rawSource.split('#', 1)[0]?.split('?', 1)[0] ?? ''
  let decoded: string
  try {
    decoded = sourceWithoutFragment
      .replaceAll('\\', '/')
      .split('/')
      .map((segment) => decodeURIComponent(segment))
      .join('/')
      .normalize('NFC')
  } catch {
    throw new DocumentMarkdownImportError('image_unsafe_source')
  }
  if (!decoded || /[\u0000-\u001f\u007f]/.test(decoded) || decoded.startsWith('/') || decoded.startsWith('//') || /^[a-zA-Z][a-zA-Z\d+.-]*:/.test(decoded)) {
    throw new DocumentMarkdownImportError('image_unsafe_source')
  }
  const segments = markdownPath.split('/').slice(0, -1)
  for (const segment of decoded.split('/')) {
    if (!segment || segment === '.') continue
    if (segment === '..') {
      if (!segments.length) throw new DocumentMarkdownImportError('image_unsafe_source')
      segments.pop()
      continue
    }
    segments.push(segment)
  }
  if (!segments.length) throw new DocumentMarkdownImportError('image_unsafe_source')
  return segments.join('/')
}

function isRemoteImageSource(source: string): boolean {
  return /^https?:\/\//i.test(source)
}

function decodeMarkdown(bytes: Uint8Array): string {
  if (bytes.byteLength <= 0 || bytes.byteLength > MAX_MARKDOWN_BYTES) {
    throw new DocumentMarkdownImportError('markdown_too_large')
  }
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes)
  } catch {
    throw new DocumentMarkdownImportError('markdown_invalid_encoding')
  }
}

function inlineText(node: JSONContent): string {
  if (node.type === 'text') return node.text || ''
  return (node.content || []).map(inlineText).join('')
}

function importTitle(content: JSONContent, markdownPath: string): string {
  let heading = ''
  const visit = (node: JSONContent) => {
    if (heading) return
    if (node.type === 'heading' && Number(node.attrs?.level) === 1) {
      heading = inlineText(node).trim()
      return
    }
    node.content?.forEach(visit)
  }
  visit(content)
  const candidate = heading || withoutExtension(baseName(markdownPath)).trim() || baseName(markdownPath)
  const bytes = new TextEncoder()
  let title = ''
  for (const character of candidate) {
    if (bytes.encode(title + character).byteLength > 220) break
    title += character
  }
  return title.trim() || baseName(markdownPath)
}

function assertImportedNodes(node: JSONContent): void {
  if (!node.type || !importedNodeTypes.has(node.type)) {
    throw new DocumentMarkdownImportError('markdown_invalid')
  }
  node.content?.forEach(assertImportedNodes)
}

function imageSources(content: JSONContent): string[] {
  const sources: string[] = []
  const visit = (node: JSONContent) => {
    if (node.type === 'image') {
      const source = typeof node.attrs?.src === 'string' ? node.attrs.src.trim() : ''
      if (!source) throw new DocumentMarkdownImportError('image_unsafe_source')
      sources.push(source)
    }
    node.content?.forEach(visit)
  }
  visit(content)
  return sources
}

async function fileBytes(file: File): Promise<Uint8Array> {
  return new Uint8Array(await file.arrayBuffer())
}

function markdownDocument(markdown: string, labels: RichTextDocumentLabels): JSONContent {
  let content: JSONContent
  try {
    content = parseRichTextMarkdown(markdown, labels)
  } catch {
    throw new DocumentMarkdownImportError('markdown_invalid')
  }
  if (content.type !== 'doc') throw new DocumentMarkdownImportError('markdown_invalid')
  assertImportedNodes(content)
  return content
}

async function parsePlainMarkdown(file: File, labels: RichTextDocumentLabels): Promise<DocumentMarkdownImportBundle> {
  const markdownPath = canonicalArchivePath(file.name)
  const content = markdownDocument(decodeMarkdown(await fileBytes(file)), labels)
  const sources = imageSources(content)
  const externalImageCount = sources.filter(isRemoteImageSource).length
  if (externalImageCount !== sources.length) throw new DocumentMarkdownImportError('image_missing')
  return { title: importTitle(content, markdownPath), content, images: [], externalImageCount }
}

async function parseMarkdownArchive(file: File, labels: RichTextDocumentLabels): Promise<DocumentMarkdownImportBundle> {
  if (file.size <= 0 || file.size > MAX_ARCHIVE_BYTES) {
    throw new DocumentMarkdownImportError('archive_too_large')
  }
  let entryCount = 0
  let uncompressedBytes = 0
  let extracted: Record<string, Uint8Array>
  try {
    extracted = unzipSync(await fileBytes(file), {
      filter: (entry) => {
        entryCount += 1
        if (entryCount > MAX_ARCHIVE_ENTRIES) throw new DocumentMarkdownImportError('archive_too_many_files')
        uncompressedBytes += entry.originalSize
        if (uncompressedBytes > MAX_ARCHIVE_UNCOMPRESSED_BYTES) {
          throw new DocumentMarkdownImportError('archive_uncompressed_too_large')
        }
        canonicalArchivePath(entry.name.replace(/\/$/, ''))
        return !entry.name.endsWith('/')
      },
    })
  } catch (error) {
    if (error instanceof DocumentMarkdownImportError) throw error
    throw new DocumentMarkdownImportError('archive_invalid')
  }

  const files = new Map<string, Uint8Array>()
  const seenCaseFoldedPaths = new Set<string>()
  for (const [rawPath, bytes] of Object.entries(extracted)) {
    const path = canonicalArchivePath(rawPath)
    const folded = path.toLocaleLowerCase()
    if (files.has(path) || seenCaseFoldedPaths.has(folded)) {
      throw new DocumentMarkdownImportError('archive_unsafe_path')
    }
    seenCaseFoldedPaths.add(folded)
    if (path.startsWith('__MACOSX/') || baseName(path) === '.DS_Store') continue
    files.set(path, bytes)
  }
  const markdownPaths = [...files.keys()].filter((path) => ['.md', '.markdown'].includes(extension(path)))
  if (!markdownPaths.length) throw new DocumentMarkdownImportError('markdown_missing')
  if (markdownPaths.length > 1) throw new DocumentMarkdownImportError('markdown_multiple')

  const markdownPath = markdownPaths[0]
  const content = markdownDocument(decodeMarkdown(files.get(markdownPath) as Uint8Array), labels)
  const images = new Map<string, { sources: string[]; archivePath: string; file: File }>()
  let externalImageCount = 0
  for (const source of imageSources(content)) {
    if (isRemoteImageSource(source)) {
      externalImageCount += 1
      continue
    }
    const path = archiveReferencePath(markdownPath, source)
    const bytes = files.get(path)
    if (!bytes) throw new DocumentMarkdownImportError('image_missing')
    const mime = DOCUMENT_IMAGE_MIME_BY_EXTENSION[extension(path)]
    if (!mime || !matchesDocumentImageMime(bytes, mime)) throw new DocumentMarkdownImportError('image_unsupported')
    if (new TextEncoder().encode(baseName(path)).byteLength > 255) {
      throw new DocumentMarkdownImportError('image_unsupported')
    }
    const existing = images.get(path)
    if (existing) existing.sources.push(source)
    else images.set(path, {
      sources: [source],
      archivePath: path,
      file: new File([bytes as BlobPart], baseName(path), { type: mime }),
    })
  }
  return { title: importTitle(content, markdownPath), content, images: [...images.values()], externalImageCount }
}

export async function parseDocumentMarkdownImport(
  file: File,
  labels: RichTextDocumentLabels,
): Promise<DocumentMarkdownImportBundle> {
  const fileExtension = extension(file.name)
  if (fileExtension === '.zip') return parseMarkdownArchive(file, labels)
  if (fileExtension === '.md' || fileExtension === '.markdown') return parsePlainMarkdown(file, labels)
  throw new DocumentMarkdownImportError('markdown_missing')
}

export function rewriteDocumentMarkdownImages(
  content: JSONContent,
  resolved: ReadonlyMap<string, { assetId: number; src: string; alt: string }>,
): JSONContent {
  if (content.type === 'image') {
    const source = typeof content.attrs?.src === 'string' ? content.attrs.src : ''
    const image = resolved.get(source)
    if (!image) return content
    return {
      ...content,
      attrs: {
        ...content.attrs,
        src: image.src,
        alt: typeof content.attrs?.alt === 'string' && content.attrs.alt.trim()
          ? content.attrs.alt
          : image.alt,
        assetId: image.assetId,
      },
    }
  }
  return content.content
    ? { ...content, content: content.content.map((node) => rewriteDocumentMarkdownImages(node, resolved)) }
    : content
}
