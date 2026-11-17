import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { useWorkflowInstanceControlHandlers } from './WorkflowInstanceControlHandlers'
import type { CoreHandlersParams } from './WorkflowInstanceControl-types'
import type { WorkflowInstance } from '../model/types'

function intakeNodeFixture(requiresConfirmation: boolean) {
  return {
    id: 'intake', type: 'human_task', name: '需求录入', status: 'active',
    can_act: true, allowed_actions: ['complete' as const],
    requires_requirement_lock_confirmation: requiresConfirmation,
  }
}

function handlerParams(overrides?: {
  actionNode?: ReturnType<typeof intakeNodeFixture> | null
  mutate?: ReturnType<typeof vi.fn>
  activeNodeById?: Map<string, ReturnType<typeof intakeNodeFixture> | null>
}) {
  const instance = {
    id: 168, subject_type: 'plan_unit', subject_id: 57, subject_title: 'V1.2',
    status: 'running', current_node_ids: ['intake'],
  } as unknown as WorkflowInstance
  const actionNode = overrides?.actionNode === null
    ? null
    : overrides?.actionNode ?? intakeNodeFixture(true)
  const mutate = overrides?.mutate ?? vi.fn()
  const activeNodeById = overrides?.activeNodeById
    ?? new Map([[actionNode?.id ?? 'intake', actionNode]])
  const params = {
    instance, actionNode, actionNodeIdRef: { current: actionNode?.id ?? null },
    actionIsActive: true, actionIsReadOnly: false, actionAIExecution: undefined,
    actionAllows: (action: string) => action === 'complete',
    canEditNodePlan: true, planScheduleStaleError: false,
    nodePlanReloading: false, nodePlanDraft: {}, nodePlanBaselines: {},
    actionPlanDates: { planned_start_at: null, planned_end_at: null },
    actionConditionDecision: '',
    nextAssignees: {}, comments: {}, designOwners: [], laneOwners: { frontend: null, backend: null },
    rollbackSourceNodeId: null, rollbackTargetNodeId: null, rollbackComment: '',
    requirement: undefined, activeNodeById,
    mutations: {
      action: { mutate, reset: vi.fn(), isPending: false, isError: false, error: null },
      planNodes: { mutate: vi.fn(), reset: vi.fn(), isPending: false, isError: false, error: null },
      uploadDesignImage: { isPending: false, isError: false, mutateAsync: vi.fn() },
    },
    requirementMutations: { patch: { mutateAsync: vi.fn(), isPending: false, reset: vi.fn() } },
    query: { refetch: vi.fn() },
    workflowAccess: { canActOnInstance: () => true },
    requirementAccess: { canUpdateRequirement: () => true },
    setActionNodeId: vi.fn(), setPlanNodesMutationSource: vi.fn(),
    setNodePlanDraft: vi.fn(), setNodePlanBaselines: vi.fn(), setNodePlanError: vi.fn(),
    setNodePlanReloading: vi.fn(), setRollbackSourceNodeId: vi.fn(),
    setRollbackTargetNodeId: vi.fn(), setRollbackComment: vi.fn(),
    nodePlanReloadOperation: { current: null },
    chrome: 'dialog' as const, onDismiss: vi.fn(), onInitialNodeClear: undefined,
  } as unknown as CoreHandlersParams
  return { params, mutate }
}

describe('requirement intake two-step lock confirmation', () => {
  it('opens the confirmation dialog on first submit and submits nothing yet', () => {
    const { params, mutate } = handlerParams()
    const { result } = renderHook(() => useWorkflowInstanceControlHandlers(params))
    act(() => { result.current.submit('intake', 'complete') })
    expect(mutate).not.toHaveBeenCalled()
    expect(result.current.intakeLockConfirmOpen).toBe(true)
    expect(result.current.intakeLockConfirmNodeId).toBe('intake')
  })

  it('keeps the confirmation bound to the intake node and clears on cancel', () => {
    const { params } = handlerParams()
    const { result } = renderHook(() => useWorkflowInstanceControlHandlers(params))
    act(() => { result.current.submit('intake', 'complete') })
    expect(result.current.intakeLockConfirmOpen).toBe(true)
    act(() => { result.current.cancelIntakeLockConfirm() })
    expect(result.current.intakeLockConfirmOpen).toBe(false)
    expect(result.current.intakeLockConfirmNodeId).toBeNull()
  })

  it('submits the atomic completion with confirm_requirement_lock on confirm', async () => {
    const { params, mutate } = handlerParams()
    const { result } = renderHook(() => useWorkflowInstanceControlHandlers(params))
    act(() => { result.current.submit('intake', 'complete') })
    act(() => { result.current.confirmIntakeLock('intake') })
    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(1))
    const payload = mutate.mock.calls[0][0]
    expect(payload).toMatchObject({ id: 168, action: 'complete', node_id: 'intake', confirm_requirement_lock: true })
    expect(result.current.intakeLockConfirmOpen).toBe(false)
  })

  it('completes a plain node directly without the lock confirmation', () => {
    const { params, mutate } = handlerParams({ actionNode: intakeNodeFixture(false) })
    const { result } = renderHook(() => useWorkflowInstanceControlHandlers(params))
    act(() => { result.current.submit('intake', 'complete') })
    expect(mutate).toHaveBeenCalledTimes(1)
    expect(mutate.mock.calls[0][0].confirm_requirement_lock).toBeUndefined()
    expect(result.current.intakeLockConfirmOpen).toBe(false)
  })
})

describe('requirement intake lock shortcut entry', () => {
  it('opens the lock confirmation for the active intake node with the node dialog closed', () => {
    const { params, mutate } = handlerParams({
      actionNode: null,
      activeNodeById: new Map([['intake', intakeNodeFixture(true)]]),
    })
    const { result } = renderHook(() => useWorkflowInstanceControlHandlers(params))
    act(() => { result.current.requestIntakeLock() })
    expect(mutate).not.toHaveBeenCalled()
    expect(result.current.intakeLockConfirmOpen).toBe(true)
    expect(result.current.intakeLockConfirmNodeId).toBe('intake')
  })

  it('does nothing when no active node requires lock confirmation', () => {
    const { params } = handlerParams({
      actionNode: null,
      activeNodeById: new Map([['intake', intakeNodeFixture(false)]]),
    })
    const { result } = renderHook(() => useWorkflowInstanceControlHandlers(params))
    act(() => { result.current.requestIntakeLock() })
    expect(result.current.intakeLockConfirmOpen).toBe(false)
  })

  it('shortcut confirm submits the composite action directly while the node dialog is closed', async () => {
    const { params, mutate } = handlerParams({
      actionNode: null,
      activeNodeById: new Map([['intake', intakeNodeFixture(true)]]),
    })
    const { result } = renderHook(() => useWorkflowInstanceControlHandlers(params))
    act(() => { result.current.requestIntakeLock() })
    expect(result.current.intakeLockConfirmOpen).toBe(true)
    act(() => { result.current.confirmIntakeLock('intake') })
    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(1))
    const [payload, options] = mutate.mock.calls[0]
    expect(payload).toMatchObject({ id: 168, action: 'complete', node_id: 'intake', confirm_requirement_lock: true })
    // 弹窗保持打开直到提交成功：成功回调之后才关闭。
    expect(result.current.intakeLockConfirmOpen).toBe(true)
    act(() => { options.onSuccess({ current_node_ids: ['review'] }) })
    expect(result.current.intakeLockConfirmOpen).toBe(false)
  })
})
