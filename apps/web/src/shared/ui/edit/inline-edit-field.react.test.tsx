import { createRef } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { InlineEditText, InlineEditTrigger } from './inline-edit-field'

describe('InlineEditText', () => {
  it('keeps editable and read-only display geometry aligned', () => {
    render(
      <>
        <InlineEditTrigger
          label="可编辑跟进人"
          display={<span data-testid="editable-owner">wayne</span>}
          editable
          triggerRef={createRef<HTMLButtonElement>()}
          onOpen={vi.fn()}
        />
        <InlineEditTrigger
          label="只读跟进人"
          display={<span data-testid="readonly-owner">Feng</span>}
          editable={false}
          triggerRef={createRef<HTMLButtonElement>()}
          onOpen={vi.fn()}
        />
      </>,
    )

    const editable = screen.getByTestId('editable-owner').closest('[data-inline-field-control]')
    const readOnly = screen.getByTestId('readonly-owner').closest('[data-inline-field-control]')
    for (const className of [
      'h-8',
      'w-full',
      'items-center',
      'justify-start',
      'border',
      'border-transparent',
      'px-2',
    ]) {
      expect(editable?.classList.contains(className)).toBe(true)
      expect(readOnly?.classList.contains(className)).toBe(true)
    }
  })

  it('uses a normal read style and saves a changed value on blur', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineEditText
        id="shared-inline-title"
        label="编辑标题"
        value="常态标题"
        editable
        onSave={onSave}
      />,
    )

    const trigger = screen.getByRole('button', { name: '编辑标题' })
    expect(trigger.className).toContain('font-normal')
    expect(trigger.className).not.toContain('text-primary')

    fireEvent.click(trigger)
    fireEvent.change(screen.getByRole('textbox', { name: '编辑标题' }), {
      target: { value: '失焦提交' },
    })
    fireEvent.blur(screen.getByRole('textbox', { name: '编辑标题' }))
    await waitFor(() => expect(onSave).toHaveBeenCalledWith('失焦提交'))
    await waitFor(() => expect(screen.getByRole('button', { name: '编辑标题' })).toBeTruthy())
  })

  it('cancels an unchanged value on blur and a changed value on Escape', () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineEditText
        id="shared-inline-title-cancel"
        label="编辑标题"
        value="常态标题"
        editable
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '编辑标题' }))
    fireEvent.blur(screen.getByRole('textbox', { name: '编辑标题' }))
    expect(onSave).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: '编辑标题' }))
    const input = screen.getByRole('textbox', { name: '编辑标题' })
    fireEvent.change(input, { target: { value: '取消标题' } })
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: '编辑标题' }).textContent).toBe('常态标题')
  })
})
