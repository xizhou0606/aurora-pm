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
}

function renderLauncher(requirement: Requirement, options: { withRequirement?: boolean } = {}) {
  const root = createRootRoute({ component: Outlet })
  const withRequirement = options.withRequirement !== false
  const page = createRoute({
    getParentRoute: () => root,
    path: '/task-plan-confirm',
    component: () => (
      <WorkflowSubjectLauncher
        workspaceId={7}
        subjectType="requirement"
        subjectId={requirement.id}
        requirement={withRequirement ? requirement : undefined}
      />
    ),
  })
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: ['/task-plan-confirm'] }),
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  seedActionPermissions([
    PM_ACTION.WorkflowsInstancesView,
    PM_ACTION.WorkflowsInstancesAct,
    PM_ACTION.RequirementsPoolUpdate,
    PM_ACTION.RequirementsRoleOwnersAssign,
    PM_ACTION.WorkItemsTaskTableView,
  ])
  return render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
}

describe('AI task plan guard for requirements with existing tasks', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(7)
    coreApi.defaults.adapter = async (config) => response(config, { data: [
      { id: 21, username: 'front', display_name: '前端同学' },
      { id: 22, username: 'back', display_name: '后端同学' },
    ], total: 2 })
  })
  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    coreApi.defaults.adapter = originalCoreAdapter
    runtimeSessionStore.getState().setIdentity(null)
    runtimeWorkspaceStore.getState().clearWorkspace()
  })

  it('lists existing requirement tasks and asks for confirmation before starting', async () => {
    const requirement = requirementFixture({ workflow_instance_id: 82 })
    let actionBody: Record<string, unknown> | undefined
    let tasksQuery: Record<string, unknown> | undefined
    let instance: WorkflowInstance = {
      id: 82, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['task-plan'],
      current_nodes: [{ id: 'task-plan', type: 'ai_generate', name: 'AI任务拆解', capability: 'requirement.task_plan', allowed_actions: [], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'task-plan', type: 'ai_generate', name: 'AI任务拆解', capability: 'requirement.task_plan', status: 'active', allowed_actions: [], can_act: true },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 82, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/82' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/tasks' && config.method === 'get') {
        tasksQuery = { ...(config.params as Record<string, unknown>) }
        return response(config, { items: [
          { id: 641, task_key: 'MIGSINFRA202-641', title: '全仓三方依赖版本统一', status: '待处理' },
          { id: 638, task_key: 'MIGSINFRA202-638', title: '后端-用户信息显示调整', status: '进行中' },
        ], total: 2 })
      }
      if (config.url === '/pm/workflows/instances/82/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        instance = {
          ...instance,
          timeline_nodes: instance.timeline_nodes.map((node) => node.id === 'task-plan'
            ? { ...node, ai_execution: { run_id: 91, status: 'queued', capability: 'requirement.task_plan' } }
            : node),
        }
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirement)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('AI任务拆解') }))
    expect(await screen.findByTestId('workflow-task-plan-existing-tasks')).toBeTruthy()
    await waitFor(() => expect(tasksQuery).toMatchObject({ requirement_ids: 42 }))
    expect(screen.getByText('MIGSINFRA202-641')).toBeTruthy()
    expect(screen.getByText('全仓三方依赖版本统一')).toBeTruthy()
    expect(screen.getByText(subjectCopy.taskPlanExistingCount(2))).toBeTruthy()

    await selectUser(subjectCopy.fieldFrontendOwner, '前端同学')
    await selectUser(subjectCopy.fieldBackendOwner, '后端同学')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.taskPlanStart }))

    expect(await screen.findByText(subjectCopy.taskPlanConfirmTitle)).toBeTruthy()
    expect(screen.getByText(subjectCopy.taskPlanConfirmDescription(2))).toBeTruthy()
    expect(actionBody).toBeUndefined()

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    await waitFor(() => expect(screen.queryByText(subjectCopy.taskPlanConfirmTitle)).toBeNull())
    expect(actionBody).toBeUndefined()

    fireEvent.click(screen.getByRole('button', { name: subjectCopy.taskPlanStart }))
    await screen.findByText(subjectCopy.taskPlanConfirmTitle)
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.taskPlanConfirmContinue }))

    await waitFor(() => expect(actionBody).toMatchObject({ action: 'retry_ai', node_id: 'task-plan' }))
  })

  it('still lists existing tasks when the mount passes no requirement object', async () => {
    const requirement = requirementFixture({ workflow_instance_id: 82 })
    let tasksQuery: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 82, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['task-plan'],
      current_nodes: [{ id: 'task-plan', type: 'ai_plan', name: 'AI任务拆解', capability: 'requirement.task_plan', allowed_actions: [], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'task-plan', type: 'ai_plan', name: 'AI任务拆解', capability: 'requirement.task_plan', status: 'active', allowed_actions: [], can_act: true },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 82, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/82' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/tasks' && config.method === 'get') {
        tasksQuery = { ...(config.params as Record<string, unknown>) }
        return response(config, { items: [
          { id: 6129, task_key: 'MIGSINFRA202-370', title: 'pre环境业务数据清空', status: 'completed' },
        ], total: 1 })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirement, { withRequirement: false })
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openTodo('AI任务拆解') }))
    expect(await screen.findByTestId('workflow-task-plan-existing-tasks')).toBeTruthy()
    await waitFor(() => expect(tasksQuery).toMatchObject({ requirement_ids: 42 }))
    expect(screen.getByText('MIGSINFRA202-370')).toBeTruthy()
  })

  it('offers skipping the generated plan when tasks already exist', async () => {
    const requirement = requirementFixture({ workflow_instance_id: 82 })
    let actionBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 82, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['task-plan'],
      current_nodes: [{ id: 'task-plan', type: 'ai_plan', name: 'AI任务拆解', capability: 'requirement.task_plan', allowed_actions: [], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        {
          id: 'task-plan', type: 'ai_plan', name: 'AI任务拆解', capability: 'requirement.task_plan', status: 'active', allowed_actions: ['complete', 'recheck_ai'], can_act: true,
          ai_execution: {
            run_id: 91, status: 'completed', capability: 'requirement.task_plan',
            output: {
              summary: '基于需求信息生成的任务方案。',
              changes: [], items: [], risks: [], humanReviewRequired: false,
              taskPlan: {
                chains: [
                  { lane: 'frontend', summary: '前端链路', tasks: [{ title: '前端任务一', description: '实现前端入口', acceptanceCriteria: ['入口可用'] }] },
                  { lane: 'backend', summary: '后端链路', tasks: [{ title: '后端任务一', description: '实现复制接口', acceptanceCriteria: ['接口可用'] }] },
                ],
              },
            },
          },
        },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [],
      completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 82, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/82' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/tasks' && config.method === 'get') return response(config, { items: [
        { id: 6129, task_key: 'MIGSINFRA202-370', title: 'pre环境业务数据清空', status: 'completed' },
      ], total: 1 })
      if (config.url === '/pm/workflows/instances/82/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }

    renderLauncher(requirement)
    fireEvent.click(await screen.findByRole('button', { name: subjectCopy.openAIReport('AI任务拆解') }))
    const skip = await screen.findByRole('button', { name: subjectCopy.taskPlanSkipComplete })
    fireEvent.click(skip)

    await waitFor(() => expect(actionBody).toMatchObject({
      action: 'complete', node_id: 'task-plan', skip_task_plan: true,
      comment: subjectCopy.taskPlanSkipComment,
    }))
  })
})
