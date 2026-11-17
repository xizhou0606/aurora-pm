import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import {
  InlineDateField,
  InlineEstimatedField,
  InlineSelectField,
  InlineTextField,
  InlineUserField,
} from './InlineField'

describe('Work-items inline editor lifecycle', () => {
  it('saves a changed title on blur and cancels with Escape', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineTextField
        id="title"
        label="任务"
        rowTitle="任务一"
        value="任务一"
        display="任务一"
        editable
        mutation={{ status: 'idle' }}
        maxLength={200}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '任务：任务一' }))
    const input = screen.getByRole('textbox', { name: '任务：任务一' })
    fireEvent.change(input, { target: { value: '失焦提交' } })
    fireEvent.blur(input)
    await waitFor(() => expect(onSave).toHaveBeenCalledWith('失焦提交'))

    fireEvent.click(screen.getByRole('button', { name: '任务：任务一' }))
    fireEvent.keyDown(screen.getByRole('textbox', { name: '任务：任务一' }), { key: 'Escape' })
    expect(screen.getByRole('button', { name: '任务：任务一' })).toBeTruthy()
    expect(onSave).toHaveBeenCalledTimes(1)
  })

  it('keeps the same stable control frame in read and edit modes', () => {
    render(
      <InlineUserField
        id="assignee-stable-frame"
        label="经办人"
        rowTitle="任务一"
        value={7}
        displayName="Teo"
        editable
        mutation={{ status: 'idle' }}
        options={[{ id: 7, label: 'Teo' }]}
        onSave={vi.fn().mockResolvedValue(true)}
      />,
    )

    const readControl = screen.getByRole('button', { name: '经办人：任务一' })
    expect(readControl.getAttribute('data-inline-field-control')).toBe('true')
    expect(readControl.className).toContain('h-8')
    expect(readControl.className).toContain('w-full')
    expect(readControl.className).toContain('min-w-0')

    fireEvent.click(readControl)
    const editControl = screen.getByRole('combobox', { name: '经办人：任务一' })
      .closest('[data-inline-field-control="true"]')
    expect(editControl).toBeTruthy()
    expect(editControl?.className).toContain('h-8')
    expect(editControl?.className).toContain('w-full')
    expect(editControl?.className).toContain('min-w-0')
  })

  it('returns to read mode when a select popup loses focus without a change', async () => {
    render(
      <InlineSelectField
        id="priority"
        label="优先级"
        rowTitle="任务一"
        value="normal"
        display="普通"
        editable
        mutation={{ status: 'idle' }}
        options={[{ value: 'normal', label: '普通' }, { value: 'high', label: '高' }]}
        onSave={vi.fn().mockResolvedValue(true)}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '优先级：任务一' }))
    const select = screen.getByRole('combobox', { name: '优先级：任务一' })
    fireEvent.click(select)
    await screen.findByRole('option', { name: '高' })
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })

    await waitFor(() => expect(screen.queryByRole('combobox', { name: '优先级：任务一' })).toBeNull())
    expect(screen.getByRole('button', { name: '优先级：任务一' })).toBeTruthy()
  })

  it('returns untouched select and user editors to view mode on an outside click', () => {
    const saveUser = vi.fn().mockResolvedValue(true)
    render(
      <>
        <InlineSelectField
          id="priority-outside"
          label="优先级"
          rowTitle="任务一"
          value="normal"
          display="普通"
          editable
          mutation={{ status: 'idle' }}
          options={[{ value: 'normal', label: '普通' }]}
          onSave={saveUser}
        />
        <InlineUserField
          id="assignee-outside"
          label="经办人"
          rowTitle="任务一"
          value={7}
          displayName="Teo"
          editable
          mutation={{ status: 'idle' }}
          options={[{ id: 7, label: 'Teo' }]}
          onSave={vi.fn().mockResolvedValue(true)}
        />
      </>,
    )

    fireEvent.click(screen.getByRole('button', { name: '优先级：任务一' }))
    expect(screen.getByRole('combobox', { name: '优先级：任务一' })).toBeTruthy()
    fireEvent.pointerDown(document.body)
    expect(screen.getByRole('button', { name: '优先级：任务一' })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: '经办人：任务一' }))
    expect(screen.getByRole('combobox', { name: '经办人：任务一' })).toBeTruthy()
    expect(saveUser).not.toHaveBeenCalled()
    fireEvent.pointerDown(document.body)
    expect(screen.getByRole('button', { name: '经办人：任务一' })).toBeTruthy()
    expect(saveUser).not.toHaveBeenCalled()
  })

  it('does not clear an assignee when Escape closes the untouched editor', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineUserField
        id="assignee-escape"
        label="经办人"
        rowTitle="任务一"
        value={73}
        displayName="Koko"
        editable
        mutation={{ status: 'idle' }}
        options={[{ id: 73, label: 'Koko', username: 'koko' }]}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '经办人：任务一' }))
    const input = screen.getByRole('combobox', { name: '经办人：任务一' })
    expect((input as HTMLInputElement).value).toBe('Koko')
    fireEvent.click(input)
    expect(screen.queryByRole('option', { name: '未分配' })).toBeNull()
    fireEvent.keyDown(input, { key: 'Escape' })

    await waitFor(() => expect(screen.queryByRole('combobox', { name: '经办人：任务一' })).toBeNull())
    expect(onSave).not.toHaveBeenCalled()
  })

  it('keeps a submitted date pending and retains the editor on failure', async () => {
    let resolveSave: ((saved: boolean) => void) | undefined
    const onSave = vi.fn(() => new Promise<boolean>((resolve) => { resolveSave = resolve }))
    render(
      <InlineDateField
        id="due"
        label="截止"
        rowTitle="任务一"
        value="2026-08-12"
        display="2026/08/12"
        editable
        mutation={{ status: 'idle' }}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '截止：任务一' }))
    fireEvent.click(await screen.findByRole('button', { name: '选择 2026年8月20日' }))

    expect(onSave).toHaveBeenCalledWith('2026-08-20')
    expect((screen.getByLabelText('截止：任务一') as HTMLInputElement).disabled).toBe(true)
    resolveSave?.(false)
    await waitFor(() => expect((screen.getByLabelText('截止：任务一') as HTMLInputElement).disabled).toBe(false))

    fireEvent.keyDown(screen.getByLabelText('截止：任务一'), { key: 'Escape' })
    expect(screen.getByRole('button', { name: '截止：任务一' })).toBeTruthy()
  })

  it('keeps date and estimated-time editors open when their inputs are clicked directly', () => {
    render(
      <>
        <InlineDateField
          id="due-direct-edit"
          label="截止"
          rowTitle="任务一"
          value="2026-08-12"
          display="2026/08/12"
          editable
          mutation={{ status: 'idle' }}
          onSave={vi.fn().mockResolvedValue(true)}
        />
        <InlineEstimatedField
          id="estimate-direct-edit"
          label="工时"
          rowTitle="任务一"
          value={60}
          display="1 小时"
          editable
          mutation={{ status: 'idle' }}
          onSave={vi.fn().mockResolvedValue(true)}
        />
      </>,
    )

    fireEvent.click(screen.getByRole('button', { name: '截止：任务一' }))
    const date = screen.getByLabelText('截止：任务一')
    fireEvent.pointerDown(date)
    expect(screen.getByLabelText('截止：任务一')).toBe(date)

    fireEvent.click(screen.getByRole('button', { name: '工时：任务一' }))
    const estimate = screen.getByLabelText('工时：任务一')
    fireEvent.pointerDown(estimate)
    expect(screen.getByLabelText('工时：任务一')).toBe(estimate)
  })

  it('saves a selected date only once even after an outside pointerdown', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineDateField
        id="due-outside-commit"
        label="截止"
        rowTitle="任务一"
        value="2026-08-12"
        display="2026/08/12"
        editable
        mutation={{ status: 'idle' }}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '截止：任务一' }))
    fireEvent.click(await screen.findByRole('button', { name: '选择 2026年8月20日' }))
    fireEvent.pointerDown(document.body)

    await waitFor(() => expect(onSave).toHaveBeenCalledWith('2026-08-20'))
  })

  it('submits null when the calendar date is cleared', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineDateField
        id="due-clear"
        label="截止"
        rowTitle="任务一"
        value="2026-08-12"
        display="2026/08/12"
        editable
        mutation={{ status: 'idle' }}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '截止：任务一' }))
    fireEvent.click(await screen.findByRole('button', { name: '清空' }))

    await waitFor(() => expect(onSave).toHaveBeenCalledWith(null))
  })

  it('saves changed estimated hours on blur', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineEstimatedField
        id="estimate-blur"
        label="工时"
        rowTitle="任务一"
        value={60}
        display="1 小时"
        editable
        mutation={{ status: 'idle' }}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '工时：任务一' }))
    const input = screen.getByLabelText('工时：任务一')
    fireEvent.change(input, { target: { value: '1.5' } })
    fireEvent.blur(input)

    await waitFor(() => expect(onSave).toHaveBeenCalledWith(90))
  })

  it('keeps a select editor mounted while its popup closes during a pending save', async () => {
    let resolveSave: ((saved: boolean) => void) | undefined
    const onSave = vi.fn(() => new Promise<boolean>((resolve) => { resolveSave = resolve }))
    render(
      <InlineSelectField
        id="priority-pending"
        label="优先级"
        rowTitle="任务二"
        value="normal"
        display="普通"
        editable
        mutation={{ status: 'idle' }}
        options={[{ value: 'normal', label: '普通' }, { value: 'high', label: '高' }]}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '优先级：任务二' }))
    fireEvent.click(screen.getByRole('combobox', { name: '优先级：任务二' }))
    const highOption = await screen.findByRole('option', { name: '高' })
    fireEvent.pointerDown(highOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(highOption)

    expect(onSave).toHaveBeenCalledWith('high')
    expect((screen.getByRole('combobox', { name: '优先级：任务二' }) as HTMLButtonElement).disabled)
      .toBe(true)
    await act(async () => resolveSave?.(false))
    await waitFor(() => expect(
      (screen.getByRole('combobox', { name: '优先级：任务二' }) as HTMLButtonElement).disabled,
    ).toBe(false))

    fireEvent.keyDown(screen.getByRole('combobox', { name: '优先级：任务二' }), { key: 'Escape' })
    expect(screen.getByRole('button', { name: '优先级：任务二' })).toBeTruthy()
  })

  it('discards an invalid estimated-hours draft on blur but keeps it for Enter correction', () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <InlineEstimatedField
        id="estimate"
        label="工时"
        rowTitle="任务三"
        value={60}
        display="1 小时"
        editable
        mutation={{ status: 'idle' }}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: '工时：任务三' }))
    let input = screen.getByLabelText('工时：任务三')
    fireEvent.change(input, { target: { value: '16667' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.getByLabelText('工时：任务三')).toBeTruthy()

    input = screen.getByLabelText('工时：任务三')
    fireEvent.blur(input)
    expect(screen.getByRole('button', { name: '工时：任务三' })).toBeTruthy()
    expect(onSave).not.toHaveBeenCalled()
  })
})
