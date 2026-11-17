import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { InlineText } from './InlineField'

describe('Quality list inline fields', () => {
  it('saves a changed trimmed title on blur', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    render(
      <InlineText
        id="quality-title"
        label="标题：原始标题"
        value="原始标题"
        editable
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '标题：原始标题' }))
    const input = screen.getByRole('textbox', { name: '标题：原始标题' })
    fireEvent.change(input, { target: { value: '  更新标题  ' } })
    fireEvent.blur(input)

    await waitFor(() => expect(onSave).toHaveBeenCalledWith('更新标题'))
  })

  it('cancels an unchanged or blank title without saving', () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    render(
      <InlineText
        id="quality-title-cancel"
        label="标题：原始标题"
        value="原始标题"
        editable
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '标题：原始标题' }))
    fireEvent.blur(screen.getByRole('textbox', { name: '标题：原始标题' }))
    expect(onSave).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: '标题：原始标题' }))
    const input = screen.getByRole('textbox', { name: '标题：原始标题' })
    fireEvent.change(input, { target: { value: '   ' } })
    fireEvent.blur(input)
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.getByRole('textbox', { name: '标题：原始标题' })).toBeTruthy()
  })
})
