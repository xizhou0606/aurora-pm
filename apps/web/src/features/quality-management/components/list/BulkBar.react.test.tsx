import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { QUALITY_COPY } from '@/shared/copy'
import { BulkBar } from './BulkBar'

afterEach(cleanup)

const props = () => ({
  count: 2,
  mutation: { status: 'idle' as const, action: 'delete' as const },
  assignmentMutation: { status: 'idle' as const, action: 'delete' as const },
  onAssign: vi.fn(),
  onDelete: vi.fn().mockResolvedValue(undefined),
  onSelectAll: vi.fn(),
  onInvertSelection: vi.fn(),
  canDelete: true,
  canAssign: true,
})

describe('Quality bulk bar', () => {
  it('portals selection controls and delegates selection and assignment', () => {
    const input = props()
    const { container } = render(<BulkBar {...input} />)
    const bar = screen.getByTestId('quality-bulk')
    expect(bar.parentElement).toBe(document.body)
    expect(container.contains(bar)).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.bulk.selectAll }))
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.bulk.invertSelection }))
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.bulk.assign }))
    expect(input.onSelectAll).toHaveBeenCalledTimes(1)
    expect(input.onInvertSelection).toHaveBeenCalledTimes(1)
    expect(input.onAssign).toHaveBeenCalledTimes(1)
  })

  it('keeps permission and pending decisions in the feature', () => {
    const input = props()
    const { rerender } = render(<BulkBar {...input} canAssign={false} canDelete={false} />)
    expect(screen.queryByTestId('quality-bulk')).toBeNull()
    rerender(<BulkBar {...input} count={0} />)
    expect(screen.queryByTestId('quality-bulk')).toBeNull()
    rerender(<BulkBar {...input} assignmentMutation={{ status: 'pending', action: 'delete' }} />)
    expect((screen.getByRole('button', { name: QUALITY_COPY.bulk.assign }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: QUALITY_COPY.bulk.delete }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: QUALITY_COPY.bulk.selectAll }) as HTMLButtonElement).disabled).toBe(false)
  })

  it('keeps a rejected deletion open for retry', async () => {
    const input = props()
    input.onDelete.mockImplementation(async () => { throw new Error('retry') })
    render(<BulkBar {...input} />)
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.bulk.delete }))
    const buttons = await screen.findAllByRole('button', { name: QUALITY_COPY.bulk.delete })
    fireEvent.click(buttons[buttons.length - 1]!)
    await waitFor(() => expect(input.onDelete).toHaveBeenCalledTimes(1))
    expect(screen.getByRole('heading', { name: QUALITY_COPY.bulk.confirmDeleteTitle })).toBeTruthy()
  })
})
