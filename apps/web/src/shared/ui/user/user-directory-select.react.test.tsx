import { useState } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import {
  UserDirectoryMultiSelect,
  UserDirectorySelect,
  type UserDirectoryState,
} from './user-directory-select'

const readyState = {
  status: 'ready',
  items: [
    { id: 7, label: 'Alice', username: 'alice', avatarUrl: '/alice.png' },
    { id: 9, label: 'Bob', username: 'bob', avatarUrl: '/bob.png' },
  ],
} satisfies UserDirectoryState

function ControlledUser({ onChange }: { onChange: (value?: number) => void }) {
  const [value, setValue] = useState<number | undefined>(7)
  return (
    <UserDirectorySelect
      id="owner"
      label="Owner"
      value={value}
      state={readyState}
      onValueChange={(nextValue) => {
        setValue(nextValue)
        onChange(nextValue)
      }}
    />
  )
}

describe('UserDirectorySelect', () => {
  it('uses shared avatar choices and supports keyboard selection and clearing', async () => {
    const onChange = vi.fn()
    const { container } = render(<ControlledUser onChange={onChange} />)

    const input = screen.getByRole('combobox', { name: 'Owner' }) as HTMLInputElement
    expect(input.value).toBe('Alice')
    expect(container.querySelector('[data-slot="user-preview"] img')?.getAttribute('src'))
      .toBe('/alice.png')

    input.focus()
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.change(input, { target: { value: 'Bob' } })
    const bob = await screen.findByRole('option', { name: /Bob/ })
    expect(bob.querySelector('img')?.getAttribute('src')).toBe('/bob.png')
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(9)
      expect(input.value).toBe('Bob')
    })

    onChange.mockClear()
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.change(input, { target: { value: copy.ui.userSelect.clear } })
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(undefined)
      expect(input.value).toBe('')
    })
  })

  it('propagates deleting the selected text as a cleared directory value', async () => {
    const onChange = vi.fn()
    render(<ControlledUser onChange={onChange} />)

    const input = screen.getByRole('combobox', { name: 'Owner' }) as HTMLInputElement
    expect(input.value).toBe('Alice')

    fireEvent.change(input, { target: { value: '' } })

    await waitFor(() => {
      expect(onChange).toHaveBeenCalledWith(undefined)
      expect(input.value).toBe('')
    })
    expect(input.placeholder).toBe(copy.ui.userSelect.placeholder)
  })

  it('preserves a historical value that is missing from the directory', async () => {
    const onChange = vi.fn()
    render(
      <UserDirectorySelect
        id="historical-owner"
        label="Historical owner"
        value={42}
        state={readyState}
        onValueChange={onChange}
      />,
    )

    const input = screen.getByRole('combobox', { name: 'Historical owner' }) as HTMLInputElement
    expect(input.value).toBe(copy.ui.userSelect.fallback(42))
    input.focus()
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(await screen.findByRole('option', { name: copy.ui.userSelect.fallback(42) })).toBeTruthy()
  })

  it('announces loading, error, and empty directory states', () => {
    const retry = vi.fn()
    const { rerender } = render(
      <UserDirectorySelect id="state-owner" label="State owner" state={{ status: 'loading' }} onValueChange={vi.fn()} />,
    )
    const input = screen.getByRole('combobox', { name: 'State owner' }) as HTMLInputElement
    expect(input.disabled).toBe(true)
    expect(screen.getByRole('status').textContent).toContain(copy.ui.userSelect.loading)

    rerender(
      <UserDirectorySelect id="state-owner" label="State owner" state={{ status: 'error', retry }} onValueChange={vi.fn()} />,
    )
    expect(screen.getByRole('alert').textContent).toContain(copy.ui.userSelect.loadFailed)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.retry }))
    expect(retry).toHaveBeenCalledTimes(1)

    rerender(
      <UserDirectorySelect id="state-owner" label="State owner" state={{ status: 'empty' }} onValueChange={vi.fn()} />,
    )
    expect(screen.getByRole('status').textContent).toContain(copy.ui.userSelect.empty)
    expect(input.disabled).toBe(true)
  })

  it('uses the same avatar directory for multi-person selection', async () => {
    const onChange = vi.fn()
    const { container } = render(
      <UserDirectoryMultiSelect
        id="developers"
        label="Developers"
        value={[7]}
        state={readyState}
        onValueChange={onChange}
      />,
    )

    expect(container.querySelector('[data-slot="user-preview"] img')?.getAttribute('src'))
      .toBe('/alice.png')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('Developers') }))
    const bob = await screen.findByRole('option', { name: /Bob/ })
    expect(bob.querySelector('img')?.getAttribute('src')).toBe('/bob.png')
    fireEvent.pointerDown(bob, { button: 0, pointerType: 'mouse' })
    fireEvent.click(bob)
    expect(onChange).toHaveBeenCalledWith([7, 9])
  })

  it('filters directory people by pinyin in the same shared combobox', async () => {
    render(
      <UserDirectorySelect
        id="next-owner"
        label="下一节点负责人"
        state={{
          status: 'ready',
          items: [
            { id: 7, label: '西洲', username: 'west' },
            { id: 9, label: '小龙', username: 'teo' },
          ],
        }}
        onValueChange={vi.fn()}
      />,
    )

    const input = screen.getByRole('combobox', { name: '下一节点负责人' })
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('下一节点负责人') }))
    fireEvent.change(input, { target: { value: 'xizhou' } })

    expect(await screen.findByRole('option', { name: /西洲/ })).toBeTruthy()
    expect(screen.queryByRole('option', { name: /小龙/ })).toBeNull()
  })
})
