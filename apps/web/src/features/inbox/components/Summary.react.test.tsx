import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { INBOX_COPY } from '@/shared/copy'
import { Summary } from './Summary'

describe('Inbox Summary', () => {
  it('keeps the error actions stacked through the 720px narrow boundary', () => {
    render(<Summary state={{ status: 'error', retry: vi.fn() }} canReadBugs userId={47} />)

    const alert = screen.getByRole('alert')
    expect(alert.className).toContain('min-[721px]:flex-row')
    expect(alert.className).toContain('min-[721px]:items-center')
    expect(alert.className).not.toContain('sm:flex-row')
  })

  it('fills the first summary cell with the daily Tao Te Ching quote', () => {
    render(<Summary state={{
      status: 'ready',
      data: {
        total: 5,
        tasks: 4,
        bugs: 1,
        workflows: 2,
        overdue: 0,
        dueToday: 0,
        blockedTasks: 0,
        reviewBugs: 0,
        workflowAIFailed: 0,
        mentions: 0,
      },
      refresh: 'idle',
    }} canReadBugs userId={47} />)

    expect(screen.getByText(/\d+月\d+日 · 今日语录/)).not.toBeNull()
    expect(screen.getByText(INBOX_COPY.summary.dailyQuote.source)).not.toBeNull()
    expect(document.querySelector('[data-slot="metric-strip"] > div:first-child dt')?.textContent)
      .toMatch(/\d+月\d+日 · 今日语录/)
    expect(INBOX_COPY.summary.dailyQuote.quotes.some((quote) => (
      screen.queryByText(quote) !== null
    ))).toBe(true)
  })
})
