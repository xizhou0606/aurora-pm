import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, cleanup } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { afterEach, expect, it, vi } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import { accessRequestsClient } from '../api/requests'
import { useDeniedDocumentAccess } from './useDeniedDocumentAccess'

vi.mock('@/features/access-control', () => ({ useOperationCapabilities: () => ({ allows: () => true }) }))
afterEach(() => { cleanup(); vi.restoreAllMocks(); runtimeWorkspaceStore.setState({ currentWorkspaceId: 0 }) })

it.each([401, 403, 404])('settles denied context HTTP %s after a single request', async (status) => {
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
  const request = vi.spyOn(accessRequestsClient, 'documentContext').mockRejectedValue(new PMSafeApiError('RESOURCE_ACCESS_FORBIDDEN', status))
  const cache = new QueryClient()
  const wrapper = ({ children }: PropsWithChildren) => <QueryClientProvider client={cache}>{children}</QueryClientProvider>
  const hook = renderHook(() => useDeniedDocumentAccess(14), { wrapper })
  await waitFor(() => expect(hook.result.current.status).toBe('fallback'))
  expect(request).toHaveBeenCalledTimes(1)
  hook.unmount(); cache.clear()
})

it('uses the same denied workflow for a native bitable id without requesting document content', async () => {
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
  const document = vi.spyOn(accessRequestsClient, 'documentContext')
  const bitable = vi.spyOn(accessRequestsClient, 'bitableContext').mockResolvedValue({
    resourceId: 'resource-uuid', resourceType: 'bitable', ownerUserId: 3, ownerName: 'Owner',
    canRequest: true, canReview: false, hasAccess: false, hasMore: false, pendingCount: 0, items: [],
  })
  const cache = new QueryClient()
  const wrapper = ({ children }: PropsWithChildren) => <QueryClientProvider client={cache}>{children}</QueryClientProvider>
  const hook = renderHook(() => useDeniedDocumentAccess({ resourceType: 'bitable', bitableId: 'native-uuid' }), { wrapper })
  await waitFor(() => expect(hook.result.current.status).toBe('ready'))
  expect(bitable).toHaveBeenCalledWith('native-uuid', expect.any(AbortSignal))
  expect(document).not.toHaveBeenCalled()
  expect(hook.result.current.canRequest).toBe(true)
  hook.unmount(); cache.clear()
})
