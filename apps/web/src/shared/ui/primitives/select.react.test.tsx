import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import {
  Select,
  SelectContent,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from './select'

const options = [
  { value: 7, label: 'Workspace Seven' },
  { value: 9, label: 'Workspace Nine' },
]

afterEach(() => {
  vi.restoreAllMocks()
})

function Fixture({
  value,
  disabled = false,
  disabledSecond = false,
  onValueChange,
}: {
  value: number | null
  disabled?: boolean
  disabledSecond?: boolean
  onValueChange: (value: number | null) => void
}) {
  return (
    <Select<number>
      items={options}
      value={value}
      disabled={disabled}
      onValueChange={(nextValue) => onValueChange(nextValue)}
    >
      <SelectLabel>Workspace</SelectLabel>
      <SelectTrigger>
        <SelectValue placeholder="Choose workspace" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={7}>Workspace Seven</SelectItem>
        <SelectItem value={9} disabled={disabledSecond}>
          Workspace Nine
        </SelectItem>
      </SelectContent>
    </Select>
  )
}

describe('Select', () => {
  it('provides an accessible controlled numeric select contract', async () => {
    const onValueChange = vi.fn()
    const { rerender } = render(
      <Fixture value={7} onValueChange={onValueChange} />,
    )
    const trigger = screen.getByRole('combobox', { name: 'Workspace' })
    const label = document.querySelector('[data-slot="select-label"]')

    expect(label?.className).toContain('hidden')
    expect(trigger.textContent).toContain('Workspace Seven')
    expect(trigger.getAttribute('aria-expanded')).toBe('false')

    fireEvent.click(trigger)
    const secondOption = await screen.findByRole('option', {
      name: 'Workspace Nine',
    })
    const positioner = document.querySelector('[data-slot="select-positioner"]') as HTMLElement | null
    expect(screen.getByRole('listbox')).toBeTruthy()
    expect(positioner?.style.position).toBe('fixed')
    expect(screen.getByRole('option', { name: 'Workspace Seven' }).getAttribute('aria-selected'))
      .toBe('true')

    fireEvent.pointerDown(secondOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(secondOption)
    expect(onValueChange).toHaveBeenCalledWith(9)
    expect(trigger.textContent).toContain('Workspace Seven')

    rerender(<Fixture value={9} onValueChange={onValueChange} />)
    expect(trigger.textContent).toContain('Workspace Nine')
    expect(trigger.textContent).not.toContain('9Workspace')
  })

  it('restores focus on Escape and keeps disabled options unavailable', async () => {
    const onValueChange = vi.fn()
    render(
      <Fixture
        value={7}
        disabledSecond
        onValueChange={onValueChange}
      />,
    )
    const trigger = screen.getByRole('combobox', { name: 'Workspace' })

    trigger.focus()
    fireEvent.keyDown(trigger, { key: 'ArrowDown' })
    const disabledOption = await screen.findByRole('option', {
      name: 'Workspace Nine',
    })
    expect(disabledOption.getAttribute('aria-disabled')).toBe('true')

    fireEvent.keyDown(document.activeElement ?? trigger, { key: 'Escape' })
    await waitFor(() => {
      expect(trigger.getAttribute('aria-expanded')).toBe('false')
      // A boolean identity check avoids expensive DOM diffs before queued focus settles.
      expect(document.activeElement === trigger).toBe(true)
    })
    expect(onValueChange).not.toHaveBeenCalled()
  })

  it('commits keyboard selection as the controlled numeric value', async () => {
    const onValueChange = vi.fn()
    render(<Fixture value={7} onValueChange={onValueChange} />)
    const trigger = screen.getByRole('combobox', { name: 'Workspace' })

    trigger.focus()
    fireEvent.keyDown(trigger, { key: 'ArrowDown' })
    await screen.findByRole('listbox')
    await waitFor(() => {
      expect(document.activeElement?.getAttribute('role')).toBe('option')
    })
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowDown' })
    fireEvent.keyDown(document.activeElement!, { key: 'Enter' })

    await waitFor(() => {
      expect(onValueChange).toHaveBeenCalledWith(9)
    })
  })

  it('keeps long-list scroll arrows inside the popup card', async () => {
    const onValueChange = vi.fn()
    const manyOptions = Array.from({ length: 24 }, (_, index) => ({
      value: index + 1,
      label: `Workspace ${index + 1}`,
    }))
    render(
      <Select<number>
        items={manyOptions}
        value={1}
        onValueChange={(nextValue) => onValueChange(nextValue)}
      >
        <SelectLabel>Workspace</SelectLabel>
        <SelectTrigger>
          <SelectValue placeholder="Choose workspace" />
        </SelectTrigger>
        <SelectContent>
          {manyOptions.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>,
    )

    fireEvent.click(screen.getByRole('combobox', { name: 'Workspace' }))
    await screen.findByRole('listbox')

    const popup = document.querySelector('[data-slot="select-content"]') as HTMLElement | null
    expect(popup?.className.split(/\s+/)).toEqual(expect.arrayContaining(['relative', 'overflow-hidden']))

    const downArrow = popup?.querySelector('[data-slot="select-scroll-down-arrow"]')
    if (downArrow) {
      expect(downArrow.className.split(/\s+/)).toEqual(
        expect.arrayContaining(['bottom-0', 'left-0', 'w-full']),
      )
    }
    const upArrow = popup?.querySelector('[data-slot="select-scroll-up-arrow"]')
    if (upArrow) {
      expect(upArrow.className.split(/\s+/)).toEqual(
        expect.arrayContaining(['top-0', 'left-0', 'w-full']),
      )
    }
  })

  it('uses pointer cursors for selectable controls and a blocked cursor for disabled states', async () => {
    const onValueChange = vi.fn()
    const { rerender } = render(
      <Fixture value={7} disabledSecond onValueChange={onValueChange} />,
    )
    const trigger = screen.getByRole('combobox', { name: 'Workspace' })

    expect(trigger.className).toContain('cursor-pointer')
    fireEvent.click(trigger)

    const option = await screen.findByRole('option', { name: 'Workspace Seven' })
    const disabledOption = screen.getByRole('option', { name: 'Workspace Nine' })
    expect(option.className).toContain('cursor-pointer')
    expect(disabledOption.className).toContain('data-disabled:cursor-not-allowed')

    fireEvent.keyDown(document.activeElement ?? trigger, { key: 'Escape' })
    rerender(<Fixture value={7} disabled onValueChange={onValueChange} />)
    expect(trigger.className).toContain('data-disabled:cursor-not-allowed')
  })

  it('mounts its popup inside the active fullscreen element', async () => {
    const fullscreenHost = document.createElement('section')
    document.body.append(fullscreenHost)
    let fullscreenElement: Element | null = null
    Object.defineProperty(document, 'fullscreenElement', {
      configurable: true,
      get: () => fullscreenElement,
    })

    const view = render(
      <Fixture value={7} onValueChange={vi.fn()} />,
      { container: fullscreenHost },
    )

    act(() => {
      fullscreenElement = fullscreenHost
      document.dispatchEvent(new Event('fullscreenchange'))
    })
    fireEvent.click(screen.getByRole('combobox', { name: 'Workspace' }))

    const listbox = await screen.findByRole('listbox')
    expect(fullscreenHost.contains(listbox)).toBe(true)

    view.unmount()
    fullscreenHost.remove()
    Reflect.deleteProperty(document, 'fullscreenElement')
  })
})
