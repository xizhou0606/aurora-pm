import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { coreApi, platformApi, PMSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'

import { seedActionPermissions } from '../utils/test-action-permissions'
import { WorkflowBatchPlanPanel } from './WorkflowBatchPlanPanel'
import { WorkflowSubjectLauncher } from './WorkflowSubjectLauncher'
import type { WorkflowInstance } from '../model/types'

const originalPlatformAdapter = platformApi.defaults.adapter
const originalCoreAdapter = coreApi.defaults.adapter
const subjectCopy = copy.aiPM.workflowSubject

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data, status: 200, statusText: 'OK', headers: {}, config }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((complete, fail) => { resolve = complete; reject = fail })
  return { promise, resolve, reject }
}

function workflowInstance(overrides: Partial<WorkflowInstance> = {}): WorkflowInstance {
  return {
    id: 91,
    template_id: 8,
    template_name: '项目交付流程',
    template_version: 2,
    status: 'running',
    can_manage_node_plans: true,
    subject_type: 'plan_unit',
    subject_id: 27,
    current_node_ids: ['qa'],
    current_nodes: [{ id: 'qa', type: 'human_task', name: 'QA 测试', assignee_user_id: 8 }],
    timeline_nodes: [
      { id: 'start', type: 'start', name: '开始', status: 'completed' },
      { id: 'delivery', type: 'human_task', name: '研发交付', status: 'completed', assignee_user_id: 7, assignee_name: '林工', planned_start_at: '2026-08-10', planned_end_at: '2026-08-18' },
      { id: 'split', type: 'parallel_split', name: '并行执行', status: 'completed' },
      { id: 'qa', type: 'human_task', name: 'QA 测试', status: 'active', assignee_user_id: 8, assignee_name: '周可', planned_start_at: '2026-08-19', planned_end_at: '2026-08-22' },
      { id: 'excluded', type: 'approval', name: '条件未选节点', status: 'skipped', assignee_user_id: 7 },
      { id: 'join', type: 'parallel_join', name: '并行汇聚', status: 'waiting' },
      { id: 'end', type: 'end', name: '结束', status: 'pending' },
    ],
    node_plans: {
      delivery: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-18', assignee_user_id: 7 },
      qa: { planned_start_at: '2026-08-19', planned_end_at: '2026-08-22', assignee_user_id: 8 },
    },
    definition_edges: [],
    node_runs: [],
    approvals: [],
    transition_history: [],
    completed_steps: 1,
    total_steps: 2,
    progress: 50,
    started_at: '2026-08-09T00:00:00Z',
    ...overrides,
  }
}

function renderLauncher(instance: WorkflowInstance, workspaceId = 7) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  seedActionPermissions([PM_ACTION.WorkflowsInstancesView, PM_ACTION.WorkflowsNodePlansUpdate])
  return render(<QueryClientProvider client={client}><WorkflowSubjectLauncher workspaceId={workspaceId} subjectType={instance.subject_type as 'requirement' | 'plan_unit'} subjectId={instance.subject_id} instanceId={instance.id} /></QueryClientProvider>)
}

async function selectDate(label: string, value: string) {
  const [year, month, day] = value.split('-').map(Number)
  const dayName = `选择 ${year}年${month}月${day}日`
  fireEvent.click(screen.getByRole('button', { name: label }))
  // The calendar opens on the field value's month, or the current month once
  // the value was cleared — navigate to the target month before picking a day.
  for (let steps = 0; steps < 48; steps += 1) {
    const dayButton = screen.queryByRole('button', { name: dayName })
    if (dayButton) {
      fireEvent.click(dayButton)
      return
    }
    const title = screen.getByText(/^\d{4}年\d{1,2}月$/)
    const [titleYear, titleMonth] = title.textContent!.match(/(\d+)年(\d+)月/)!.slice(1).map(Number)
    const delta = (year - titleYear) * 12 + (month - titleMonth)
    if (delta === 0) throw new Error(`calendar day not rendered: ${dayName}`)
    fireEvent.click(screen.getByRole('button', {
      name: delta < 0 ? copy.ui.datePicker.previousMonth : copy.ui.datePicker.nextMonth,
    }))
  }
  throw new Error(`could not navigate to ${dayName}`)
}

