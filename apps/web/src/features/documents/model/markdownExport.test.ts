import type { JSONContent } from '@tiptap/core'
import { unzipSync } from 'fflate'
import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'bun:test'

import { DOCUMENT_RICH_TEXT_LABELS as labels } from '@/shared/copy/documents'
import { parseRichTextMarkdown } from '@/shared/ui/file-media/rich-text-markdown'
import { buildDocumentMarkdownExport } from './markdownExport'
import { parseDocumentMarkdownImport, rewriteDocumentMarkdownImages } from './markdownImport'

const png = new Blob([new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])], { type: 'image/png' })
const image = (id = 1, documentId = 5): JSONContent => ({ type: 'image', attrs: { assetId: id, src: `/api/v1/platform/documents/${documentId}/assets/${id}/content`, alt: 'preview' } })
const content = (...nodes: JSONContent[]) => JSON.stringify({ type: 'doc', content: nodes })
const exportInput = { documentId: 5, title: 'Release', labels, format: 'zip' as const, loadAsset: vi.fn(async () => png) }
function collect(node: JSONContent, type: string): JSONContent[] {
  return [...(node.type === type ? [node] : []), ...(node.content || []).flatMap((child) => collect(child, type))]
}

describe('document Markdown export', () => {
  it('exports an embedded table as a permission-controlled reference link without fetching table data', async () => {
    const loadAsset = vi.fn()
    const result = await buildDocumentMarkdownExport({ ...exportInput, format: 'markdown', loadAsset, content: content({ type: 'bitable', attrs: { schemaVersion: 1, resourceId: 'resource', tableId: 'table', viewId: 'view', height: 420 } }) })
    expect(await result.blob.text()).toContain('/workspace/bitables/resource?table=table&view=view')
    expect(result.warnings).toContain('simplified_formatting')
    expect(loadAsset).not.toHaveBeenCalled()
  })
  it('roundtrips GFM blocks and inline marks through the same TipTap registry', async () => {
    const markdown = '# Title\n\n#### Deep\n\n- bullet\n  - child\n\n3. order\n4. next\n\n- [ ] todo\n- [x] done\n\n> quote\n\n```js\nconst x = 1\n```\n\n| A | B |\n| --- | --- |\n| **one** | first<br>second |\n\n**bold**, _italic_, ~~strike~~, `code`, [link](https://example.com).\n\n---\n'
    const before = parseRichTextMarkdown(markdown, labels)
    const result = await buildDocumentMarkdownExport({ ...exportInput, content: JSON.stringify(before), format: 'markdown' })
    const after = parseRichTextMarkdown(await result.blob.text(), labels)
    for (const type of ['heading', 'bulletList', 'orderedList', 'taskList', 'taskItem', 'blockquote', 'codeBlock', 'table', 'hardBreak', 'horizontalRule']) {
      expect(collect(after, type).length, type).toBe(collect(before, type).length)
    }
    expect(collect(after, 'text')).toEqual(collect(before, 'text'))
    expect(result.warnings).toEqual([])
  })

  it('bundles unique authenticated assets and roundtrips relative image paths', async () => {
    const loadAsset = vi.fn(async () => png)
    const result = await buildDocumentMarkdownExport({ ...exportInput, content: content(image(), image()), loadAsset })
    const entries = unzipSync(new Uint8Array(await result.blob.arrayBuffer()))
    expect(Object.keys(entries)).toEqual(['assets/1.png', 'Release.md'])
    expect(loadAsset).toHaveBeenCalledTimes(1)
    const markdown = new TextDecoder().decode(entries['Release.md'])
    expect(markdown).not.toContain('/api/')
    const imported = await parseDocumentMarkdownImport(new File([result.blob], result.fileName), labels)
    expect(imported.images).toHaveLength(1)
    expect(collect(imported.content, 'image')).toHaveLength(2)
  })

  it('roundtrips pasted block images between table-cell paragraphs without losing order', async () => {
    const paragraph = (text: string): JSONContent => ({ type: 'paragraph', content: [{ type: 'text', text }] })
    const cell: JSONContent = { type: 'tableCell', attrs: { colspan: 1, rowspan: 1 }, content: [
      paragraph('before image'), image(164), paragraph('after image'),
    ] }
    const table: JSONContent = { type: 'table', content: [
      { type: 'tableRow', content: [{ type: 'tableHeader', content: [paragraph('Screenshot')] }] },
      { type: 'tableRow', content: [cell] },
    ] }
    const loadAsset = vi.fn(async () => png)
    const result = await buildDocumentMarkdownExport({ ...exportInput, content: content(table), loadAsset })
    expect(loadAsset).toHaveBeenCalledWith(164)
    expect(result.warnings).toContain('simplified_formatting')
    const imported = await parseDocumentMarkdownImport(new File([result.blob], result.fileName), labels)
    expect(imported.images).toHaveLength(1)
    const cells = collect(imported.content, 'tableCell')
    expect(cells).toHaveLength(1)
    expect(collect(cells[0], 'image')).toHaveLength(1)
    const orderedContent = JSON.stringify(cells[0])
    expect(orderedContent.indexOf('before image')).toBeLessThan(orderedContent.indexOf('assets/164.png'))
    expect(orderedContent.indexOf('assets/164.png')).toBeLessThan(orderedContent.indexOf('after image'))
    expect(cell.content?.map((node) => node.type)).toEqual(['paragraph', 'image', 'paragraph'])
    for (const unsupported of [{ type: 'bulletList' }, { type: 'table' }]) {
      await expect(buildDocumentMarkdownExport({ ...exportInput, content: content({ ...table, content: [
        { type: 'tableRow', content: [{ ...cell, content: [unsupported] }] },
      ] }) })).rejects.toMatchObject({ code: 'unsupported_content' })
    }
  })

  it('loads copied document assets through the current document callback, not the original source URL', async () => {
    const loadAsset = vi.fn(async () => png)
    await buildDocumentMarkdownExport({ ...exportInput, documentId: 99, content: content(image(4, 5)), loadAsset })
    expect(loadAsset).toHaveBeenCalledTimes(1)
    expect(loadAsset).toHaveBeenCalledWith(4)
    await expect(buildDocumentMarkdownExport({ ...exportInput, documentId: 99, content: content(image(4, 5)), loadAsset: async () => { throw new Error('forbidden') } })).rejects.toMatchObject({ code: 'asset_failed' })
  })

  it.each(['http://localhost:8081', 'https://previous-environment.example'])(
    'bundles canonical absolute PM image URLs without fetching their stored host: %s', async (origin) => {
      const loadAsset = vi.fn(async () => png)
      const privateImage = { type: 'image', attrs: { assetId: 164, src: `${origin}/api/v1/platform/documents/5/assets/164/content` } }
      const result = await buildDocumentMarkdownExport({ ...exportInput, documentId: 99, content: content(privateImage), loadAsset })
      expect(loadAsset).toHaveBeenCalledTimes(1)
      expect(loadAsset).toHaveBeenCalledWith(164)
      expect(result.warnings).toEqual([])
      const entries = unzipSync(new Uint8Array(await result.blob.arrayBuffer()))
      expect(new TextDecoder().decode(entries['Release.md'])).toContain('assets/164.png')
      expect(new TextDecoder().decode(entries['Release.md'])).not.toContain(origin)
      await expect(buildDocumentMarkdownExport({ ...exportInput, content: content(privateImage), format: 'markdown', loadAsset })).rejects.toMatchObject({ code: 'requires_bundle' })
      expect(loadAsset).toHaveBeenCalledTimes(1)
    },
  )

  it.each([
    { src: 'http://localhost:8081/api/v1/platform/documents/5/assets/164/content', assetId: 165 },
    { src: 'http://localhost:8081/api/v1/platform/documents/5/assets/164/content', assetId: null },
    { src: 'https://user:password@example.com/api/v1/platform/documents/5/assets/164/content', assetId: 164 },
    { src: 'http://localhost:8081/api/v1/platform/documents/5/assets/164/content?width=100', assetId: 164 },
    { src: 'http://localhost:8081/api/v1/platform/documents/5/assets/164/content?', assetId: 164 },
    { src: 'http://localhost:8081/api/v1/platform/documents/5/assets/164/content#preview', assetId: 164 },
    { src: 'http://localhost:8081/other/../api/v1/platform/documents/5/assets/164/content', assetId: 164 },
  ])('rejects noncanonical or inconsistent absolute PM asset references: $src', async (attrs) => {
    const loadAsset = vi.fn()
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content({ type: 'image', attrs }), loadAsset })).rejects.toMatchObject({ code: 'unsafe_source' })
    expect(loadAsset).not.toHaveBeenCalled()
  })

  it('rejects a plain Markdown export with private images before making asset requests', async () => {
    const loadAsset = vi.fn()
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content(image()), format: 'markdown', loadAsset })).rejects.toMatchObject({ code: 'requires_bundle' })
    expect(loadAsset).not.toHaveBeenCalled()
  })

  it.each(['javascript:alert(1)', 'data:image/png;base64,xxx', 'blob:https://example.com/a', '//example.com/a', 'https://example.com/a?token=secret', 'https://u:p@example.com/a', '/api/v1/platform/documents/5/assets/2/content?token=x'])(
    'rejects unsafe or credential-bearing sources: %s', async (src) => {
      await expect(buildDocumentMarkdownExport({ ...exportInput, content: content({ type: 'image', attrs: { src } }) })).rejects.toMatchObject({ code: 'unsafe_source' })
    },
  )

  it('rejects asset ID mismatches and unsafe links', async () => {
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content({ ...image(), attrs: { ...image().attrs, assetId: 9 } }) })).rejects.toMatchObject({ code: 'unsafe_source' })
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content({ type: 'paragraph', content: [{ type: 'text', text: 'link', marks: [{ type: 'link', attrs: { href: 'https://example.com?X-Amz-Signature=secret' } }] }] }) })).rejects.toMatchObject({ code: 'unsafe_source' })
  })

  it('retains public images as links, reports portability warning and never downloads them', async () => {
    const loadAsset = vi.fn()
    const result = await buildDocumentMarkdownExport({ ...exportInput, content: content({ type: 'image', attrs: { src: 'https://example.com/public.png' } }), format: 'markdown', loadAsset })
    expect(result.warnings).toEqual(['external_images'])
    expect(await result.blob.text()).toContain('https://example.com/public.png')
    expect(loadAsset).not.toHaveBeenCalled()
  })

  it('escapes image and link attribute punctuation instead of injecting Markdown or changing image count', async () => {
    const alt = 'caption ](https://unexpected.example) [bracket]'
    const result = await buildDocumentMarkdownExport({ ...exportInput, content: content(
      { type: 'image', attrs: { src: 'https://example.com/a(b).png', alt, title: 'title "quoted"' } },
    ), format: 'markdown' })
    const parsed = parseRichTextMarkdown(await result.blob.text(), labels)
    expect(collect(parsed, 'image')).toHaveLength(1)
    expect(collect(parsed, 'image')[0].attrs).toMatchObject({ alt, title: 'title "quoted"', src: 'https://example.com/a%28b%29.png' })
  })

  it('warns explicitly for nonportable formatting and fails closed for unknown blocks and merged tables', async () => {
    const result = await buildDocumentMarkdownExport({ ...exportInput, format: 'markdown', content: content(
      { type: 'callout', content: [{ type: 'text', text: 'Important' }] },
      { type: 'tableOfContents' },
      { type: 'paragraph', content: [{ type: 'mention', attrs: { label: 'Ada' } }, { type: 'text', text: ' underline', marks: [{ type: 'underline' }] }] },
    ) })
    expect(result.warnings).toEqual(['simplified_formatting'])
    expect(await result.blob.text()).toContain('> Important')
    expect(await result.blob.text()).toContain('@Ada underline')
    for (const node of [{ type: 'unsupportedBlock' }, { type: 'futureWidget' }, { type: 'tableCell', attrs: { colspan: 2 } }]) {
      await expect(buildDocumentMarkdownExport({ ...exportInput, content: content(node) })).rejects.toMatchObject({ code: 'unsupported_content' })
    }
  })

  it('fails atomically on missing images and mismatched MIME bytes', async () => {
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content(image()), loadAsset: async () => new Blob(['not-png'], { type: 'image/png' }) })).rejects.toMatchObject({ code: 'image_unsupported' })
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content(image()), loadAsset: async () => { throw new Error('network') } })).rejects.toMatchObject({ code: 'asset_failed' })
  })

  it('produces safe UTF-8 file names and rejects excessive asset count', async () => {
    const result = await buildDocumentMarkdownExport({ ...exportInput, title: '../bad\\name\u0000', content: content() })
    expect(result.fileName).not.toMatch(/[\/\\\u0000]/)
    const entries = unzipSync(new Uint8Array(await result.blob.arrayBuffer()))
    expect(Object.keys(entries)).toHaveLength(1)
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content(...Array.from({ length: 512 }, (_, i) => image(i + 1))) })).rejects.toMatchObject({ code: 'archive_too_many_files' })
  })

  it('exports empty documents and preserves safe relative and Unicode fragment links', async () => {
    const empty = await buildDocumentMarkdownExport({ ...exportInput, title: 'CON', content: content(), format: 'markdown' })
    expect(empty.fileName).toBe('document-5.md')
    expect(await empty.blob.text()).toBe('\n')
    await expect(parseDocumentMarkdownImport(new File([empty.blob], empty.fileName), labels)).resolves.toMatchObject({ images: [] })
    const result = await buildDocumentMarkdownExport({ ...exportInput, format: 'markdown', content: content({ type: 'paragraph', content: [
      { type: 'text', text: 'relative', marks: [{ type: 'link', attrs: { href: '../notes.md' } }] },
      { type: 'text', text: 'fragment', marks: [{ type: 'link', attrs: { href: '#章节' } }] },
    ] }) })
    expect(await result.blob.text()).toContain('../notes.md')
    expect(await result.blob.text()).toContain('#章节')
  })

  it('enforces Markdown and image size limits before returning a partial artifact', async () => {
    await expect(buildDocumentMarkdownExport({ ...exportInput, format: 'markdown', content: content({ type: 'paragraph', content: [{ type: 'text', text: 'x'.repeat(4 * 1024 * 1024 + 1) }] }) })).rejects.toMatchObject({ code: 'markdown_too_large' })
    const oversizedImage = new Blob([], { type: 'image/png' })
    Object.defineProperty(oversizedImage, 'size', { value: 32 * 1024 * 1024 + 1 })
    await expect(buildDocumentMarkdownExport({ ...exportInput, content: content(image()), loadAsset: async () => oversizedImage })).rejects.toMatchObject({ code: 'image_unsupported' })
  })

  const corpusPath = process.env.PM_MARKDOWN_IMPORT_CORPUS
  it.skipIf(!(Boolean(corpusPath)))('roundtrips all 54 images and 40 headings in the supplied requirement archive', async () => {
    const initial = await parseDocumentMarkdownImport(new File([readFileSync(corpusPath as string)], 'requirement.zip'), labels)
    const assets = new Map<number, Blob>()
    const resolved = new Map<string, { assetId: number; src: string; alt: string }>()
    initial.images.forEach((entry, index) => {
      const id = index + 1
      assets.set(id, entry.file)
      entry.sources.forEach((src) => resolved.set(src, { assetId: id, src: `/api/v1/platform/documents/5/assets/${id}/content`, alt: entry.file.name }))
    })
    const result = await buildDocumentMarkdownExport({ ...exportInput, title: initial.title, content: JSON.stringify(rewriteDocumentMarkdownImages(initial.content, resolved)), loadAsset: async (id) => assets.get(id)! })
    const roundtrip = await parseDocumentMarkdownImport(new File([result.blob], result.fileName), labels)
    expect(roundtrip.images).toHaveLength(54)
    expect(collect(roundtrip.content, 'heading')).toHaveLength(40)
    const beforeText = collect(initial.content, 'text')
    const afterText = collect(roundtrip.content, 'text')
    expect(afterText).toHaveLength(beforeText.length)
    beforeText.forEach((node, index) => {
      // GFM trims insignificant trailing table-cell whitespace; marks and all other text remain.
      expect({ ...afterText[index], text: afterText[index].text?.trimEnd() }, `text ${index}`)
        .toEqual({ ...node, text: node.text?.trimEnd() })
    })
    expect(collect(roundtrip.content, 'codeBlock')).toEqual(collect(initial.content, 'codeBlock'))
    for (const nodeType of ['bulletList', 'orderedList', 'blockquote', 'codeBlock', 'table', 'image']) {
      expect(collect(roundtrip.content, nodeType).length, nodeType).toBe(collect(initial.content, nodeType).length)
    }
    expect(roundtrip.images.map((entry) => entry.file.size)).toEqual(initial.images.map((entry) => entry.file.size))
  })
})
