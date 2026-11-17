import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItem } from '../../model/types'
import { BulkBar } from './BulkBar'

afterEach(cleanup)

const row: WorkItem = {
  id: 31,
  rowId: 'task:31',
  title: '固定操作栏任务',
  watched: false,
  capabilities: {
    canUpdate: true,
    canDelete: true,
    canComment: false,
    canAttach: false,
    canWatch: false,
    canClone: false,
    allowedTransitions: ['in_dev'],
  },
}

describe('Work-items bulk bar layout', () => {
  it('portals the selected-row actions to a fixed viewport bar', () => {
    const { container } = render(
      <div data-testid="list-layout">
        <BulkBar
          rows={[row]}
          mutation={{ status: 'idle' }}
          transitions={['in_dev']}
          canDelete
          onUpdateStatus={vi.fn().mockResolvedValue(true)}
          onDelete={vi.fn().mockResolvedValue(true)}
          onSelectAll={vi.fn()}
          onInvertSelection={vi.fn()}
        />
      </div>,
    )

    const bar = screen.getByTestId('work-items-bulk')
    expect(container.querySelector('[data-testid="work-items-bulk"]')).toBeNull()
    expect(bar.parentElement).toBe(document.body)
    expect(bar.className).toContain('fixed')
    expect(bar.className).toContain('bottom-4')
    expect(screen.queryByText('目标状态')).toBeNull()
    expect(screen.getByText('已选择 1 项')).toBeTruthy()
    expect(screen.getByRole('button', { name: '全选' })).toBeTruthy()
    expect(screen.getByRole('button', { name: '反选' })).toBeTruthy()
    expect(bar.querySelector('[data-slot="bulk-selection-actions"]')).toBeTruthy()
    expect(bar.querySelector('[data-slot="bulk-status-control"]')).toBeTruthy()
    expect(bar.querySelector('[data-slot="bulk-mutation-actions"]')).toBeTruthy()
  })
})

const callbacks = () => ({
  onUpdateStatus: vi.fn(async () => true),
  onDelete: vi.fn(async () => false),
  onSelectAll: vi.fn(),
  onInvertSelection: vi.fn(),
})

it('preserves unavailable actions and partial failure feedback', () => {
  render(<BulkBar rows={[row]} transitions={[]} canDelete={false} {...callbacks()}
    mutation={{ status: 'partial', action: 'delete', succeeded: 1, failures: [{ id: 31, code: 'denied', message: 'Permission changed' }] }} />)
  expect(screen.getByText(WORK_ITEMS_COPY.bulk.unavailable)).toBeTruthy()
  expect(screen.getByText(WORK_ITEMS_COPY.mutation.partial)).toBeTruthy()
  expect(screen.getByText(/Permission changed/)).toBeTruthy()
  expect(screen.queryByRole('button', { name: WORK_ITEMS_COPY.bulk.delete })).toBeNull()
})

it('keeps an unsuccessful deletion confirmation open', async () => {
  const actions = callbacks()
  render(<BulkBar rows={[row]} transitions={[]} canDelete {...actions} mutation={{ status: 'idle' }} />)
  fireEvent.click(screen.getByRole('button', { name: WORK_ITEMS_COPY.bulk.delete }))
  const buttons = await screen.findAllByRole('button', { name: WORK_ITEMS_COPY.bulk.delete })
  fireEvent.click(buttons[buttons.length - 1]!)
  await waitFor(() => expect(actions.onDelete).toHaveBeenCalledTimes(1))
  expect(screen.getByRole('heading', { name: WORK_ITEMS_COPY.bulk.confirmDeleteTitle })).toBeTruthy()
})
