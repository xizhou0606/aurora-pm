import type { JSONContent } from '@tiptap/core'
import { zipSync } from 'fflate'
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'bun:test'

import { DOCUMENT_RICH_TEXT_LABELS } from '@/shared/copy/documents'
import {
  DocumentMarkdownImportError,
  parseDocumentMarkdownImport,
  rewriteDocumentMarkdownImages,
} from './markdownImport'

const encoder = new TextEncoder()
const png = new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10])

function nodeTypes(node: JSONContent): string[] {
  return [node.type || '', ...(node.content || []).flatMap(nodeTypes)]
}

function markTypes(node: JSONContent): string[] {
  return [
    ...(node.marks || []).map((mark) => mark.type || ''),
    ...(node.content || []).flatMap(markTypes),
  ]
}

function imageNodes(node: JSONContent): JSONContent[] {
  return [
    ...(node.type === 'image' ? [node] : []),
    ...(node.content || []).flatMap(imageNodes),
  ]
}

describe('document Markdown import', () => {
  const goldenCorpusPath = process.env.PM_MARKDOWN_IMPORT_CORPUS

  it.skipIf(!(Boolean(goldenCorpusPath)))('parses the supplied product requirement ZIP as the golden corpus', async () => {
    const archive = readFileSync(goldenCorpusPath as string)
    const result = await parseDocumentMarkdownImport(
      new File([archive], 'v1.4-后台需求-功能迭代.zip', { type: 'application/zip' }),
      DOCUMENT_RICH_TEXT_LABELS,
    )
    const types = nodeTypes(result.content)
    expect(result.images).toHaveLength(54)
    expect(types.filter((type) => type === 'heading')).toHaveLength(40)
    expect(types).toEqual(expect.arrayContaining([
      'bulletList', 'orderedList', 'blockquote', 'codeBlock', 'table', 'image',
    ]))
    const resolved = new Map<string, { assetId: number; src: string; alt: string }>()
    result.images.forEach((image, index) => {
      image.sources.forEach((source) => resolved.set(source, {
        assetId: index + 1,
        src: `/api/v1/platform/documents/99/assets/${index + 1}/content`,
        alt: image.file.name,
      }))
    })
    const rewrittenImages = imageNodes(rewriteDocumentMarkdownImages(result.content, resolved))
    expect(rewrittenImages).toHaveLength(54)
    expect(rewrittenImages.every((image) => Number(image.attrs?.assetId) > 0)).toBe(true)
    expect(rewrittenImages.some((image) => String(image.attrs?.src).startsWith('图片和附件/'))).toBe(false)
  })

  it('parses standard GFM blocks and resolves percent-encoded bundled images', async () => {
    const markdown = `# 发布说明

#### 四级标题

- 无序列表
  - 嵌套项目

1. 第一步
2. 第二步

- [ ] 待处理
- [x] 已完成

> 引用内容

\`\`\`Plain Text
code block
\`\`\`

| 字段 | 说明 |
| --- | --- |
| **名称** | 第一行<br>第二行 |

包含 \`inline code\`、**bold**、_italic_、~~strike~~ 和 [link](https://example.com)。

---

![截图](图片和附件/image%201.png)
`
    const archive = zipSync({
      '发布说明.md': encoder.encode(markdown),
      '图片和附件/image 1.png': png,
    })
    const result = await parseDocumentMarkdownImport(
      new File([archive], 'release.zip', { type: 'application/zip' }),
      DOCUMENT_RICH_TEXT_LABELS,
    )

    expect(result.title).toBe('发布说明')
    expect(result.images).toHaveLength(1)
    expect(result.images[0]).toMatchObject({
      sources: ['图片和附件/image%201.png'],
      archivePath: '图片和附件/image 1.png',
    })
    expect(nodeTypes(result.content)).toEqual(expect.arrayContaining([
      'heading', 'bulletList', 'orderedList', 'taskList', 'taskItem',
      'blockquote', 'codeBlock', 'table', 'tableHeader', 'tableCell', 'hardBreak',
      'horizontalRule', 'image',
    ]))
    expect(markTypes(result.content)).toEqual(expect.arrayContaining([
      'bold', 'italic', 'strike', 'code', 'link',
    ]))
  })

  it('uses the file name when the Markdown has no H1 and permits remote images without fetching them', async () => {
    const result = await parseDocumentMarkdownImport(
      new File([encoder.encode('## Section\n\n![remote](https://example.com/image.png)')], 'notes.md', { type: 'text/markdown' }),
      DOCUMENT_RICH_TEXT_LABELS,
    )
    expect(result.title).toBe('notes')
    expect(result.externalImageCount).toBe(1)
    expect(result.images).toEqual([])
  })

  it('fails closed for missing local assets and unsafe archive paths', async () => {
    await expect(parseDocumentMarkdownImport(
      new File([encoder.encode('![missing](assets/missing.png)')], 'notes.md', { type: 'text/markdown' }),
      DOCUMENT_RICH_TEXT_LABELS,
    )).rejects.toMatchObject({ code: 'image_missing' })

    const unsafe = zipSync({ '../escape.md': encoder.encode('# Unsafe') })
    await expect(parseDocumentMarkdownImport(
      new File([unsafe], 'unsafe.zip', { type: 'application/zip' }),
      DOCUMENT_RICH_TEXT_LABELS,
    )).rejects.toBeInstanceOf(DocumentMarkdownImportError)
  })

  it('rewrites uploaded image nodes to authenticated document assets', () => {
    const content: JSONContent = {
      type: 'doc',
      content: [{ type: 'image', attrs: { src: 'assets/image.png', alt: 'old' } }],
    }
    expect(rewriteDocumentMarkdownImages(content, new Map([[
      'assets/image.png',
      { assetId: 9, src: '/api/v1/platform/documents/4/assets/9/content', alt: 'image.png' },
    ]]))).toEqual({
      type: 'doc',
      content: [{
        type: 'image',
        attrs: {
          src: '/api/v1/platform/documents/4/assets/9/content',
          alt: 'old',
          assetId: 9,
        },
      }],
    })
  })
})
