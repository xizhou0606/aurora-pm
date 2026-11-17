import { describe, expect, it } from 'bun:test'

import {
  canonicalRichTextValue,
  isRichTextEmpty,
  parseRichTextValue,
  richTextVisibleCharacterCount,
  serializeRichTextValue,
} from '../edit/rich-text-serialization'

describe('rich-text serialization', () => {
  it('keeps HTML as the default editor contract', () => {
    expect(parseRichTextValue('<p>Hello</p>', 'html')).toBe('<p>Hello</p>')
    expect(serializeRichTextValue({
      getHTML: () => '<p>Updated</p>',
      getJSON: () => ({ type: 'doc' }),
    }, 'html')).toBe('<p>Updated</p>')
  })

  it('parses and emits canonical Tiptap JSON', () => {
    const document = { type: 'doc', content: [{ type: 'paragraph' }] }
    expect(parseRichTextValue(JSON.stringify(document), 'json')).toEqual(document)
    expect(serializeRichTextValue({
      getHTML: () => '<p>Ignored</p>',
      getJSON: () => document,
    }, 'json')).toBe(JSON.stringify(document))
  })

  it('wraps legacy plain text in a Tiptap document', () => {
    expect(canonicalRichTextValue('First line\nSecond line', 'json')).toBe(JSON.stringify({
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          content: [{ type: 'text', text: 'First line' }],
        },
        {
          type: 'paragraph',
          content: [{ type: 'text', text: 'Second line' }],
        },
      ],
    }))
  })

  it('imports legacy HTML instead of displaying its source as JSON text', () => {
    expect(parseRichTextValue('<p><strong>Legacy</strong> HTML</p>', 'json'))
      .toBe('<p><strong>Legacy</strong> HTML</p>')
    expect(serializeRichTextValue({
      getHTML: () => '<p><strong>Legacy</strong> HTML</p>',
      getJSON: () => ({
        type: 'doc',
        content: [{
          type: 'paragraph',
          content: [{ type: 'text', marks: [{ type: 'bold' }], text: 'Legacy' }],
        }],
      }),
    }, 'json')).toBe(JSON.stringify({
      type: 'doc',
      content: [{
        type: 'paragraph',
        content: [{ type: 'text', marks: [{ type: 'bold' }], text: 'Legacy' }],
      }],
    }))
  })

  it('treats empty HTML, placeholder paragraphs and image-only HTML correctly', () => {
    expect(isRichTextEmpty('', 'html')).toBe(true)
    expect(isRichTextEmpty('<p></p>', 'html')).toBe(true)
    expect(isRichTextEmpty('<p><br></p>', 'html')).toBe(true)
    expect(isRichTextEmpty('<p>补充边界</p>', 'html')).toBe(false)
    expect(isRichTextEmpty('<p><img src="https://cdn.example.test/shot.png" alt="截图"></p>', 'html')).toBe(false)
    expect(isRichTextEmpty(JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph' }],
    }), 'json')).toBe(true)
    expect(isRichTextEmpty(JSON.stringify({
      type: 'doc',
      content: [{ type: 'image', attrs: { src: 'https://cdn.example.test/shot.png' } }],
    }), 'json')).toBe(false)
  })

  it('counts visible Unicode characters instead of serialized JSON metadata', () => {
    const value = JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: '说明😀' }] },
        { type: 'image', attrs: { src: 'https://cdn.example.test/very-long-image-name.png' } },
      ],
    })
    expect(value.length).toBeGreaterThan(20)
    expect(richTextVisibleCharacterCount(value)).toBe(9)
    expect(richTextVisibleCharacterCount('普通说明')).toBe(4)
  })

  it('treats a structured mention as visible rich-text content', () => {
    const value = JSON.stringify({
      type: 'doc',
      content: [{
        type: 'paragraph',
        content: [{ type: 'mention', attrs: { id: '47', label: '铁奥', username: 'teo' } }],
      }],
    })
    expect(isRichTextEmpty(value, 'json')).toBe(false)
    expect(richTextVisibleCharacterCount(value)).toBe(3)
  })
})
