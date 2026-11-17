import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'bun:test'
import type { InternalAxiosRequestConfig } from 'axios'
import { PM_ACTION } from '@/generated/access-catalog'
import { coreApi, platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { seedActionPermissions } from '../utils/test-action-permissions'
import { WorkflowSubjectLauncher } from './WorkflowSubjectLauncher'
import type { Requirement, WorkflowInstance } from '../model/types'

const originalAdapter = platformApi.defaults.adapter
const originalCoreAdapter = coreApi.defaults.adapter
const subjectCopy = copy.aiPM.workflowSubject
const detailCopy = copy.aiPM.requirementDetail

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data, status: 200, statusText: 'OK', headers: {}, config }
}

function requirementFixture(overrides: Partial<Requirement> = {}): Requirement {
  return {
    id: 42, workspace_id: 7, team_id: 1, project_new_id: 11, version_task_new_id: 12, version: 3,
    title: '支付流程改造', description: '优化收银台交互', acceptance_criteria: '支付成功可核验',
    priority: 'high', status: 'grooming', task_count: 2, progress: 25, allowed_actions: [],
    created_at: '2026-08-06T00:00:00Z', updated_at: '2026-08-07T00:00:00Z',
    ...overrides,
  }
}

async function selectUser(label: string, option: string) {
  fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions(label) }))
  const item = await screen.findByRole('option', { name: new RegExp(option) })
  fireEvent.pointerDown(item, { button: 0, pointerType: 'mouse' })
  fireEvent.click(item)
  fireEvent.keyDown(item, { key: 'Escape' })
}

function renderLauncher(requirement?: Requirement) {
  const root = createRootRoute({ component: Outlet })
  const page = createRoute({
    getParentRoute: () => root,
    path: '/next-assignees',
    component: () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} requirement={requirement} />,
  })
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: ['/next-assignees'] }),
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  seedActionPermissions([
    PM_ACTION.WorkflowsInstancesView,
    PM_ACTION.WorkflowsInstancesAct,
    PM_ACTION.RequirementsPoolUpdate,
    PM_ACTION.RequirementsRoleOwnersAssign,
  ])
  return render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
}

