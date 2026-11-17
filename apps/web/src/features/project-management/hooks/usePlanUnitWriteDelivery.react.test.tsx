import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import type { PlanUnitEditorState } from '../model'
import { planUnitDraftFromCreate, planUnitDraftFromUnit } from '../model'
import { usePlanUnitWriteDelivery } from './usePlanUnitWriteDelivery'

const mocks = (() => ({
  query: {
    data: undefined as unknown,
    error: null as unknown,
    isError: false,
    isFetching: false,
    refetch: vi.fn(),
  },
  create: vi.fn(),
  rename: vi.fn(),
  remove: vi.fn(),
}))()

mock.module('../model', () => ({
  useDeliveryLabelsQuery: () => mocks.query,
}))

vi.mock('./useDeliveryMutations', () => ({
  useCreateDeliveryLabelMutation: () => ({ execute: mocks.create }),
  useRenameDeliveryLabelMutation: () => ({ execute: mocks.rename }),
  useDeleteDeliveryLabelMutation: () => ({ execute: mocks.remove }),
}))

vi.mock('./useProjectMutations', () => ({
  isPMMutationTicketCurrent: () => true,
}))

const project = {
  id: 9,
  owner_user_id: 11,
}

function setup(editor: PlanUnitEditorState) {
  const dispatch = vi.fn()
  const operationLock = { current: null as symbol | null }
  const result = renderHook(() => usePlanUnitWriteDelivery({
    editor,
    dispatch,
    operationLock,
    scopeKey: '7:1',
    currentScopeKey: () => '7:1',
    canManage: () => true,
    onContextChanged: vi.fn(),
  }))
  return { ...result, dispatch, operationLock }
}

describe('project item editor delivery workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.query.data = undefined
    mocks.query.error = null
    mocks.query.isError = false
    mocks.query.isFetching = false
    mocks.create.mockResolvedValue({
      status: 'current',
      ticket: {},
      data: { id: 81, name: '灰度发布', sort_order: 50 },
    })
  })

  it('manages create labels only in the local draft', async () => {
    const draft = planUnitDraftFromCreate(project)
    const editor: PlanUnitEditorState = {
      status: 'editing',
      draft,
      deliveryCatalog: { status: 'ready' },
      validation: {},
      validationVisible: false,
      submit: { status: 'idle' },
    }
    const hook = setup(editor)
    act(() => hook.result.current.openLabelManager())
    act(() => hook.result.current.changeLabelName('灰度发布'))
    await act(async () => hook.result.current.createLabel())

    expect(mocks.create).not.toHaveBeenCalled()
    expect(hook.dispatch).toHaveBeenCalledWith({
      type: 'change',
      draft: expect.objectContaining({
        deliveryNodeLabels: expect.arrayContaining([
          expect.objectContaining({ client_key: 'custom_1', name: '灰度发布' }),
        ]),
      }),
    })
  })

  it('loads the persisted catalog before an edit can save', async () => {
    mocks.query.data = [{ id: 81, name: '灰度发布', sort_order: 50 }]
    const draft = planUnitDraftFromUnit({
      id: 31,
      team_id: 2,
      project_new_id: 9,
      title: 'Release',
      status: 'planning',
      delivery_nodes: [{ label_id: 81, label_name: '历史名称' }],
    })
    const editor: PlanUnitEditorState = {
      status: 'editing',
      draft,
      deliveryCatalog: { status: 'loading' },
      validation: {},
      validationVisible: false,
      submit: { status: 'idle' },
    }
    const hook = setup(editor)
    await waitFor(() => {
      expect(hook.dispatch).toHaveBeenCalledWith({
        type: 'delivery-loaded',
        labels: mocks.query.data,
      })
    })
  })

  it('uses the persisted label API only for edit mode', async () => {
    const draft = {
      ...planUnitDraftFromUnit({
        id: 31,
        team_id: 2,
        project_new_id: 9,
        title: 'Release',
        status: 'planning',
      }),
      deliveryNodeLabels: [{ id: 80, name: '验收', sort_order: 10 }],
    }
    const editor: PlanUnitEditorState = {
      status: 'editing',
      draft,
      deliveryCatalog: { status: 'ready' },
      validation: {},
      validationVisible: false,
      submit: { status: 'idle' },
    }
    const hook = setup(editor)
    act(() => hook.result.current.openLabelManager())
    act(() => hook.result.current.changeLabelName('灰度发布'))
    await act(async () => hook.result.current.createLabel())

    expect(mocks.create).toHaveBeenCalledWith(9, 31, '灰度发布')
    expect(hook.dispatch).toHaveBeenCalledWith({
      type: 'change',
      draft: expect.objectContaining({
        deliveryNodeLabels: expect.arrayContaining([
          expect.objectContaining({ id: 81, name: '灰度发布' }),
        ]),
      }),
    })
  })
})
