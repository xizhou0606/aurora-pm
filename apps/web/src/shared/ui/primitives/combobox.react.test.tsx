import { useState } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxInputGroup,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
} from './combobox'
import { Label } from './label'

type Option = { label: string; value: number }

const options: Option[] = [
  { value: 7, label: 'Workspace Seven' },
  { value: 9, label: 'Workspace Nine' },
]

function Fixture({ onValueChange = vi.fn() }: {
  onValueChange?: (value: Option | null) => void
}) {
  const [value, setValue] = useState<Option | null>(options[0])
  return (
    <div>
      <Label htmlFor="workspace-combobox">Workspace</Label>
      <Combobox<Option>
        items={options}
        value={value}
        onValueChange={(nextValue) => {
          setValue(nextValue)
          onValueChange(nextValue)
        }}
      >
        <ComboboxInputGroup>
          <ComboboxInput
            id="workspace-combobox"
            placeholder="Choose workspace"
          />
          <ComboboxTrigger aria-label="Open workspace options" />
        </ComboboxInputGroup>
        <ComboboxContent>
          <ComboboxEmpty>No matching workspace</ComboboxEmpty>
          <ComboboxList>
            {(item: Option) => (
              <ComboboxItem key={item.value} value={item}>
                {item.label}
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </div>
  )
}

describe('Combobox', () => {
  it('exposes a focusable visible filter with a controlled numeric identity', async () => {
    const onValueChange = vi.fn()
    render(<Fixture onValueChange={onValueChange} />)
    const input = screen.getByRole('combobox', { name: 'Workspace' }) as HTMLInputElement

    const label = screen.getByText('Workspace') as HTMLLabelElement
    expect(label.control).toBe(input)

    input.focus()
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.change(input, { target: { value: 'Nine' } })
    expect(await screen.findByRole('option', { name: 'Workspace Nine' })).toBeTruthy()
    const positioner = document.querySelector('[data-slot="combobox-positioner"]') as HTMLElement | null
    expect(positioner?.style.position).toBe('fixed')
    expect(screen.queryByRole('option', { name: 'Workspace Seven' })).toBeNull()

    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => {
      expect(onValueChange).toHaveBeenCalledWith(options[1])
      expect(input.value).toBe('Workspace Nine')
    })
  })

  it('announces an empty filtered result and restores focus on Escape', async () => {
    render(<Fixture />)
    const input = screen.getByRole('combobox', { name: 'Workspace' }) as HTMLInputElement

    input.focus()
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    fireEvent.change(input, { target: { value: 'missing' } })
    expect(await screen.findByText('No matching workspace')).toBeTruthy()

    fireEvent.keyDown(input, { key: 'Escape' })
    await waitFor(() => {
      expect(input.getAttribute('aria-expanded')).toBe('false')
      expect(document.activeElement).toBe(input)
    })
  })

  it('shows pointer affordances on the dropdown trigger and selectable options', async () => {
    render(<Fixture />)
    const trigger = screen.getByRole('button', { name: 'Open workspace options' })
    expect(trigger.className).toContain('cursor-pointer')

    fireEvent.click(trigger)
    const option = await screen.findByRole('option', { name: 'Workspace Seven' })
    expect(option.className).toContain('cursor-pointer')
  })

  it('collapses the empty-state container while matching options are available', async () => {
    render(<Fixture />)
    fireEvent.click(screen.getByRole('button', { name: 'Open workspace options' }))
    await screen.findByRole('option', { name: 'Workspace Seven' })

    const emptyState = document.querySelector('[data-slot="combobox-empty"]')
    expect(emptyState?.textContent).toBe('')
    expect(emptyState?.classList.contains('empty:hidden')).toBe(true)
  })
})