async function selectUser(label: string, option: string) {
  fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions(label) }))
  const item = await screen.findByRole('option', { name: new RegExp(option) })
  fireEvent.pointerDown(item, { button: 0, pointerType: 'mouse' })
  fireEvent.click(item)
}

describe('workflow batch plan and assignee editing', () => {
  beforeEach(() => {
    coreApi.defaults.adapter = async (config) => response(config, { data: [
      { id: 7, username: 'lin', display_name: '林工' },
      { id: 8, username: 'zhou', display_name: '周可' },
      { id: 9, username: 'chen', display_name: '陈敏' },
    ], total: 3 })
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalPlatformAdapter
    coreApi.defaults.adapter = originalCoreAdapter
  })

  it('edits two project workflow nodes with independently expanded editors and excludes structural or skipped nodes', async () => {
    let patchBody: { nodes?: unknown[] } | undefined
    const base = workflowInstance()
    let current = workflowInstance({
      timeline_nodes: [
        ...base.timeline_nodes,
        { id: 'recovery', type: 'human_task', name: '未发生的恢复节点', status: 'pending', assignee_user_id: 7 },
      ],
      selected_node_ids: ['start', 'delivery', 'split', 'qa', 'join', 'end'],
    })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data))
        current = {
          ...current,
          timeline_nodes: current.timeline_nodes.map((node) => node.id === 'delivery'
            ? { ...node, planned_start_at: null }
            : node.id === 'qa' ? { ...node, planned_end_at: '2026-08-25', assignee_user_id: 9, assignee_name: '陈敏' } : node),
          node_plans: {
            ...current.node_plans,
            delivery: { planned_start_at: null, planned_end_at: '2026-08-18', assignee_user_id: 7 },
            qa: { planned_start_at: '2026-08-19', planned_end_at: '2026-08-25', assignee_user_id: 9 },
          },
        }
        return response(config, { data: current })
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    expect(await screen.findByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeTruthy()
    expect(screen.getByTestId('workflow-batch-node-delivery')).toBeTruthy()
    expect(screen.getByTestId('workflow-batch-node-qa')).toBeTruthy()
    expect(screen.queryByTestId('workflow-batch-node-start')).toBeNull()
    expect(screen.queryByTestId('workflow-batch-node-split')).toBeNull()
    expect(screen.queryByTestId('workflow-batch-node-excluded')).toBeNull()
    expect(screen.queryByTestId('workflow-batch-node-recovery')).toBeNull()
    expect(screen.queryByRole('checkbox')).toBeNull()
    const saveButton = screen.getByRole('button', { name: subjectCopy.batchPlanSave }) as HTMLButtonElement
    expect(saveButton.disabled).toBe(true)
    expect(screen.getByText(subjectCopy.batchPlanChanged(0, 2))).toBeTruthy()

    const deliveryToggle = screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', false) })
    expect(deliveryToggle.getAttribute('aria-expanded')).toBe('false')
    fireEvent.click(deliveryToggle)
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', true) }).getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByText(subjectCopy.batchPlanCompletedAssigneeHint)).toBeTruthy()
    expect(saveButton.disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.nodePlanStartLabel('研发交付') }))
    fireEvent.click(await screen.findByRole('button', { name: copy.ui.datePicker.clear }))
    expect(screen.getByText(subjectCopy.batchPlanChanged(1, 2))).toBeTruthy()
    expect(saveButton.disabled).toBe(false)

    await selectDate(subjectCopy.nodePlanStartLabel('研发交付'), '2026-08-10')
    expect(screen.getByText(subjectCopy.batchPlanChanged(0, 2))).toBeTruthy()
    expect(saveButton.disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.nodePlanStartLabel('研发交付') }))
    fireEvent.click(await screen.findByRole('button', { name: copy.ui.datePicker.clear }))

    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', true) }).getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', true) }).getAttribute('aria-expanded')).toBe('true')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', true) }))
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', false) }).getAttribute('aria-expanded')).toBe('false')
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', true) }).getAttribute('aria-expanded')).toBe('true')
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    await selectUser(subjectCopy.nodeAssigneeLabel('QA 测试'), '陈敏')
    expect(screen.getByText(subjectCopy.batchPlanChanged(2, 2))).toBeTruthy()

    await waitFor(() => expect(saveButton.disabled).toBe(false))
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))
    await waitFor(() => expect(patchBody).toEqual({ nodes: [
      { node_id: 'delivery', planned_start_at: null, expected_planned_start_at: '2026-08-10' },
      { node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22', assignee_user_id: 9 },
    ] }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeNull())
  })

  it('hides the assignee picker for AI nodes and still lets dates be planned', async () => {
    let patchBody: { nodes?: unknown[] } | undefined
    const base = workflowInstance()
    const current = workflowInstance({
      timeline_nodes: [
        ...base.timeline_nodes,
        { id: 'ai', type: 'ai_check', name: 'AI 完整性检查', status: 'pending', capability: 'requirement.completeness' },
      ],
      selected_node_ids: ['start', 'delivery', 'split', 'qa', 'join', 'ai', 'end'],
    })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data))
        return response(config, { data: current })
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    expect(await screen.findByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('AI 完整性检查', false) }))
    expect(screen.getAllByText(subjectCopy.nodeAssigneeAuto).length).toBeGreaterThan(0)
    expect(screen.queryByRole('combobox', { name: subjectCopy.nodeAssigneeLabel('AI 完整性检查') })).toBeNull()
    await selectDate(subjectCopy.nodePlanEndLabel('AI 完整性检查'), '2026-08-30')

    const saveButton = screen.getByRole('button', { name: subjectCopy.batchPlanSave }) as HTMLButtonElement
    await waitFor(() => expect(saveButton.disabled).toBe(false))
    fireEvent.click(saveButton)
    await waitFor(() => expect(patchBody).toEqual({ nodes: [
      { node_id: 'ai', planned_end_at: '2026-08-30', expected_planned_end_at: null },
    ] }))
  })

  it('keeps an expanded row live across instance refreshes without submitting stale fields', async () => {
    const current = workflowInstance()
    const refreshed: WorkflowInstance = {
      ...current,
      timeline_nodes: current.timeline_nodes.map((node) => node.id === 'delivery'
        ? { ...node, planned_start_at: '2026-08-12' }
        : node),
      node_plans: {
        ...current.node_plans,
        delivery: { planned_start_at: '2026-08-12', planned_end_at: '2026-08-18', assignee_user_id: 7 },
      },
    }
    const save = vi.fn()
    const onOpenChange = vi.fn()
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const panel = (instance: WorkflowInstance) => <QueryClientProvider client={client}>
      <WorkflowBatchPlanPanel workspaceId={7} instance={instance} pending={false} saveError={false} reloadInstance={async () => instance} save={save} onOpenChange={onOpenChange} />
    </QueryClientProvider>
    const view = render(panel(current))
    await screen.findByRole('dialog', { name: subjectCopy.batchPlanTitle })

    const deliveryToggle = screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', false) })
    const editorId = deliveryToggle.getAttribute('aria-controls')
    const editor = editorId ? document.getElementById(editorId) : null
    expect(editor?.getAttribute('role')).toBe('region')
    expect(editor?.hidden).toBe(true)
    fireEvent.click(deliveryToggle)
    expect(editor?.hidden).toBe(false)
    expect((screen.getByRole('button', { name: subjectCopy.batchPlanSave }) as HTMLButtonElement).disabled).toBe(true)

    view.rerender(panel(refreshed))
    expect(await screen.findByText('2026-08-12 - 2026-08-18')).toBeTruthy()
    expect(screen.getByText(subjectCopy.batchPlanChanged(0, 2))).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', true) }).getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', true) }).getAttribute('aria-expanded')).toBe('true')
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))
    expect(save).toHaveBeenCalledWith([
      { node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' },
    ], expect.any(Function))
  })

  it('keeps a freshly picked assignee labeled after the picker closes', async () => {
    // Mirrors the production directory: 小豪 (id 101) is an active user. The
    // picker loads the directory once as a snapshot and filters locally, so
    // picking him never falls back to 已注销用户_101 and needs no extra requests.
    const lin = { id: 7, username: 'lin', display_name: '林工' }
    const zhou = { id: 8, username: 'zhou', display_name: '周可' }
    const hao = { id: 101, username: 'fs_a47236dc9bf552f3', display_name: '小豪' }
    let directoryRequests = 0
    coreApi.defaults.adapter = async (config) => {
      if (config.url === '/directory/users') {
        directoryRequests += 1
        return response(config, { data: [lin, zhou, hao], total: 3 })
      }
      return response(config, { data: [] })
    }
    const current = workflowInstance()
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(<QueryClientProvider client={client}>
      <WorkflowBatchPlanPanel workspaceId={7} instance={current} pending={false} saveError={false} reloadInstance={async () => current} save={vi.fn()} onOpenChange={vi.fn()} />
    </QueryClientProvider>)
    await screen.findByRole('dialog', { name: subjectCopy.batchPlanTitle })

    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    const assigneeLabel = subjectCopy.nodeAssigneeLabel('QA 测试')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions(assigneeLabel) }))
    const option = await screen.findByRole('option', { name: /小豪/ })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)

    await waitFor(() => expect(screen.getAllByText(/小豪/).length).toBeGreaterThan(0))
    expect(screen.queryByText(/已注销用户_101/)).toBeNull()
    expect(directoryRequests).toBe(1)
  })

  it('keeps a batch stale draft locked after rejection, then accepts an authoritative ABA reload', async () => {
    const current = workflowInstance()
    const firstReload = deferred<WorkflowInstance | undefined>()
    const reloadInstance = vi.fn()
      .mockReturnValueOnce(firstReload.promise)
      .mockResolvedValueOnce(current)
    const clearSaveError = vi.fn()
    const save = vi.fn()
    const onOpenChange = vi.fn()
    const staleError = new PMSafeApiError(
      'SCHEDULE_SUBJECT_STALE',
      409,
      false,
      undefined,
      { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE', node_id: 'qa' }] },
    )
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const panel = (saveError: unknown) => <QueryClientProvider client={client}>
      <WorkflowBatchPlanPanel
        workspaceId={7}
        instance={current}
        pending={false}
        saveError={saveError}
        clearSaveError={clearSaveError}
        reloadInstance={reloadInstance}
        save={save}
        onOpenChange={onOpenChange}
      />
    </QueryClientProvider>
    const view = render(panel(null))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    view.rerender(panel(staleError))
    clearSaveError.mockClear()

    const reloadLatest = () => screen.getByRole('button', { name: subjectCopy.scheduleReloadLatest }) as HTMLButtonElement
    const saveButton = () => screen.getByRole('button', { name: subjectCopy.batchPlanSave }) as HTMLButtonElement
    expect(saveButton().disabled).toBe(true)
    expect((screen.getByRole('button', { name: subjectCopy.nodePlanEndLabel('QA 测试') }) as HTMLButtonElement).disabled).toBe(true)

    fireEvent.click(reloadLatest())
    fireEvent.click(reloadLatest())
    expect(reloadInstance).toHaveBeenCalledTimes(1)
    await act(async () => {
      firstReload.reject(new Error('refresh failed'))
      await Promise.resolve()
    })
    await waitFor(() => expect(reloadLatest().disabled).toBe(false))
    expect(clearSaveError).not.toHaveBeenCalled()
    expect(saveButton().disabled).toBe(true)

    fireEvent.click(reloadLatest())
    await waitFor(() => expect(reloadInstance).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(clearSaveError).toHaveBeenCalledTimes(1))
    expect(screen.getByRole('button', { name: subjectCopy.nodePlanEndLabel('QA 测试') }).textContent).toContain('2026年8月25日')
    view.rerender(panel(null))
    expect(saveButton().disabled).toBe(false)
    fireEvent.click(saveButton())
    expect(save).toHaveBeenCalledWith([
      { node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' },
    ], expect.any(Function))
  })

  it('does not prefill batch assignees from leftover template config', async () => {
    const current = workflowInstance({
      current_node_ids: ['review'],
      current_nodes: [{ id: 'review', type: 'approval', name: '评审', assignee_user_id: 7 }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '评审', status: 'active', assignee_user_id: 7, assignee_name: '林工', config: { assignee_user_id: 9 } },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      node_plans: {},
    })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanToggleNode('评审', false) }))
    expect((screen.getByRole('combobox', { name: subjectCopy.nodeAssigneeLabel('评审') }) as HTMLInputElement).value).toBe('林工')
  })

  it('validates requirement dates and required assignees, then retains drafts after a failed save', async () => {
    let failPatch = true
    let patchBody: { nodes?: unknown[] } | undefined
    const requirement = workflowInstance({
      id: 77,
      template_name: '需求生命周期',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['input'],
      current_nodes: [{ id: 'input', type: 'form', name: '需求录入', assignee_user_id: 7 }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'active', assignee_user_id: 7, assignee_name: '林工', planned_start_at: '2026-08-20', planned_end_at: '2026-08-25' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      node_plans: { input: { planned_start_at: '2026-08-20', planned_end_at: '2026-08-25', assignee_user_id: 7 } },
      completed_steps: 0,
      total_steps: 1,
      progress: 0,
    })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 77 }], total: 1 })
      if (config.url === '/pm/workflows/instances/77' && config.method === 'get') return response(config, { data: requirement })
      if (config.url === '/pm/workflows/instances/77/node-plans' && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data))
        if (failPatch) throw new Error('temporary failure')
        return response(config, { data: requirement })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirement)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    await screen.findByRole('dialog', { name: subjectCopy.batchPlanTitle })
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('需求录入', false) }))

    await selectDate(subjectCopy.nodePlanStartLabel('需求录入'), '2026-08-30')
    await waitFor(() => expect((screen.getByRole('button', { name: subjectCopy.batchPlanSave }) as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))
    expect(await screen.findByText(subjectCopy.nodePlanDateOrder)).toBeTruthy()
    await selectDate(subjectCopy.nodePlanStartLabel('需求录入'), '2026-08-21')
    await selectUser(subjectCopy.nodeAssigneeLabel('需求录入'), copy.ui.userSelect.clear)
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))
    expect(await screen.findByText(subjectCopy.batchPlanAssigneeRequired)).toBeTruthy()

    await selectUser(subjectCopy.nodeAssigneeLabel('需求录入'), '陈敏')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))
    expect(await screen.findByText(subjectCopy.batchPlanSaveFailed)).toBeTruthy()
    expect((screen.getByRole('combobox', { name: subjectCopy.nodeAssigneeLabel('需求录入') }) as HTMLInputElement).value).toBe('陈敏')
    expect(screen.getByRole('button', { name: subjectCopy.nodePlanStartLabel('需求录入') }).textContent).toContain('2026年8月21日')

    failPatch = false
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))
    await waitFor(() => expect(patchBody).toEqual({ nodes: [{ node_id: 'input', planned_start_at: '2026-08-21', expected_planned_start_at: '2026-08-20', assignee_user_id: 9 }] }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeNull())
  })

  it('retains node drafts through 409 and resubmits the same plan only after explicit schedule confirmation', async () => {
    const current = workflowInstance()
    const patchBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        patchBodies.push(body)
        if (!body.confirm_subject_schedule) {
          throw {
            config,
            response: {
              config,
              status: 409,
              data: {
                code: 'WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED',
                message: 'private schedule details',
                schedule: {
                  state: 'partial',
                  can_confirm: true,
                  suggested: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-25' },
                  conflicts: [],
                },
              },
            },
          }
        }
        return response(config, { data: { ...current, subject_schedule: {
          state: 'scheduled',
          can_confirm: false,
          subject: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-25' },
          conflicts: [],
        } } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))

    expect(await screen.findByTestId('workflow-schedule-confirmation')).toBeTruthy()
    expect(screen.queryByText('private schedule details')).toBeNull()
    expect(screen.getByRole('button', { name: subjectCopy.nodePlanEndLabel('QA 测试') }).textContent).toContain('2026年8月25日')
    expect(patchBodies).toEqual([{ nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }] }])

    fireEvent.click(screen.getByRole('button', { name: subjectCopy.scheduleConfirmSuggested }))
    await waitFor(() => expect(patchBodies).toEqual([
      { nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }] },
      { nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }], confirm_subject_schedule: true },
    ]))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeNull())
  })

  it('rebases a batch draft onto the refreshed node dates after a stale schedule response', async () => {
    let current = workflowInstance()
    let instanceGets = 0
    const patchBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') {
        instanceGets += 1
        return response(config, { data: current })
      }
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        patchBodies.push(body)
        if (patchBodies.length === 1) {
          current = {
            ...current,
            timeline_nodes: current.timeline_nodes.map((node) => node.id === 'qa' ? { ...node, planned_end_at: '2026-08-24T12:00:00Z' } : node),
            node_plans: {
              ...current.node_plans,
              qa: { ...current.node_plans?.qa, planned_end_at: '2026-08-24T12:00:00Z' },
            },
          }
          throw {
            config,
            response: {
              config,
              status: 409,
              data: {
                code: 'SCHEDULE_SUBJECT_STALE',
                message: 'stale schedule',
                schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE', node_id: 'qa' }] },
              },
            },
          }
        }
        current = {
          ...current,
          timeline_nodes: current.timeline_nodes.map((node) => node.id === 'qa' ? { ...node, planned_end_at: '2026-08-25' } : node),
          node_plans: {
            ...current.node_plans,
            qa: { ...current.node_plans?.qa, planned_end_at: '2026-08-25' },
          },
        }
        return response(config, { data: current })
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))

    expect(await screen.findByText(subjectCopy.scheduleStale)).toBeTruthy()
    expect(screen.getByRole('button', { name: subjectCopy.nodePlanEndLabel('QA 测试') }).textContent).toContain('2026年8月25日')
    await waitFor(() => expect(patchBodies).toEqual([{
      nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }],
    }]))

    fireEvent.click(screen.getByRole('button', { name: subjectCopy.scheduleReloadLatest }))
    await waitFor(() => expect(instanceGets).toBeGreaterThanOrEqual(3))
    await waitFor(() => expect(screen.queryByRole('button', { name: subjectCopy.scheduleReloadLatest })).toBeNull())
    expect(screen.getByRole('button', { name: subjectCopy.nodePlanEndLabel('QA 测试') }).textContent).toContain('2026年8月25日')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))

    await waitFor(() => expect(patchBodies).toEqual([
      { nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }] },
      { nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-24T12:00:00Z' }] },
    ]))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeNull())
  })

  it('keeps a single-node stale draft through reload failures, then accepts an authoritative ABA read', async () => {
    const current = workflowInstance()
    let scheduleRejected = false
    let instanceGets = 0
    const patchBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') {
        instanceGets += 1
        if (scheduleRejected && (instanceGets === 2 || instanceGets === 3)) {
          throw {
            config,
            response: { config, status: 503, data: { code: 'TEMPORARY_FAILURE' } },
          }
        }
        return response(config, { data: current })
      }
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        patchBodies.push(body)
        if (patchBodies.length === 1) {
          scheduleRejected = true
          throw {
            config,
            response: {
              config,
              status: 409,
              data: {
                code: 'SCHEDULE_SUBJECT_STALE',
                message: 'stale schedule',
                schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE', node_id: 'qa' }] },
              },
            },
          }
        }
        return response(config, { data: current })
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('QA 测试') }))
    await screen.findByRole('dialog', { name: subjectCopy.todoTitle('QA 测试') })
    await selectDate(subjectCopy.nodePlanStartLabel('QA 测试'), '2026-08-20')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.nodePlanSaveOne('QA 测试') }))

    expect(await screen.findByText(subjectCopy.scheduleStale)).toBeTruthy()
    expect(patchBodies).toEqual([{
      nodes: [{ node_id: 'qa', planned_start_at: '2026-08-20', expected_planned_start_at: '2026-08-19' }],
    }])
    const reloadLatest = () => screen.getByRole('button', { name: subjectCopy.scheduleReloadLatest }) as HTMLButtonElement
    const saveButton = () => screen.getByRole('button', { name: subjectCopy.nodePlanSaveOne('QA 测试') }) as HTMLButtonElement

    fireEvent.click(reloadLatest())
    fireEvent.click(reloadLatest())
    await waitFor(() => expect(instanceGets).toBeGreaterThanOrEqual(3))
    expect(instanceGets).toBe(3)
    expect(reloadLatest()).toBeTruthy()
    expect(saveButton().disabled).toBe(true)

    fireEvent.click(reloadLatest())
    await waitFor(() => expect(instanceGets).toBeGreaterThanOrEqual(4))
    await waitFor(() => expect(screen.queryByRole('button', { name: subjectCopy.scheduleReloadLatest })).toBeNull())
    expect(saveButton().disabled).toBe(false)
    fireEvent.click(saveButton())

    await waitFor(() => expect(patchBodies).toEqual([
      { nodes: [{ node_id: 'qa', planned_start_at: '2026-08-20', expected_planned_start_at: '2026-08-19' }] },
      { nodes: [{ node_id: 'qa', planned_start_at: '2026-08-20', expected_planned_start_at: '2026-08-19' }] },
    ]))
  })

  it('keeps an authoritative null from a successful single-node response as the next baseline', async () => {
    const current = workflowInstance()
    const patchBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        patchBodies.push(body)
        return response(config, { data: { node_plans: {
          qa: {
            planned_start_at: patchBodies.length === 1 ? null : '2026-08-20',
            planned_end_at: '2026-08-25',
          },
        } } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('QA 测试') }))
    await screen.findByRole('dialog', { name: subjectCopy.todoTitle('QA 测试') })
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.nodePlanSaveOne('QA 测试') }))
    await waitFor(() => expect(patchBodies).toEqual([{
      nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }],
    }]))

    await selectDate(subjectCopy.nodePlanStartLabel('QA 测试'), '2026-08-20')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.nodePlanSaveOne('QA 测试') }))
    await waitFor(() => expect(patchBodies).toEqual([
      { nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }] },
      { nodes: [{ node_id: 'qa', planned_start_at: '2026-08-20', expected_planned_start_at: null }] },
    ]))
  })

  it('does not promote closed batch or single-node 409 errors into the schedule summary', async () => {
    const current = workflowInstance()
    const patchBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        patchBodies.push(JSON.parse(String(config.data)) as Record<string, unknown>)
        throw {
          config,
          response: {
            config,
            status: 409,
            data: {
              code: 'WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED',
              message: 'private schedule details',
              schedule: {
                state: 'partial',
                can_confirm: true,
                suggested: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-26' },
                conflicts: [],
              },
            },
          },
        }
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))
    expect(await screen.findByTestId('workflow-schedule-confirmation')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeNull())
    expect(screen.queryByTestId('workflow-schedule-confirmation')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: subjectCopy.openTodo('QA 测试') }))
    await screen.findByRole('dialog', { name: subjectCopy.todoTitle('QA 测试') })
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-26')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.nodePlanSaveOne('QA 测试') }))
    expect(await screen.findByTestId('workflow-schedule-confirmation')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: subjectCopy.todoTitle('QA 测试') })).toBeNull())
    expect(screen.queryByTestId('workflow-schedule-confirmation')).toBeNull()
    expect(patchBodies).toEqual([
      { nodes: [{ node_id: 'qa', planned_end_at: '2026-08-25', expected_planned_end_at: '2026-08-22' }] },
      { nodes: [{ node_id: 'qa', planned_end_at: '2026-08-26', expected_planned_end_at: '2026-08-22' }] },
    ])
  })

  it('keeps a 409 from the summary confirmation visible at the summary', async () => {
    const current = workflowInstance({
      subject_schedule: {
        state: 'partial',
        can_confirm: true,
        subject: { planned_start_at: null, planned_end_at: null },
        suggested: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-22' },
        conflicts: [],
      },
    })
    let patchBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data)) as Record<string, unknown>
        throw {
          config,
          response: {
            config,
            status: 409,
            data: {
              code: 'WORKFLOW_SCHEDULE_CONFLICT',
              message: 'private sibling workflow details',
              schedule: {
                state: 'conflict',
                can_confirm: false,
                conflicts: [{ code: 'NODE_AFTER_SUBJECT_END', node_id: 'qa', value: '2026-08-25' }],
              },
            },
          },
        }
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.scheduleConfirmSuggested }))

    expect(await screen.findByTestId('workflow-schedule-conflict')).toBeTruthy()
    expect(screen.queryByText('private sibling workflow details')).toBeNull()
    expect(patchBody).toEqual({ nodes: [], confirm_subject_schedule: true })
  })

  it('names conflicting workflow nodes locally, expands them and keeps the draft for adjustment', async () => {
    const current = workflowInstance()
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      if (config.url === `/pm/workflows/instances/${current.id}/node-plans` && config.method === 'patch') {
        throw {
          config,
          response: {
            config,
            status: 409,
            data: {
              code: 'WORKFLOW_SCHEDULE_CONFLICT',
              message: 'private conflict message',
              schedule: {
                state: 'conflict',
                can_confirm: false,
                conflicts: [{ code: 'NODE_AFTER_SUBJECT_END', node_id: 'qa', boundary: '2026-08-23', value: '2026-08-25' }],
              },
            },
          },
        }
      }
      return response(config, { data: [] })
    }

    renderLauncher(current)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', false) }))
    await selectDate(subjectCopy.nodePlanEndLabel('QA 测试'), '2026-08-25')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanSave }))

    expect(await screen.findByText(subjectCopy.scheduleNodeAfterEnd('QA 测试', '2026-08-25'))).toBeTruthy()
    expect(screen.queryByText('private conflict message')).toBeNull()
    expect(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('QA 测试', true) }).getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('button', { name: subjectCopy.nodePlanEndLabel('QA 测试') }).textContent).toContain('2026年8月25日')
    expect(screen.queryByRole('button', { name: subjectCopy.scheduleConfirmSuggested })).toBeNull()
  })

  it('resets batch drafts when the workspace and workflow instance change', async () => {
    const first = workflowInstance({ id: 77, subject_type: 'requirement', subject_id: 42, template_name: '旧需求流程' })
    const second = workflowInstance({
      id: 88,
      subject_type: 'requirement',
      subject_id: 43,
      template_name: '新需求流程',
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '新需求评审', status: 'pending', assignee_user_id: 9, assignee_name: '陈敏' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      node_plans: { review: { assignee_user_id: 9 } },
    })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances/77' && config.method === 'get') return response(config, { data: first })
      if (config.url === '/pm/workflows/instances/88' && config.method === 'get') return response(config, { data: second })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    seedActionPermissions([PM_ACTION.WorkflowsInstancesView, PM_ACTION.WorkflowsNodePlansUpdate])
    const view = render(<QueryClientProvider client={client}><WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} instanceId={77} /></QueryClientProvider>)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.batchPlanButton }))
    await screen.findByRole('dialog', { name: subjectCopy.batchPlanTitle })
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanToggleNode('研发交付', false) }))
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.nodePlanStartLabel('研发交付') }))
    fireEvent.click(await screen.findByRole('button', { name: copy.ui.datePicker.clear }))
    expect(screen.getAllByText(subjectCopy.batchPlanChanged(1, 2)).length).toBeGreaterThan(0)

    view.rerender(<QueryClientProvider client={client}><WorkflowSubjectLauncher workspaceId={8} subjectType="requirement" subjectId={43} instanceId={88} /></QueryClientProvider>)
    expect(await screen.findByText('新需求流程')).toBeTruthy()
    expect(screen.queryByRole('dialog', { name: subjectCopy.batchPlanTitle })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.batchPlanButton }))
    expect((await screen.findAllByText(subjectCopy.batchPlanChanged(0, 1))).length).toBeGreaterThan(0)
    expect(screen.getByTestId('workflow-batch-node-review')).toBeTruthy()
    expect(screen.queryByTestId('workflow-batch-node-delivery')).toBeNull()
  })
})


afterAll(() => { console.log('FILE-AFTERALL-RAN') })
