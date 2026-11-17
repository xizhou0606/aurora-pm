import { describe, expect, it } from 'bun:test'

import { renderMarkdown } from './markdown'

describe('renderMarkdown', () => {
  it('renders GFM structures such as headings, lists and tables', () => {
    const html = renderMarkdown('# 标题\n\n- 项目 A\n- 项目 B\n\n| 列一 | 列二 |\n| --- | --- |\n| a | b |')
    expect(html).toContain('<h1')
    expect(html).toContain('<ul>')
    expect(html).toContain('<table>')
  })

  it('keeps single newlines visible as line breaks', () => {
    const html = renderMarkdown('第一行\n第二行')
    expect(html).toContain('<br')
  })

  it('strips scripts, event handlers and inline styles from untrusted source', () => {
    const html = renderMarkdown('<script>alert(1)</script><img src=x onerror=alert(1)><p style="color:red">文案</p>')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('onerror')
    expect(html).not.toContain('style=')
    expect(html).toContain('文案')
  })

  it('hardens rendered links with target and rel', () => {
    const html = renderMarkdown('[文档](https://example.com)')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer nofollow"')
  })
})
