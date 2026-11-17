import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { DateField, EstimatedMinutesField } from './Fields'

describe('work-item detail blur-save fields', () => {
  it.each([
    ['success', true],
    ['failure', false],
  ])('saves a date exactly once on calendar selection after a %s result', async (_label, result) => {
    const onSave = vi.fn().mockResolvedValue(result)
    render(
      <DateField
        id="detail-due-date"
        label={WORK_ITEMS_COPY.gantt.dueDate}
        value="2026-08-05"
        mutation={{ status: 'idle' }}
        resetKey={31}
        onSave={onSave}
      />,
    )

    const input = screen.getByLabelText(WORK_ITEMS_COPY.gantt.dueDate)
    fireEvent.click(input)
    fireEvent.click(await screen.findByRole('button', { name: '选择 2026年8月6日' }))

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1))
    expect(onSave).toHaveBeenCalledWith('2026-08-06')
  })

  it.each([
    ['success', true],
    ['failure', false],
  ])('saves estimated minutes exactly once on Enter after a %s result', async (_label, result) => {
    const onSave = vi.fn().mockResolvedValue(result)
    render(
      <EstimatedMinutesField
        value={60}
        mutation={{ status: 'idle' }}
        resetKey={31}
        onSave={onSave}
      />,
    )

    const input = screen.getByLabelText(WORK_ITEMS_COPY.detail.estimatedTime)
    fireEvent.change(input, { target: { value: '90' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1))
    expect(onSave).toHaveBeenCalledWith(90)
  })

  it('saves valid drafts on blur and restores an invalid estimate', async () => {
    const saveDate = vi.fn().mockResolvedValue(true)
    const saveEstimate = vi.fn().mockResolvedValue(true)
    render(
      <>
        <DateField
          id="detail-start-date"
          label={WORK_ITEMS_COPY.gantt.startDate}
          value="2026-08-05"
          mutation={{ status: 'idle' }}
          resetKey={31}
          onSave={saveDate}
        />
        <EstimatedMinutesField
          value={60}
          mutation={{ status: 'idle' }}
          resetKey={31}
          onSave={saveEstimate}
        />
      </>,
    )

    const date = screen.getByLabelText(WORK_ITEMS_COPY.gantt.startDate)
    fireEvent.click(date)
    fireEvent.click(await screen.findByRole('button', { name: '选择 2026年8月7日' }))
    await waitFor(() => expect(saveDate).toHaveBeenCalledWith('2026-08-07'))

    const estimate = screen.getByLabelText(WORK_ITEMS_COPY.detail.estimatedTime) as HTMLInputElement
    fireEvent.change(estimate, { target: { value: '-1' } })
    fireEvent.blur(estimate)
    expect(saveEstimate).not.toHaveBeenCalled()
    await waitFor(() => expect(estimate.value).toBe('60'))
  })
})
