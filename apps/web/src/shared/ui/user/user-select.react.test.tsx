import { useState } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import { UserSelect } from './user-select'

function ControlledUserSelect({ onValueChange }: { onValueChange: (value?: number) => void }) {
  const [value, setValue] = useState<number | undefined>(7)
  return (
    <UserSelect
      label="负责人"
      value={value}
      options={[
        { id: 7, name: 'Teo' },
        { id: 8, name: 'Codex' },
      ]}
      placeholder="未分配"
      onValueChange={(next) => {
        setValue(next)
        onValueChange(next)
      }}
    />
  )
}

describe('UserSelect', () => {
  it('stays closed by default but accepts explicit initial opening for cell editors', async () => {
    const { unmount } = render(<UserSelect label="Owner" options={[{ id: 1, name: 'Alice' }]} onValueChange={vi.fn()} />)
    expect(screen.queryByRole('option')).toBeNull()
    unmount()
    render(<UserSelect defaultOpen label="Owner" options={[{ id: 1, name: 'Alice' }]} onValueChange={vi.fn()} />)
    expect(await screen.findByRole('option', { name: 'Alice' })).toBeTruthy()
  })
  it('requests the next page when the option list reaches the bottom', () => {
    const onLoadMore = vi.fn()
    render(
      <UserSelect
        label="Owner"
        options={[{ id: 7, name: 'Teo' }, { id: 8, name: 'Codex' }]}
        hasMore
        onLoadMore={onLoadMore}
        onValueChange={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('Owner') }))
    const list = document.querySelector('[data-slot="combobox-list"]') as HTMLElement
    Object.defineProperties(list, {
      scrollHeight: { configurable: true, value: 400 },
      clientHeight: { configurable: true, value: 200 },
      scrollTop: { configurable: true, value: 180 },
    })
    fireEvent.scroll(list)

    expect(onLoadMore).toHaveBeenCalledTimes(1)
  })
  it('renders avatar identities and emits the selected user id', async () => {
    const onValueChange = vi.fn()
    render(
      <UserSelect
        label="提交人"
        value={7}
        options={[
          { id: 7, name: 'Teo', avatarUrl: '/teo.png' },
          { id: 8, name: 'Codex' },
        ]}
        emptyLabel="全部提交人"
        onValueChange={onValueChange}
      />,
    )

    expect((screen.getByRole('combobox', { name: '提交人' }) as HTMLInputElement).value)
      .toBe('Teo')
    expect(document.querySelector('img')?.getAttribute('src')).toBe('/teo.png')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('提交人') }))
    const option = await screen.findByRole('option', { name: 'Codex' })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)
    expect(onValueChange).toHaveBeenCalledWith(8)
  })

  it('uses caller-provided empty copy and clears without owning domain semantics', async () => {
    const onValueChange = vi.fn()
    render(
      <UserSelect
        label="当前跟进人"
        options={[{ id: 7, name: 'Teo' }]}
        emptyLabel="当前跟进人"
        onValueChange={onValueChange}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('当前跟进人') }))
    const option = await screen.findByRole('option', { name: '当前跟进人' })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)
    expect(onValueChange).toHaveBeenCalledWith(undefined)
  })

  it('keeps the controlled value empty when the user deletes the selected text', async () => {
    const onValueChange = vi.fn()
    render(<ControlledUserSelect onValueChange={onValueChange} />)

    const input = screen.getByRole('combobox', { name: '负责人' }) as HTMLInputElement
    expect(input.value).toBe('Teo')

    fireEvent.change(input, { target: { value: '' } })

    await waitFor(() => {
      expect(onValueChange).toHaveBeenCalledWith(undefined)
      expect(input.value).toBe('')
    })
    expect(input.placeholder).toBe('未分配')
  })

  it('does not clear the controlled value when an untouched popup is dismissed', async () => {
    const onValueChange = vi.fn()
    render(<ControlledUserSelect onValueChange={onValueChange} />)

    const input = screen.getByRole('combobox', { name: '负责人' }) as HTMLInputElement
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('负责人') }))
    await screen.findByRole('option', { name: 'Codex' })
    fireEvent.keyDown(input, { key: 'Escape' })

    await waitFor(() => expect(screen.queryByRole('option', { name: 'Codex' })).toBeNull())
    expect(onValueChange).not.toHaveBeenCalled()
    expect(input.value).toBe('Teo')
  })

  it('keeps disabled options unavailable and commits keyboard selection', async () => {
    const onValueChange = vi.fn()
    render(
      <UserSelect
        label="负责人"
        value={7}
        options={[
          { id: 7, name: 'Teo' },
          { id: 8, name: 'Codex' },
          { id: 9, name: 'Unavailable', disabled: true },
        ]}
        onValueChange={onValueChange}
      />,
    )
    const trigger = screen.getByRole('combobox', { name: '负责人' })
    trigger.focus()
    fireEvent.keyDown(trigger, { key: 'ArrowDown' })
    const unavailable = await screen.findByRole('option', { name: 'Unavailable' })
    expect(unavailable.getAttribute('aria-disabled')).toBe('true')
    await waitFor(() => expect(trigger.getAttribute('aria-activedescendant')).toBeTruthy())
    fireEvent.keyDown(trigger, { key: 'ArrowDown' })
    fireEvent.keyDown(trigger, { key: 'Enter' })
    await waitFor(() => expect(onValueChange).toHaveBeenCalledWith(8))
  })

  it('forwards popup visibility without owning caller workflow state', async () => {
    const onOpenChange = vi.fn()
    render(
      <UserSelect
        label="经办人"
        options={[{ id: 7, name: 'Teo' }]}
        onOpenChange={onOpenChange}
        onValueChange={vi.fn()}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('经办人') }))
    await screen.findByRole('option', { name: 'Teo' })
    expect(onOpenChange).toHaveBeenCalledWith(true)
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })

  it('filters by display name and username while rendering avatar identities', async () => {
    render(
      <UserSelect
        label="负责人"
        options={[
          { id: 7, name: '小龙', username: 'teo', avatarUrl: '/teo.png' },
          { id: 8, name: 'Codex', username: 'agent' },
        ]}
        onValueChange={vi.fn()}
      />,
    )
    const input = screen.getByRole('combobox', { name: '负责人' })
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('负责人') }))
    fireEvent.change(input, { target: { value: 'teo' } })

    const option = await screen.findByRole('option', { name: /小龙/ })
    expect(option.textContent).toContain('teo')
    expect(screen.queryByRole('option', { name: /Codex/ })).toBeNull()
    expect(option.querySelector('img')?.getAttribute('src')).toBe('/teo.png')
  })

  it('filters Chinese names by pinyin so xizhou finds 西洲', async () => {
    render(
      <UserSelect
        label="负责人"
        options={[
          { id: 7, name: '西洲', username: 'west' },
          { id: 8, name: 'Codex', username: 'agent' },
        ]}
        onValueChange={vi.fn()}
      />,
    )
    const input = screen.getByRole('combobox', { name: '负责人' })
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('负责人') }))
    fireEvent.change(input, { target: { value: 'xizhou' } })

    expect(await screen.findByRole('option', { name: /西洲/ })).toBeTruthy()
    expect(screen.queryByRole('option', { name: /Codex/ })).toBeNull()
  })

  it('allows disabling pinyin while preserving literal search', async () => {
    render(<UserSelect label="Owner" search={{ pinyin: false }} options={[{ id: 1, name: '西洲' }]} onValueChange={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('Owner') }))
    const input = screen.getByRole('combobox', { name: 'Owner' })
    fireEvent.change(input, { target: { value: 'xizhou' } })
    await waitFor(() => expect(screen.queryByRole('option', { name: /西洲/ })).toBeNull())
    fireEvent.change(input, { target: { value: '西' } })
    expect(await screen.findByRole('option', { name: /西洲/ })).toBeTruthy()
  })

  it('virtualizes large directories while local filtering and selection still work', async () => {
    // jsdom reports zero layout; give the virtual scroller a viewport so the
    // windowed range computes on mount.
    const originalOffsetHeight = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype, 'offsetHeight',
    )
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
      configurable: true,
      get(this: HTMLElement) {
        if (this.matches?.('[data-slot="virtualized-combobox-scroller"]')) return 320
        return originalOffsetHeight ? originalOffsetHeight.get!.call(this) : 0
      },
    })

    const onValueChange = vi.fn()
    const options = Array.from({ length: 80 }, (_, index) => ({
      id: index + 1,
      name: `成员${index + 1}`,
    }))
    render(
      <UserSelect label="负责人" options={options} onValueChange={onValueChange} />,
    )
    try {
      fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('负责人') }))
      await screen.findByRole('option', { name: '成员1' })

      const renderedCount = screen.getAllByRole('option').length
      expect(renderedCount).toBeGreaterThan(0)
      expect(renderedCount).toBeLessThan(options.length)

      const input = screen.getByRole('combobox', { name: '负责人' })
      fireEvent.change(input, { target: { value: '成员80' } })
      const option = await screen.findByRole('option', { name: '成员80' })
      fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
      fireEvent.click(option)
      expect(onValueChange).toHaveBeenCalledWith(80)
    } finally {
      if (originalOffsetHeight) {
        Object.defineProperty(HTMLElement.prototype, 'offsetHeight', originalOffsetHeight)
      }
    }
  })
})
