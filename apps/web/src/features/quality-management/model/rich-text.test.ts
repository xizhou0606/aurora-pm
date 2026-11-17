import { describe, expect, it } from 'bun:test'

import {
  isQualityRichTextEmpty,
  qualityReproductionTemplatePayload,
  qualityRichTextPlainText,
} from './rich-text'

describe('quality reproduction template', () => {
  it('builds the PM standard reproduction sections', () => {
    const payload = qualityReproductionTemplatePayload()
    const text = JSON.stringify(JSON.parse(payload))

    expect(text).toContain('[步骤]')
    expect(text).toContain('1. ')
    expect(text).toContain('[实际结果]')
    expect(text).toContain('[期望结果]')
    expect(qualityReproductionTemplatePayload('unknown')).toBe('')
  })

  it('distinguishes an empty Tiptap document from existing content', () => {
    expect(isQualityRichTextEmpty('')).toBe(true)
    expect(isQualityRichTextEmpty(JSON.stringify({
      type: 'doc', content: [{ type: 'paragraph' }],
    }))).toBe(true)
    expect(isQualityRichTextEmpty(qualityReproductionTemplatePayload())).toBe(false)
    expect(isQualityRichTextEmpty('legacy reproduction steps')).toBe(false)
  })

  it('extracts readable activity text from Tiptap JSON and preserves legacy text', () => {
    expect(qualityRichTextPlainText(JSON.stringify({
      type: 'doc',
      content: [{
        type: 'paragraph',
        content: [
          { type: 'text', text: 'Open checkout' },
          { type: 'hardBreak' },
          { type: 'text', text: 'Submit order' },
        ],
      }],
    }))).toBe('Open checkout\nSubmit order')
    expect(qualityRichTextPlainText('legacy reproduction steps'))
      .toBe('legacy reproduction steps')
    expect(qualityRichTextPlainText(JSON.stringify({
      type: 'doc', content: [{ type: 'image', attrs: { src: 'https://example.com/image.png' } }],
    }), { imageLabel: '[图片]' })).toBe('[图片]')
  })
})
