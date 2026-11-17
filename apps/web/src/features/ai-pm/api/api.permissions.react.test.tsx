import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'
import { useRequirementMutations } from '../api/api'

let canMove = true
let canDelete = true

vi.mock('../hooks/useOperationAccess', () => ({
  useRequirementsOperationAccess: () => ({
    status: 'ready', retry: vi.fn(),
    canViewPool: () => true,
    canCreateRequirement: () => true,
    canUpdateRequirement: () => true,
    canBatchMoveRequirement: (local = true) => canMove && local,
    canDeleteRequirement: (local = true) => canDelete && local,
    canScheduleRequirement: () => true,
    canValidateDocument: () => true,
    canUploadDesignAsset: () => true,
    canAssignRoleOwners: () => true,
    canViewMaterials: () => true,
    canUploadMaterial: () => true,
    canDownloadMaterial: () => true,
    canDeleteMaterial: () => true,
  }),
  useAIRunsOperationAccess: () => ({}),
  useTestingOperationAccess: () => ({}),
  useWorkflowOperationAccess: () => ({}),
}))

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient({ defaultOptions: { mutations: { retry: false } } })}>{children}</QueryClientProvider>
}

describe('requirement permission mutations', () => {
  afterEach(() => {
    canMove = true
    canDelete = true
    vi.restoreAllMocks()
  })

  it('uses the cataloged atomic batch-move endpoint for a single-row move', async () => {
    const post = vi.spyOn(platformApi, 'post').mockResolvedValue({ data: { data: { moved_count: 1, batch_id: 'batch-1' } } })
    const { result } = renderHook(() => useRequirementMutations(7), { wrapper })

    await act(async () => {
      await result.current.move.mutateAsync({ id: 42, version_task_new_id: 19, expected_version: 3 })
    })

    expect(post).toHaveBeenCalledWith('/requirements/batch-move', {
      items: [{ requirement_id: 42, expected_version: 3 }],
      version_task_new_id: 19,
    })
  })

  it('rejects delete before transport when requirement pool delete is not granted', async () => {
    canDelete = false
    const remove = vi.spyOn(platformApi, 'delete').mockResolvedValue({ data: {} })
    const { result } = renderHook(() => useRequirementMutations(7), { wrapper })

    await expect(result.current.remove.mutateAsync(42)).rejects.toMatchObject({
      code: 'ACTION_PERMISSION_DENIED',
      status: 403,
    })
    expect(remove).not.toHaveBeenCalled()
  })

  it('uses the independently grantable role-owner endpoint for owner-only updates', async () => {
    const patch = vi.spyOn(platformApi, 'patch').mockResolvedValue({ data: { data: { id: 42 } } })
    const { result } = renderHook(() => useRequirementMutations(7), { wrapper })

    await act(async () => {
      await result.current.patch.mutateAsync({ id: 42, expected_version: 3, tester_user_ids: [7] })
    })

    expect(patch).toHaveBeenCalledWith('/requirements/42/role-owners', {
      expected_version: 3,
      tester_user_ids: [7],
    })
  })
})
