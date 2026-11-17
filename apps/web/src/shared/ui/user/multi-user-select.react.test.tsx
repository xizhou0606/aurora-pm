import { useState } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { MultiUserSelect } from './multi-user-select'
import type { UserSelectOption } from './user-select-option'

const teo = { id: 7, name: '小龙', username: 'teo', avatarUrl: '/teo.png' }
const codex = { id: 8, name: 'Codex', username: 'agent', secondary: '研发' }

function ControlledMultiUserSelect() {
  const [value, setValue] = useState<UserSelectOption[]>([teo])
  return (
    <MultiUserSelect
      label="经办人"
      value={value}
      options={[teo, codex]}
      placeholder="选择经办人"
      clearable
      onValueChange={setValue}
    />
  )
}

describe('MultiUserSelect', () => {
  it('stays closed by default but accepts explicit initial opening for cell editors', async () => {
    const { unmount } = render(<MultiUserSelect label="Owners" value={[]} options={[{ id: 1, name: 'Alice' }]} onValueChange={vi.fn()} />)
    expect(screen.queryByRole('option')).toBeNull()
    unmount()
    render(<MultiUserSelect defaultOpen label="Owners" value={[]} options={[{ id: 1, name: 'Alice' }]} onValueChange={vi.fn()} />)
    expect(await screen.findByRole('option', { name: 'Alice' })).toBeTruthy()
  })
  it('uses the same compact clear control size as ordinary filter selects', () => {
    render(<ControlledMultiUserSelect />)
    const clear = screen.getByRole('button', { name: copy.ui.clearSelection })
    expect(clear.className).toContain('size-6')
    expect(clear.className).not.toContain('size-7')
    expect(clear.className).toContain('right-7')
    expect(clear.className).toContain('absolute')
  })

  it('keeps selected users visible below the search field', () => {
    render(
      <MultiUserSelect
        label="经办人"
        value={[teo, codex]}
        options={[teo, codex]}
        onValueChange={vi.fn()}
      />,
    )

    const trigger = document.querySelector('[data-slot="combobox-input-group"]')
    const valueRow = document.querySelector('[data-slot="multi-user-value-row"]')
    expect(trigger?.className).toContain('h-8')
    expect(trigger?.className).not.toContain('overflow-hidden')
    expect(valueRow?.className).toContain('flex-wrap')
    expect(screen.getByText('小龙')).toBeTruthy()
    expect(screen.getByText('Codex')).toBeTruthy()
  })

  it('keeps the control empty after the last selected user is removed', async () => {
    render(<ControlledMultiUserSelect />)

    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.userSelect.removeOption(teo.name),
    }))

    const input = screen.getByRole('combobox', { name: '经办人' }) as HTMLInputElement
    await waitFor(() => {
      expect(screen.queryByText(teo.name)).toBeNull()
      expect(input.value).toBe('')
    })
    expect(input.placeholder).toBe('选择经办人')
  })

  it('keeps selected users inside compact filter controls with a clear hover trigger', () => {
    render(
      <MultiUserSelect
        label="经办人"
        value={[teo, codex]}
        options={[teo, codex]}
        placeholder="搜索并选择经办人"
        selectionDisplay="inline"
        onValueChange={vi.fn()}
      />,
    )

    const trigger = screen.getByRole('button', { name: '经办人：小龙、Codex' })
    expect(trigger.textContent).toBe('小龙、Codex')
    expect(trigger.querySelectorAll('[data-slot="user-preview-avatar"]')).toHaveLength(2)
    expect(trigger.querySelector('img')?.getAttribute('src')).toBe('/teo.png')
    expect(trigger.querySelector('[data-slot="user-preview"]')?.className).toContain('max-w-28')
    expect(trigger.getAttribute('title')).toBeNull()
    expect(document.querySelector('[data-slot="multi-user-value-row"]')).toBeNull()

  })

  it('focuses the search input when opening an inline selection', async () => {
    render(
      <MultiUserSelect
        label="经办人"
        value={[teo]}
        options={[teo, codex]}
        placeholder="搜索并选择经办人"
        selectionDisplay="inline"
        onValueChange={vi.fn()}
      />,
    )

    const input = screen.getByRole('combobox', { name: '经办人' })
    fireEvent.click(screen.getByRole('button', { name: '经办人：小龙' }))

    await waitFor(() => expect(document.activeElement).toBe(input))
    fireEvent.change(input, { target: { value: 'Codex' } })
    expect(await screen.findByRole('option', { name: /Codex/ })).toBeTruthy()
    expect(screen.queryByRole('option', { name: /小龙/ })).toBeNull()
  })

  it('filters by username and secondary while preserving object values and avatars', async () => {
    const onValueChange = vi.fn()
    render(
      <MultiUserSelect
        label="经办人"
        value={[teo]}
        options={[teo, codex]}
        placeholder="搜索用户"
        onValueChange={onValueChange}
      />,
    )

    expect(document.querySelector('img')?.getAttribute('src')).toBe('/teo.png')
    const input = screen.getByRole('combobox', { name: '经办人' })
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('经办人') }))
    fireEvent.change(input, { target: { value: '研发' } })

    const option = await screen.findByRole('option', { name: /Codex/ })
    expect(screen.queryByRole('option', { name: /小龙/ })).toBeNull()
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)
    expect(onValueChange).toHaveBeenCalledWith([teo, codex])
  })

  it('supports keyboard selection and exposes disabled and empty states', async () => {
    const onValueChange = vi.fn()
    const { rerender } = render(
      <MultiUserSelect
        label="抄送人"
        value={[]}
        options={[teo, { ...codex, disabled: true }]}
        onValueChange={onValueChange}
      />,
    )

    const input = screen.getByRole('combobox', { name: '抄送人' })
    input.focus()
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    await screen.findByRole('option', { name: /小龙/ })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(onValueChange).toHaveBeenCalledWith([teo]))

    rerender(
      <MultiUserSelect
        label="抄送人"
        value={[]}
        options={[]}
        disabled
        onValueChange={onValueChange}
      />,
    )
    expect((screen.getByRole('combobox', { name: '抄送人' }) as HTMLInputElement).disabled).toBe(true)
  })
})
