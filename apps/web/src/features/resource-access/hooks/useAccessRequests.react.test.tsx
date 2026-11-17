import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { PropsWithChildren } from 'react'
import { afterEach, expect, it, vi } from 'bun:test'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { accessRequestsClient } from '../api/requests'
import { useCreateAccessRequest, useUpdateAccessRequest } from './useAccessRequests'

afterEach(() => { vi.restoreAllMocks(); runtimeWorkspaceStore.setState({ currentWorkspaceId: 0 }) })
it('rejects stale workspace and session submissions before issuing HTTP', async () => {
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
  const scope = { workspaceId: 7, sessionRevision: runtimeSessionStore.getState().revision, resourceId: 'dr_7' }
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const wrapper = ({ children }: PropsWithChildren) => <QueryClientProvider client={client}>{children}</QueryClientProvider>
  const create = vi.spyOn(accessRequestsClient, 'create').mockResolvedValue()
  const update = vi.spyOn(accessRequestsClient, 'update').mockResolvedValue()
  const { result } = renderHook(() => ({ create: useCreateAccessRequest(scope), update: useUpdateAccessRequest(scope) }), { wrapper })
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 8 })
  await act(async () => {
    await expect(result.current.create.mutateAsync({ reason: 'edit', notifyOwner: true })).rejects.toThrow()
    await expect(result.current.update.mutateAsync({ id: 1, decision: 'approved' })).rejects.toThrow()
  })
  expect(create).not.toHaveBeenCalled(); expect(update).not.toHaveBeenCalled()
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
  runtimeSessionStore.setState({ revision: scope.sessionRevision + 1 })
  await act(async () => { await expect(result.current.create.mutateAsync({ reason: 'edit', notifyOwner: false })).rejects.toThrow() })
  expect(create).not.toHaveBeenCalled()
  client.clear()
})
