import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import {
  runtimeSessionStore,
  runtimeWorkspaceStore,
} from '@/shared/runtime'
import type {
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import { useDeliveryWorkflow } from './useDeliveryWorkflow'

const api = (() => ({
  listLabels: vi.fn(),
}))()
const mutations = (() => ({
  updateNodes: vi.fn(),
  createLabel: vi.fn(),
  renameLabel: vi.fn(),
  deleteLabel: vi.fn(),
}))()

mock.module('../api', () => ({
  listScrumProjectDeliveryNodeLabelsNew: api.listLabels,
}))

vi.mock('./useDeliveryMutations', () => ({
  useUpdateDeliveryNodesMutation: () => ({ execute: mutations.updateNodes }),
  useCreateDeliveryLabelMutation: () => ({ execute: mutations.createLabel }),
  useRenameDeliveryLabelMutation: () => ({ execute: mutations.renameLabel }),
  useDeleteDeliveryLabelMutation: () => ({ execute: mutations.deleteLabel }),
}))

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'PM React',
  status: 'active',
}

const editableUnit: ScrumVersionTaskNew = {
  id: 31,
  team_id: 2,
  project_new_id: 9,
  title: 'Release',
  status: 'development',
  delivery_nodes: [{
    label_id: 1,
    label_name: 'Alpha',
    due_at: null,
    actual_completed_at: null,
    owner_user_id: null,
  }],
}

function readyRuntime(workspaceId = 7) {
  runtimeSessionStore.getState().clearSession()
  runtimeSessionStore.getState().setStandaloneToken('p8-delivery-token')
  runtimeWorkspaceStore.getState().selectWorkspace(workspaceId)
}

function ticket() {
  return {
    workspaceId: runtimeWorkspaceStore.getState().currentWorkspaceId,
    sessionRevision: runtimeSessionStore.getState().revision,
  }
}

function setup(unit = editableUnit) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  const hook = renderHook(
    ({ currentUnit }) => useDeliveryWorkflow({
      project,
      units: [currentUnit],
      users: [],
    }),
    { initialProps: { currentUnit: unit }, wrapper },
  )
  return Object.assign(hook, { client })
}

describe('P8 delivery workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    readyRuntime()
    api.listLabels.mockResolvedValue([{ id: 1, name: 'Alpha', sort_order: 10 }])
    mutations.updateNodes.mockResolvedValue({
      status: 'applied',
      data: editableUnit,
      ticket: ticket(),
    })
    mutations.createLabel.mockResolvedValue({
      status: 'applied',
      data: { id: 2, name: 'Beta', sort_order: 20 },
      ticket: ticket(),
    })
  })

  it('opens without authorization metadata and rechecks the latest resource scope at submit time', async () => {
    const hook = setup()
    act(() => hook.result.current.editor.open(editableUnit))
    expect(hook.result.current.editor.state.status).toBe('editing')
    hook.rerender({ currentUnit: { ...editableUnit, project_new_id: 10 } })
    await act(async () => { await hook.result.current.editor.submit() })
    expect(mutations.updateNodes).not.toHaveBeenCalled()
    expect(hook.result.current.editor.state).toMatchObject({
      status: 'editing',
      submit: { status: 'error', errorCode: 'RESOURCE_CHANGED' },
    })
  })

  it('resets editors on workspace change and gives a stale result zero UI side effects', async () => {
    let settle: ((value: unknown) => void) | undefined
    mutations.updateNodes.mockReturnValueOnce(new Promise((resolve) => { settle = resolve }))
    const hook = setup()
    act(() => hook.result.current.editor.open(editableUnit))
    let pending: Promise<void> | undefined
    act(() => { pending = hook.result.current.editor.submit() })
    expect(hook.result.current.editor.state).toMatchObject({
      status: 'editing',
      submit: { status: 'pending' },
    })

    act(() => runtimeWorkspaceStore.getState().selectWorkspace(8))
    await waitFor(() => expect(hook.result.current.editor.state).toEqual({ status: 'closed' }))
    settle?.({ status: 'stale', data: editableUnit, ticket: ticket() })
    await act(async () => { await pending })
    expect(hook.result.current.feedback).toEqual({ status: 'idle' })
    expect(hook.result.current.editor.state).toEqual({ status: 'closed' })
  })

  it('resets delivery state and discards completion after the project or plan-unit scope changes', async () => {
    let settle: ((value: unknown) => void) | undefined
    mutations.updateNodes.mockReturnValueOnce(new Promise((resolve) => { settle = resolve }))
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    )
    const hook = renderHook(
      ({ currentProject, currentUnit }) => useDeliveryWorkflow({
        project: currentProject,
        units: [currentUnit],
        users: [],
      }),
      { initialProps: { currentProject: project, currentUnit: editableUnit }, wrapper },
    )
    act(() => hook.result.current.editor.open(editableUnit))
    act(() => { void hook.result.current.editor.submit() })
    expect(hook.result.current.editor.state).toMatchObject({
      status: 'editing',
      submit: { status: 'pending' },
    })

    hook.rerender({
      currentProject: { ...project, id: 10 },
      currentUnit: { ...editableUnit, id: 32, project_new_id: 10 },
    })
    await waitFor(() => expect(hook.result.current.editor.state).toEqual({ status: 'closed' }))
    await act(async () => {
      settle?.({ status: 'applied', data: editableUnit, ticket: ticket() })
      await Promise.resolve()
    })
    expect(hook.result.current.feedback).toEqual({ status: 'idle' })
    expect(hook.result.current.editor.state).toEqual({ status: 'closed' })
  })

  it('trims names and rejects case-insensitive duplicate labels before mutation', async () => {
    const hook = setup()
    act(() => hook.result.current.labelManager.open(editableUnit))
    await waitFor(() => expect(hook.result.current.labels.status).toBe('ready'))
    act(() => hook.result.current.labelManager.changeName('  alpha  '))
    await act(async () => { await hook.result.current.labelManager.create() })
    expect(mutations.createLabel).not.toHaveBeenCalled()
    expect(hook.result.current.labelManager.state).toMatchObject({
      status: 'open',
      draftName: '  alpha  ',
      action: { status: 'error', errorCode: 'LABEL_DUPLICATE' },
    })
  })

  it('keeps the loaded label catalog ready when a background refresh fails', async () => {
    const hook = setup()
    act(() => hook.result.current.labelManager.open(editableUnit))
    await waitFor(() => expect(hook.result.current.labels).toMatchObject({
      status: 'ready',
      items: [{ id: 1, name: 'Alpha' }],
    }))

    api.listLabels.mockRejectedValueOnce(new Error('refresh failed'))
    await act(async () => {
      await hook.client.refetchQueries()
    })
    await waitFor(() => expect(hook.result.current.labels).toMatchObject({
      status: 'ready',
      items: [{ id: 1, name: 'Alpha' }],
      staleError: { retry: expect.any(Function) },
    }))
  })
})
