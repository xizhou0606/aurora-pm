import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { MarkdownTextField } from './markdown-text-field'

const uiCopy = copy.ui

function renderField(overrides?: Partial<Parameters<typeof MarkdownTextField>[0]>) {
  const onChange = vi.fn()
  const props = { label: '需求描述', name: 'requirement-description', value: '', onChange, ...overrides }
  const view = render(<MarkdownTextField {...props} />)
  return { onChange, container: view.container }
}

function fileInput(container: HTMLElement) {
  return container.querySelector('input[type="file"]') as HTMLInputElement
}

describe('MarkdownTextField', () => {
  it('edits markdown source through the labeled textarea', () => {
    const { onChange } = renderField({ value: '# 标题' })
    const textarea = screen.getByRole('textbox', { name: '需求描述' }) as HTMLTextAreaElement
    expect(textarea.value).toBe('# 标题')
    fireEvent.change(textarea, { target: { value: '## 新内容' } })
    expect(onChange).toHaveBeenCalledWith('## 新内容')
  })

  it('renders a preview instead of the raw source and hides the import action', async () => {
    const { container } = renderField({ value: '# 标题\n\n**加粗**' })
    fireEvent.click(screen.getByRole('button', { name: uiCopy.markdownPreview }))
    const preview = await waitFor(() => {
      const node = container.querySelector('[data-slot="markdown-preview"]')
      expect(node?.querySelector('h1')?.textContent).toBe('标题')
      expect(node?.querySelector('strong')?.textContent).toBe('加粗')
      return node
    })
    expect(preview).not.toBeNull()
    expect(screen.queryByRole('textbox', { name: '需求描述' })).toBeNull()
    expect(screen.queryByRole('button', { name: uiCopy.markdownImport })).toBeNull()
  })

  it('shows the placeholder hint when previewing empty content', () => {
    const { container } = renderField()
    fireEvent.click(screen.getByRole('button', { name: uiCopy.markdownPreview }))
    expect(container.querySelector('[data-slot="markdown-preview"]')).toBeNull()
    expect(container.textContent).toContain(uiCopy.markdownPreviewEmpty)
  })

  it('imports a markdown file into the field', async () => {
    const { onChange, container } = renderField()
    const file = new File(['## 导入内容\n\n- 要点'], '需求.md', { type: 'text/markdown' })
    fireEvent.change(fileInput(container), { target: { files: [file] } })
    await waitFor(() => expect(onChange).toHaveBeenCalledWith('## 导入内容\n\n- 要点'))
  })

  it('surfaces an alert when the import fails', async () => {
    const { container } = renderField()
    const broken = { text: () => Promise.reject(new Error('boom')) } as unknown as File
    fireEvent.change(fileInput(container), { target: { files: [broken] } })
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe(uiCopy.markdownImportFailed))
  })
})
