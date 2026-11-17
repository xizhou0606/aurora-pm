import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'bun:test'
import { ACCESS_REQUESTS_COPY as copy } from '@/shared/copy'
import { DeniedDocumentAccess } from './DeniedDocumentAccess'

const flow = (() => ({ status: 'ready', data: { resourceId: 'private-document', ownerName: 'Owner', ownerUserId: 3 }, canRequest: true, latestRequest: undefined as undefined | { status: string }, retry: vi.fn() }))()
vi.mock('../hooks/useDeniedDocumentAccess', () => ({ useDeniedDocumentAccess: () => flow }))
vi.mock('./ResourceAccessRequestsToolbar', () => ({ ResourceAccessRequestsToolbar: () => <button>{copy.request}</button> }))
afterEach(cleanup)
beforeEach(() => { flow.status = 'ready'; flow.canRequest = true; flow.data.ownerUserId = 3; flow.latestRequest = undefined; flow.retry.mockReset() })

it('shows administrator guidance instead of a request action when no owner exists', () => {
  flow.data.ownerUserId = 0
  flow.canRequest = false
  render(<DeniedDocumentAccess documentId={14} />)
  expect(screen.getByText(copy.ownerMissing)).toBeTruthy()
  expect(screen.queryByRole('button')).toBeNull()
})

it('offers a request with owner identity without exposing document content', () => {
  render(<DeniedDocumentAccess documentId={14} />)
  expect(screen.getByText('Owner')).toBeTruthy()
  expect(screen.getByRole('button', { name: copy.request })).toBeTruthy()
})

it('shows pending guidance without a submission control', () => {
  flow.latestRequest = { status: 'pending' }
  flow.canRequest = false
  render(<DeniedDocumentAccess documentId={14} />)
  expect(screen.getByText(copy.waitingHint)).toBeTruthy()
  expect(screen.queryByRole('button')).toBeNull()
})

it.each(['expired', 'rejected'])('allows reapplication after %s', (status) => {
  flow.latestRequest = { status }
  render(<DeniedDocumentAccess documentId={14} />)
  expect(screen.getByRole('button', { name: copy.request })).toBeTruthy()
  expect(screen.getByText(status === 'expired' ? copy.expiredHint : copy.rejectedHint)).toBeTruthy()
})

it('preserves the original fallback for unavailable or out-of-scope documents', () => {
  flow.status = 'fallback'
  render(<DeniedDocumentAccess documentId={14} fallback={<p>Original error</p>} />)
  expect(screen.getByText('Original error')).toBeTruthy()
  expect(screen.queryByText('Owner')).toBeNull()
})

it('offers retry on network failure instead of claiming a missing document', () => {
  flow.status = 'error'
  render(<DeniedDocumentAccess documentId={14} fallback={<p>Original error</p>} />)
  fireEvent.click(screen.getByRole('button', { name: copy.retry }))
  expect(flow.retry).toHaveBeenCalledTimes(1)
  expect(screen.queryByText('Original error')).toBeNull()
})
