import { useState } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { DatePicker, DateTimePicker } from './date-picker'

function DateFixture({ onChange = () => undefined }: { onChange?: (value: string) => void }) {
  const [value, setValue] = useState('2026-08-05')
  return <DatePicker aria-label="计划开始" value={value} onChange={(next) => { setValue(next); onChange(next) }} />
}

describe('DatePicker', () => {
  it('renders a localized shadcn-style trigger and selects from the calendar', async () => {
    const onChange = vi.fn()
    render(<DateFixture onChange={onChange} />)

    const trigger = screen.getByRole('button', { name: '计划开始' })
    expect(trigger.textContent).toContain('2026年8月5日')
    expect(trigger.className).toContain('cursor-pointer')

    fireEvent.click(trigger)
    expect(await screen.findByText('2026年8月')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: '选择 2026年8月12日' }))

    expect(onChange).toHaveBeenCalledWith('2026-08-12')
    expect(trigger.textContent).toContain('2026年8月12日')
  })

  it('supports clearing a selected date', async () => {
    const onChange = vi.fn()
    render(<DateFixture onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: '计划开始' }))
    fireEvent.click(await screen.findByRole('button', { name: '清空' }))

    expect(onChange).toHaveBeenCalledWith('')
    expect(screen.getByRole('button', { name: '计划开始' }).textContent).toContain('选择日期')
  })
})

describe('DateTimePicker', () => {
  it('keeps the minute-accurate local datetime in its shared trigger', () => {
    render(<DateTimePicker aria-label="计划完成" value="2026-08-06T16:35" onChange={() => undefined} />)
    expect(screen.getByRole('button', { name: '计划完成' }).textContent).toContain('2026年8月6日 16:35')
  })

  it('keeps disabled date controls visibly non-interactive', () => {
    render(<DateTimePicker aria-label="计划完成" value="" disabled onChange={() => undefined} />)
    expect(screen.getByRole('button', { name: '计划完成' }).className).toContain('disabled:cursor-not-allowed')
  })
})
