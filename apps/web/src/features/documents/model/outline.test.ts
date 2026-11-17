import { describe, expect, it } from 'bun:test'

import { deriveDocumentOutline } from './outline'

describe('document outline model', () => {
  it('returns no second fact for invalid, empty, or heading-free content', () => {
    expect(deriveDocumentOutline('not json')).toEqual([])
    expect(deriveDocumentOutline({ type: 'doc', content: [] })).toEqual([])
    expect(deriveDocumentOutline({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: '正文' }] }],
    })).toEqual([])
  })

  it('derives hierarchy and ProseMirror positions across skipped levels', () => {
    const outline = deriveDocumentOutline({
      type: 'doc',
      content: [
        { type: 'heading', attrs: { level: 1 }, content: [{ type: 'text', text: '范围' }] },
        { type: 'paragraph', content: [{ type: 'text', text: '正文' }] },
        { type: 'heading', attrs: { level: 3 }, content: [{ type: 'text', text: '约束' }] },
        { type: 'heading', attrs: { level: 2 }, content: [{ type: 'text', text: '方案' }] },
        { type: 'heading', attrs: { level: 3 }, content: [{ type: 'text', text: '细节' }] },
      ],
    })

    expect(outline).toMatchObject([{
      target: { text: '范围', level: 1, position: 0 },
      children: [
        { target: { text: '约束', level: 3, position: 8 } },
        {
          target: { text: '方案', level: 2, position: 12 },
          children: [{ target: { text: '细节', level: 3, position: 16 } }],
        },
      ],
    }])
  })

  it('keeps duplicate titles addressable and reads canonical inline labels', () => {
    const outline = deriveDocumentOutline(JSON.stringify({
      type: 'doc',
      content: [
        { type: 'heading', attrs: { level: 2, id: 'decision' }, content: [{ type: 'text', text: '决定' }] },
        {
          type: 'heading',
          attrs: { level: 2, id: 'decision' },
          content: [
            { type: 'text', text: '决定 ' },
            { type: 'mention', attrs: { id: '8', label: 'Teo' } },
          ],
        },
      ],
    }))

    expect(outline.map(({ target }) => target.text)).toEqual(['决定', '决定 Teo'])
    expect(outline[0].target.anchor).toBe('decision')
    expect(outline[1].target.anchor).toBe('decision')
    expect(outline[0].target.id).not.toBe(outline[1].target.id)
    expect(outline.map(({ target }) => target.position)).toEqual([0, 4])
  })

  it('keeps heading positions aligned after empty text blocks', () => {
    const outline = deriveDocumentOutline({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: 'A' }] },
        { type: 'paragraph' },
        { type: 'paragraph' },
        { type: 'heading', attrs: { level: 2 }, content: [{ type: 'text', text: '目标' }] },
      ],
    })

    expect(outline[0].target).toMatchObject({ text: '目标', position: 7 })
  })

  it('keeps heading positions aligned after nested table content', () => {
    const outline = deriveDocumentOutline({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: 'A' }] },
        {
          type: 'table',
          content: [{
            type: 'tableRow',
            content: [
              { type: 'tableHeader', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'B' }] }] },
              { type: 'tableCell', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'C' }] }] },
            ],
          }],
        },
        { type: 'heading', attrs: { level: 2 }, content: [{ type: 'text', text: '表格后的标题' }] },
      ],
    })

    expect(outline[0].target).toMatchObject({ text: '表格后的标题', position: 17 })
  })
})
