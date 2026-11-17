import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'
import { ACCESS_REQUESTS_COPY as copy, RESOURCE_ACCESS_COPY as accessCopy } from '@/shared/copy'
import { ResourceAccessRequestsToolbar } from './ResourceAccessRequestsToolbar'

const mocks = (() => ({ update: vi.fn().mockResolvedValue(undefined) }))()
vi.mock('@/features/access-control', () => ({ useOperationCapabilities: () => ({ allows: () => true }) }))
vi.mock('@/shared/state', () => ({ useSessionStore: (select: (s: unknown) => unknown) => select({ revision: 1 }), useWorkspaceStore: (select: (s: unknown) => unknown) => select({ currentWorkspaceId: 7 }) }))
vi.mock('../hooks/useAccessRequests', () => ({
  isCurrentRequestScope: () => true,
  useAccessRequestsQuery: () => ({ data: { resourceId: 'table-resource', resourceType: 'bitable', ownerName: 'Owner', canReview: true, canRequest: false, pendingCount: 1, items: [{ id: 5, applicantName: 'Applicant', reason: 'edit', status: 'pending', read: false }] } }),
  useCreateAccessRequest: () => ({ isPending: false, isError: false }),
  useUpdateAccessRequest: () => ({ isPending: false, isError: false, mutateAsync: mocks.update }),
}))
afterEach(() => { cleanup(); mocks.update.mockClear() })

it('defaults approval to permanent editor and submits selected viewer with a seven-day duration', async () => {
  const cache = new QueryClient()
  render(<QueryClientProvider client={cache}><ResourceAccessRequestsToolbar resourceId="table-resource" resourceType="bitable" canEdit /></QueryClientProvider>)
  fireEvent.click(screen.getByRole('button', { name: copy.pendingCount(1) }))
  expect((await screen.findByRole('combobox', { name: copy.grantLevel })).textContent).toContain(accessCopy.level.editor)
  expect(screen.getByRole('combobox', { name: copy.grantDuration }).textContent).toContain(accessCopy.links.expiryNever)
  fireEvent.click(screen.getByRole('combobox', { name: copy.grantLevel }))
  const viewer = await screen.findByRole('option', { name: accessCopy.level.viewer })
  fireEvent.pointerDown(viewer, { button: 0, pointerType: 'mouse' })
  fireEvent.pointerUp(viewer, { button: 0, pointerType: 'mouse' })
  fireEvent.click(viewer)
  await waitFor(() => expect(screen.getByRole('combobox', { name: copy.grantLevel }).textContent).toContain(accessCopy.level.viewer))
  fireEvent.click(screen.getByRole('combobox', { name: copy.grantDuration }))
  const sevenDays = await screen.findByRole('option', { name: accessCopy.links.expiry7d })
  fireEvent.pointerDown(sevenDays, { button: 0, pointerType: 'mouse' })
  fireEvent.pointerUp(sevenDays, { button: 0, pointerType: 'mouse' })
  fireEvent.click(sevenDays)
  await waitFor(() => expect(screen.getByRole('combobox', { name: copy.grantDuration }).textContent).toContain(accessCopy.links.expiry7d))
  fireEvent.click(screen.getByRole('button', { name: copy.approve }))
  await waitFor(() => expect(mocks.update).toHaveBeenCalledWith({ id: 5, decision: 'approved', grantLevel: 'viewer', grantDuration: '7d' }))
  cache.clear()
})
