import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { REPORTING_COPY } from '@/shared/copy'

import { BugReportPanel } from './BugReportPanel'
import { RequirementPanel } from './RequirementPanel'
import { UiBugPanel } from './UiBugPanel'

// Distinct values detect accidental metric-key/label reassignment during layout reuse.
function expectMetrics(title: string, entries: [string, number][], footnote: string) {
  const panel = screen.getByRole('region', { name: title })
  for (const [label, value] of entries) {
    const metric = within(panel).getByText(label).parentElement!
    expect(within(metric).getByText(`${value} ${REPORTING_COPY.metrics.unit}`)).toBeTruthy()
  }
  expect(within(panel).getByText(footnote)).toBeTruthy()
}

describe('reporting fact panels', () => {
  it('retains all requirement snapshot metrics and their scope note', () => {
    const copy = REPORTING_COPY.requirement
    render(<RequirementPanel metrics={{ total: 21, createdInPeriod: 6, done: 5, acceptance: 4, development: 3, pending: 2 }} />)
    expectMetrics(copy.title, [[copy.total, 21], [copy.createdInPeriod, 6], [copy.done, 5], [copy.acceptance, 4], [copy.development, 3], [copy.pending, 2]], copy.footnote)
  })

  it('retains reported bug severity counts including zero', () => {
    const copy = REPORTING_COPY.bugReport
    render(<BugReportPanel metrics={{ total: 9, p0: 0, p1: 3, p2: 5 }} />)
    expectMetrics(copy.title, [[copy.total, 9], [copy.p0, 0], [copy.p1, 3], [copy.p2, 5]], copy.footnote)
  })

  it('retains resolved, open and overdue UI bug metrics', () => {
    const copy = REPORTING_COPY.uiBug
    render(<UiBugPanel metrics={{ resolved: 8, open: 4, overdue: 1 }} />)
    expectMetrics(copy.title, [[copy.resolved, 8], [copy.open, 4], [copy.overdue, 1]], copy.footnote)
  })
})
