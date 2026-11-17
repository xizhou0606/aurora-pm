import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'bun:test'
import { ACCESS_REQUESTS_COPY as copy } from '@/shared/copy'
import { ResourceAccessRequestsToolbar } from './ResourceAccessRequestsToolbar'

const flow = (() => ({ visible: true, status: 'error', data: undefined, retry: vi.fn() }))()
vi.mock('../hooks/useAccessRequestsWorkflow', () => ({ useAccessRequestsWorkflow: () => flow }))

it('keeps a safe retry entry when an authorized context query fails', () => {
  const view = render(<ResourceAccessRequestsToolbar resourceId="6ec3fde4-22c4-40f9-b0b9-c3660c961cba" canEdit />)
  fireEvent.click(screen.getByRole('button', { name: `${copy.review} · ${copy.retry}` }))
  expect(flow.retry).toHaveBeenCalledTimes(1)
  flow.visible = false
  view.rerender(<ResourceAccessRequestsToolbar resourceId="6ec3fde4-22c4-40f9-b0b9-c3660c961cba" canEdit />)
  expect(screen.queryByRole('button')).toBeNull()
})