describe('workflow next assignees in the todo dialog', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(7)
    coreApi.defaults.adapter = async (config) => response(config, { data: [
      { id: 21, username: 'reviewer', display_name: '评审同学' },
      { id: 31, username: 'designer', display_name: '设计同学' },
      { id: 41, username: 'developer', display_name: '研发同学' },
      { id: 51, username: 'bill', display_name: 'bill' },
    ], total: 4 })
  })
  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    coreApi.defaults.adapter = originalCoreAdapter
    runtimeSessionStore.getState().setIdentity(null)
    runtimeWorkspaceStore.getState().clearWorkspace()
  })

  it('blocks complete until the next node assignee is chosen and then sends it', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 90, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['input'],
      current_nodes: [{ id: 'input', type: 'form', name: '需求录入', allowed_actions: ['complete'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'active', allowed_actions: ['complete'], can_act: true },
        { id: 'review', type: 'approval', name: '产品评审', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'input-review', source: 'input', target: 'review' }],
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 90, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/90' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/90/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['review'] } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 90 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('需求录入') }))
    expect(await screen.findByTestId('workflow-next-assignees')).toBeTruthy()
    const complete = screen.getByRole('button', { name: subjectCopy.completeNode }) as HTMLButtonElement
    expect(complete.disabled).toBe(true)

    await selectUser(subjectCopy.nextAssigneeLabel('产品评审'), '评审同学')
    expect(complete.disabled).toBe(false)
    fireEvent.click(complete)
    await waitFor(() => expect(actionBody).toEqual({
      action: 'complete',
      node_id: 'input',
      next_node_assignees: [{ node_id: 'review', assignee_user_id: 21 }],
    }))
  })

  it('lets the UI/UX node set design owners without using them as the next assignee', async () => {
    let patched: Record<string, unknown> | undefined
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 91, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['design'],
      current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计', config: { require_design_image: true }, allowed_actions: ['complete', 'supplement_design'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true }, allowed_actions: ['complete', 'supplement_design'], can_act: true },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'design-end', source: 'design', target: 'end' }],
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 91, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/91' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/requirements/42/role-owners' && config.method === 'patch') {
        // design owner 赋值同样走专用 role-owners 端点（pool.ts 契约拆分）。
        patched = JSON.parse(String(config.data))
        return response(config, { data: { ...requirementFixture(), version: 4, design_owner_user_ids: [31] } })
      }
      if (config.url === '/pm/workflows/instances/91/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, status: 'completed', current_node_ids: [] } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 91 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('UI/UX设计') }))
    expect(await screen.findByTestId('workflow-design-owners')).toBeTruthy()
    expect(screen.queryByTestId('workflow-next-assignees')).toBeNull()
    fireEvent.change(screen.getByLabelText(subjectCopy.designLinkLabel), { target: { value: 'https://www.figma.com/design/abc/PM' } })
    await selectUser(detailCopy.fieldUI, '设计同学')
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.completeNode }))
    await waitFor(() => expect(patched).toEqual({ expected_version: 3, design_owner_user_ids: [31] }))
    await waitFor(() => expect(actionBody).toEqual({
      action: 'complete',
      node_id: 'design',
      design_url: 'https://www.figma.com/design/abc/PM',
    }))
  })

  it('lets the condition decision confirm or override the planned next assignee', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 92, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['ui'],
      current_nodes: [{ id: 'ui', type: 'condition', name: '是否需要UI设计', allowed_actions: ['condition_yes', 'condition_no'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'ui', type: 'condition', name: '是否需要UI设计', status: 'active', allowed_actions: ['condition_yes', 'condition_no'], can_act: true },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'pending' },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'start-ui', source: 'start', target: 'ui' },
        { id: 'yes', source: 'ui', target: 'design', condition: 'context.decision == true' },
        { id: 'no', source: 'ui', target: 'develop', condition: 'context.decision == false' },
      ],
      node_plans: { design: { assignee_user_id: 1 }, develop: { assignee_user_id: 1 } },
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 92, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/92' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/92/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['design'] } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 92 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('是否需要UI设计') }))
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.conditionYes }))
    expect(await screen.findByTestId('workflow-next-assignees')).toBeTruthy()
    await selectUser(subjectCopy.nextAssigneeLabel('UI/UX设计'), '设计同学')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.conditionConfirm }))
    await waitFor(() => expect(actionBody).toEqual({
      action: 'condition_yes',
      node_id: 'ui',
      next_node_assignees: [{ node_id: 'design', assignee_user_id: 31 }],
    }))
  })

  it('lets form complete and approval override the planned next assignee', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 93, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['review'],
      current_nodes: [{ id: 'review', type: 'approval', name: '产品评审', allowed_actions: ['approve', 'reject'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active', allowed_actions: ['approve', 'reject'], can_act: true },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'review-develop', source: 'review', target: 'develop', kind: 'approve' }],
      node_plans: { develop: { assignee_user_id: 1 } },
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 93, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/93' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/93/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['develop'] } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 93 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('产品评审') }))
    expect(await screen.findByTestId('workflow-next-assignees')).toBeTruthy()
    await selectUser(subjectCopy.nextAssigneeLabel('研发实现'), '研发同学')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.approve }))
    await waitFor(() => expect(actionBody).toEqual({
      action: 'approve',
      node_id: 'review',
      next_node_assignees: [{ node_id: 'develop', assignee_user_id: 41 }],
    }))
  })

  it('lets pre-review approval assign the next node owners', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 97, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['requirement-pre-review'],
      current_nodes: [{ id: 'requirement-pre-review', type: 'approval', name: '需求预审审批', allowed_actions: ['approve', 'reject'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'requirement-pre-review', type: 'approval', name: '需求预审审批', status: 'active', allowed_actions: ['approve', 'reject'], can_act: true },
        { id: 'split', type: 'parallel_split', name: '并行开始', status: 'pending' },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'pending' },
        { id: 'test-cases', type: 'human_task', name: '测试用例编写', status: 'pending' },
        { id: 'join', type: 'parallel_join', name: '并行汇合', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'pre-split', source: 'requirement-pre-review', target: 'split', kind: 'approve' },
        { id: 'split-develop', source: 'split', target: 'develop' },
        { id: 'split-test', source: 'split', target: 'test-cases' },
        { id: 'develop-join', source: 'develop', target: 'join' },
        { id: 'test-join', source: 'test-cases', target: 'join' },
      ],
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 1, total_steps: 4, progress: 25, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 97, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/97' && config.method === 'get') return response(config, { data: instance })
      if (config.url?.includes('/nodes/requirement-pre-review/opinions') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances/97/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['develop', 'test-cases'] } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 97 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('需求预审审批') }))
    expect(await screen.findByTestId('workflow-next-assignees')).toBeTruthy()
    expect(screen.getByText(subjectCopy.nextAssigneeLabel('研发实现'))).toBeTruthy()
    expect(screen.getByText(subjectCopy.nextAssigneeLabel('测试用例编写'))).toBeTruthy()
    const approve = screen.getByRole('button', { name: subjectCopy.approve }) as HTMLButtonElement
    expect(approve.disabled).toBe(true)
    await selectUser(subjectCopy.nextAssigneeLabel('研发实现'), '研发同学')
    expect(approve.disabled).toBe(true)
    await selectUser(subjectCopy.nextAssigneeLabel('测试用例编写'), '评审同学')
    await waitFor(() => expect(approve.disabled).toBe(false))
    fireEvent.click(approve)
    await waitFor(() => expect(actionBody).toEqual({
      action: 'approve',
      node_id: 'requirement-pre-review',
      next_node_assignees: [
        { node_id: 'develop', assignee_user_id: 41 },
        { node_id: 'test-cases', assignee_user_id: 21 },
      ],
    }))
  })

  it('completes requirement entry into the AI node without an assignee picker', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 94, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['input'],
      current_nodes: [{ id: 'input', type: 'form', name: '需求录入', allowed_actions: ['complete'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'active', allowed_actions: ['complete'], can_act: true },
        { id: 'ai', type: 'ai_check', name: '完整性检查', status: 'pending', capability: 'requirement.completeness' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'input-ai', source: 'input', target: 'ai' }],
      node_plans: {},
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 94, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/94' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/94/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: {
          ...instance,
          current_node_ids: ['ai'],
          current_nodes: [{ id: 'ai', type: 'ai_check', name: '完整性检查', can_act: false }],
          timeline_nodes: instance.timeline_nodes.map((node) => node.id === 'ai'
            ? { ...node, status: 'active' }
            : node.id === 'input' ? { ...node, status: 'completed', can_act: false, allowed_actions: [] } : node),
        } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 94 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('需求录入') }))
    await waitFor(() => expect(screen.queryByTestId('workflow-next-assignees')).toBeNull())
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.completeNode }))
    await waitFor(() => expect(actionBody).toEqual({
      action: 'complete',
      node_id: 'input',
    }))
  })

  it('sends the chosen pre-review owner when confirming completeness instead of the planned Administrator', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 95, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['ai'],
      current_nodes: [{ id: 'ai', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness', allowed_actions: ['complete'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'completed' },
        {
          id: 'ai', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness', status: 'active',
          allowed_actions: ['complete'], can_act: true,
          ai_execution: {
            run_id: 81, status: 'completed', capability: 'requirement.completeness', subject_version: 3,
            output: {
              summary: '完整性检查通过',
              changes: [],
              items: [],
              risks: [],
              humanReviewRequired: false,
              completenessReport: {
                canProceed: true, score: 90, threshold: 80, verdict: 'pass', checkedSubjectVersion: 3,
                dimensions: [], blockingIssues: [], recommendations: [], missingRequiredFields: [],
              },
            },
          },
        },
        { id: 'review', type: 'approval', name: '需求预审审批', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'ai-review', source: 'ai', target: 'review', kind: 'success' }],
      node_plans: { review: { assignee_user_id: 1 } },
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 1, total_steps: 3, progress: 33, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 95, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/95' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/95/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: {
          ...instance,
          current_node_ids: ['review'],
          current_nodes: [{ id: 'review', type: 'approval', name: '需求预审审批', assignee_user_id: 51, assignee_name: 'bill', can_act: false }],
        } })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 95 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openAIReport('完整性检查') }))
    expect(await screen.findByTestId('workflow-next-assignees')).toBeTruthy()
    const confirm = screen.getByRole('button', { name: subjectCopy.aiConfirmAndContinue }) as HTMLButtonElement
    expect(confirm.disabled).toBe(true)
    await selectUser(subjectCopy.nextAssigneeLabel('需求预审审批'), 'bill')
    await waitFor(() => expect(confirm.disabled).toBe(false))
    fireEvent.click(confirm)
    await waitFor(() => expect(actionBody).toEqual({
      action: 'complete',
      node_id: 'ai',
      next_node_assignees: [{ node_id: 'review', assignee_user_id: 51 }],
    }))
  })

  it('offers directory-backed candidates in the todo dialog and persists the pick', async () => {
    // 目录选择器改为服务端搜索（/directory/users?q=）后，输入过滤由服务端负责；
    // 这里断言弹层候选来自目录数据且选中后会随 complete 一起持久化。
    coreApi.defaults.adapter = async (config) => {
      if (config.url === '/directory/users') {
        const q = String((config.params as Record<string, unknown> | undefined)?.q ?? '')
        const all = [
          { id: 21, username: 'reviewer', display_name: '评审同学' },
          { id: 61, username: 'west', display_name: '西洲' },
        ]
        const filtered = q
          ? all.filter((user) => user.username.includes(q) || user.display_name.includes(q))
          : all
        return response(config, { data: filtered, total: filtered.length })
      }
      return response(config, { data: [] })
    }
    const instance: WorkflowInstance = {
      id: 96, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['input'],
      current_nodes: [{ id: 'input', type: 'form', name: '需求录入', allowed_actions: ['complete'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'active', allowed_actions: ['complete'], can_act: true },
        { id: 'review', type: 'approval', name: '产品评审', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'input-review', source: 'input', target: 'review' }],
      node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    let actionBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 96, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/96' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/96/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirementFixture({ workflow_instance_id: 96 }))
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('需求录入') }))
    const label = subjectCopy.nextAssigneeLabel('产品评审')
    await selectUser(label, '西洲')

    const complete = screen.getByRole('button', { name: subjectCopy.completeNode }) as HTMLButtonElement
    expect(complete.disabled).toBe(false)
    fireEvent.click(complete)
    await waitFor(() => expect(actionBody).toEqual({
      action: 'complete',
      node_id: 'input',
      next_node_assignees: [{ node_id: 'review', assignee_user_id: 61 }],
    }))
  })
})
