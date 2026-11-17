import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, screen } from '@testing-library/react'
import { type PropsWithChildren } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { OperationFeedbackProvider } from '@/shared/ui'
import type { QualityClient } from '../api'
import type { QualityEnvironment } from '../components/environment'
import type { QualityBug, QualityRouteState } from '../model/types'
import { useInlineEditMutation, useUploadMutation } from './useMutations'

function queryWrapper(client: QueryClient) {
  return ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

describe('useUploadMutation', () => {
  it('sends non-media attachments to PM and always reports failures by Toast', async () => {
    const upload = vi.fn().mockRejectedValue(new PMSafeApiError('PM_QUALITY_UPLOAD_FAILED', 503))
    const environment = {
      client: { upload } as unknown as QualityClient,
      workspaceId: 2,
      userId: 47,
      sessionRevision: 1,
      pathname: '/workspace/quality-management/bugs/31',
      search: {},
      allowed: true,
      navigate: vi.fn().mockResolvedValue(undefined),
    } satisfies QualityEnvironment
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const wrapper = ({ children }: PropsWithChildren) => (
      <QueryClientProvider client={client}>
        <OperationFeedbackProvider>{children}</OperationFeedbackProvider>
      </QueryClientProvider>
    )
    const { result } = renderHook(() => useUploadMutation(environment, true), { wrapper })
    const file = new File(['title,status'], '1.4 需求迭代.csv', { type: 'text/csv' })

    await act(async () => {
      await expect(result.current.run({ requirementId: 101, file })).rejects.toMatchObject({ status: 503 })
    })

    expect(upload).toHaveBeenCalledWith(101, file)
    expect(await screen.findByText(OPERATION_FEEDBACK_COPY.quality.upload.failure)).not.toBeNull()
    expect(await screen.findByText(OPERATION_FEEDBACK_COPY.error.server)).not.toBeNull()
  })

  it('fails closed before calling the upload client when the action is denied', async () => {
    const upload = vi.fn()
    const environment = {
      client: { upload } as unknown as QualityClient,
      workspaceId: 2,
      userId: 47,
      sessionRevision: 1,
      pathname: '/workspace/quality-management/bugs/31',
      search: {},
      allowed: true,
      navigate: vi.fn().mockResolvedValue(undefined),
    } satisfies QualityEnvironment
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const wrapper = ({ children }: PropsWithChildren) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )
    const { result } = renderHook(() => useUploadMutation(environment, false), { wrapper })

    await act(async () => {
      await expect(result.current.run({
        requirementId: 101,
        file: new File(['evidence'], 'evidence.txt'),
      })).rejects.toMatchObject({ code: 'QUALITY_ACTION_PERMISSION_REQUIRED', status: 403 })
    })
    expect(upload).not.toHaveBeenCalled()
  })
})

describe('useInlineEditMutation', () => {
  it('routes workflow-only payloads to transition without widening sensitive edit permission', async () => {
    const patch = vi.fn().mockResolvedValue({
      id: 31, workspace_id: 2, title: 'Bug', status: 'triage',
    })
    const transition = vi.fn().mockResolvedValue({
      id: 31, workspace_id: 2, title: 'Bug', status: 'review',
    })
    const environment = {
      client: { patch, transition } as unknown as QualityClient,
      workspaceId: 2,
      userId: 47,
      sessionRevision: 1,
      pathname: '/workspace/quality-management',
      search: {},
      allowed: true,
      navigate: vi.fn().mockResolvedValue(undefined),
    } satisfies QualityEnvironment
    const route = {
      kind: 'list', view: 'list', mode: 'view', scope: {
        projectSetIds: [], projectIds: [], requirementIds: [],
      }, filters: {
        q: '', status: '', severities: [], environments: [], assigneeUserIds: [],
        ownerUserIds: [], reporterUserIds: [], relation: '', defectDomain: '', createdFrom: '', createdTo: '',
        sortBy: 'created_at', sortOrder: 'desc',
      }, page: 1, pageSize: 20,
    } satisfies QualityRouteState
    const bug = {
      id: 31, workspaceId: 2, title: 'Bug', status: 'triage',
      reproductionMaterials: [], resolutionMaterials: [], capabilities: {
        canUpdate: false, canDelete: false, canAssign: false, canResolve: true,
        canReturn: true, canTransition: true, canUpload: false, allowedTransitions: ['review'],
      },
    } satisfies QualityBug
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const { result } = renderHook(() => useInlineEditMutation(
      environment, route, () => false, () => true,
    ), { wrapper: queryWrapper(client) })

    await act(async () => {
      await expect(result.current.run({ bug, payload: { status: 'review' } })).resolves.toBeTruthy()
    })
    expect(transition).toHaveBeenCalledWith(31, { status: 'review' })
    expect(patch).not.toHaveBeenCalled()

    await act(async () => {
      await expect(result.current.run({
        bug, payload: { status: 'review', severity_key: 'p0' },
      })).rejects.toMatchObject({ code: 'QUALITY_ACTION_PERMISSION_REQUIRED', status: 403 })
    })
    expect(patch).not.toHaveBeenCalled()
  })
})
