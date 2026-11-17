import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { QualitySelectField } from './SelectField'

describe('QualitySelectField', () => {
  it('keeps an empty filter as a placeholder and exposes an explicit clear action', async () => {
    const onValueChange = vi.fn()
    const { rerender } = render(
      <QualitySelectField
        id="quality-status"
        label="状态"
        value=""
        options={[{ value: 'triage', label: '待确认' }]}
        emptyLabel="清除筛选"
        placeholder="选择状态"
        clearable
        onValueChange={onValueChange}
      />,
    )

    expect(screen.getByRole('combobox', { name: '状态' }).textContent).toContain('选择状态')

    rerender(
      <QualitySelectField
        id="quality-status"
        label="状态"
        value="triage"
        options={[{ value: 'triage', label: '待确认' }]}
        emptyLabel="清除筛选"
        placeholder="选择状态"
        clearable
        onValueChange={onValueChange}
      />,
    )
    const trigger = screen.getByRole('combobox', { name: '状态' })
    expect(trigger.className).not.toContain('pr-14')
    expect(trigger.querySelector('[data-slot="select-value"]')?.className).toContain('pr-7')
    fireEvent.click(trigger)
    const clear = await screen.findByRole('option', { name: '清除筛选' })
    fireEvent.pointerDown(clear, { button: 0, pointerType: 'mouse' })
    fireEvent.click(clear)
    expect(onValueChange).toHaveBeenCalledWith('')
  })
})
