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
import { RequirementLaneOwnersEcho } from './WorkflowTaskPlanOwners'
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
}

function renderLauncher(requirement: Requirement) {
  const root = createRootRoute({ component: Outlet })
  const page = createRoute({
    getParentRoute: () => root,
    path: '/task-plan-owners',
    component: () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={requirement.id} requirement={requirement} />,
  })
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: ['/task-plan-owners'] }),
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

describe('requirement lane owners for AI task plan', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(7)
    coreApi.defaults.adapter = async (config) => {
      if (config.url === '/access/me/action-permissions') {
        // 身份引导会 bump session revision 并触发权限重载；假 adapter 必须按
        // 契约返回种子动作，否则作用域缓存会把授权清空（fail closed）。
        return response(config, { data: { action_codes: [
          PM_ACTION.WorkflowsInstancesView, PM_ACTION.WorkflowsInstancesAct,
          PM_ACTION.RequirementsPoolUpdate, PM_ACTION.RequirementsRoleOwnersAssign,
        ] } })
      }
      return response(config, { data: [
        { id: 21, username: 'front', display_name: '前端同学' },
        { id: 22, username: 'back', display_name: '后端同学' },
      ], total: 2 })
    }
  })
  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    coreApi.defaults.adapter = originalCoreAdapter
    runtimeSessionStore.getState().setIdentity(null)
    runtimeWorkspaceStore.getState().clearWorkspace()
  })

  it('echoes assigned frontend and backend owners on the requirement detail', () => {
    render(
      <RequirementLaneOwnersEcho
        requirement={requirementFixture({ frontend_owner_user_id: 21, backend_owner_user_id: 22 })}
        people={[
          { id: 21, username: 'front', display_name: '前端同学' },
          { id: 22, username: 'back', display_name: '后端同学' },
        ]}
      />,
    )
    expect(screen.getByLabelText(detailCopy.laneOwnersTitle).textContent).toContain('前端同学')
    expect(screen.getByLabelText(detailCopy.laneOwnersTitle).textContent).toContain('后端同学')
  })

  it('requires both lane owners before starting AI task plan and then persists them', async () => {
    const requirement = requirementFixture({ workflow_instance_id: 82 })
    let patched: Record<string, unknown> | undefined
    let actionBody: Record<string, unknown> | undefined
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
      if (config.url === '/requirements/42/role-owners' && config.method === 'patch') {
        // owner 赋值走专用 role-owners 端点（与 pool.ts 的契约拆分一致）。
        patched = JSON.parse(String(config.data))
        return response(config, { data: { ...requirement, version: 4, frontend_owner_user_id: 21, backend_owner_user_id: 22 } })
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
    expect(await screen.findByText(subjectCopy.taskPlanOwnersTitle)).toBeTruthy()
    const start = screen.getByRole('button', { name: subjectCopy.taskPlanStart }) as HTMLButtonElement
    expect(start.disabled).toBe(true)

    await selectUser(subjectCopy.fieldFrontendOwner, '前端同学')
    await selectUser(subjectCopy.fieldBackendOwner, '后端同学')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.taskPlanStart }))

    await waitFor(() => expect(patched).toEqual({
      expected_version: 3, frontend_owner_user_id: 21, backend_owner_user_id: 22,
    }))
    await waitFor(() => expect(actionBody).toMatchObject({ action: 'retry_ai', node_id: 'task-plan' }))
  })
})
