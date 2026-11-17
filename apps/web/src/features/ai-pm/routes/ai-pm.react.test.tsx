import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'
import axios, { type InternalAxiosRequestConfig } from 'axios'
import { parseRequirementDetailSearch, parseRequirementPoolSearch, parseWorkItemsSearch } from '@/app/router/search'
import { coreApi, platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { REQUIREMENT_DOCUMENTS_COPY as documentCopy } from '@/shared/copy/requirement-documents'
import { TABLE_COLUMN_PREFERENCES_STORAGE_KEY } from '@/shared/hooks'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { PlanUnitAIActions } from '../components/PlanUnitAIActions'
import { AIRunsPage } from './AIRunsPage'
import { RequirementPoolPage } from './RequirementPoolPage'
import { RequirementDetailPage } from './RequirementDetailPage'
import { WorkflowListPage } from './WorkflowListPage'
import { WorkflowAIAssistant } from '../components/WorkflowAIAssistant'
import { WorkflowEditorPage } from './WorkflowEditorPage'
import { latestApprovalResult, latestApprovalRollback, incomingApprovalTrail, latestIncomingApproval, latestIncomingApprovalRollback, latestIncomingRollback } from '../components/WorkflowApprovalResult'
import { WorkflowSubjectLauncher } from '../components/WorkflowSubjectLauncher'
import type { Requirement, WorkflowAssignee, WorkflowDefinition, WorkflowInstance } from '../model/types'


const __stubPrev_URL = (globalThis as any).URL as unknown
const __stubPrev_ResizeObserver = (globalThis as any).ResizeObserver as unknown
const originalAdapter = platformApi.defaults.adapter
const originalCoreAdapter = axios.getAdapter(coreApi.defaults.adapter)
const poolCopy = copy.aiPM.requirementPool

function requirementCreateScope(
  projects: Array<{ id: number; name: string; project_new_id: number }> = [{ id: 12, name: 'test', project_new_id: 11 }],
) {
  return { project_sets: [{ id: 11, name: 'AI PM' }], projects, requirements: [] }
}

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data, status: 200, statusText: 'OK', headers: {}, config }
}

function scopeOptionsResponse(
  config: InternalAxiosRequestConfig,
  projects: Array<{ id: number; project_new_id: number; title: string; status?: string }>,
) {
  const projectSetIds = [...new Set(projects.map((project) => project.project_new_id))]
  return response(config, { data: {
    project_sets: projectSetIds.map((id) => ({ id, name: `项目集 ${id}` })),
    projects: projects.map((project) => ({
      id: project.id,
      name: project.title,
      project_new_id: project.project_new_id,
    })),
    requirements: [],
  } })
}

function requirementFixture(overrides: Partial<Requirement> = {}): Requirement {
  return {
    id: 42,
    workspace_id: 7,
    team_id: 1,
    project_new_id: 11,
    version_task_new_id: 12,
    version: 3,
    title: '支付流程改造',
    description: '优化收银台交互',
    acceptance_criteria: '支付成功可核验',
    priority: 'high',
    status: 'grooming',
    task_count: 2,
    progress: 25,
    allowed_actions: [],
    created_at: '2026-08-06T00:00:00Z',
    updated_at: '2026-08-07T00:00:00Z',
    ...overrides,
  }
}

async function selectOption(label: string | RegExp, option: string) {
  const trigger = screen.getByRole('combobox', { name: label })
  if (trigger.getAttribute('aria-expanded') !== 'true') {
    fireEvent.click(trigger)
  }
  const item = await screen.findByRole('option', { name: option })
  fireEvent.pointerDown(item, { button: 0, pointerType: 'mouse' })
  fireEvent.click(item)
}

async function selectUser(label: string, option: string) {
  fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions(label) }))
  const item = await screen.findByRole('option', { name: new RegExp(option) })
  fireEvent.pointerDown(item, { button: 0, pointerType: 'mouse' })
  fireEvent.click(item)
  fireEvent.keyDown(item, { key: 'Escape' })
}

function mockWorkflowUsers(users: WorkflowAssignee[]) {
  coreApi.defaults.adapter = async (config) => String(config.url ?? '').endsWith('/access/me/action-permissions')
    ? originalCoreAdapter(config)
    : response(config, { data: users, total: users.length })
}

async function selectDate(label: string, value: string) {
  const [year, month, day] = value.split('-').map(Number)
  fireEvent.click(screen.getByRole('button', { name: label }))
  fireEvent.click(await screen.findByRole('button', { name: `选择 ${year}年${month}月${day}日` }))
}

function renderRoute(path: string, component: () => React.ReactNode, detailPath: string) {
  const root = createRootRoute({ component: Outlet })
  const page = createRoute({
    getParentRoute: () => root,
    path,
    component,
    ...(path === '/workspace/requirements' ? { validateSearch: parseRequirementPoolSearch } : {}),
  })
  const detail = createRoute({ getParentRoute: () => root, path: detailPath, component: () => <div>detail target</div> })
  const project = createRoute({ getParentRoute: () => root, path: '/workspace/projects/$projectId', component: () => <div>project target</div> })
  const planUnit = createRoute({ getParentRoute: () => root, path: '/workspace/projects/$projectId/items/$projectItemId', component: () => <div>version target</div> })
  const router = createRouter({ routeTree: root.addChildren([page, detail, project, planUnit]), history: createMemoryHistory({ initialEntries: [path] }) })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return {
    client,
    router,
    view: render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>),
  }
}

function renderRequirementPoolWithWorkItems() {
  const root = createRootRoute({ component: Outlet })
  const page = createRoute({
    getParentRoute: () => root,
    path: '/workspace/requirements',
    validateSearch: parseRequirementPoolSearch,
    component: RequirementPoolPage,
  })
  const detail = createRoute({
    getParentRoute: () => root,
    path: '/workspace/requirements/$requirementId',
    validateSearch: parseRequirementDetailSearch,
    component: () => <div>detail target</div>,
  })
  const workItems = createRoute({
    getParentRoute: () => root,
    path: '/workspace/work-items',
    validateSearch: parseWorkItemsSearch,
    component: () => <div>work items target</div>,
  })
  const router = createRouter({
    routeTree: root.addChildren([page, detail, workItems]),
    history: createMemoryHistory({ initialEntries: ['/workspace/requirements'] }),
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return {
    router,
    view: render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>),
  }
}

function renderWorkflowEditorRoute() {
  const root = createRootRoute({ component: Outlet })
  const editor = createRoute({ getParentRoute: () => root, path: '/workspace/pm/workflows/$templateId', component: WorkflowEditorPage })
  const router = createRouter({ routeTree: root.addChildren([editor]), history: createMemoryHistory({ initialEntries: ['/workspace/pm/workflows/5'] }) })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
}

function renderRequirementDetailRoute() {
  const root = createRootRoute({ component: Outlet })
  const detail = createRoute({ getParentRoute: () => root, path: '/workspace/requirements/$requirementId', component: RequirementDetailPage })
  const pool = createRoute({ getParentRoute: () => root, path: '/workspace/requirements', component: () => <div>pool target</div> })
  const router = createRouter({ routeTree: root.addChildren([detail, pool]), history: createMemoryHistory({ initialEntries: ['/workspace/requirements/42'] }) })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return {
    client,
    view: render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>),
  }
}

describe('AI-native PM interactions', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(7)
    coreApi.defaults.adapter = async (config) => String(config.url ?? '').endsWith('/access/me/action-permissions')
      ? originalCoreAdapter(config)
      : response(config, { data: [], total: 0 })
  })
  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    coreApi.defaults.adapter = originalCoreAdapter
    runtimeSessionStore.setState({ token: '' })
    runtimeSessionStore.getState().setIdentity(null)
    runtimeWorkspaceStore.getState().clearWorkspace()
    window.history.replaceState({}, '', '/')
    window.sessionStorage.clear()
    window.localStorage.removeItem('pm.requirement-pool.view')
    window.localStorage.removeItem(TABLE_COLUMN_PREFERENCES_STORAGE_KEY)
    delete window.microApp
    delete window.__MICRO_APP_ENVIRONMENT__
    ;(globalThis as any).URL = __stubPrev_URL; (globalThis as any).ResizeObserver = __stubPrev_ResizeObserver
  })

  it('selects the latest decision when an approval node is handled more than once', () => {
    const history: NonNullable<WorkflowInstance['transition_history']> = [
      { id: 2, node_id: 'acceptance', node_name: '产品验收', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '产品负责人', comment: '首次通过', occurred_at: '2026-08-07T02:00:00Z' },
      { id: 4, node_id: 'acceptance', node_name: '产品验收', node_type: 'approval', action: 'rejected', status: 'rejected', operator_type: 'user', operator_name: '验收负责人', comment: '最新驳回意见', occurred_at: '2026-08-08T02:00:00Z' },
    ]

    expect(latestApprovalResult(history, 'acceptance')).toMatchObject({ id: 4, action: 'rejected', comment: '最新驳回意见' })
  })

  it('selects only the latest rollback record owned by the approval node', () => {
    const history: NonNullable<WorkflowInstance['transition_history']> = [
      { id: 2, node_id: 'acceptance', node_name: '产品验收', node_type: 'approval', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '产品负责人', comment: '首次退回', target_node_id: 'qa', target_node_name: 'QA 测试', occurred_at: '2026-08-07T02:00:00Z' },
      { id: 3, node_id: 'qa', node_name: 'QA 测试', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '测试负责人', occurred_at: '2026-08-08T01:00:00Z' },
      { id: 4, node_id: 'acceptance', node_name: '产品验收', node_type: 'approval', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '验收负责人', comment: '最新退回原因', target_node_id: 'qa', target_node_name: 'QA 测试执行', occurred_at: '2026-08-08T02:00:00Z' },
    ]

    expect(latestApprovalRollback(history, 'acceptance')).toMatchObject({ id: 4, comment: '最新退回原因', target_node_name: 'QA 测试执行' })
  })

  it('selects the latest rollback received by a target node', () => {
    const history: NonNullable<WorkflowInstance['transition_history']> = [
      { id: 2, node_id: 'ai-check-completeness', node_name: '完整性检查', node_type: 'ai_check', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '需求负责人', comment: '首次退回', target_node_id: 'form-requirement-input', target_node_name: '需求录入', occurred_at: '2026-08-09T06:40:00Z' },
      { id: 4, node_id: 'ai-check-completeness', node_name: '完整性检查', node_type: 'ai_check', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '需求负责人', comment: '最新退回原因', target_node_id: 'form-requirement-input', target_node_name: '需求录入', occurred_at: '2026-08-09T06:45:00Z' },
    ]

    expect(latestIncomingRollback(history, 'form-requirement-input')).toMatchObject({ id: 4, node_name: '完整性检查', comment: '最新退回原因' })
  })

  it('selects the latest passed approval from a direct upstream node', () => {
    const history: NonNullable<WorkflowInstance['transition_history']> = [
      { id: 2, node_id: 'requirement-pre-review', node_name: '需求预审审批', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '预审负责人', comment: '旧审批意见', occurred_at: '2026-08-07T02:00:00Z' },
      { id: 4, node_id: 'requirement-pre-review', node_name: '需求预审审批', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '产品负责人', comment: '需求边界清晰，同意进入产品评审', occurred_at: '2026-08-08T02:00:00Z' },
    ]
    const edges: WorkflowInstance['definition_edges'] = [
      { id: 'pre-review-product-review', source: 'requirement-pre-review', target: 'product-review', kind: 'approve' },
    ]

    expect(latestIncomingApproval(history, edges, 'product-review')).toMatchObject({ id: 4, operator_name: '产品负责人', comment: '需求边界清晰，同意进入产品评审' })
  })

  it('keeps a consecutive upstream approval chain for the next approval node', () => {
    const history: NonNullable<WorkflowInstance['transition_history']> = [
      { id: 2, node_id: 'tech-review', node_name: '技术预审', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '技术负责人', comment: '技术方案可行', target_node_id: 'requirement-pre-review', occurred_at: '2026-08-07T02:00:00Z' },
      { id: 4, node_id: 'requirement-pre-review', node_name: '需求预审审批', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '产品负责人', comment: '同意进入产品评审', target_node_id: 'product-review', occurred_at: '2026-08-08T02:00:00Z' },
    ]
    const edges: WorkflowInstance['definition_edges'] = [
      { id: 'tech-pre', source: 'tech-review', target: 'requirement-pre-review', kind: 'approve' },
      { id: 'pre-product', source: 'requirement-pre-review', target: 'product-review', kind: 'approve' },
    ]

    expect(incomingApprovalTrail(history, edges, 'product-review').map((item) => item.node_id)).toEqual([
      'requirement-pre-review',
      'tech-review',
    ])
  })

  it('uses the recorded target for a rejected approval instead of a stale template edge', () => {
    const history: NonNullable<WorkflowInstance['transition_history']> = [
      { id: 9, node_id: 'product-review', node_name: '产品评审', node_type: 'approval', action: 'rejected', status: 'rejected', operator_type: 'user', operator_name: '产品负责人', comment: '返回上一节点补充', target_node_id: 'pre-review', target_node_name: '需求预审', occurred_at: '2026-08-09T08:00:00Z' },
    ]
    const edges: WorkflowInstance['definition_edges'] = [
      { id: 'legacy-reject', source: 'product-review', target: 'requirement-input', kind: 'reject' },
    ]

    expect(latestIncomingApproval(history, edges, 'pre-review')).toMatchObject({ id: 9, action: 'rejected' })
    expect(latestIncomingApproval(history, edges, 'requirement-input')).toBeUndefined()
  })

  it('accepts an incoming rollback only when it came from an approval node', () => {
    const history: NonNullable<WorkflowInstance['transition_history']> = [
      { id: 8, node_id: 'ai-check', node_name: 'AI 检查', node_type: 'ai_check', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '需求负责人', target_node_id: 'input', occurred_at: '2026-08-09T07:00:00Z' },
      { id: 9, node_id: 'product-review', node_name: '产品评审', node_type: 'approval', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '产品负责人', target_node_id: 'input', occurred_at: '2026-08-09T08:00:00Z' },
    ]

    expect(latestIncomingApprovalRollback(history, 'input')).toMatchObject({ id: 9, node_id: 'product-review' })
    expect(latestIncomingApprovalRollback([
      ...history,
      { id: 10, node_id: 'ai-check', node_name: 'AI 检查', node_type: 'ai_check', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '需求负责人', target_node_id: 'input', occurred_at: '2026-08-09T09:00:00Z' },
    ], 'input')).toBeUndefined()
  })

  it('does not present a non-approval rollback as approval content', async () => {
    const instance: WorkflowInstance = {
      id: 8,
      template_id: 1,
      template_name: 'test',
      template_version: 3,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 133,
      current_node_ids: ['form-requirement-input'],
      current_nodes: [{ id: 'form-requirement-input', type: 'form', name: '需求录入' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'form-requirement-input', type: 'form', name: '需求录入', status: 'active' },
        { id: 'ai-check-completeness', type: 'ai_check', name: '完整性检查', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'input-check', source: 'form-requirement-input', target: 'ai-check-completeness' },
        { id: 'check-input', source: 'ai-check-completeness', target: 'form-requirement-input', kind: 'failure' },
      ],
      node_runs: [],
      approvals: [],
      transition_history: [
        { id: 69, node_id: 'start', node_name: '开始', node_type: 'start', action: 'started', status: 'completed', operator_type: 'user', operator_name: '需求负责人', occurred_at: '2026-08-09T06:44:48Z' },
        { id: 72, node_id: 'ai-check-completeness', node_name: '完整性检查', node_type: 'ai_check', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '需求负责人', comment: '不合格', target_node_id: 'form-requirement-input', target_node_name: '需求录入', occurred_at: '2026-08-09T06:45:57Z' },
      ],
      completed_steps: 0,
      total_steps: 2,
      progress: 0,
      started_at: '2026-08-09T06:44:48Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 8, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/8' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/completeness-rollback-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={133} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('需求录入') }))
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('完整性检查') }))
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()
  })

  it('requires a plan unit while creating a requirement', async () => {
    let postBody: Record<string, unknown> | undefined
    runtimeSessionStore.getState().setIdentity({ id: 9, username: 'owner', role: 'user' })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') return response(config, { data: requirementCreateScope() })
      if (config.url === '/pm/workflows/templates') return response(config, { data: [{ id: 8, name: '需求评审流', subject_type: 'requirement', published_version: 2 }], total: 1 })
      if (config.url === '/requirements' && config.method === 'post') {
        postBody = JSON.parse(String(config.data))
        return response(config, { data: { id: 42, workspace_id: 7, team_id: 1, project_new_id: 11, version_task_new_id: 12, version: 1, title: '智能需求', priority: 'normal', status: 'backlog', task_count: 0, progress: 0, allowed_actions: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() } })
      }
      return response(config, { data: [], total: 0 })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    fireEvent.click(await screen.findByRole('button', { name: '新建需求' }))
    await selectOption(poolCopy.project, 'AI PM')
    fireEvent.change(screen.getByLabelText('需求标题'), { target: { value: '智能需求' } })
    const createButton = screen.getByRole('button', { name: '创建需求' }) as HTMLButtonElement
    expect(createButton.disabled).toBe(true)
    expect(screen.getByRole('combobox', { name: new RegExp(poolCopy.workflowTemplate) }).getAttribute('aria-required')).toBe('true')
    expect(screen.getByRole('combobox', { name: new RegExp(poolCopy.requiredSchedule) }).getAttribute('aria-required')).toBe('true')
    await selectOption(new RegExp(poolCopy.workflowTemplate), '需求评审流 · v2')
    expect(createButton.disabled).toBe(true)
    await selectOption(new RegExp(poolCopy.requiredSchedule), 'test')
    expect(createButton.disabled).toBe(false)
    fireEvent.click(createButton)
    await waitFor(() => expect(postBody).toMatchObject({
      project_new_id: 11,
      version_task_new_id: 12,
      workflow_template_id: 8,
      title: '智能需求',
      product_owner_user_ids: [9],
      design_owner_user_ids: [],
      frontend_owner_user_ids: [],
      backend_owner_user_ids: [],
      tester_user_ids: [],
    }))
    expect(postBody).not.toHaveProperty('status')
  })

  it('blocks requirement creation when no published requirement workflow exists', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') return response(config, { data: requirementCreateScope() })
      if (config.url === '/pm/workflows/templates') return response(config, { data: [{ id: 8, name: '未发布需求流', subject_type: 'requirement', published_version: 0 }], total: 1 })
      return response(config, { data: [], total: 0 })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.create }))
    await selectOption(poolCopy.project, 'AI PM')
    fireEvent.change(screen.getByLabelText(poolCopy.requirementTitle), { target: { value: '无法创建的需求' } })

    expect(await screen.findByText(poolCopy.workflowTemplateEmpty)).toBeTruthy()
    expect(screen.queryByRole('combobox', { name: new RegExp(poolCopy.workflowTemplate) })).toBeNull()
    expect((screen.getByRole('button', { name: poolCopy.createSubmit }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('blocks a stale workflow selection after the published catalog changes', async () => {
    let workflowTemplates = [{ id: 8, name: '需求评审流', subject_type: 'requirement', published_version: 2 }]
    let createCalls = 0
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') return response(config, { data: requirementCreateScope() })
      if (config.url === '/pm/workflows/templates') return response(config, { data: workflowTemplates, total: workflowTemplates.length })
      if (config.url === '/requirements' && config.method === 'post') createCalls += 1
      return response(config, { data: [], total: 0 })
    }
    const { client } = renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.create }))
    await selectOption(poolCopy.project, 'AI PM')
    fireEvent.change(screen.getByLabelText(poolCopy.requirementTitle), { target: { value: '目录变化需求' } })
    await selectOption(new RegExp(poolCopy.workflowTemplate), '需求评审流 · v2')
    await selectOption(new RegExp(poolCopy.requiredSchedule), 'test')
    const createButton = screen.getByRole('button', { name: poolCopy.createSubmit }) as HTMLButtonElement
    expect(createButton.disabled).toBe(false)

    workflowTemplates = [{ id: 9, name: '新需求评审流', subject_type: 'requirement', published_version: 1 }]
    await act(async () => {
      await client.invalidateQueries({ queryKey: ['ai-pm', 7, 'workflows'] })
    })

    await waitFor(() => expect(createButton.disabled).toBe(true))
    fireEvent.click(createButton)
    expect(createCalls).toBe(0)
  })

  it('clears requirement creation fields after cancel and when the workspace changes', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') return response(config, { data: requirementCreateScope() })
      if (config.url === '/pm/workflows/templates') return response(config, { data: [{ id: 8, name: '需求评审流', subject_type: 'requirement', published_version: 2 }], total: 1 })
      return response(config, { data: [], total: 0 })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')

    fireEvent.click(await screen.findByRole('button', { name: poolCopy.create }))
    await selectOption(poolCopy.project, 'AI PM')
    await selectOption(new RegExp(poolCopy.workflowTemplate), '需求评审流 · v2')
    fireEvent.change(screen.getByLabelText(poolCopy.requirementTitle), { target: { value: '待清理的需求' } })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    await waitFor(() => expect((screen.queryByRole('dialog', { name: poolCopy.create })) === null).toBe(true))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.create }))
    expect((screen.getByLabelText(poolCopy.requirementTitle) as HTMLInputElement).value).toBe('')
    expect(screen.getByRole('combobox', { name: poolCopy.project }).textContent).toContain(poolCopy.projectPlaceholder)
    expect(screen.getByRole('combobox', { name: new RegExp(poolCopy.workflowTemplate) }).textContent).toContain(poolCopy.workflowTemplatePlaceholder)
    expect(screen.getByRole('combobox', { name: new RegExp(poolCopy.requiredSchedule) }).textContent).toContain(poolCopy.schedulePlaceholder)

    await selectOption(poolCopy.project, 'AI PM')
    await selectOption(new RegExp(poolCopy.workflowTemplate), '需求评审流 · v2')
    fireEvent.change(screen.getByLabelText(poolCopy.requirementTitle), { target: { value: '不应跨空间保留' } })
    act(() => runtimeWorkspaceStore.getState().selectWorkspace(8))

    await waitFor(() => {
      expect((screen.getByLabelText(poolCopy.requirementTitle) as HTMLInputElement).value).toBe('')
      expect(screen.getByRole('combobox', { name: poolCopy.project }).textContent).toContain(poolCopy.projectPlaceholder)
      expect(screen.getByRole('combobox', { name: new RegExp(poolCopy.workflowTemplate) }).textContent).toContain(poolCopy.workflowTemplatePlaceholder)
      expect(screen.getByRole('combobox', { name: new RegExp(poolCopy.requiredSchedule) }).textContent).toContain(poolCopy.schedulePlaceholder)
    })
  })

  it('ignores a requirement create result that settles after the workspace changes', async () => {
    let settleCreate: (() => void) | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') return response(config, { data: requirementCreateScope() })
      if (config.url === '/pm/workflows/templates') return response(config, { data: [{ id: 8, name: '需求评审流', subject_type: 'requirement', published_version: 2 }], total: 1 })
      if (config.url === '/requirements' && config.method === 'post') {
        return new Promise((resolve) => {
          settleCreate = () => resolve(response(config, { data: { id: 42, workspace_id: 7, team_id: 1, project_new_id: 11, version_task_new_id: null, version: 1, title: '延迟需求', priority: 'normal', status: 'backlog', task_count: 0, progress: 0, allowed_actions: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() } }))
        })
      }
      return response(config, { data: [], total: 0 })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.create }))
    await selectOption(poolCopy.project, 'AI PM')
    await selectOption(new RegExp(poolCopy.workflowTemplate), '需求评审流 · v2')
    await selectOption(new RegExp(poolCopy.requiredSchedule), 'test')
    fireEvent.change(screen.getByLabelText(poolCopy.requirementTitle), { target: { value: '延迟需求' } })
    fireEvent.click(screen.getByRole('button', { name: poolCopy.createSubmit }))
    await waitFor(() => expect(settleCreate).toBeTypeOf('function'))

    act(() => runtimeWorkspaceStore.getState().selectWorkspace(8))
    act(() => settleCreate?.())

    await waitFor(() => expect((screen.getByLabelText(poolCopy.requirementTitle) as HTMLInputElement).value).toBe(''))
    expect(screen.queryByText('detail target')).toBeNull()
    expect(screen.getByRole('dialog', { name: poolCopy.create })).toBeTruthy()
  })

  it('replaces the requirement AI assistant with node-owned deliverables', async () => {
    runtimeSessionStore.setState({ token: 'test-session' })
    const requestedURLs: string[] = []
    const requirement = {
      id: 42, workspace_id: 7, team_id: 1, project_new_id: 11, version_task_new_id: 12, version: 3, workflow_instance_id: 77,
      design_url: 'https://www.figma.com/design/checkout/PM', product_doc_url: 'https://example.feishu.cn/wiki/checkout',
      title: '支付流程改造', description: '优化收银台交互', acceptance_criteria: '支付成功可核验', priority: 'high', status: 'grooming', task_count: 2, progress: 25, allowed_actions: [], created_at: '2026-08-06T00:00:00Z', updated_at: '2026-08-07T00:00:00Z',
    }
    const workflow = {
      id: 77, template_id: 5, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['design'], current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计' }],
      timeline_nodes: [{ id: 'start', type: 'start', name: '开始', status: 'completed' }, { id: 'input', type: 'form', name: '需求录入', status: 'completed', config: { requirement_input: true } }, { id: 'review', type: 'approval', name: '产品评审', status: 'completed', actual_completed_at: '2026-08-07T02:00:00Z' }, { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true } }, { id: 'end', type: 'end', name: '结束', status: 'pending' }],
      definition_edges: [{ id: 'input-review', source: 'input', target: 'review' }], node_runs: [], approvals: [], transition_history: [{ id: 2, node_id: 'review', node_name: '产品评审', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '产品负责人', comment: '同意进入设计', occurred_at: '2026-08-07T02:00:00Z' }], completed_steps: 2, total_steps: 3, progress: 67, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      requestedURLs.push(String(config.url))
      if (config.url?.includes('/document-resources')) return response(config, { data: { items: [], requirement_version: 3, project_set_id: 11, product_doc_url: 'https://example.feishu.cn/wiki/checkout' } })
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, [{ id: 12, project_new_id: 11, title: 'V1.1' }])
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 77, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/77') return response(config, { data: workflow })
      return response(config, { data: [] })
    }

    renderRequirementDetailRoute()

    expect(await screen.findByRole('heading', { name: copy.aiPM.requirementDetail.deliverablesTitle })).toBeTruthy()
    expect((await screen.findByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenRequirementDocument })).getAttribute('href')).toBe('https://example.feishu.cn/wiki/checkout')
    expect((await screen.findByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign })).getAttribute('href')).toBe('https://www.figma.com/design/checkout/PM')
    expect(screen.queryByRole('textbox', { name: copy.aiPM.requirementDetail.feishuTitle })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('需求录入') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('需求录入') })).toBeTruthy()
    expect(await within(screen.getByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('需求录入') })).findByRole('textbox', { name: documentCopy.title })).toBeTruthy()
    expect(screen.queryByText('同意进入设计')).toBeNull()
    expect(screen.queryByRole('heading', { name: 'AI 需求助手' })).toBeNull()
    expect(requestedURLs.some((url) => url.startsWith('/ai/'))).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('产品评审') }))
    const approvalDialog = await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('产品评审') })
    const nodeInformation = screen.getByTestId('workflow-node-information')
    expect(nodeInformation.textContent).toContain(copy.aiPM.workflowSubject.nodeInformation)
    expect(nodeInformation.contains(screen.getByRole('region', { name: copy.aiPM.workflowSubject.approvalResult }))).toBe(true)
    expect(screen.getAllByText(copy.aiPM.workflowSubject.approvalApproved).length).toBeGreaterThan(0)
    expect(screen.getByText('产品负责人')).toBeTruthy()
    expect(screen.getByText('同意进入设计')).toBeTruthy()
    expect(within(approvalDialog).queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()
    expect(within(approvalDialog).queryByRole('heading', { name: copy.aiPM.requirementDetail.deliverablesTitle })).toBeNull()
    expect(within(approvalDialog).queryByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenRequirementDocument })).toBeNull()
    expect(within(approvalDialog).queryByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign })).toBeNull()
  })

  it('shows the workflow-owned requirement status without sending it in an edit', async () => {
    let patchBody: Record<string, unknown> | undefined
    const requirement = {
      id: 42,
      workspace_id: 7,
      team_id: 1,
      project_new_id: 11,
      version_task_new_id: 12,
      version: 3,
      title: '支付流程改造',
      description: '优化收银台交互',
      acceptance_criteria: '支付成功可核验',
      priority: 'high',
      status: 'grooming',
      task_count: 2,
      progress: 25,
      allowed_actions: [],
      created_at: '2026-08-06T00:00:00Z',
      updated_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42' && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data))
        return response(config, { data: { ...requirement, ...patchBody, version: 4 } })
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, [{ id: 12, project_new_id: 11, title: 'V1.1' }])
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    renderRequirementDetailRoute()

    const status = await screen.findByLabelText(copy.aiPM.common.status)
    expect(status.textContent).toContain(poolCopy.grooming)
    expect(status.textContent).toContain(copy.aiPM.requirementDetail.statusWorkflowOwned)
    expect(status.querySelector('[data-slot="status-pill"]')?.getAttribute('data-tone')).toBe('warning')
    expect(screen.queryByRole('combobox', { name: copy.aiPM.common.status })).toBeNull()

    fireEvent.change(screen.getByRole('textbox', { name: copy.aiPM.requirementDetail.titleField }), {
      target: { value: '支付流程改造 v2' },
    })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.save }))

    await waitFor(() => expect(patchBody).toMatchObject({
      expected_version: 3,
      title: '支付流程改造 v2',
    }))
    expect(patchBody).not.toHaveProperty('status')
  })

  it('keeps unsaved requirement fields through a real schedule version increment and saves against the safe new base', async () => {
    const bodies: Array<Record<string, unknown>> = []
    let patchBody: Record<string, unknown> | undefined
    let requirement = {
      id: 42,
      workspace_id: 7,
      team_id: 1,
      project_new_id: 11,
      version_task_new_id: 12,
      version: 3,
      title: '支付流程改造',
      description: '',
      priority: 'high',
      status: 'grooming',
      task_count: 2,
      progress: 25,
      allowed_actions: [],
      created_at: '2026-08-06T00:00:00Z',
      updated_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42' && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data)) as Record<string, unknown>
        requirement = { ...requirement, ...patchBody, version: requirement.version + 1 }
        return response(config, { data: requirement })
      }
      if (config.url === '/requirements/42/schedule' && config.method === 'post') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        bodies.push(body)
        if (!body.confirm_subject_schedule) {
          throw {
            config,
            response: {
              config,
              status: 409,
              data: {
                code: 'WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED',
                message: 'private schedule explanation',
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
        requirement = { ...requirement, version_task_new_id: 13, version: 4 }
        return response(config, { data: requirement })
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, [
        { id: 12, project_new_id: 11, title: 'V1', status: 'development' },
        { id: 13, project_new_id: 11, title: 'V2', status: 'planning' },
      ])
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    renderRequirementDetailRoute()
    fireEvent.change(await screen.findByRole('textbox', { name: copy.aiPM.requirementDetail.titleField }), { target: { value: '支付流程改造草稿' } })
    fireEvent.change(screen.getByRole('textbox', { name: copy.aiPM.requirementDetail.descriptionField }), { target: { value: '未保存的说明' } })
    fireEvent.change(screen.getByRole('textbox', { name: copy.aiPM.requirementDetail.acceptanceField }), { target: { value: '未保存的验收标准' } })
    await selectOption(copy.aiPM.common.priority, poolCopy.p1)
    await screen.findByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion })
    await selectOption(copy.aiPM.requirementDetail.chooseVersion, 'V2')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))

    expect(await screen.findByTestId('workflow-schedule-confirmation')).toBeTruthy()
    expect(screen.queryByText('private schedule explanation')).toBeNull()
    expect(screen.getByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion }).textContent).toContain('V2')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleConfirmSuggested }))

    await waitFor(() => expect(bodies).toEqual([
      { version_task_new_id: 13, expected_version: 3 },
      { version_task_new_id: 13, expected_version: 3, confirm_subject_schedule: true },
    ]))
    await waitFor(() => expect((screen.getByRole('textbox', { name: copy.aiPM.requirementDetail.titleField }) as HTMLInputElement).value).toBe('支付流程改造草稿'))
    expect((screen.getByRole('textbox', { name: copy.aiPM.requirementDetail.descriptionField }) as HTMLTextAreaElement).value).toBe('未保存的说明')
    expect((screen.getByRole('textbox', { name: copy.aiPM.requirementDetail.acceptanceField }) as HTMLTextAreaElement).value).toBe('未保存的验收标准')
    expect(screen.getByRole('combobox', { name: copy.aiPM.common.priority }).textContent).toContain(poolCopy.p1)
    expect((screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }) as HTMLButtonElement).disabled).toBe(true)

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.save }))
    await waitFor(() => expect(patchBody).toEqual({
      expected_version: 4,
      title: '支付流程改造草稿',
      description: '未保存的说明',
      acceptance_criteria: '未保存的验收标准',
      priority: 'p1',
    }))
  })

  it('keeps the old content base version when an external edit conflicts with a local requirement draft', async () => {
    let requirement = requirementFixture()
    let patchBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42' && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data)) as Record<string, unknown>
        throw {
          config,
          response: { config, status: 409, data: { code: 'REQUIREMENT_STALE', message: 'stale requirement' } },
        }
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, [{ id: 12, project_new_id: 11, title: 'V1' }])
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    const { client } = renderRequirementDetailRoute()
    const title = await screen.findByRole('textbox', { name: copy.aiPM.requirementDetail.titleField }) as HTMLInputElement
    fireEvent.change(title, { target: { value: '本地标题草稿' } })

    requirement = requirementFixture({ version: 4, title: '外部更新标题' })
    await act(async () => {
      await client.refetchQueries({ queryKey: ['ai-pm', 7, 'requirement', 42], type: 'active' })
    })

    expect(title.value).toBe('本地标题草稿')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.save }))
    await waitFor(() => expect(patchBody).toEqual({ expected_version: 3, title: '本地标题草稿' }))
    expect(await screen.findByText(copy.aiPM.requirementDetail.saveFailed)).toBeTruthy()
  })

  it('explicitly rebases a stale schedule draft onto the refetched version without losing content drafts', async () => {
    let requirement = requirementFixture()
    const scheduleBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42/schedule' && config.method === 'post') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        scheduleBodies.push(body)
        if (scheduleBodies.length === 1) {
          requirement = requirementFixture({ version: 6, version_task_new_id: 14 })
          throw {
            config,
            response: {
              config,
              status: 409,
              data: {
                code: 'SCHEDULE_SUBJECT_STALE',
                message: 'stale schedule',
                schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
              },
            },
          }
        }
        requirement = requirementFixture({ version: 7, version_task_new_id: 13 })
        return response(config, { data: requirement })
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, [
        { id: 12, project_new_id: 11, title: 'V1', status: 'development' },
        { id: 13, project_new_id: 11, title: 'V2', status: 'planning' },
        { id: 14, project_new_id: 11, title: 'V3', status: 'planning' },
      ])
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    renderRequirementDetailRoute()
    const title = await screen.findByRole('textbox', { name: copy.aiPM.requirementDetail.titleField }) as HTMLInputElement
    const description = screen.getByRole('textbox', { name: copy.aiPM.requirementDetail.descriptionField }) as HTMLTextAreaElement
    fireEvent.change(title, { target: { value: '本地标题草稿' } })
    fireEvent.change(description, { target: { value: '本地说明草稿' } })
    await screen.findByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion })
    await selectOption(copy.aiPM.requirementDetail.chooseVersion, 'V2')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))

    expect(await screen.findByText(copy.aiPM.workflowSubject.scheduleStale)).toBeTruthy()
    expect(scheduleBodies).toEqual([{ version_task_new_id: 13, expected_version: 3 }])
    expect(title.value).toBe('本地标题草稿')
    expect(description.value).toBe('本地说明草稿')
    expect(screen.getByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion }).textContent).toContain('V2')

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }))
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }) === null).toBe(true)
      expect(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule })).toHaveProperty('disabled', false)
    })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))

    await waitFor(() => expect(scheduleBodies).toEqual([
      { version_task_new_id: 13, expected_version: 3 },
      { version_task_new_id: 13, expected_version: 6 },
    ]))
    expect(title.value).toBe('本地标题草稿')
    expect(description.value).toBe('本地说明草稿')
    await waitFor(() => expect((screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }) as HTMLButtonElement).disabled).toBe(true))
  })

  it('keeps a stale schedule frozen across background and explicit reload failures until a later reload advances', async () => {
    let requirement = requirementFixture()
    let scheduleRejected = false
    let requirementGets = 0
    const scheduleBodies: Array<Record<string, unknown>> = []
    const units = [
      { id: 12, project_new_id: 11, title: 'V1', status: 'development' },
      { id: 13, project_new_id: 11, title: 'V2', status: 'planning' },
    ]
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42/schedule' && config.method === 'post') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        scheduleBodies.push(body)
        if (scheduleBodies.length === 1) {
          scheduleRejected = true
          throw {
            config,
            response: {
              config,
              status: 409,
              data: {
                code: 'SCHEDULE_SUBJECT_STALE',
                message: 'stale schedule',
                schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
              },
            },
          }
        }
        requirement = requirementFixture({ version: 7, version_task_new_id: 13 })
        return response(config, { data: requirement })
      }
      if (config.url === '/requirements/42') {
        requirementGets += 1
        if (scheduleRejected && (requirementGets === 2 || requirementGets === 3)) {
          throw {
            config,
            response: { config, status: 503, data: { code: 'TEMPORARY_FAILURE' } },
          }
        }
        if (scheduleRejected && requirementGets >= 4) {
          requirement = requirementFixture({ version: 6 })
        }
        return response(config, { data: requirement })
      }
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, units)
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    renderRequirementDetailRoute()
    await screen.findByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion })
    await selectOption(copy.aiPM.requirementDetail.chooseVersion, 'V2')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))

    expect(await screen.findByText(copy.aiPM.workflowSubject.scheduleStale)).toBeTruthy()
    const reloadLatest = () => screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest })
    const confirm = () => screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }) as HTMLButtonElement

    fireEvent.click(reloadLatest())
    fireEvent.click(reloadLatest())
    await waitFor(() => expect(requirementGets).toBeGreaterThanOrEqual(3))
    expect(requirementGets).toBe(3)
    expect(reloadLatest()).toBeTruthy()
    expect(confirm().disabled).toBe(true)

    fireEvent.click(reloadLatest())
    await waitFor(() => expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }) === null).toBe(true))
    await waitFor(() => expect(confirm().disabled).toBe(false))
    fireEvent.click(confirm())
    await waitFor(() => expect(scheduleBodies).toEqual([
      { version_task_new_id: 13, expected_version: 3 },
      { version_task_new_id: 13, expected_version: 6 },
    ]))
  })

  it('keeps a stale schedule frozen when an explicit reload returns unchanged facts', async () => {
    const requirement = requirementFixture()
    let requirementGets = 0
    const scheduleBodies: Array<Record<string, unknown>> = []
    const units = [
      { id: 12, project_new_id: 11, title: 'V1', status: 'development' },
      { id: 13, project_new_id: 11, title: 'V2', status: 'planning' },
    ]
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42/schedule' && config.method === 'post') {
        scheduleBodies.push(JSON.parse(String(config.data)) as Record<string, unknown>)
        throw {
          config,
          response: {
            config,
            status: 409,
            data: {
              code: 'SCHEDULE_SUBJECT_STALE',
              message: 'stale schedule',
              schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
            },
          },
        }
      }
      if (config.url === '/requirements/42') {
        requirementGets += 1
        return response(config, { data: requirement })
      }
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, units)
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    renderRequirementDetailRoute()
    await screen.findByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion })
    await selectOption(copy.aiPM.requirementDetail.chooseVersion, 'V2')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))

    expect(await screen.findByText(copy.aiPM.workflowSubject.scheduleStale)).toBeTruthy()
    await waitFor(() => expect(requirementGets).toBeGreaterThanOrEqual(2))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }))

    await waitFor(() => expect(requirementGets).toBeGreaterThanOrEqual(3))
    const reloadLatest = screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }) as HTMLButtonElement
    await waitFor(() => expect(reloadLatest.disabled).toBe(false))
    expect((screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }) as HTMLButtonElement).disabled).toBe(true)
    expect(scheduleBodies).toEqual([{ version_task_new_id: 13, expected_version: 3 }])
  })

  it('drops a deleted schedule target after stale reload instead of submitting it again', async () => {
    let requirement = requirementFixture()
    let units = [
      { id: 12, project_new_id: 11, title: 'V1', status: 'development' },
      { id: 13, project_new_id: 11, title: 'V2', status: 'planning' },
    ]
    let unitRequests = 0
    const scheduleBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42/schedule' && config.method === 'post') {
        scheduleBodies.push(JSON.parse(String(config.data)) as Record<string, unknown>)
        requirement = requirementFixture({ version: 3, version_task_new_id: 12 })
        units = [{ id: 12, project_new_id: 11, title: 'V1', status: 'development' }]
        throw {
          config,
          response: {
            config,
            status: 409,
            data: {
              code: 'SCHEDULE_SUBJECT_STALE',
              message: 'target deleted',
              schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
            },
          },
        }
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') {
        unitRequests += 1
        return scopeOptionsResponse(config, units)
      }
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    renderRequirementDetailRoute()
    await screen.findByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion })
    await selectOption(copy.aiPM.requirementDetail.chooseVersion, 'V2')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))

    expect(await screen.findByText(copy.aiPM.workflowSubject.scheduleStale)).toBeTruthy()
    await waitFor(() => expect(unitRequests).toBeGreaterThanOrEqual(2))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }))

    await waitFor(() => expect(screen.getByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion }).textContent).toContain('V1'))
    expect((screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }) as HTMLButtonElement).disabled).toBe(true)
    expect(scheduleBodies).toEqual([{ version_task_new_id: 13, expected_version: 3 }])
  })

  it('adopts the latest project scope on stale reload and never reuses the old-project target', async () => {
    let requirement = requirementFixture()
    const requestedProjectIds: number[] = []
    const scheduleBodies: Array<Record<string, unknown>> = []
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42/schedule' && config.method === 'post') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        scheduleBodies.push(body)
        if (scheduleBodies.length === 1) {
          requirement = requirementFixture({
            project_new_id: 15,
            version_task_new_id: 20,
            version: 6,
          })
          throw {
            config,
            response: {
              config,
              status: 409,
              data: {
                code: 'SCHEDULE_SUBJECT_STALE',
                message: 'scope changed',
                schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
              },
            },
          }
        }
        requirement = requirementFixture({
          project_new_id: 15,
          version_task_new_id: 21,
          version: 7,
        })
        return response(config, { data: requirement })
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') {
        requestedProjectIds.push(requirement.project_new_id)
        return scopeOptionsResponse(config, [
          { id: 12, project_new_id: 11, title: 'V1' },
          { id: 13, project_new_id: 11, title: 'V2' },
          { id: 20, project_new_id: 15, title: 'P15 Base' },
          { id: 21, project_new_id: 15, title: 'P15 Target' },
        ])
      }
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    renderRequirementDetailRoute()
    const title = await screen.findByRole('textbox', { name: copy.aiPM.requirementDetail.titleField }) as HTMLInputElement
    fireEvent.change(title, { target: { value: '跨项目本地草稿' } })
    await selectOption(copy.aiPM.requirementDetail.chooseVersion, 'V2')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))

    expect(await screen.findByText(copy.aiPM.workflowSubject.scheduleStale)).toBeTruthy()
    await waitFor(() => expect(requestedProjectIds).toContain(15))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }))
    await waitFor(() => expect(screen.getByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion }).textContent).toContain('P15 Base'))
    expect(title.value).toBe('跨项目本地草稿')

    await selectOption(copy.aiPM.requirementDetail.chooseVersion, 'P15 Target')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }))
    await waitFor(() => expect(scheduleBodies).toEqual([
      { version_task_new_id: 13, expected_version: 3 },
      { version_task_new_id: 21, expected_version: 6 },
    ]))
    expect(title.value).toBe('跨项目本地草稿')
  })

  it('adopts an external plan-unit update when there is no local schedule draft', async () => {
    let requirement = requirementFixture()
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/project-requirement-scope-options') return scopeOptionsResponse(config, [
        { id: 12, project_new_id: 11, title: 'V1', status: 'development' },
        { id: 13, project_new_id: 11, title: 'V2', status: 'planning' },
      ])
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [], total: 0 })
    }

    const { client } = renderRequirementDetailRoute()
    const selector = await screen.findByRole('combobox', { name: copy.aiPM.requirementDetail.chooseVersion })
    await waitFor(() => expect(selector.textContent).toContain('V1'))

    requirement = requirementFixture({ version: 4, version_task_new_id: 13 })
    await act(async () => {
      await client.refetchQueries({ queryKey: ['ai-pm', 7, 'requirement', 42], type: 'active' })
    })

    await waitFor(() => expect(selector.textContent).toContain('V2'))
    expect((screen.getByRole('button', { name: copy.aiPM.requirementDetail.confirmSchedule }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('separates requirement scope and deliverables, then downloads the effective test-case artifact', async () => {
    window.localStorage.setItem('pm.requirement-pool.view', 'cards')
    const requirementRequests: Array<Record<string, unknown>> = []
    const directoryRequests: Array<Record<string, unknown>> = []
    let artifactDownloadRequests = 0
    const requirement = {
      id: 42,
      workspace_id: 7,
      team_id: 1,
      project_new_id: 11,
      version_task_new_id: 12,
      version: 1,
      title: '支付流程改造',
      description: '优化收银台交互',
      product_doc_url: 'https://docs.example.test/checkout',
      design_url: 'https://design.example.test/checkout.png',
      priority: 'high',
      status: 'development',
      task_count: 4,
      completed_task_count: 2,
      total_task_count: 4,
      progress: 50,
      product_owner_user_ids: [124],
      workflow_overview: {
        instance_id: 77,
        status: 'running',
        current_node_names: ['UI/UX设计'],
        completed_steps: 3,
        total_steps: 6,
      },
      deliverable_overview: {
        requirement_document: true,
        requirement_document_url: 'https://docs.example.test/checkout',
        design: true,
        design_url: 'https://design.example.test/checkout.png',
        test_cases: true,
        test_case_artifact: {
          id: 91,
          instance_id: 77,
          node_id: 'test-cases',
          kind: 'uploaded',
          name: 'checkout-cases.csv',
          content_type: 'text/csv',
          size: 24,
          created_at: '2026-08-07T01:00:00Z',
        },
      },
      allowed_actions: [],
      created_at: '2026-08-06T00:00:00Z',
      updated_at: '2026-08-07T00:00:00Z',
    }
    coreApi.defaults.adapter = async (config) => {
      if (String(config.url ?? '').endsWith('/access/me/action-permissions')) {
        return originalCoreAdapter(config)
      }
      if (config.url === '/directory/users') {
        directoryRequests.push({ ...(config.params as Record<string, unknown>) })
        return response(config, { data: [], total: 83 })
      }
      return response(config, { data: [], total: 0 })
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        requirementRequests.push({ ...(config.params as Record<string, unknown>) })
        return response(config, {
          data: [requirement],
          total: 42,
          summary: { total: 42, unscheduled: 7, delivery: 18 },
          users: [{ id: 124, username: 'sidecar-owner', display_name: '接口负责人', avatar_url: '/sidecar.png' }],
        })
      }
      if (config.url === '/pm/workflow-node-artifacts/91/download') {
        artifactDownloadRequests += 1
        return response(config, new Blob(['test cases'], { type: 'text/csv' }))
      }
      return response(config, { data: [] })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')

    expect(await screen.findByText('支付流程改造')).toBeTruthy()
    expect(screen.getByText('接口负责人')).toBeTruthy()
    await waitFor(() => expect(directoryRequests).toEqual([{ limit: 500, offset: 0 }]))
    expect(screen.queryByText('#42')).toBeNull()
    expect(screen.getByRole('list', { name: poolCopy.cards })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.openRow(requirement.title) })).toBeTruthy()
    expect(screen.getByLabelText(poolCopy.workflowNodeTag('UI/UX设计'))).toBeTruthy()
    expect(screen.queryByRole('columnheader')).toBeNull()
    expect(screen.queryByText('AI PM')).toBeNull()
    expect(screen.queryByText('V1.1')).toBeNull()
    expect(screen.queryByRole('progressbar')).toBeNull()
    const designActions = screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDesign) })
    const documentActions = screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDocument) })
    const testCaseActions = screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) })
    expect(screen.queryByRole('link', { name: poolCopy.deliverablePreview(poolCopy.deliverableDesign) })).toBeNull()

    fireEvent.click(designActions)
    const designPreview = await screen.findByRole('link', { name: poolCopy.deliverableOpenFile(poolCopy.deliverableDesign, 'checkout.png') })
    expect(designPreview.getAttribute('href')).toBe('https://design.example.test/checkout.png')
    expect(screen.getByRole('button', { name: poolCopy.deliverableDownloadUnavailable(poolCopy.deliverableDesign) }).getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(designPreview)
    expect(screen.queryByText('detail target')).toBeNull()

    await waitFor(() => expect(document.activeElement === designActions).toBe(true))
    documentActions.focus()
    fireEvent.keyDown(documentActions, { key: 'Enter' })
    const documentPreview = await screen.findByRole('link', { name: poolCopy.deliverablePreview(poolCopy.deliverableDocument) })
    expect(documentPreview.getAttribute('href')).toBe('https://docs.example.test/checkout')
    await waitFor(() => expect(document.activeElement === documentPreview).toBe(true))
    fireEvent.click(documentPreview)

    fireEvent.click(testCaseActions)
    expect(await screen.findByText('checkout-cases.csv')).toBeTruthy()
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.deliverablePreviewFile(poolCopy.deliverableTestCases, 'checkout-cases.csv') }))
    expect(await screen.findByRole('dialog', { name: poolCopy.deliverablePreview(poolCopy.deliverableTestCases) })).toBeTruthy()
    expect(screen.getByText('test cases')).toBeTruthy()
    expect(artifactDownloadRequests).toBe(1)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))

    const NativeURL = URL
    const createObjectURL = vi.fn(() => 'blob:test-cases')
    const revokeObjectURL = vi.fn()
    class DownloadURL extends NativeURL {}
    Object.defineProperties(DownloadURL, {
      createObjectURL: { configurable: true, value: createObjectURL },
      revokeObjectURL: { configurable: true, value: revokeObjectURL },
    })
    ;(globalThis as any).URL = DownloadURL
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    fireEvent.click(testCaseActions)
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.deliverableDownloadFile(poolCopy.deliverableTestCases, 'checkout-cases.csv') }))
    await waitFor(() => expect(artifactDownloadRequests).toBe(2))
    expect(createObjectURL).toHaveBeenCalled()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:test-cases')
    expect(screen.queryByText('detail target')).toBeNull()
    anchorClick.mockRestore()
    expect(screen.queryByText('优化收银台交互')).toBeNull()
    expect(screen.queryByText('50% · 4 项任务')).toBeNull()
    expect(screen.queryByRole('combobox', { name: poolCopy.statusFilter })).toBeNull()
    expect(screen.queryByRole('combobox', { name: poolCopy.priorityFilter })).toBeNull()
    expect(screen.queryByRole('combobox', { name: poolCopy.projectFilter })).toBeNull()
    expect(screen.queryByRole('combobox', { name: poolCopy.planUnitFilter })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: poolCopy.scopeLevel }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'AI PM' }))
    await waitFor(() => expect(requirementRequests.some((params) => params.project_new_id === '11')).toBe(true))

    expect(screen.queryByRole('button', { name: poolCopy.unscheduledOnly })).toBeNull()
    expect(requirementRequests.some((params) => params.limit === 'all')).toBe(true)
    expect(screen.queryByLabelText(poolCopy.pagination)).toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: poolCopy.viewList }))
    expect(await screen.findByRole('table', { name: poolCopy.table })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: poolCopy.reset }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.pagination.next }))
    await waitFor(() => expect(requirementRequests.some((params) => params.offset === 20)).toBe(true))

    fireEvent.click(screen.getByRole('tab', { name: poolCopy.viewCards }))
    fireEvent.click(screen.getByRole('button', { name: poolCopy.openRow(requirement.title) }))
    expect(await screen.findByText('detail target')).toBeTruthy()
  })

  it('switches the requirement pool between card and list views', async () => {
    const requirement = {
      id: 42,
      workspace_id: 7,
      team_id: 1,
      project_new_id: 11,
      version_task_new_id: 12,
      version: 1,
      title: '支付流程改造',
      priority: 'high',
      status: 'development',
      task_count: 4,
      progress: 50,
      workflow_overview: {
        instance_id: 77,
        status: 'running',
        current_node_names: ['UI/UX设计'],
        completed_steps: 3,
        total_steps: 6,
      },
      allowed_actions: [],
      created_at: '2026-08-06T00:00:00Z',
      updated_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      return response(config, { data: [] })
    }
    const { router } = renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')

    expect(await screen.findByRole('table', { name: poolCopy.table })).toBeTruthy()
    const pageTitle = screen.getByRole('heading', { level: 1, name: poolCopy.title })
    const poolBody = pageTitle.closest('[data-slot="operational-page"]')
      ?.querySelector('[data-slot="operational-page-scroll"] > div')
    expect(poolBody?.className).toContain('grid')
    expect(poolBody?.className).toContain('min-w-0')
    expect(screen.getByRole('table', { name: poolCopy.table }).className).toContain('min-w-full')
    expect(screen.getByRole('table', { name: poolCopy.table }).className).toContain('table-fixed')
    expect(document.querySelector('[data-slot="table-container"]')?.className)
      .toContain('max-h-[calc(100vh-100px)]')
    const headerWidths = screen.getAllByRole('columnheader').map((header) => (
      Number.parseFloat(header.style.getPropertyValue('--data-table-column-size') || '0')
    ))
    expect(headerWidths.every((width) => width > 0)).toBe(true)
    expect(headerWidths.reduce((sum, width) => sum + width, 0)).toBeGreaterThanOrEqual(2000)
    expect(screen.getByRole('columnheader', { name: poolCopy.actions }).style.getPropertyValue('--data-table-column-size')).toBe('152px')
    expect(pageTitle.querySelector('.pm-requirement-title')).toBeNull()
    expect(screen.getByRole('tab', { name: poolCopy.viewList }).getAttribute('aria-selected')).toBe('true')
    fireEvent.click(screen.getByRole('tab', { name: poolCopy.viewCards }))
    expect(await screen.findByRole('list', { name: poolCopy.cards })).toBeTruthy()
    fireEvent.click(screen.getByRole('tab', { name: poolCopy.viewList }))
    expect(await screen.findByRole('table', { name: poolCopy.table })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.requirement })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.workflowOverview })).toBeTruthy()
    expect(screen.getByText('AI PM')).toBeTruthy()
    expect(screen.getByText('V1.1')).toBeTruthy()
    expect(screen.getByText('UI/UX设计')).toBeTruthy()
    expect(screen.queryByRole('list', { name: poolCopy.cards })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openProjectSet('AI PM') }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace/projects/11'))
    await router.navigate({ to: '/workspace/requirements' })
    expect(await screen.findByRole('table', { name: poolCopy.table })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openPlanUnit('V1.1') }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace/projects/11/items/12'))
    await router.navigate({ to: '/workspace/requirements' })
    expect(await screen.findByRole('table', { name: poolCopy.table })).toBeTruthy()

    fireEvent.click(screen.getByRole('tab', { name: poolCopy.viewCards }))
    expect(await screen.findByRole('list', { name: poolCopy.cards })).toBeTruthy()
    expect(screen.queryByRole('table', { name: poolCopy.table })).toBeNull()
  })

  it('filters the requirement pool by published workflow nodes', async () => {
    const requirementRequests: Array<Record<string, unknown>> = []
    const running = {
      id: 42,
      workspace_id: 7,
      team_id: 1,
      project_new_id: 11,
      version_task_new_id: 12,
      version: 1,
      title: '支付流程改造',
      priority: 'high',
      status: 'development',
      task_count: 4,
      progress: 50,
      workflow_overview: {
        instance_id: 77,
        status: 'running',
        current_node_names: ['产品评审'],
        current_nodes: [{ id: 'product_review', name: '产品评审' }],
        completed_steps: 3,
        total_steps: 6,
      },
      allowed_actions: [],
      created_at: '2026-08-06T00:00:00Z',
      updated_at: '2026-08-07T00:00:00Z',
    }
    const idle = {
      ...running,
      id: 43,
      title: '尚未启动的需求',
      workflow_overview: undefined,
    }
    const taskPlan = {
      ...running,
      id: 44,
      title: 'AI拆解中的需求',
      workflow_overview: {
        instance_id: 88,
        status: 'running',
        current_node_names: ['AI任务拆解'],
        current_nodes: [{ id: 'task-plan', name: 'AI任务拆解' }],
        completed_steps: 4,
        total_steps: 6,
      },
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/pm/workflows/templates') {
        return response(config, {
          data: [{
            id: 8,
            name: '需求评审流',
            subject_type: 'requirement',
            published_version: 2,
            nodes: [
              { id: 'product_review', type: 'approval', name: '产品评审' },
              { id: 'task_plan', type: 'ai_generate', name: 'AI任务拆解' },
            ],
          }],
          total: 1,
        })
      }
      if (config.url === '/requirements') {
        requirementRequests.push({ ...(config.params as Record<string, unknown>) })
        return response(config, { data: [running, idle, taskPlan], total: 137, summary: { total: 137, unscheduled: 0, delivery: 1 } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')

    expect(await screen.findByText('支付流程改造')).toBeTruthy()
    expect(screen.getByText('尚未启动的需求')).toBeTruthy()
    expect(screen.getByText('AI拆解中的需求')).toBeTruthy()
    expect(screen.getByText(copy.ui.pagination.total(137))).toBeTruthy()
    await selectOption(poolCopy.workflowOverview, poolCopy.workflowNotStarted)
    await waitFor(() => expect(requirementRequests.some((params) => params.current_node_id === '__idle__')).toBe(true))
    expect(screen.queryByText('支付流程改造')).toBeNull()
    expect(screen.getByText('尚未启动的需求')).toBeTruthy()
    expect(screen.queryByText('AI拆解中的需求')).toBeNull()
    expect(screen.getByText(copy.ui.pagination.total(1))).toBeTruthy()

    await selectOption(poolCopy.workflowOverview, poolCopy.workflowNotStarted)
    await selectOption(poolCopy.workflowOverview, '产品评审')
    await waitFor(() => expect(requirementRequests.some((params) => params.current_node_id === 'product_review')).toBe(true))
    expect(screen.getByText('支付流程改造')).toBeTruthy()
    expect(screen.queryByText('尚未启动的需求')).toBeNull()
    expect(screen.queryByText('AI拆解中的需求')).toBeNull()
    expect(screen.getByText(copy.ui.pagination.total(1))).toBeTruthy()

    await selectOption(poolCopy.workflowOverview, '产品评审')
    await selectOption(poolCopy.workflowOverview, 'AI任务拆解')
    await waitFor(() => expect(requirementRequests.some((params) => params.current_node_id === 'task_plan')).toBe(true))
    expect(screen.getByText('AI拆解中的需求')).toBeTruthy()
    expect(screen.queryByText('支付流程改造')).toBeNull()
    expect(screen.queryByText('尚未启动的需求')).toBeNull()
    expect(screen.getByText(copy.ui.pagination.total(1))).toBeTruthy()
  })

  it('patches a requirement role assignment from the list with version protection', async () => {
    let patchBody: Record<string, unknown> | undefined
    const requirement = requirementFixture({
      allowed_actions: ['update'],
      product_owner_user_ids: [7],
    })
    mockWorkflowUsers([
      { id: 7, username: 'alice', display_name: 'Alice' },
      { id: 9, username: 'bob', display_name: 'Bob' },
    ])
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, { data: requirementCreateScope() })
      }
      if (config.url === '/requirements/42/role-owners' && config.method === 'patch') {
        patchBody = JSON.parse(String(config.data))
        return response(config, { data: { ...requirement, ...patchBody, version: 4 } })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      return response(config, { data: [], total: 0 })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')

    fireEvent.click(await screen.findByRole('button', { name: poolCopy.assignRole(poolCopy.roleProduct, requirement.title) }))
    const assignment = screen.getByRole('dialog', { name: poolCopy.assignRole(poolCopy.roleProduct, requirement.title) })
    fireEvent.click(within(assignment).getByRole('button', { name: copy.ui.userSelect.openOptions(poolCopy.roleProduct) }))
    const bob = await screen.findByRole('option', { name: /Bob/ })
    fireEvent.pointerDown(bob, { button: 0, pointerType: 'mouse' })
    fireEvent.click(bob)
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.save }))

    await waitFor(() => expect(patchBody).toEqual({
      expected_version: 3,
      product_owner_user_ids: [7, 9],
    }))
  })

  it('filters the requirement pool by product owners even when the API ignores the query', async () => {
    mockWorkflowUsers([{ id: 88, username: 'ziwei', display_name: '子玮', avatar_url: '' }])
    const requirementRequests: Array<Record<string, unknown>> = []
    const owned = {
      id: 42,
      workspace_id: 7,
      team_id: 1,
      project_new_id: 11,
      version_task_new_id: 12,
      version: 1,
      title: '子玮负责的需求',
      priority: 'high',
      status: 'development',
      task_count: 4,
      progress: 50,
      product_owner_user_ids: [88],
      workflow_overview: {
        instance_id: 77,
        status: 'running',
        current_node_names: ['产品评审'],
        current_nodes: [{ id: 'product_review', name: '产品评审' }],
        completed_steps: 3,
        total_steps: 6,
      },
      allowed_actions: [],
      created_at: '2026-08-06T00:00:00Z',
      updated_at: '2026-08-07T00:00:00Z',
    }
    const other = {
      ...owned,
      id: 43,
      title: '别人负责的需求',
      product_owner_user_id: 99,
      product_owner_user_ids: [99],
      workflow_overview: {
        instance_id: 88,
        status: 'completed',
        current_node_names: ['流程已完成'],
        current_nodes: [],
        completed_steps: 6,
        total_steps: 6,
      },
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        requirementRequests.push({ ...(config.params as Record<string, unknown>) })
        return response(config, { data: [owned, other], total: 137, summary: { total: 137, unscheduled: 0, delivery: 1 } })
      }
      return response(config, { data: [] })
    }
    const { router } = renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')

    expect(await screen.findByText('子玮负责的需求')).toBeTruthy()
    expect(screen.getByText('别人负责的需求')).toBeTruthy()
    expect(screen.getByText(copy.ui.pagination.total(137))).toBeTruthy()

    await selectUser(poolCopy.roleProduct, '子玮')
    await waitFor(() => expect(requirementRequests.some((params) => params.product_owner_user_id === '88')).toBe(true))
    expect(router.state.location.search).toMatchObject({ product_owner_user_id: '88' })
    expect(screen.getByText('子玮负责的需求')).toBeTruthy()
    expect(screen.queryByText('别人负责的需求')).toBeNull()
    expect(screen.getByText(copy.ui.pagination.total(1))).toBeTruthy()
  })

  it('saves visible requirement pool filters from the configure panel', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [], total: 0, summary: { total: 0, unscheduled: 0, delivery: 0 } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    expect(await screen.findByRole('combobox', { name: poolCopy.workflowOverview })).toBeTruthy()
    expect(screen.getByRole('combobox', { name: poolCopy.roleProduct })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: poolCopy.filterSettings.trigger }))
    expect(await screen.findByRole('heading', { name: poolCopy.filterSettings.controlsCard })).toBeTruthy()
    fireEvent.click(screen.getByRole('checkbox', { name: copy.ui.tableColumns.toggle(poolCopy.roleProduct) }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    expect(screen.queryByRole('combobox', { name: poolCopy.roleProduct })).toBeNull()
    expect(screen.getByRole('combobox', { name: poolCopy.workflowOverview })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: poolCopy.filterSettings.trigger }))
    fireEvent.click(screen.getByRole('button', { name: poolCopy.filterSettings.reset }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    expect(screen.getByRole('combobox', { name: poolCopy.roleProduct })).toBeTruthy()
  })

  it('opens the task module scoped to the selected requirement', async () => {
    const requirement = requirementFixture({
      task_count: 4,
      progress: 100,
      total_task_count: 4,
      completed_task_count: 4,
    })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      return response(config, { data: [] })
    }
    const { router } = renderRequirementPoolWithWorkItems()
    fireEvent.click(await screen.findByRole('tab', { name: poolCopy.viewList }))
    expect(await screen.findByRole('table', { name: poolCopy.table })).toBeTruthy()
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.openTasks(requirement.title) }))

    await waitFor(() => {
      expect(router.state.location.pathname).toBe('/workspace/work-items')
    })
    expect(screen.getByText('work items target')).toBeTruthy()
    expect(screen.queryByText('detail target')).toBeNull()
    expect(router.state.location.search).toMatchObject({
      view: 'list',
      project_set_ids: '11',
      project_ids: '12',
      requirement_ids: '42',
    })
  })

  it('opens the requirement workflow node from the current-node cell', async () => {
    const requirement = requirementFixture({
      workflow_instance_id: 77,
      workflow_overview: {
        instance_id: 77,
        status: 'running',
        current_node_names: ['产品评审'],
        current_nodes: [{ id: 'review', name: '产品评审', overdue: false }],
        completed_steps: 1,
        total_steps: 4,
      },
    })
    const instance = {
      id: 77,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['review'],
      current_nodes: [{ id: 'review', type: 'approval', name: '产品评审', can_act: true, allowed_actions: ['approve', 'reject'] }],
      timeline_nodes: [
        { id: 'review', type: 'approval', name: '产品评审', status: 'active', can_act: true, allowed_actions: ['approve', 'reject'] },
      ],
      definition_edges: [],
      node_runs: [],
      approvals: [],
      transition_history: [],
      completed_steps: 1,
      total_steps: 4,
      progress: 25,
      started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') {
        return response(config, { data: [{ id: 77, status: 'running' }], total: 1 })
      }
      if (config.url === '/pm/workflows/instances/77' && config.method === 'get') {
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }
    const { router } = renderRequirementPoolWithWorkItems()
    fireEvent.click(await screen.findByRole('tab', { name: poolCopy.viewList }))
    expect(await screen.findByRole('table', { name: poolCopy.table })).toBeTruthy()
    const nodeButton = await screen.findByRole('button', {
      name: poolCopy.openWorkflowNode(requirement.title, '产品评审'),
    })
    expect(nodeButton.getAttribute('aria-label')).toBe(poolCopy.openWorkflowNode(requirement.title, '产品评审'))
    fireEvent.pointerDown(nodeButton, { button: 0, pointerType: 'mouse' })
    fireEvent.click(nodeButton)

    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })).toBeTruthy()
    expect(router.state.location.pathname).toBe('/workspace/requirements')
    expect(screen.queryByText('detail target')).toBeNull()
  })

  it('opens the named requirement-input node from the current-node cell without a node id', async () => {
    const requirement = requirementFixture({
      workflow_instance_id: 77,
      workflow_overview: {
        instance_id: 77,
        status: 'running',
        current_node_names: ['需求录入'],
        completed_steps: 0,
        total_steps: 4,
      },
    })
    const instance = {
      id: 77,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['form-requirement-input'],
      current_nodes: [{
        id: 'form-requirement-input',
        type: 'form',
        name: '需求录入',
        can_act: true,
        allowed_actions: ['complete'],
      }],
      timeline_nodes: [
        {
          id: 'form-requirement-input',
          type: 'form',
          name: '需求录入',
          status: 'active',
          can_act: true,
          allowed_actions: ['complete'],
        },
      ],
      definition_edges: [],
      node_runs: [],
      approvals: [],
      transition_history: [],
      completed_steps: 0,
      total_steps: 4,
      progress: 0,
      started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') {
        return response(config, { data: [{ id: 77, status: 'running' }], total: 1 })
      }
      if (config.url === '/pm/workflows/instances/77' && config.method === 'get') {
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }
    const { router } = renderRequirementPoolWithWorkItems()
    fireEvent.click(await screen.findByRole('tab', { name: poolCopy.viewList }))
    fireEvent.click(await screen.findByRole('button', {
      name: poolCopy.openWorkflowNode(requirement.title, '需求录入'),
    }))

    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('需求录入') })).toBeTruthy()
    expect(await screen.findByRole('heading', { name: documentCopy.entryTitle })).toBeTruthy()
    expect(screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.currentTodo })).toBeNull()
    expect(screen.queryByText(copy.aiPM.workflowSubject.waiting)).toBeNull()
    expect(router.state.location.pathname).toBe('/workspace/requirements')
    expect(screen.queryByText('detail target')).toBeNull()
  })

  it('opens the list node dialog once without flashing an empty todo shell', async () => {
    const requirement = requirementFixture({
      workflow_instance_id: 77,
      workflow_overview: {
        instance_id: 77,
        status: 'running',
        current_node_names: ['需求录入'],
        completed_steps: 0,
        total_steps: 4,
      },
    })
    const instance = {
      id: 77,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['form-requirement-input'],
      current_nodes: [{
        id: 'form-requirement-input',
        type: 'form',
        name: '需求录入',
        can_act: true,
        allowed_actions: ['complete'],
      }],
      timeline_nodes: [
        {
          id: 'form-requirement-input',
          type: 'form',
          name: '需求录入',
          status: 'active',
          can_act: true,
          allowed_actions: ['complete'],
        },
      ],
      definition_edges: [],
      node_runs: [],
      approvals: [],
      transition_history: [],
      completed_steps: 0,
      total_steps: 4,
      progress: 0,
      started_at: '2026-08-07T00:00:00Z',
    }
    let releaseInstance: () => void = () => undefined
    const instanceGate = new Promise<void>((resolve) => { releaseInstance = resolve })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }],
            projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      if (config.url === '/requirements/42') return response(config, { data: requirement })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') {
        return response(config, { data: [{ id: 77, status: 'running' }], total: 1 })
      }
      if (config.url === '/pm/workflows/instances/77' && config.method === 'get') {
        await instanceGate
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }
    renderRequirementPoolWithWorkItems()
    fireEvent.click(await screen.findByRole('tab', { name: poolCopy.viewList }))
    fireEvent.click(await screen.findByRole('button', {
      name: poolCopy.openWorkflowNode(requirement.title, '需求录入'),
    }))

    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('需求录入') })).toBeTruthy()
    expect(screen.queryByRole('heading', { name: documentCopy.entryTitle })).toBeNull()
    expect(screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.currentTodo })).toBeNull()
    expect(screen.queryByText(copy.aiPM.workflowSubject.waiting)).toBeNull()
    await act(async () => { releaseInstance() })
    expect(await screen.findByRole('heading', { name: documentCopy.entryTitle })).toBeTruthy()
    expect(screen.getByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('需求录入') })).toBeTruthy()
    expect(screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.currentTodo })).toBeNull()
  })

  it('moves a requirement pool row to another project version', async () => {
    const requirement = requirementFixture({ version: 3, version_task_new_id: 12 })
    let moveBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }, { id: 21, name: '目标项目' }],
            projects: [
              { id: 12, project_new_id: 11, title: 'V1.1' },
              { id: 32, project_new_id: 21, title: 'V2.0' },
            ],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      if (config.url === '/requirements/batch-move' && config.method === 'post') {
        moveBody = JSON.parse(String(config.data))
        return response(config, { data: { moved_count: 1, items: [] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    fireEvent.click(await screen.findByRole('tab', { name: poolCopy.viewList }))
    expect(await screen.findByRole('columnheader', { name: poolCopy.actions })).toBeTruthy()
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.moveTo }))
    expect(screen.queryByText('detail target')).toBeNull()
    expect(await screen.findByRole('dialog', { name: copy.projectManagement.content.requirement.moveTitle })).toBeTruthy()
    await selectOption(poolCopy.project, '目标项目')
    await selectOption(new RegExp(copy.projectManagement.content.requirement.moveTarget), 'V2.0')
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.content.common.move }))
    await waitFor(() => expect(moveBody).toEqual({
      items: [{ requirement_id: 42, expected_version: 3 }],
      version_task_new_id: 32,
    }))
  })

  it('deletes a requirement pool row after confirmation', async () => {
    const requirement = requirementFixture({ title: '待删除需求' })
    let deleted = false
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, { data: { project_sets: [{ id: 11, name: 'AI PM' }], projects: [{ id: 12, project_new_id: 11, title: 'V1.1' }], requirements: [] } })
      }
      if (config.url === '/requirements') {
        return response(config, {
          data: deleted ? [] : [requirement],
          total: deleted ? 0 : 1,
          summary: { total: deleted ? 0 : 1, unscheduled: 0, delivery: deleted ? 0 : 1 },
        })
      }
      if (config.url === '/requirements/42' && config.method === 'delete') {
        deleted = true
        return response(config, { data: { deleted: true } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    fireEvent.click(await screen.findByRole('tab', { name: poolCopy.viewList }))
    const table = await screen.findByRole('table', { name: poolCopy.table })
    fireEvent.click(await within(table).findByRole('button', { name: poolCopy.delete }))
    expect(screen.queryByText('detail target')).toBeNull()
    expect(await screen.findByRole('alertdialog', { name: copy.projectManagement.content.requirement.deleteTitle })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.content.requirement.confirmDelete }))
    await waitFor(() => expect(deleted).toBe(true))
    await waitFor(() => expect(screen.getByText(poolCopy.emptyTitle)).toBeTruthy())
  })

  it('moves a requirement card from the cover overflow menu', async () => {
    const requirement = requirementFixture({ title: '卡片移动需求', version: 3, version_task_new_id: 12 })
    let moveBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/project-requirement-scope-options') {
        return response(config, {
          data: {
            project_sets: [{ id: 11, name: 'AI PM' }, { id: 21, name: '目标项目' }],
            projects: [
              { id: 12, project_new_id: 11, title: 'V1.1' },
              { id: 32, project_new_id: 21, title: 'V2.0' },
            ],
            requirements: [],
          },
        })
      }
      if (config.url === '/requirements') {
        return response(config, { data: [requirement], total: 1, summary: { total: 1, unscheduled: 0, delivery: 1 } })
      }
      if (config.url === '/requirements/batch-move' && config.method === 'post') {
        moveBody = JSON.parse(String(config.data))
        return response(config, { data: { moved_count: 1, items: [] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/workspace/requirements', RequirementPoolPage, '/workspace/requirements/$requirementId')
    fireEvent.click(await screen.findByRole('tab', { name: poolCopy.viewCards }))
    expect(await screen.findByRole('heading', { name: '卡片移动需求' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: poolCopy.moveTo })).toBeNull()
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.moreActionsFor('卡片移动需求') }))
    fireEvent.click(await screen.findByRole('menuitem', { name: poolCopy.moveTo }))
    expect(screen.queryByText('detail target')).toBeNull()
    expect(await screen.findByRole('dialog', { name: copy.projectManagement.content.requirement.moveTitle })).toBeTruthy()
    await selectOption(poolCopy.project, '目标项目')
    await selectOption(new RegExp(copy.projectManagement.content.requirement.moveTarget), 'V2.0')
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.content.common.move }))
    await waitFor(() => expect(moveBody).toEqual({
      items: [{ requirement_id: 42, expected_version: 3 }],
      version_task_new_id: 32,
    }))
  })

  it('launches a plan-unit AI capability with the selected business object', async () => {
    let createBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen3.6-max-preview', capabilities: [{ key: 'release.readiness', name: '发布就绪检查', description: '检查门禁', subject_types: ['plan_unit'], allowed_fields: [], mode: 'analysis' }] } })
      if (config.url === '/ai/runs' && config.method === 'post') { createBody = JSON.parse(String(config.data)); return response(config, { data: { id: 9, capability: 'release.readiness', subject_type: 'plan_unit', subject_id: 88, subject_version: 1, status: 'queued', model: 'qwen3.6-max-preview', created_at: new Date().toISOString(), updated_at: new Date().toISOString() } }) }
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><PlanUnitAIActions planUnitId={88} /></QueryClientProvider>)
    fireEvent.click(await screen.findByRole('button', { name: 'AI 助手' }))
    await selectOption('AI 能力', '发布就绪检查')
    fireEvent.click(screen.getByRole('button', { name: '生成建议' }))
    await waitFor(() => expect(createBody).toEqual({ capability: 'release.readiness', subject_type: 'plan_unit', subject_id: 88 }))
    expect(await screen.findByText(/运行 #9/)).toBeTruthy()
  })

  it('renders and operates the embedded workflow execution card', async () => {
    let actionBody: Record<string, unknown> | undefined
    let planBody: Record<string, unknown> | undefined
    const requirement = {
      id: 42, workspace_id: 7, team_id: 1, project_new_id: 11, version_task_new_id: 12, version: 3,
      title: '支付流程改造', description: '优化收银台交互', acceptance_criteria: '支付成功可核验', priority: 'high', status: 'grooming', task_count: 2, progress: 25, allowed_actions: [], created_at: '2026-08-06T00:00:00Z', updated_at: '2026-08-07T00:00:00Z',
    }
    const instance = {
      id: 77,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['review'],
      current_nodes: [{ id: 'review', type: 'approval', name: '产品评审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'completed', actual_completed_at: '2026-08-07T00:30:00Z' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active' },
        { id: 'delivery', type: 'human_task', name: '研发交付', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'input-review', source: 'input', target: 'review' }],
      node_runs: [{ id: 1, node_id: 'start', node_type: 'start', status: 'completed', started_at: '2026-08-07T00:00:00Z', completed_at: '2026-08-07T00:00:00Z' }],
      approvals: [],
      transition_history: [
        { id: 1, node_id: 'start', node_name: '开始', node_type: 'start', action: 'started', status: 'completed', operator_type: 'user', operator_user_id: 7, operator_name: '产品负责人', occurred_at: '2026-08-07T00:00:00Z' },
        { id: 2, node_id: 'review', node_name: '产品评审', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_user_id: 8, operator_name: '历史审批人', comment: '上一轮审批意见', occurred_at: '2026-08-07T01:00:00Z' },
        { id: 3, node_id: 'review', node_name: '产品评审', node_type: 'approval', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_user_id: 7, operator_name: '产品负责人', comment: '需求范围需要补充', target_node_id: 'input', target_node_name: '需求录入', occurred_at: '2026-08-07T02:00:00Z' },
      ],
      completed_steps: 0,
      total_steps: 2,
      progress: 0,
      started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 77, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/77' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/77/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: instance })
      }
      if (config.url === '/pm/workflows/instances/77/node-plans' && config.method === 'patch') {
        planBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, node_plans: { delivery: { planned_start_at: '2026-08-12T00:00:00Z', planned_end_at: '2026-08-18T00:00:00Z' } }, timeline_nodes: instance.timeline_nodes.map((node) => node.id === 'delivery' ? { ...node, planned_start_at: '2026-08-12T00:00:00Z', planned_end_at: '2026-08-18T00:00:00Z' } : node) } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} requirement={requirement} />, '/detail')
    expect(await screen.findByText('需求生命周期')).toBeTruthy()
    expect(screen.getAllByText('产品评审')).toHaveLength(1)
    expect(screen.getByText('研发交付')).toBeTruthy()
    expect(screen.getByText('0 / 2 个业务阶段已完成')).toBeTruthy()
    const todoNode = screen.getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('产品评审') })
    expect(todoNode.tagName).toBe('BUTTON')
    expect(todoNode.textContent).toContain('产品评审')
    expect(screen.getByText(copy.aiPM.workflowSubject.instanceOnlyHint)).toBeTruthy()
    expect(todoNode.contains(screen.getByTestId('workflow-todo-indicator-review'))).toBe(true)
    expect(screen.getByTestId('workflow-stage-review').contains(todoNode)).toBe(true)
    const futureNode = screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('研发交付') })
    fireEvent.click(futureNode)
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('研发交付') })).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.completeNode })).toBeNull()
    expect(screen.getByText(copy.aiPM.workflowSubject.actualCompletedAuto)).toBeTruthy()
    await selectDate(copy.aiPM.workflowSubject.nodePlanStartLabel('研发交付'), '2026-08-12')
    await selectDate(copy.aiPM.workflowSubject.nodePlanEndLabel('研发交付'), '2026-08-18')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanSaveOne('研发交付') }))
    await waitFor(() => expect(planBody).toEqual({ nodes: [{ node_id: 'delivery', planned_start_at: '2026-08-12', expected_planned_start_at: null, planned_end_at: '2026-08-18', expected_planned_end_at: null }] }))
    await waitFor(() => expect(futureNode.textContent).toContain('8/12–8/18'))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    expect(screen.queryByText(copy.aiPM.workflowSubject.historyStarted)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /流转记录/ }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.historyTitle })).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.historyStarted)).toBeTruthy()
    expect(screen.getAllByText('产品负责人').length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    expect(screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })).toBeNull()
    fireEvent.click(todoNode)
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })).toBeTruthy()
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.approvalResult })).toBeNull()
    expect(screen.queryByText('上一轮审批意见')).toBeNull()
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflowSubject.commentLabel), { target: { value: '范围清晰，同意进入研发' } })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.approve }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'approve', node_id: 'review', comment: '范围清晰，同意进入研发' }))
  })

  it('closes the workflow todo as soon as the action succeeds without waiting for refresh', async () => {
    let actionPosted = false
    let releaseRefresh: () => void = () => undefined
    const refreshGate = new Promise<void>((resolve) => { releaseRefresh = resolve })
    const instance = {
      id: 77, template_id: 5, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['review'], current_nodes: [{ id: 'review', type: 'approval', name: '产品评审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    const nextInstance = {
      ...instance,
      current_node_ids: [],
      current_nodes: [],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'completed' },
        { id: 'end', type: 'end', name: '结束', status: 'completed' },
      ],
      status: 'completed',
      completed_steps: 1,
      progress: 100,
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 77, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/77' && config.method === 'get') {
        if (actionPosted) await refreshGate
        return response(config, { data: actionPosted ? nextInstance : instance })
      }
      if (config.url === '/pm/workflows/instances/77/actions' && config.method === 'post') {
        actionPosted = true
        return response(config, { data: nextInstance })
      }
      return response(config, { data: [] })
    }
    renderRoute('/workflow-action-close', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('产品评审') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.approve }))
    await waitFor(() => expect((screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })) === null).toBe(true))
    expect(screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('产品评审') })).toBeNull()
    releaseRefresh()
  })

  it('shows a safe conflict when top-level schedule confirmation is rejected', async () => {
    let planBody: Record<string, unknown> | undefined
    const instance: WorkflowInstance = {
      id: 78,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['delivery'],
      current_nodes: [{ id: 'delivery', type: 'human_task', name: '研发交付' }],
      timeline_nodes: [{ id: 'delivery', type: 'human_task', name: '研发交付', status: 'active' }],
      definition_edges: [],
      node_runs: [],
      approvals: [],
      completed_steps: 0,
      total_steps: 1,
      progress: 0,
      started_at: '2026-08-07T00:00:00Z',
      subject_schedule: {
        state: 'partial',
        can_confirm: true,
        subject: { planned_start_at: null, planned_end_at: null },
        suggested: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-20' },
        conflicts: [],
      },
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') {
        return response(config, { data: [{ id: 78, status: 'running' }], total: 1 })
      }
      if (config.url === '/pm/workflows/instances/78' && config.method === 'get') {
        return response(config, { data: instance })
      }
      if (config.url === '/pm/workflows/instances/78/node-plans' && config.method === 'patch') {
        planBody = JSON.parse(String(config.data))
        throw {
          config,
          response: {
            config,
            status: 409,
            data: {
              code: 'WORKFLOW_SCHEDULE_CONFLICT',
              error: 'private sibling workflow details',
              schedule: {
                state: 'conflict',
                can_confirm: false,
                conflicts: [{ code: 'NODE_AFTER_SUBJECT_END', node_id: 'delivery', value: '2026-08-20' }],
              },
            },
          },
        }
      }
      return response(config, { data: [] })
    }

    renderRoute('/workflow-schedule-confirmation', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.scheduleConfirmSuggested }))

    expect(await screen.findByTestId('workflow-schedule-conflict')).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.scheduleNodeAfterEnd('研发交付', '2026-08-20'))).toBeTruthy()
    expect(screen.queryByText('private sibling workflow details')).toBeNull()
    expect(planBody).toEqual({ nodes: [], confirm_subject_schedule: true })
  })

  it('does not show node deliverables in a requirement pre-review approval dialog', async () => {
    const requirement = {
      id: 42, workspace_id: 7, team_id: 1, project_new_id: 11, version_task_new_id: 12, version: 3, workflow_instance_id: 178,
      design_url: 'https://www.figma.com/design/checkout/PM', product_doc_url: 'https://example.feishu.cn/wiki/checkout',
      title: '支付流程改造', description: '优化收银台交互', acceptance_criteria: '支付成功可核验', priority: 'high', status: 'grooming', task_count: 2, progress: 25, allowed_actions: [], created_at: '2026-08-06T00:00:00Z', updated_at: '2026-08-07T00:00:00Z',
    }
    const instance = {
      id: 178,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 4,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['requirement-pre-review'],
      current_nodes: [{ id: 'requirement-pre-review', type: 'approval', name: '需求预审审批' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'completed', config: { requirement_input: true } },
        { id: 'requirement-pre-review', type: 'approval', name: '需求预审审批', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'input-pre-review', source: 'input', target: 'requirement-pre-review' }],
      node_runs: [],
      approvals: [],
      transition_history: [],
      completed_steps: 1,
      total_steps: 2,
      progress: 50,
      started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 178, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/178' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/pre-review-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} requirement={requirement} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('需求预审审批') }))
    const dialog = await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('需求预审审批') })
    expect(within(dialog).queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()
    expect(within(dialog).getByTestId('workflow-pre-review-opinions').getAttribute('data-layout')).toBe('split')
    expect(within(dialog).queryByRole('region', { name: copy.aiPM.workflowSubject.approvalResult })).toBeNull()
    expect(within(dialog).queryByRole('textbox', { name: copy.aiPM.workflowSubject.commentLabel })).toBeNull()
    expect(within(dialog).getByText(copy.aiPM.workflow.approval)).toBeTruthy()
    expect(within(dialog).queryByRole('heading', { name: copy.aiPM.requirementDetail.deliverablesTitle })).toBeNull()
    expect(within(dialog).queryByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenRequirementDocument })).toBeNull()
  })

  it('shows the passed pre-review opinion in the next approval node', async () => {
    const instance: WorkflowInstance = {
      id: 179,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 4,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['product-review'],
      current_nodes: [{ id: 'product-review', type: 'approval', name: '产品评审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'requirement-pre-review', type: 'approval', name: '需求预审审批', status: 'completed' },
        { id: 'product-review', type: 'approval', name: '产品评审', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'pre-review-product-review', source: 'requirement-pre-review', target: 'product-review', kind: 'approve' },
      ],
      node_runs: [],
      approvals: [],
      transition_history: [
        { id: 7, node_id: 'product-review', node_name: '产品评审', node_type: 'approval', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '评审负责人', comment: '补充边界后重新预审', target_node_id: 'requirement-pre-review', target_node_name: '需求预审审批', occurred_at: '2026-08-09T06:20:00Z' },
        { id: 8, node_id: 'requirement-pre-review', node_name: '需求预审审批', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '产品负责人', comment: '需求边界清晰，同意进入产品评审', occurred_at: '2026-08-09T06:30:00Z' },
      ],
      completed_steps: 1,
      total_steps: 2,
      progress: 50,
      started_at: '2026-08-09T06:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 179, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/179' && config.method === 'get') return response(config, { data: instance })
      if (config.url?.includes('/nodes/requirement-pre-review/opinions') && config.method === 'get') {
        return response(config, {
          data: [
            {
              id: 1, workspace_id: 7, instance_id: 179, node_id: 'requirement-pre-review', author_user_id: 9,
              author_display_name: '测试同学', body: '建议补齐验收标准', created_at: '2026-08-09T06:10:00Z', updated_at: '2026-08-09T06:10:00Z',
            },
          ],
          total: 1,
        })
      }
      return response(config, { data: [] })
    }
    renderRoute('/passed-pre-review-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('产品评审') }))
    const dialog = await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })
    const approvalContent = within(dialog).getByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })
    expect(approvalContent.textContent).toContain(copy.aiPM.workflowSubject.approvalSource)
    expect(approvalContent.textContent).toContain('需求预审审批')
    expect(approvalContent.textContent).toContain(copy.aiPM.workflowSubject.approvalApproved)
    expect(approvalContent.textContent).toContain('产品负责人')
    expect(approvalContent.textContent).toContain('需求边界清晰，同意进入产品评审')
    expect(await within(approvalContent).findByText('建议补齐验收标准')).toBeTruthy()
    expect(approvalContent.textContent).not.toContain('补充边界后重新预审')
    expect(approvalContent.textContent).not.toContain(copy.aiPM.workflowSubject.approvalRollbackEmpty)
  })

  it('shows a rejected approval only in the node it actually returned to', async () => {
    const instance: WorkflowInstance = {
      id: 181,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 4,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['pre-review'],
      current_nodes: [{ id: 'pre-review', type: 'approval', name: '需求预审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'pre-review', type: 'approval', name: '需求预审', status: 'active' },
        { id: 'product-review', type: 'approval', name: '产品评审', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'legacy-reject', source: 'product-review', target: 'requirement-input', kind: 'reject' },
      ],
      node_runs: [],
      approvals: [],
      transition_history: [
        { id: 12, node_id: 'product-review', node_name: '产品评审', node_type: 'approval', action: 'rejected', status: 'rejected', operator_type: 'user', operator_name: '产品负责人', comment: '请先补充预审结论', target_node_id: 'pre-review', target_node_name: '需求预审', occurred_at: '2026-08-09T08:00:00Z' },
      ],
      completed_steps: 0,
      total_steps: 2,
      progress: 0,
      started_at: '2026-08-09T06:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 181, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/181' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/rejected-approval-context-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('需求预审') }))
    const content = await screen.findByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })
    expect(content.textContent).toContain('产品评审')
    expect(content.textContent).toContain(copy.aiPM.workflowSubject.approvalRejected)
    expect(content.textContent).toContain('请先补充预审结论')
  })

  it('hides an older approval after a non-approval node has run', async () => {
    const instance: WorkflowInstance = {
      id: 182,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 4,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['development'],
      current_nodes: [{ id: 'development', type: 'human_task', name: '研发实现' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'product-review', type: 'approval', name: '产品评审', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'completed' },
        { id: 'development', type: 'human_task', name: '研发实现', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'review-design', source: 'product-review', target: 'design', kind: 'approve' },
        { id: 'design-development', source: 'design', target: 'development' },
      ],
      node_runs: [],
      approvals: [],
      transition_history: [
        { id: 20, node_id: 'product-review', node_name: '产品评审', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_name: '产品负责人', comment: '同意进入设计', target_node_id: 'design', target_node_name: 'UI/UX设计', occurred_at: '2026-08-09T08:00:00Z' },
        { id: 21, node_id: 'design', node_name: 'UI/UX设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '设计负责人', occurred_at: '2026-08-09T09:00:00Z' },
      ],
      completed_steps: 2,
      total_steps: 3,
      progress: 67,
      started_at: '2026-08-09T06:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 182, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/182' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/non-approval-predecessor-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('研发实现') }))
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()
  })

  it('opens a linked active node in assignee-only read-only mode and preserves unrelated search', async () => {
    window.history.replaceState({}, '', '/workflow-owner-card?tab=overview&workflow_node=review#workflow')
    let actionCalls = 0
    let planCalls = 0
    const instance: WorkflowInstance = {
      id: 180,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'running',
      can_manage_node_plans: true,
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['review'],
      current_nodes: [{
        id: 'review', type: 'approval', name: '产品评审', assignee_user_id: 9, assignee_name: '周可', can_act: false, allowed_actions: [],
      }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active', assignee_user_id: 9, assignee_name: '周可', can_act: false, allowed_actions: [] },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [],
      node_runs: [],
      approvals: [],
      transition_history: [],
      completed_steps: 0,
      total_steps: 1,
      progress: 0,
      started_at: '2026-08-09T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 180, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/180' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/180/actions' && config.method === 'post') actionCalls += 1
      if (config.url === '/pm/workflows/instances/180/node-plans' && config.method === 'patch') planCalls += 1
      return response(config, { data: instance })
    }

    renderRoute('/workflow-owner-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    const dialog = await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })
    expect(screen.getByTestId('workflow-stage-assignee-review').textContent).toContain('周可')
    expect(within(dialog).getByTestId('workflow-node-readonly').textContent).toContain(copy.aiPM.workflowSubject.assigneeReadOnly)
    expect(within(dialog).queryByRole('button', { name: copy.aiPM.workflowSubject.approve })).toBeNull()
    expect(within(dialog).queryByRole('button', { name: copy.aiPM.workflowSubject.reject })).toBeNull()
    expect(within(dialog).getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanStartLabel('产品评审') })).toHaveProperty('disabled', false)
    expect(within(dialog).getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanSaveOne('产品评审') })).toHaveProperty('disabled', false)

    fireEvent.click(within(dialog).getByRole('button', { name: copy.aiPM.common.cancel }))
    await waitFor(() => expect((screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('产品评审') })) === null).toBe(true))
    const search = new URLSearchParams(window.location.search)
    expect(search.get('workflow_node')).toBeNull()
    expect(search.get('tab')).toBe('overview')
    expect(window.location.hash).toBe('#workflow')
    expect(actionCalls).toBe(0)
    expect(planCalls).toBe(0)
  })

  it('submits an active approval rejection with the decision comment', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 79, template_id: 5, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['review'], current_nodes: [{ id: 'review', type: 'approval', name: '产品评审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active' },
        { id: 'input', type: 'form', name: '需求录入', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'reject-review', source: 'review', target: 'input', kind: 'reject', loop: true }],
      node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 79, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/79' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/79/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, status: 'rejected', current_node_ids: [], current_nodes: [] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/reject-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('产品评审') }))
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflowSubject.commentLabel), { target: { value: '需求边界不清晰' } })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.reject }))

    await waitFor(() => expect(actionBody).toEqual({ action: 'reject', node_id: 'review', comment: '需求边界不清晰' }))
  })

  it('discards node form state when the workflow instance or workspace changes', async () => {
    const workflow = (id: number, subjectId: number, reviewName: string): WorkflowInstance => ({
      id, template_id: 5, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: subjectId,
      current_node_ids: ['review'], current_nodes: [{ id: 'review', type: 'approval', name: reviewName }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: reviewName, status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-09T00:00:00Z',
    })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances/77') return response(config, { data: workflow(77, 42, '空间 A 审批') })
      if (config.url === '/pm/workflows/instances/88') return response(config, { data: workflow(88, 43, '空间 B 审批') })
      return response(config, { data: [], total: 0 })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const view = render(<QueryClientProvider client={client}><WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} instanceId={77} /></QueryClientProvider>)

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('空间 A 审批') }))
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflowSubject.commentLabel), { target: { value: '只属于空间 A' } })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    await waitFor(() => expect((screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('空间 A 审批') })) === null).toBe(true))

    view.rerender(<QueryClientProvider client={client}><WorkflowSubjectLauncher workspaceId={8} subjectType="requirement" subjectId={43} instanceId={88} /></QueryClientProvider>)
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('空间 B 审批') }))
    expect((screen.getByLabelText(copy.aiPM.workflowSubject.commentLabel) as HTMLTextAreaElement).value).toBe('')
  })

  it('shows one-click AI generation inside a supplemental legacy test-case node', async () => {
    const instance = {
      id: 177, template_id: 5, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['review'], current_nodes: [{ id: 'review', type: 'approval', name: '产品评审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active' },
        { id: 'test-cases', type: 'human_task', name: '测试用例编写', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 177, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/177' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url === '/pm/workflows/instances/177/nodes/test-cases/test-case-artifacts') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }
    renderRoute('/test-case-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openNode('测试用例编写') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('测试用例编写') })).toBeTruthy()
    expect(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })).toBeTruthy()
    expect(screen.queryByText(copy.aiPM.workflowSubject.manualActionHint)).toBeNull()
  })

  it('renders project parallel branches as a compact clickable group without a workflow canvas', async () => {
    const instance = {
      id: 93, template_id: 8, template_name: '并行交付流程', template_version: 4, status: 'running', subject_type: 'plan_unit', subject_id: 42,
      subject_schedule: {
        state: 'partial',
        subject: { planned_start_at: '1999-01-01', planned_end_at: '2999-01-01' },
        conflicts: [],
      },
      current_node_ids: ['qa', 'join'],
      current_nodes: [
        { id: 'qa', type: 'human_task', name: 'QA 测试' },
        { id: 'join', type: 'parallel_join', name: '交付汇聚' },
      ],
      timeline_nodes: [
        { id: 'acceptance', type: 'approval', name: '产品验收', status: 'pending', planned_end_at: '2999-01-01' },
        { id: 'integration', type: 'human_task', name: '联调验证', status: 'pending' },
        { id: 'qa', type: 'human_task', name: 'QA 测试', status: 'active', planned_start_at: '2000-01-01', planned_end_at: '2000-01-02' },
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'join', type: 'parallel_join', name: '交付汇聚', status: 'waiting', parallel_completed: 1, parallel_total: 2 },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'completed', planned_end_at: '2000-01-01' },
        { id: 'split', type: 'parallel_split', name: '研发测试并行', status: 'completed' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'e1', source: 'start', target: 'split' },
        { id: 'e2', source: 'split', target: 'develop' },
        { id: 'e3', source: 'split', target: 'qa' },
        { id: 'e4', source: 'develop', target: 'integration' },
        { id: 'e5', source: 'integration', target: 'join' },
        { id: 'e6', source: 'qa', target: 'join' },
        { id: 'e7', source: 'join', target: 'acceptance' },
        { id: 'e8', source: 'acceptance', target: 'end' },
        { id: 'e9', source: 'develop', target: 'join', kind: 'failure' },
      ],
      selected_node_ids: ['start', 'split', 'develop', 'integration', 'qa', 'join', 'acceptance', 'end'],
      selected_edge_ids: ['e1', 'e2', 'e3', 'e4', 'e5', 'e6', 'e7', 'e8'],
      node_runs: [], approvals: [], transition_history: [
        { id: 6, node_id: 'acceptance', node_name: '产品验收', node_type: 'approval', action: 'rolled_back', status: 'rolled_back', operator_type: 'user', operator_name: '产品负责人', comment: 'QA 验证未通过，需要补测', target_node_id: 'qa', target_node_name: 'QA 测试', occurred_at: '2026-08-08T02:00:00Z' },
      ], completed_steps: 1, total_steps: 4, progress: 25, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 93, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/93' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/parallel-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="plan_unit" subjectId={42} />, '/detail')

    const stages = await screen.findByTestId('workflow-runtime-stages')
    expect(stages.dataset.layout).toBe('compact-track')
    expect(stages.className).toContain('overflow-x-auto')
    const group = screen.getByTestId('workflow-parallel-split')
    expect(group.dataset.layout).toBe('compact-parallel-stack')
    expect(group.className).not.toContain('overflow-x-auto')
    const developLane = screen.getByTestId('workflow-fork-lane-split-0')
    const qaLane = screen.getByTestId('workflow-fork-lane-split-1')
    expect(within(developLane).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual([
      copy.aiPM.workflowSubject.openNode('研发实现'),
      copy.aiPM.workflowSubject.openNode('联调验证'),
    ])
    expect(within(qaLane).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual([
      copy.aiPM.workflowSubject.openTodo('QA 测试'),
    ])
    for (const nodeID of ['develop', 'qa', 'integration', 'acceptance']) {
      expect(screen.getAllByTestId(`workflow-stage-${nodeID}`)).toHaveLength(1)
    }
    for (const nodeID of ['start', 'split', 'join', 'end']) {
      expect(screen.queryByTestId(`workflow-stage-${nodeID}`)).toBeNull()
    }
    expect(screen.queryByTestId('workflow-runtime-graph')).toBeNull()
    expect(stages.querySelector('.react-flow')).toBeNull()
    expect(screen.getByTestId('workflow-stage-develop').dataset.density).toBe('branch')
    expect(screen.getByTestId('workflow-stage-integration').dataset.density).toBe('branch')
    expect(screen.getByTestId('workflow-stage-qa').dataset.density).toBe('branch')
    expect(screen.getByTestId('workflow-stage-acceptance').dataset.density).toBe('track')
    expect(screen.getByTestId('workflow-stage-qa').className).toContain('w-[8.5rem]')
    expect(screen.getByTestId('workflow-node-action-qa').className).toContain('min-h-11')
    expect(screen.getByTestId('workflow-node-action-qa').getAttribute('aria-current')).toBe('step')
    expect(screen.getByTestId('workflow-node-action-qa').className).toContain('focus-visible:ring-inset')
    expect(screen.getByTestId('workflow-node-action-qa').className).not.toContain('focus-visible:ring-offset-2')
    expect(screen.getByTestId('workflow-stage-qa').dataset.overdue).toBe('true')
    expect(screen.getByTestId('workflow-overdue-qa').getAttribute('aria-label')).toBe(copy.aiPM.workflowSubject.nodeOverdueLabel('QA 测试'))
    expect(screen.queryByTestId('workflow-overdue-develop')).toBeNull()
    expect(screen.queryByTestId('workflow-overdue-acceptance')).toBeNull()
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('QA 测试') })).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.openNode('研发测试并行') })).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.openTodo('交付汇聚') })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('QA 测试') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('QA 测试') })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    await waitFor(() => expect((screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('QA 测试') })) === null).toBe(true))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('产品验收') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('产品验收') })).toBeTruthy()
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()
  })

  it('preserves the real project risk and development lanes before parallel work starts', async () => {
    const instance = {
      id: 11, template_id: 2, template_name: 'test', template_version: 1, status: 'running', subject_type: 'plan_unit', subject_id: 27,
      current_node_ids: ['node_requirement_entry'], current_nodes: [{ id: 'node_requirement_entry', type: 'form', name: '需求录入' }],
      timeline_nodes: [
        { id: 'node_acceptance_test', type: 'human_task', name: '测试与验收准备', status: 'pending' },
        { id: 'node_risk_handle', type: 'human_task', name: '风险应对处理', status: 'pending' },
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'node_product_review', type: 'approval', name: '产品评审', status: 'pending' },
        { id: 'node_execution_join', type: 'parallel_join', name: '执行阶段汇聚', status: 'waiting', parallel_completed: 0, parallel_total: 2 },
        { id: 'node_risk_check_ai', type: 'ai_check', name: 'AI风险智能巡检', status: 'pending' },
        { id: 'node_requirement_entry', type: 'form', name: '需求录入', status: 'active' },
        { id: 'node_dev_task', type: 'human_task', name: '开发与实施', status: 'pending' },
        { id: 'node_execution_split', type: 'parallel_split', name: '执行阶段并行分支', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'e-start-req', source: 'start', target: 'node_requirement_entry' },
        { id: 'e-req-review', source: 'node_requirement_entry', target: 'node_product_review' },
        { id: 'e-review-exec', source: 'node_product_review', target: 'node_execution_split', kind: 'approve' },
        { id: 'e-split-dev', source: 'node_execution_split', target: 'node_dev_task', loop: true },
        { id: 'e-split-risk', source: 'node_execution_split', target: 'node_risk_check_ai', loop: true },
        { id: 'e-risk-handle', source: 'node_risk_check_ai', target: 'node_risk_handle', loop: true },
        { id: 'e-risk-failure', source: 'node_risk_check_ai', target: 'node_risk_handle', kind: 'failure', loop: true },
        { id: 'e-dev-join', source: 'node_dev_task', target: 'node_execution_join', loop: true },
        { id: 'e-risk-join', source: 'node_risk_handle', target: 'node_execution_join', loop: true },
        { id: 'e-join-test', source: 'node_execution_join', target: 'node_acceptance_test', loop: true },
        { id: 'e-test-end', source: 'node_acceptance_test', target: 'end' },
      ],
      selected_node_ids: ['start', 'node_requirement_entry', 'node_product_review', 'node_execution_split', 'node_dev_task', 'node_risk_check_ai', 'node_risk_handle', 'node_execution_join', 'node_acceptance_test', 'end'],
      selected_edge_ids: ['e-start-req', 'e-req-review', 'e-review-exec', 'e-split-dev', 'e-split-risk', 'e-risk-handle', 'e-dev-join', 'e-risk-join', 'e-join-test', 'e-test-end'],
      node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 6, progress: 0, started_at: '2026-08-10T06:27:10Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 11, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/11' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/future-join-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="plan_unit" subjectId={27} />, '/detail')

    const stages = await screen.findByTestId('workflow-runtime-stages')
    const group = within(stages).getByTestId('workflow-parallel-node_execution_split')
    expect(group.textContent).toContain(copy.aiPM.workflowSubject.parallelPending)
    const developmentLane = within(stages).getByTestId('workflow-fork-lane-node_execution_split-0')
    const riskLane = within(stages).getByTestId('workflow-fork-lane-node_execution_split-1')
    expect(within(developmentLane).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual([
      copy.aiPM.workflowSubject.openNode('开发与实施'),
    ])
    expect(within(riskLane).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual([
      copy.aiPM.workflowSubject.openNode('AI风险智能巡检'),
      copy.aiPM.workflowSubject.openNode('风险应对处理'),
    ])
    for (const nodeID of ['node_requirement_entry', 'node_product_review', 'node_dev_task', 'node_risk_check_ai', 'node_risk_handle', 'node_acceptance_test']) {
      expect(within(stages).getAllByTestId(`workflow-stage-${nodeID}`)).toHaveLength(1)
    }
    for (const nodeID of ['start', 'node_execution_split', 'node_execution_join', 'end']) {
      expect(screen.queryByTestId(`workflow-stage-${nodeID}`)).toBeNull()
    }
    expect(screen.getByTestId('workflow-stage-node_acceptance_test').dataset.density).toBe('track')
    fireEvent.click(within(stages).getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('需求录入') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('需求录入') })).toBeTruthy()
  })

  it('rolls any active project or requirement node back to a completed node with a reason', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 177, template_id: 5, template_name: '需求生命周期', template_version: 4, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['review'], current_nodes: [{ id: 'review', type: 'approval', name: '产品评审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '信息准备', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      rollback_targets: [{ id: 'input', type: 'form', name: '信息准备' }],
      definition_edges: [{ id: 'input-review', source: 'input', target: 'review' }], node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 177, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/177' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/177/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['input'], current_nodes: [{ id: 'input', type: 'form', name: '信息准备' }] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/rollback-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('产品评审') }))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.rollback }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.rollbackTitle })).toBeTruthy()
    await selectOption(copy.aiPM.workflowSubject.rollbackTarget, '信息准备')
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflowSubject.rollbackReason), { target: { value: '评审材料需要补充' } })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.rollbackConfirm }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'rollback', node_id: 'review', target_node_id: 'input', comment: '评审材料需要补充' }))
  })

  it('keeps a condition node generic and advances with an explicit yes or no decision', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 78,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['ui'],
      current_nodes: [{ id: 'ui', type: 'condition', name: '是否需要UI设计' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'ui', type: 'condition', name: '是否需要UI设计', status: 'active' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'pending', config: { require_design_image: true } },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'pending' },
        { id: 'merge', type: 'human_task', name: '共同验收准备', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'start-ui', source: 'start', target: 'ui' },
        { id: 'yes', source: 'ui', target: 'design', condition: 'context.decision == true' },
        { id: 'no', source: 'ui', target: 'develop', condition: 'context.decision == false' },
        { id: 'design-merge', source: 'design', target: 'merge' },
        { id: 'develop-merge', source: 'develop', target: 'merge' },
        { id: 'merge-end', source: 'merge', target: 'end' },
      ],
      selected_node_ids: ['start', 'ui', 'design', 'develop', 'merge', 'end'],
      selected_edge_ids: ['start-ui', 'yes', 'no', 'design-merge', 'develop-merge', 'merge-end'],
      node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 4, progress: 0, started_at: '2026-08-07T00:00:00Z',
      node_plans: { design: { assignee_user_id: 1 }, develop: { assignee_user_id: 1 } },
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 78, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/78' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/78/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['design'], current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计' }] } })
      }
      return response(config, { data: [] })
    }
    mockWorkflowUsers([{ id: 21, username: 'bill', display_name: 'bill' }])
    renderRoute('/condition-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    const stages = await screen.findByTestId('workflow-runtime-stages')
    expect(within(stages).getByTestId('workflow-choice-ui')).toBeTruthy()
    expect(screen.queryByTestId('workflow-parallel-ui')).toBeNull()
    expect(within(stages).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual([
      copy.aiPM.workflowSubject.openTodo('是否需要UI设计'),
      copy.aiPM.workflowSubject.openNode('UI/UX设计'),
      copy.aiPM.workflowSubject.openNode('研发实现'),
      copy.aiPM.workflowSubject.openNode('共同验收准备'),
    ])
    expect(screen.queryByTestId('workflow-stage-start')).toBeNull()
    expect(screen.queryByTestId('workflow-stage-end')).toBeNull()
    expect(screen.queryByTestId('workflow-runtime-graph')).toBeNull()
    expect(stages.querySelector('.react-flow')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('UI/UX设计') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('UI/UX设计') })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    await waitFor(() => expect((screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('UI/UX设计') })) === null).toBe(true))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('是否需要UI设计') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('是否需要UI设计') })).toBeTruthy()
    const confirm = screen.getByRole('button', { name: copy.aiPM.workflowSubject.conditionConfirm })
    expect((confirm as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.conditionYes }))
    expect(screen.queryByLabelText(copy.aiPM.workflowSubject.designImageInput)).toBeNull()
    expect(await screen.findByTestId('workflow-next-assignees')).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.nextAssigneeLabel('UI/UX设计'))).toBeTruthy()
    expect((confirm as HTMLButtonElement).disabled).toBe(true)
    await selectUser(copy.aiPM.workflowSubject.nextAssigneeLabel('UI/UX设计'), 'bill')
    expect((confirm as HTMLButtonElement).disabled).toBe(false)
    fireEvent.click(confirm)
    await waitFor(() => expect(actionBody).toEqual({
      action: 'condition_yes',
      node_id: 'ui',
      next_node_assignees: [{ node_id: 'design', assignee_user_id: 21 }],
    }))
  })

  it('shows only the selected condition path and keeps its business stage clickable', async () => {
    const instance = {
      id: 182, template_id: 5, template_name: '需求生命周期', template_version: 5, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['develop'], current_nodes: [{ id: 'develop', type: 'human_task', name: '研发实现' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'ui', type: 'condition', name: '是否需要UI设计', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'skipped', config: { require_design_image: true } },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'active' },
        { id: 'merge', type: 'human_task', name: '共同验收准备', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'start-ui', source: 'start', target: 'ui' },
        { id: 'yes', source: 'ui', target: 'design', condition: 'context.decision == true' },
        { id: 'design-merge', source: 'design', target: 'merge' },
        { id: 'no', source: 'ui', target: 'develop', condition: 'context.decision == false' },
        { id: 'develop-merge', source: 'develop', target: 'merge' },
        { id: 'merge-end', source: 'merge', target: 'end' },
      ],
      selected_node_ids: ['start', 'ui', 'develop', 'merge', 'end'],
      selected_edge_ids: ['start-ui', 'no', 'develop-merge', 'merge-end'],
      node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 182, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/182' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/condition-no-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    expect(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('研发实现') })).toBeTruthy()
    expect(screen.queryByTestId('workflow-stage-design')).toBeNull()
    expect(screen.getByTestId('workflow-stage-merge')).toBeTruthy()
    expect(screen.queryByTestId('workflow-stage-end')).toBeNull()
    const stages = screen.getByTestId('workflow-runtime-stages')
    expect(within(stages).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual([
      copy.aiPM.workflowSubject.openNode('是否需要UI设计'),
      copy.aiPM.workflowSubject.openTodo('研发实现'),
      copy.aiPM.workflowSubject.openNode('共同验收准备'),
    ])
    expect(screen.queryByTestId('workflow-runtime-graph')).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.openNode('UI/UX设计') })).toBeNull()
    expect(screen.queryByText(copy.aiPM.workflowSubject.designImageLabel)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('研发实现') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('研发实现') })).toBeTruthy()
  })

  it('keeps a current recovery task clickable when legacy history cannot reconstruct its selected edge', async () => {
    const instance = {
      id: 183, template_id: 5, template_name: '旧版恢复流程', template_version: 2, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['recovery'], current_nodes: [{ id: 'recovery', type: 'human_task', name: '人工恢复' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'risk', type: 'ai_check', name: '风险检查', status: 'completed' },
        { id: 'recovery', type: 'human_task', name: '人工恢复', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'start-risk', source: 'start', target: 'risk' },
        { id: 'risk-end', source: 'risk', target: 'end', kind: 'success' },
        { id: 'risk-recovery', source: 'risk', target: 'recovery', kind: 'failure' },
        { id: 'recovery-end', source: 'recovery', target: 'end' },
      ],
      selected_node_ids: ['start', 'risk', 'end'],
      selected_edge_ids: ['start-risk', 'risk-end'],
      node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 183, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/183' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/legacy-recovery-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    const recovery = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('人工恢复') })
    fireEvent.click(recovery)
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('人工恢复') })).toBeTruthy()
  })

  it('keeps a selected rollback loop in workflow order when template nodes are shuffled', async () => {
    const instance = {
      id: 184, template_id: 5, template_name: '评审回退流程', template_version: 2, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['input'], current_nodes: [{ id: 'input', type: 'form', name: '需求补充' }],
      timeline_nodes: [
        { id: 'review', type: 'approval', name: '产品评审', status: 'rejected' },
        { id: 'input', type: 'form', name: '需求补充', status: 'active' },
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
      ],
      definition_edges: [
        { id: 'start-input', source: 'start', target: 'input' },
        { id: 'input-review', source: 'input', target: 'review' },
        { id: 'review-input', source: 'review', target: 'input', kind: 'reject' },
      ],
      selected_node_ids: ['start', 'input', 'review'],
      selected_edge_ids: ['start-input', 'input-review', 'review-input'],
      node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 184, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/184' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/rollback-loop-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    const stages = await screen.findByTestId('workflow-runtime-stages')
    expect(within(stages).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual([
      copy.aiPM.workflowSubject.openTodo('需求补充'),
      copy.aiPM.workflowSubject.openNode('产品评审'),
    ])
    fireEvent.click(within(stages).getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('需求补充') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('需求补充') })).toBeTruthy()
  })

  it('requires the design image on the UI/UX task instead of on its condition', async () => {
    let actionBody: Record<string, unknown> | undefined
    let uploaded = false
    const instance = {
      id: 178,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 4,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['design'],
      current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计', config: { require_design_image: true } }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'ui', type: 'condition', name: '是否需要UI设计', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true } },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'yes', source: 'ui', target: 'design', kind: 'yes' }],
      node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 178, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/178' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/requirements/42/design-assets/upload' && config.method === 'post') {
        uploaded = config.data instanceof FormData
        return response(config, { data: { url: 'https://cdn.example.test/design.png', name: 'design.png', content_type: 'image/png', size: 3 } })
      }
      if (config.url === '/pm/workflows/instances/178/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, status: 'completed', current_node_ids: [], current_nodes: [] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/design-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('UI/UX设计') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('UI/UX设计') })).toBeTruthy()
    const complete = screen.getByRole('button', { name: copy.aiPM.workflowSubject.completeNode })
    expect((complete as HTMLButtonElement).disabled).toBe(true)
    const file = new File(['png'], 'design.png', { type: 'image/png' })
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflowSubject.designImageInput), { target: { files: [file] } })
    await waitFor(() => expect(uploaded).toBe(true))
    await waitFor(() => expect((complete as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(complete)
    await waitFor(() => expect(actionBody).toEqual({ action: 'complete', node_id: 'design', design_url: 'https://cdn.example.test/design.png' }))
  })

  it('does not reuse another design node material in the active design task', async () => {
    const instance: WorkflowInstance = {
      id: 178, template_id: 5, template_name: '多设计节点流程', template_version: 1, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['visual-design'],
      current_nodes: [{ id: 'visual-design', type: 'human_task', name: '视觉设计', config: { require_design_image: true } }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'interaction-design', type: 'human_task', name: '交互设计', status: 'completed', config: { require_design_image: true } },
        { id: 'visual-design', type: 'human_task', name: '视觉设计', status: 'active', config: { require_design_image: true } },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [],
      transition_history: [{ id: 3, node_id: 'interaction-design', node_name: '交互设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '交互设计师', materials: [{ kind: 'design_image', url: 'https://design.example.test/interaction.png' }], occurred_at: '2026-08-09T07:00:00Z' }],
      completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-09T06:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 178, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/178' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }

    renderRoute('/multi-design-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} designImageUrl="https://design.example.test/interaction.png" />, '/detail')
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('视觉设计') }))

    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('视觉设计') })).toBeTruthy()
    expect(screen.queryByText(copy.aiPM.workflowSubject.designImageExisting)).toBeNull()
    expect((screen.getByRole('button', { name: copy.aiPM.workflowSubject.completeNode }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('keeps every uploaded design image on the current node instead of replacing the previous one', async () => {
    const uploaded: string[] = []
    const instance = {
      id: 181,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 4,
      status: 'running',
      subject_type: 'requirement',
      subject_id: 42,
      current_node_ids: ['design'],
      current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计', config: { require_design_image: true } }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true } },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 181, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/181' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/requirements/42/design-assets/upload' && config.method === 'post') {
        const name = config.data instanceof FormData ? String(config.data.get('file') instanceof File ? (config.data.get('file') as File).name : 'design.png') : 'design.png'
        uploaded.push(name)
        return response(config, { data: { url: `https://cdn.example.test/${name}`, name, content_type: 'image/png', size: 3 } })
      }
      if (config.url === '/pm/workflows/instances/181/actions' && config.method === 'post') {
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }
    renderRoute('/multi-design-upload', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('UI/UX设计') }))
    expect(screen.getByLabelText(copy.aiPM.workflowSubject.designImageInput).hasAttribute('multiple')).toBe(true)
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflowSubject.designImageInput), {
      target: { files: [new File(['a'], 'draft.png', { type: 'image/png' }), new File(['b'], 'final.png', { type: 'image/png' })] },
    })
    await waitFor(() => expect(uploaded).toEqual(['draft.png', 'final.png']))
    expect(screen.getByTitle('draft.png')).toBeTruthy()
    expect(screen.getByTitle('final.png')).toBeTruthy()
  })

  it('accepts a valid effect preview link as the UI/UX task deliverable', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 179, template_id: 5, template_name: '需求生命周期', template_version: 4, status: 'running',
      subject_type: 'requirement', subject_id: 42, current_node_ids: ['design'],
      current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计', config: { require_design_image: true } }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true } },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 179, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/179' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/179/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, status: 'completed', current_node_ids: [], current_nodes: [] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/design-link-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('UI/UX设计') }))
    const complete = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.completeNode })
    const link = screen.getByLabelText(copy.aiPM.workflowSubject.designLinkLabel)
    fireEvent.change(link, { target: { value: 'javascript:alert(1)' } })
    expect((await screen.findByRole('alert')).textContent).toContain(copy.aiPM.workflowSubject.designLinkInvalid)
    expect((complete as HTMLButtonElement).disabled).toBe(true)
    fireEvent.change(link, { target: { value: 'https://www.figma.com/design/abc123/PM' } })
    expect(screen.queryByRole('alert')).toBeNull()
    expect((complete as HTMLButtonElement).disabled).toBe(false)
    fireEvent.click(complete)
    await waitFor(() => expect(actionBody).toEqual({ action: 'complete', node_id: 'design', design_url: 'https://www.figma.com/design/abc123/PM' }))
  })

  it('shows a completed workflow AI result in the project detail and confirms it before continuing', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 79,
      template_id: 8,
      template_name: '项目交付流程',
      template_version: 2,
      status: 'running',
      subject_type: 'plan_unit',
      subject_id: 25,
      current_node_ids: ['risk'],
      current_nodes: [{ id: 'risk', type: 'ai_compare', name: 'AI 风险巡检', capability: 'delivery.risk_monitor' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'delivery', type: 'human_task', name: '研发交付', status: 'completed' },
        {
          id: 'risk', type: 'ai_compare', name: 'AI 风险巡检', capability: 'delivery.risk_monitor', status: 'active',
          ai_execution: {
            run_id: 31,
            status: 'completed',
            model: 'qwen3.6-max-preview',
            output: {
              summary: '当前交付存在两项需要关注的风险。',
              changes: [],
              risks: ['计划结束时间临近，但项目仍处于开发中'],
              items: [{ title: '补齐测试准备', description: '确认提测范围与负责人', evidence: ['尚未记录实际完成时间'] }],
              humanReviewRequired: true,
            },
          },
        },
        { id: 'acceptance', type: 'approval', name: '发布验收', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [],
      completed_steps: 1, total_steps: 3, progress: 33, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 79, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/79' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/79/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['acceptance'], current_nodes: [{ id: 'acceptance', type: 'approval', name: '发布验收' }] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/ai-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="plan_unit" subjectId={25} />, '/detail')

    const aiNode = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openAIReport('AI 风险巡检') })
    expect(screen.queryByText('当前交付存在两项需要关注的风险。')).toBeNull()
    expect(screen.queryByText('计划结束时间临近，但项目仍处于开发中')).toBeNull()
    expect(screen.queryByText('补齐测试准备')).toBeNull()
    fireEvent.click(aiNode)
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('AI 风险巡检') })).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflow.aiNode)).toBeTruthy()
    expect(screen.getByText('当前交付存在两项需要关注的风险。')).toBeTruthy()
    expect(screen.getByText('计划结束时间临近，但项目仍处于开发中')).toBeTruthy()
    expect(screen.getByText('补齐测试准备')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.aiConfirmAndContinue }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'complete', node_id: 'risk' }))
  })

  it('creates a task plan after confirmation and recovers when the committed response is lost', async () => {
    const taskPlan = {
      summary: '已拆分前后端研发任务。', changes: [], items: [], risks: [], humanReviewRequired: true,
      taskPlan: { chains: [
        { lane: 'frontend' as const, summary: '前端交付', tasks: [{ title: '实现页面', description: '完成页面和接口接入', acceptanceCriteria: ['页面可操作'] }] },
        { lane: 'backend' as const, summary: '后端交付', tasks: [{ title: '实现接口', description: '完成租户隔离和持久化', acceptanceCriteria: ['接口测试通过'] }] },
      ] },
    }
    let actionBodies: Array<Record<string, unknown>> = []
    let instance: WorkflowInstance = {
      id: 82, template_id: 9, template_name: '需求交付流程', template_version: 1, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['task-plan'], current_nodes: [{ id: 'task-plan', type: 'ai_generate', name: 'AI任务拆解', capability: 'requirement.task_plan' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'task-plan', type: 'ai_generate', name: 'AI任务拆解', capability: 'requirement.task_plan', status: 'active', allowed_actions: ['complete'], ai_execution: { run_id: 81, status: 'completed', capability: 'requirement.task_plan', output: taskPlan } },
        { id: 'review', type: 'approval', name: '研发确认', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'plan-review', source: 'task-plan', target: 'review' }], node_plans: { review: { assignee_user_id: 1 } }, node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-11T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 82, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/82' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/82/actions' && config.method === 'post') {
        actionBodies = [...actionBodies, JSON.parse(String(config.data))]
        const createdTasks = [
          { id: 101, taskKey: 'DEMO-1', title: '前端研发', lane: 'frontend' as const, kind: 'task' as const },
          { id: 102, taskKey: 'DEMO-2', title: '实现页面', lane: 'frontend' as const, kind: 'subtask' as const, parentId: 101 },
          { id: 103, taskKey: 'DEMO-3', title: '后端研发', lane: 'backend' as const, kind: 'task' as const },
          { id: 104, taskKey: 'DEMO-4', title: '实现接口', lane: 'backend' as const, kind: 'subtask' as const, parentId: 103 },
        ]
        instance = {
          ...instance,
          current_node_ids: ['review'], current_nodes: [{ id: 'review', type: 'approval', name: '研发确认' }], completed_steps: 1, progress: 50,
          timeline_nodes: instance.timeline_nodes.map((node) => node.id === 'task-plan'
            ? { ...node, status: 'completed', allowed_actions: [], ai_execution: { ...node.ai_execution!, status: 'applied', output: { ...taskPlan, createdTasks } } }
            : node.id === 'review' ? { ...node, status: 'active' } : node),
        }
        throw new Error('confirmation response lost after commit')
      }
      return response(config, { data: [] })
    }

    mockWorkflowUsers([{ id: 21, username: 'bill', display_name: 'bill' }])
    renderRoute('/task-plan-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openAIReport('AI任务拆解') }))

    expect(await screen.findByText('确认采用后将创建并关联 4 个任务')).toBeTruthy()
    expect(screen.queryByRole('link', { name: /打开任务/ })).toBeNull()
    const confirm = screen.getByRole('button', { name: copy.aiPM.workflowSubject.taskPlanConfirmCreate }) as HTMLButtonElement
    expect(confirm.disabled).toBe(true)
    await selectUser(copy.aiPM.workflowSubject.nextAssigneeLabel('研发确认'), 'bill')
    expect(confirm.disabled).toBe(false)
    fireEvent.click(confirm)
    await waitFor(() => expect(actionBodies).toEqual([{
      action: 'complete',
      node_id: 'task-plan',
      next_node_assignees: [{ node_id: 'review', assignee_user_id: 21 }],
    }]))

    expect(await screen.findByText('已创建并关联 4 个任务')).toBeTruthy()
    expect(screen.getByRole('link', { name: '打开任务 DEMO-2' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.taskPlanConfirmCreate })).toBeNull()
    await waitFor(() => expect((screen.queryByText(copy.aiPM.workflowSubject.actionFailed)) === null).toBe(true))
  })

  it('supplements a missing design material from a non-active workflow node', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 180, template_id: 5, template_name: '需求生命周期', template_version: 4, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['review'], current_nodes: [{ id: 'review', type: 'approval', name: '产品评审' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'completed', config: { require_design_image: true } },
        { id: 'review', type: 'approval', name: '产品评审', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 180, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/180' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/requirements/42/design-assets/upload' && config.method === 'post') return response(config, { data: { url: 'https://cdn.example.test/late-design.png', name: 'late-design.png', content_type: 'image/png', size: 3 } })
      if (config.url === '/pm/workflows/instances/180/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }
    renderRoute('/supplement-design-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openNode('UI/UX设计') }))
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.completeNode })).toBeNull()
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflowSubject.designImageInput), { target: { files: [new File(['png'], 'late-design.png', { type: 'image/png' })] } })
    await waitFor(() => expect(actionBody).toEqual({ action: 'supplement_design', node_id: 'design', design_url: 'https://cdn.example.test/late-design.png' }))
  })

  it('attaches requirement library design materials back onto a future design node', async () => {
    const actionBodies: Array<Record<string, unknown>> = []
    const figmalink = 'https://www.figma.com/design/abc123/PM'
    let current = {
      id: 182, template_id: 5, template_name: '需求生命周期', template_version: 4, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['input'], current_nodes: [{ id: 'input', type: 'form', name: '需求录入' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'active' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'pending', config: { require_design_image: true }, allowed_actions: ['supplement_design'] },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [] as Array<{
        id: number; node_id: string; node_name: string; node_type: string; action: 'material_supplemented'; status: string
        operator_type: 'user'; operator_name: string; materials: Array<{ kind: 'design_image'; url: string }>; occurred_at: string
      }>, completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 182, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/182' && config.method === 'get') return response(config, { data: current })
      if (config.url === '/requirements/42/materials' && config.method === 'get') {
        return response(config, { data: [{
          id: 9, requirement_id: 42, kind: 'design', source: 'requirement', name: '收银台效果图', content_type: 'text/uri-list', size: 0,
          url: '/api/v1/platform/requirements/42/materials/9/download',
          origin_url: figmalink,
          artifact_kind: 'uploaded', uploader_user_id: 8, created_at: '2026-08-24T08:00:00Z',
        }] })
      }
      if (config.url === '/pm/workflows/instances/182/actions' && config.method === 'post') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        actionBodies.push(body)
        current = {
          ...current,
          transition_history: [...current.transition_history, {
            id: actionBodies.length, node_id: String(body.node_id), node_name: 'UI/UX设计', node_type: 'human_task',
            action: 'material_supplemented', status: 'supplemented', operator_type: 'user', operator_name: '测试',
            materials: [{ kind: 'design_image', url: String(body.design_url) }], occurred_at: '2026-08-24T08:01:00Z',
          }],
        }
        return response(config, { data: current })
      }
      return response(config, { data: [] })
    }
    renderRoute('/attach-design-library-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openNode('UI/UX设计') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('UI/UX设计') })).toBeTruthy()
    expect(await screen.findByRole('link', { name: copy.aiPM.workflowSubject.designLinkOpenItem(figmalink) })).toBeTruthy()
    expect((screen.getByLabelText(copy.aiPM.workflowSubject.designLinkLabel) as HTMLInputElement).value).toBe('')
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.completeNode })).toBeNull()
    await waitFor(() => expect(actionBodies).toEqual([{ action: 'supplement_design', node_id: 'design', design_url: figmalink }]))
    await waitFor(() => expect((screen.queryByRole('button', { name: copy.aiPM.workflowSubject.attachToNode })) === null).toBe(true))
    expect((screen.getByLabelText(copy.aiPM.workflowSubject.designLinkLabel) as HTMLInputElement).value).toBe('')
  })

  it('echoes a saved Figma material into the node effect-preview link after reopen', async () => {
    const figmalink = 'https://www.figma.com/design/abc123/PM'
    const instance = {
      id: 184, template_id: 5, template_name: '需求生命周期', template_version: 4, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['input'], current_nodes: [{ id: 'input', type: 'form', name: '需求录入' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'active' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'pending', config: { require_design_image: true }, allowed_actions: ['supplement_design'] },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [],
      transition_history: [{
        id: 1, node_id: 'design', node_name: 'UI/UX设计', node_type: 'human_task',
        action: 'material_supplemented', status: 'supplemented', operator_type: 'user', operator_name: '测试',
        materials: [{ kind: 'design_image', url: figmalink }], occurred_at: '2026-08-24T08:01:00Z',
      }],
      completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 184, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/184' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/requirements/42/materials' && config.method === 'get') {
        return response(config, { data: [{
          id: 9, requirement_id: 42, kind: 'design', source: 'workflow', name: 'apps', content_type: 'text/uri-list', size: 0,
          url: '/api/v1/platform/requirements/42/materials/9/download',
          origin_url: figmalink,
          artifact_kind: 'uploaded', uploader_user_id: 8, created_at: '2026-08-24T08:00:00Z',
        }] })
      }
      return response(config, { data: [] })
    }
    renderRoute('/echo-design-link-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} designImageUrl={figmalink} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openNode('UI/UX设计') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('UI/UX设计') })).toBeTruthy()
    expect(await screen.findByRole('link', { name: copy.aiPM.workflowSubject.designLinkOpenItem(figmalink) })).toBeTruthy()
    expect((screen.getByLabelText(copy.aiPM.workflowSubject.designLinkLabel) as HTMLInputElement).value).toBe('')
    expect(screen.queryByRole('link', { name: copy.ui.fileUploader.openAttachment })).toBeNull()
  })

  it('saves multiple effect-preview links from the node materials dialog', async () => {
    const actionBodies: Array<Record<string, unknown>> = []
    const posted: string[] = []
    const first = 'https://www.figma.com/design/abc/apps'
    const second = 'https://www.figma.com/developers/apps111'
    let materials: Array<Record<string, unknown>> = []
    const instance = {
      id: 185, template_id: 5, template_name: '需求生命周期', template_version: 4, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['design'], current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计', config: { require_design_image: true }, allowed_actions: ['complete', 'supplement_design'], can_act: true }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true }, allowed_actions: ['complete', 'supplement_design'], can_act: true },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [] as Array<{
        id: number; node_id: string; node_name: string; node_type: string; action: 'material_supplemented'; status: string
        operator_type: 'user'; operator_name: string; materials: Array<{ kind: 'design_image'; url: string }>; occurred_at: string
      }>, completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 185, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/185' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/requirements/42/materials' && config.method === 'get') return response(config, { data: materials })
      if (config.url === '/requirements/42/materials' && config.method === 'post' && config.data instanceof FormData) {
        const url = String(config.data.get('url') || '')
        posted.push(url)
        const row = {
          id: posted.length, requirement_id: 42, kind: 'design', source: 'workflow', name: url, content_type: 'text/uri-list', size: 0,
          url, origin_url: url, artifact_kind: 'uploaded', uploader_user_id: 8, created_at: '2026-08-25T03:00:00Z',
        }
        materials = [...materials, row]
        return response(config, { data: row })
      }
      if (config.url === '/pm/workflows/instances/185/actions' && config.method === 'post') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        actionBodies.push(body)
        instance.transition_history = [...instance.transition_history, {
          id: actionBodies.length, node_id: String(body.node_id), node_name: 'UI/UX设计', node_type: 'human_task',
          action: 'material_supplemented', status: 'supplemented', operator_type: 'user', operator_name: '测试',
          materials: [{ kind: 'design_image', url: String(body.design_url) }], occurred_at: '2026-08-25T03:01:00Z',
        }]
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }
    renderRoute('/batch-design-links-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('UI/UX设计') }))
    fireEvent.change(await screen.findByLabelText(copy.aiPM.workflowSubject.designLinkLabel), {
      target: { value: `${first}\n${second}` },
    })
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.designLinkSave }))
    await waitFor(() => expect(posted).toEqual([first, second]))
    await waitFor(() => expect(actionBodies).toEqual([
      { action: 'supplement_design', node_id: 'design', design_url: first },
      { action: 'supplement_design', node_id: 'design', design_url: second },
    ]))
    expect(await screen.findByRole('link', { name: copy.aiPM.workflowSubject.designLinkOpenItem(first) })).toBeTruthy()
    expect(screen.getByRole('link', { name: copy.aiPM.workflowSubject.designLinkOpenItem(second) })).toBeTruthy()
    expect((screen.getByLabelText(copy.aiPM.workflowSubject.designLinkLabel) as HTMLInputElement).value).toBe('')
  })

  it('attaches a library download path as an absolute URL when origin_url is missing', async () => {
    const actionBodies: Array<Record<string, unknown>> = []
    const downloadPath = '/api/v1/platform/requirements/42/materials/9/download'
    let current = {
      id: 183, template_id: 5, template_name: '需求生命周期', template_version: 4, status: 'running', subject_type: 'requirement', subject_id: 42,
      current_node_ids: ['input'], current_nodes: [{ id: 'input', type: 'form', name: '需求录入' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'active' },
        { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'pending', config: { require_design_image: true }, allowed_actions: ['supplement_design'] },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [] as Array<{
        id: number; node_id: string; node_name: string; node_type: string; action: 'material_supplemented'; status: string
        operator_type: 'user'; operator_name: string; materials: Array<{ kind: 'design_image'; url: string }>; occurred_at: string
      }>, completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 183, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/183' && config.method === 'get') return response(config, { data: current })
      if (config.url === '/requirements/42/materials' && config.method === 'get') {
        return response(config, { data: [{
          id: 9, requirement_id: 42, kind: 'design', source: 'requirement', name: '收银台效果图', content_type: 'image/png', size: 2048,
          url: downloadPath, artifact_kind: 'uploaded', uploader_user_id: 8, created_at: '2026-08-24T08:00:00Z',
        }] })
      }
      if (config.url === '/pm/workflows/instances/183/actions' && config.method === 'post') {
        const body = JSON.parse(String(config.data)) as Record<string, unknown>
        actionBodies.push(body)
        current = {
          ...current,
          transition_history: [...current.transition_history, {
            id: actionBodies.length, node_id: String(body.node_id), node_name: 'UI/UX设计', node_type: 'human_task',
            action: 'material_supplemented', status: 'supplemented', operator_type: 'user', operator_name: '测试',
            materials: [{ kind: 'design_image', url: String(body.design_url) }], occurred_at: '2026-08-24T08:01:00Z',
          }],
        }
        return response(config, { data: current })
      }
      return response(config, { data: [] })
    }
    renderRoute('/attach-design-download-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openNode('UI/UX设计') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('UI/UX设计') })).toBeTruthy()
    expect(await screen.findByTitle('收银台效果图')).toBeTruthy()
    await waitFor(() => expect(actionBodies).toEqual([{
      action: 'supplement_design',
      node_id: 'design',
      design_url: `${window.location.origin}${downloadPath}`,
    }]))
    await waitFor(() => expect((screen.queryByRole('button', { name: copy.aiPM.workflowSubject.attachToNode })) === null).toBe(true))
  })

  it('keeps a historical AI report inside its completed node', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 81, template_id: 8, template_name: '项目交付流程', template_version: 2, status: 'running', subject_type: 'plan_unit', subject_id: 26,
      current_node_ids: ['acceptance'], current_nodes: [{ id: 'acceptance', type: 'approval', name: '发布验收' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        {
          id: 'risk', type: 'ai_check', name: '历史风险巡检', capability: 'delivery.risk_monitor', status: 'completed', actual_completed_at: '2026-08-07T09:30:00Z',
          ai_execution: { run_id: 32, status: 'completed', output: { summary: '这是历史节点报告。', changes: [], risks: ['历史风险'], items: [], humanReviewRequired: true } },
        },
        { id: 'acceptance', type: 'approval', name: '发布验收', status: 'active' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'risk-acceptance', source: 'risk', target: 'acceptance' }], node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 2, progress: 50, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 81, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/81' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/81/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, timeline_nodes: instance.timeline_nodes.map((node) => node.id === 'risk' ? { ...node, ai_execution: { run_id: 33, status: 'queued' } } : node) } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/historical-ai-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="plan_unit" subjectId={26} />, '/detail')

    const reportNode = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openAIReport('历史风险巡检') })
    expect(reportNode.textContent).toContain(copy.aiPM.workflowSubject.actualCompleted)
    expect(screen.queryByText('这是历史节点报告。')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('发布验收') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('发布验收') })).toBeTruthy()
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.approvalContent })).toBeNull()
    expect(screen.queryByRole('region', { name: copy.aiPM.workflowSubject.aiResultLabel('历史风险巡检') })).toBeNull()
    expect(screen.queryByText('这是历史节点报告。')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.cancel }))
    expect(screen.queryByText('这是历史节点报告。')).toBeNull()
    fireEvent.click(reportNode)
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.aiReportTitle('历史风险巡检') })).toBeTruthy()
    expect(screen.getByText('这是历史节点报告。')).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.aiConfirmAndContinue })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.aiRecheck }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'recheck_ai', node_id: 'risk' }))
  })

  it('keeps single-node project plan editing alongside the shared batch entry', async () => {
    let planBody: Record<string, unknown> | undefined
    let savedPlans: Record<string, { planned_start_at: string; planned_end_at: string }> | undefined
    const instance = {
      id: 91, template_id: 8, template_name: '项目交付流程', template_version: 2, status: 'running', can_manage_node_plans: true, subject_type: 'plan_unit', subject_id: 27,
      current_node_ids: ['delivery'], current_nodes: [{ id: 'delivery', type: 'human_task', name: '研发交付' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'delivery', type: 'human_task', name: '研发交付', status: 'active', planned_start_at: '2026-08-08T00:00:00Z', planned_end_at: '2026-08-09T00:00:00Z' },
        { id: 'acceptance', type: 'approval', name: '发布验收', status: 'pending' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 91, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/91' && config.method === 'get') return response(config, { data: savedPlans ? { ...instance, node_plans: savedPlans, timeline_nodes: instance.timeline_nodes.map((node) => savedPlans?.[node.id] ? { ...node, ...savedPlans[node.id] } : node) } : instance })
      if (config.url === '/pm/workflows/instances/91/node-plans' && config.method === 'patch') {
        planBody = JSON.parse(String(config.data))
        const plans = {
          delivery: { planned_start_at: '2026-08-10T00:00:00Z', planned_end_at: '2026-08-20T00:00:00Z' },
          acceptance: { planned_start_at: '2026-08-21T00:00:00Z', planned_end_at: '2026-08-22T00:00:00Z' },
        }
        savedPlans = plans
        return response(config, { data: { node_plans: plans } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/node-plan-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="plan_unit" subjectId={27} />, '/detail')

    expect(await screen.findByText('8/8–8/9')).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.batchPlanButton })).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.nodePlanUnset)).toBeTruthy()
    const futureNode = screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('发布验收') })
    fireEvent.click(futureNode)
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('发布验收') })).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.approve })).toBeNull()
    await selectDate(copy.aiPM.workflowSubject.nodePlanStartLabel('发布验收'), '2026-08-21')
    await selectDate(copy.aiPM.workflowSubject.nodePlanEndLabel('发布验收'), '2026-08-22')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanSaveOne('发布验收') }))
    await waitFor(() => expect(planBody).toEqual({ nodes: [
      { node_id: 'acceptance', planned_start_at: '2026-08-21', expected_planned_start_at: null, planned_end_at: '2026-08-22', expected_planned_end_at: null },
    ] }))
    await waitFor(() => expect(screen.getByTestId('workflow-node-action-delivery').textContent).toContain('8/10–8/20'))
    expect(screen.getByTestId('workflow-node-action-acceptance').textContent).toContain('8/21–8/22')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openNode('发布验收') }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.nodeDetailTitle('发布验收') })).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanStartLabel('发布验收') }).textContent).toContain('2026年8月21日')
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanEndLabel('发布验收') }).textContent).toContain('2026年8月22日')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.batchPlanButton }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.batchPlanTitle })).toBeTruthy()
    expect(screen.getByTestId('workflow-batch-node-acceptance').textContent).toContain('2026-08-21 - 2026-08-22')
    expect(screen.getByTestId('workflow-batch-node-delivery').textContent).toContain('2026-08-10 - 2026-08-20')
  })

  it('echoes node_plans dates into the node dialog and batch panel even when timeline dates are missing', async () => {
    const instance = {
      id: 92, template_id: 8, template_name: '项目交付流程', template_version: 2, status: 'running', can_manage_node_plans: true, subject_type: 'plan_unit', subject_id: 28,
      current_node_ids: ['delivery'], current_nodes: [{ id: 'delivery', type: 'human_task', name: '研发交付', can_act: false, allowed_actions: [] }],
      node_plans: {
        delivery: { planned_start_at: '2026-08-10T00:00:00Z', planned_end_at: '2026-08-20T00:00:00Z', assignee_user_id: 7 },
        acceptance: { planned_start_at: '2026-08-21T00:00:00Z', planned_end_at: '2026-08-22T00:00:00Z', assignee_user_id: 7 },
      },
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'delivery', type: 'human_task', name: '研发交付', status: 'active', can_act: false, allowed_actions: [], assignee_user_id: 7, assignee_name: '林工' },
        { id: 'acceptance', type: 'approval', name: '发布验收', status: 'pending', can_act: false, allowed_actions: [], assignee_user_id: 7, assignee_name: '林工' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 2, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 92, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/92' && config.method === 'get') return response(config, { data: instance })
      return response(config, { data: [] })
    }
    renderRoute('/node-plan-echo-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="plan_unit" subjectId={28} />, '/detail')

    expect(await screen.findByTestId('workflow-node-action-delivery')).toBeTruthy()
    expect(screen.getByTestId('workflow-node-action-delivery').textContent).toContain('8/10–8/20')
    expect(screen.getByTestId('workflow-node-action-acceptance').textContent).toContain('8/21–8/22')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.openTodo('研发交付') }))
    const dialog = await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('研发交付') })
    expect(within(dialog).getByTestId('workflow-node-readonly')).toBeTruthy()
    expect(within(dialog).getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanStartLabel('研发交付') })).toHaveProperty('disabled', false)
    expect(within(dialog).getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanStartLabel('研发交付') }).textContent).toContain('2026年8月10日')
    expect(within(dialog).getByRole('button', { name: copy.aiPM.workflowSubject.nodePlanEndLabel('研发交付') }).textContent).toContain('2026年8月20日')
    fireEvent.click(within(dialog).getByRole('button', { name: copy.aiPM.common.cancel }))
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.batchPlanButton }))
    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.batchPlanTitle })).toBeTruthy()
    expect(screen.getByTestId('workflow-batch-node-delivery').textContent).toContain('2026-08-10 - 2026-08-20')
    expect(screen.getByTestId('workflow-batch-node-acceptance').textContent).toContain('2026-08-21 - 2026-08-22')
  })

  it('retries a failed active AI node without taking its failure branch', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 82, template_id: 8, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 44,
      current_node_ids: ['check'], current_nodes: [{ id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness' }],
      timeline_nodes: [{
        id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness', status: 'active',
        ai_execution: { run_id: 46, status: 'failed', error_message: 'AI runtime returned status 502' },
      }],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 3, progress: 33, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 82, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/82' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/82/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, timeline_nodes: [{ ...instance.timeline_nodes[0], ai_execution: { run_id: 47, status: 'queued' } }] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/failed-ai-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={44} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openAIReport('完整性检查') }))
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.aiStart }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'retry_ai', node_id: 'check' }))
  })

  it('keeps polling a newly started workflow AI node after the first refresh fails', async () => {
    let instanceReads = 0
    let actionBody: Record<string, unknown> | undefined
    const initialInstance = {
      id: 83, template_id: 8, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 44,
      current_node_ids: ['check'], current_nodes: [{ id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness' }],
      timeline_nodes: [{ id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness', status: 'active' }],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 0, total_steps: 1, progress: 0, started_at: '2026-08-07T00:00:00Z',
    }
    const queuedInstance = {
      ...initialInstance,
      timeline_nodes: [{ ...initialInstance.timeline_nodes[0], ai_execution: { run_id: 48, status: 'queued' } }],
    }
    const completedInstance = {
      ...initialInstance,
      timeline_nodes: [{ ...initialInstance.timeline_nodes[0], ai_execution: {
        run_id: 48,
        status: 'completed',
        output: { summary: '刷新失败后已恢复轮询。', changes: [], items: [], risks: [], humanReviewRequired: true },
      } }],
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 83, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/83' && config.method === 'get') {
        instanceReads += 1
        if (instanceReads === 1) return response(config, { data: initialInstance })
        if (instanceReads === 2) throw new Error('transient workflow refresh failure')
        return response(config, { data: completedInstance })
      }
      if (config.url === '/pm/workflows/instances/83/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: queuedInstance })
      }
      return response(config, { data: [] })
    }
    renderRoute('/transient-ai-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={44} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openTodo('完整性检查') }))
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.aiStart }))

    await waitFor(() => expect(actionBody).toEqual({ action: 'complete', node_id: 'check' }))
    expect(await screen.findByText('刷新失败后已恢复轮询。', {}, { timeout: 3_500 })).toBeTruthy()
    expect(instanceReads).toBeGreaterThanOrEqual(3)
  })

  it('renders a wide six-dimension completeness report and allows an explicit low-score override', async () => {
    let actionBody: Record<string, unknown> | undefined
    const dimensions = [
      ['background', '背景', 15], ['goal', '目标', 15], ['scope', '范围', 15], ['boundaries', '边界', 10],
      ['exceptions', '异常场景', 15], ['dependencies', '依赖', 10], ['acceptance_criteria', '验收标准', 20],
    ].map(([key, label, maxScore]) => ({ key, label, status: 'missing', score: 0, maxScore, findings: [`${label}信息缺失`], evidence: [], remediation: [`补充${label}`] }))
    const instance = {
      id: 80, template_id: 5, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 43,
      current_node_ids: ['check'], current_nodes: [{ id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness' }],
      timeline_nodes: [{
        id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness', status: 'active',
        ai_execution: { run_id: 45, status: 'completed', output: {
          summary: '完整性得分 0 分，存在阻断项，不能进入产品评审。', changes: [], items: [], risks: ['验收标准缺失'], humanReviewRequired: true,
          completenessReport: { verdict: 'blocked', score: 0, threshold: 80, canProceed: false, checkedSubjectVersion: 6, dimensions, missingRequiredFields: ['acceptance_criteria'], blockingIssues: ['验收标准缺失'], recommendations: ['补充可验证的 Given/When/Then 验收标准'] },
        } },
      }],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 12, progress: 8, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 80, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/80' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/80/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, current_node_ids: ['input'], current_nodes: [{ id: 'input', type: 'form', name: '需求录入' }] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/strict-completeness-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={43} />, '/detail')

    const completenessNode = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openAIReport('完整性检查') })
    expect(screen.queryByTestId('workflow-completeness-report')).toBeNull()
    fireEvent.click(completenessNode)
    expect(await screen.findByTestId('workflow-completeness-report')).toBeTruthy()
    expect(screen.getByRole('dialog').className).toContain('max-w-6xl')
    expect(screen.getByText(copy.aiPM.workflowSubject.completenessBlocked)).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.completenessThreshold(80))).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.completenessRadarTitle)).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.completenessRadarDescription)).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.completenessManualOverrideHint)).toBeTruthy()
    expect(screen.getByText('验收标准缺失')).toBeTruthy()
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.aiAcceptRiskAndContinue }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'override_ai', node_id: 'check', comment: copy.aiPM.workflowSubject.aiOverrideComment }))
  })

  it('blocks completeness continue actions until a stale report is rechecked', async () => {
    let actionBody: Record<string, unknown> | undefined
    const dimensions = [
      ['background', '背景', 15], ['goal', '目标', 15], ['scope', '范围', 15], ['boundaries', '边界', 10],
      ['exceptions', '异常场景', 15], ['dependencies', '依赖', 10], ['acceptance_criteria', '验收标准', 20],
    ].map(([key, label, maxScore]) => ({ key, label, status: 'missing', score: 0, maxScore, findings: [`${label}信息缺失`], evidence: [], remediation: [`补充${label}`] }))
    const instance = {
      id: 80, template_id: 5, template_name: '需求生命周期', template_version: 3, status: 'running', subject_type: 'requirement', subject_id: 43,
      current_node_ids: ['check'], current_nodes: [{ id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness' }],
      timeline_nodes: [{
        id: 'check', type: 'ai_check', name: '完整性检查', capability: 'requirement.completeness', status: 'active',
        allowed_actions: ['complete', 'recheck_ai', 'override_ai'],
        ai_execution: { run_id: 45, status: 'completed', subject_version: 4, output: {
          summary: '完整性得分 22 分，存在阻断项，不能进入产品评审。', changes: [], items: [], risks: ['验收标准缺失'], humanReviewRequired: true,
          completenessReport: { verdict: 'blocked', score: 22, threshold: 80, canProceed: false, checkedSubjectVersion: 4, dimensions, missingRequiredFields: ['acceptance_criteria'], blockingIssues: ['验收标准缺失'], recommendations: ['补充可验证的 Given/When/Then 验收标准'] },
        } },
      }],
      definition_edges: [], node_runs: [], approvals: [], transition_history: [], completed_steps: 1, total_steps: 12, progress: 8, started_at: '2026-08-07T00:00:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 80, status: 'running' }], total: 1 })
      if (config.url === '/pm/workflows/instances/80' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/80/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: instance })
      }
      return response(config, { data: [] })
    }
    renderRoute('/stale-completeness-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={43} requirement={requirementFixture({ id: 43, version: 5, workflow_instance_id: 80 })} />, '/detail')

    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.openAIReport('完整性检查') }))
    expect(await screen.findByText(copy.aiPM.workflowSubject.completenessStaleHint)).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.aiAcceptRiskAndContinue })).toHaveProperty('disabled', true)
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.aiConfirmAndReturn })).toHaveProperty('disabled', true)
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.aiRecheck }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'recheck_ai', node_id: 'check' }))
  })

  it('offers an audited recovery action for a legacy rejected workflow', async () => {
    let actionBody: Record<string, unknown> | undefined
    const instance = {
      id: 78,
      template_id: 5,
      template_name: '需求生命周期',
      template_version: 3,
      status: 'rejected',
      subject_type: 'requirement',
      subject_id: 43,
      current_node_ids: [],
      current_nodes: [],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'input', type: 'form', name: '需求录入', status: 'completed' },
        { id: 'review', type: 'approval', name: '产品评审', status: 'rejected' },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [{ id: 'reject-review', source: 'review', target: 'input', kind: 'reject', loop: true }],
      node_runs: [],
      approvals: [],
      transition_history: [],
      completed_steps: 2,
      total_steps: 3,
      progress: 66,
      started_at: '2026-08-07T00:00:00Z',
      completed_at: '2026-08-07T00:20:00Z',
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [], total: 0 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: 78, status: 'rejected' }], total: 1 })
      if (config.url === '/pm/workflows/instances/78' && config.method === 'get') return response(config, { data: instance })
      if (config.url === '/pm/workflows/instances/78/actions' && config.method === 'post') {
        actionBody = JSON.parse(String(config.data))
        return response(config, { data: { ...instance, status: 'running', current_node_ids: ['input'] } })
      }
      return response(config, { data: [] })
    }
    renderRoute('/legacy-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={43} />, '/detail')
    expect(await screen.findByText(copy.aiPM.workflowSubject.resumeTitle)).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.resumeDescription)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.resumeAction }))
    await waitFor(() => expect(actionBody).toEqual({ action: 'resume' }))
  })

  it('does not allow a project workflow template to be selected after creation', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates') return response(config, { data: [{ id: 8, name: '版本交付流程', subject_type: 'plan_unit', published_version: 2, updated_at: '2026-08-07T00:00:00Z' }], total: 1 })
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }
    renderRoute('/project-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="plan_unit" subjectId={25} />, '/detail')
    expect(await screen.findByText(copy.aiPM.workflowSubject.planUnitTemplateFixedTitle)).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.planUnitTemplateFixedDescription)).toBeTruthy()
    expect(screen.queryByLabelText(copy.aiPM.workflowSubject.selectLabel)).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.start })).toBeNull()
  })

  it('does not allow a requirement workflow template to be selected after creation', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }
    renderRoute('/requirement-workflow-card', () => <WorkflowSubjectLauncher workspaceId={7} subjectType="requirement" subjectId={42} />, '/detail')
    expect(await screen.findByText(copy.aiPM.workflowSubject.requirementTemplateFixedTitle)).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.requirementTemplateFixedDescription)).toBeTruthy()
    expect(screen.queryByLabelText(copy.aiPM.workflowSubject.selectLabel)).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.start })).toBeNull()
  })

  it('launches the workspace daily brief from the AI run center', async () => {
    let createBody: Record<string, unknown> | undefined
    const run = { id: 12, capability: 'inbox.daily_brief', subject_type: 'workspace', subject_id: 7, subject_version: 1, status: 'queued', model: 'qwen3.6-max-preview', created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: run.model, capabilities: [{ key: run.capability, name: '个人工作简报', description: '汇总待办和风险', subject_types: ['workspace'], allowed_fields: [], mode: 'artifact' }] } })
      if (config.url === '/ai/runs' && config.method === 'post') { createBody = JSON.parse(String(config.data)); return response(config, { data: run }) }
      if (config.url === `/ai/runs/${run.id}`) return response(config, { data: run })
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><AIRunsPage /></QueryClientProvider>)
    const dailyBriefButton = await screen.findByRole('button', { name: '生成今日简报' })
    await waitFor(() => expect(dailyBriefButton).toHaveProperty('disabled', false))
    fireEvent.click(dailyBriefButton)
    await waitFor(() => expect(createBody).toEqual({ capability: 'inbox.daily_brief', subject_type: 'workspace', subject_id: 7 }))
    expect(await screen.findByText('AI 运行 #12')).toBeTruthy()
  })

  it('creates a workflow from the safe human-approval skeleton', async () => {
    let definition: { nodes?: Array<{ type: string; name?: string; config?: { critical_gate?: boolean; test_report_artifact?: boolean; assignee_user_id?: number } }> } | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates' && config.method === 'post') { const body = JSON.parse(String(config.data)); definition = body.definition; return response(config, { data: { id: 5, name: body.name, subject_type: body.subject_type, definition: body.definition, published_version: 0, updated_at: new Date().toISOString() } }) }
      return response(config, { data: [], total: 0 })
    }
    renderRoute('/workspace/pm/workflows', WorkflowListPage, '/workspace/pm/workflows/$templateId')
    fireEvent.click(await screen.findByRole('button', { name: '新建模板' }))
    fireEvent.change(screen.getByLabelText('模板名称'), { target: { value: '需求评审流' } })
    fireEvent.click(screen.getByRole('button', { name: '创建并编排' }))
    await waitFor(() => expect(definition?.nodes?.some((node) => node.type === 'approval' && node.config?.critical_gate)).toBe(true))
    expect(definition?.nodes?.some((node) => node.name === '测试验收' && node.config?.test_report_artifact)).toBeFalsy()
    expect(definition?.nodes?.every((node) => node.config?.assignee_user_id === undefined)).toBe(true)
  })

  it('keeps workflow implementation codes out of the editor canvas', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'choice', type: 'condition', name: '是否需要 UI 设计' },
        { id: 'risk', type: 'ai_check', name: '风险智能巡检', capability: 'delivery.risk_monitor' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'choice', kind: 'success' },
        { id: 'e2', source: 'choice', target: 'risk', condition: 'subject.needs_ui == true' },
        { id: 'e3', source: 'choice', target: 'end', condition: 'subject.needs_ui == false' },
        { id: 'e4', source: 'risk', target: 'end', kind: 'failure' },
      ],
    }
    const template = { id: 5, name: '业务可读流程', description: '', subject_type: 'requirement', definition, published_version: 1, updated_at: new Date().toISOString() }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen3.6-max-preview', capabilities: [] } })
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }

    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '业务可读流程' })).toBeTruthy()
    const canvas = screen.getByRole('region', { name: copy.aiPM.workflow.canvas })
    await waitFor(() => expect(canvas.textContent).toContain(copy.aiPM.workflow.aiNode))
    expect(canvas.textContent).toContain(copy.aiPM.workflow.condition)
    expect(canvas.textContent).not.toContain('delivery.risk_monitor')
    expect(canvas.textContent).not.toContain('subject.needs_ui')
    expect(canvas.textContent).not.toContain('ai_check')
    expect(canvas.textContent).not.toContain('success')
  })

  it('publishes a saved workflow directly without rendering a compile-validation action', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    let publishCalls = 0
    const template = { id: 5, name: '直接发布流程', description: '', subject_type: 'requirement', definition, published_version: 0, updated_at: new Date().toISOString() }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/pm/workflows/templates/5/publish' && config.method === 'post') { publishCalls += 1; return response(config, { data: { template_id: 5, version: 1, definition_hash: 'hash' } }) }
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen3.6-max-preview', capabilities: [] } })
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }
    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '直接发布流程' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: '编译校验' })).toBeNull()
    expect(screen.getByRole('button', { name: copy.aiPM.workflow.startNode })).toHaveProperty('disabled', true)
    expect(screen.getByRole('button', { name: copy.aiPM.workflow.endNode })).toHaveProperty('disabled', true)
    const publishButton = screen.getByRole('button', { name: copy.aiPM.workflow.publish })
    expect(publishButton).toHaveProperty('disabled', false)
    fireEvent.click(publishButton)
    await waitFor(() => expect(publishCalls).toBe(1))
  })

  it('uses one AI node for every opened workflow capability and keeps test-case generation out', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'review', type: 'approval', name: '产品评审', config: { critical_gate: true, assignee_user_id: 7 } },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'review' },
        { id: 'e2', source: 'review', target: 'end', kind: 'approve' },
      ],
    }
    let savedDefinition: WorkflowDefinition | undefined
    let userDirectoryCalls = 0
    const template = { id: 5, name: '语义能力流程', description: '', subject_type: 'requirement', definition, published_version: 0, updated_at: new Date().toISOString() }
    coreApi.defaults.adapter = async (config) => {
      if (String(config.url ?? '').endsWith('/access/me/action-permissions')) return originalCoreAdapter(config)
      if (config.url === '/directory/users') userDirectoryCalls += 1
      return response(config, { data: [], total: 0 })
    }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/pm/workflows/templates/5' && config.method === 'patch') {
        const body = JSON.parse(String(config.data)) as { definition: WorkflowDefinition }
        savedDefinition = body.definition
        return response(config, { data: { ...template, definition: body.definition, updated_at: new Date(Date.now() + 1000).toISOString() } })
      }
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen3.6-max-preview', capabilities: [
        { key: 'requirement.completeness', name: '完整性检查', description: '检查背景、边界、异常和验收条件', subject_types: ['requirement'], workflow_subject_types: ['requirement'], allowed_fields: null, mode: 'analysis' },
        { key: 'requirement.duplicate_check', name: '重复需求检查', description: '对需求池进行语义重复检查', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'analysis' },
        { key: 'delivery.risk_monitor', name: '交付风险', description: '识别延期、阻塞和范围漂移风险', subject_types: ['requirement', 'plan_unit'], workflow_subject_types: ['plan_unit'], allowed_fields: [], mode: 'analysis' },
        { key: 'requirement.task_plan', name: '任务拆解', description: '生成任务、依赖和角色草稿', subject_types: ['requirement'], workflow_subject_types: ['requirement'], allowed_fields: [], mode: 'artifact' },
        { key: 'requirement.review_pack', name: '评审材料包', description: '汇总评审议题、风险和待决项', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        { key: 'meeting.extract_decisions', name: '会议决策提取', description: '提取结论和待办', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        { key: 'requirement.acceptance_cases', name: '验收用例', description: '生成验收用例', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        { key: 'delivery.test_handoff', name: '提测说明', description: '生成提测说明', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        { key: 'delivery.fix_plan', name: '修复计划', description: '生成修复计划', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'proposal' },
        { key: 'qa.result_summary', name: '测试总结', description: '汇总测试结果', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        { key: 'verification.aggregate', name: '验收汇总', description: '汇总验收证据', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        { key: 'requirement.draft', name: '需求草拟', description: '生成需求字段', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: ['title'], mode: 'proposal' },
        { key: 'qa.test_cases', name: 'AI 测试用例', description: '生成测试用例附件', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        { key: 'workflow.generate', name: '工作流生成', description: '生成模板', subject_types: ['requirement', 'workflow_template'], workflow_subject_types: [], allowed_fields: [], mode: 'proposal' },
      ] } })
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }

    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '语义能力流程' })).toBeTruthy()
    expect(userDirectoryCalls).toBe(0)
    expect(screen.queryByText('产品阿岚')).toBeNull()
    expect(screen.queryByRole('combobox', { name: copy.aiPM.workflow.nodeAssignee })).toBeNull()
    for (const hiddenNode of [copy.aiPM.workflow.form, copy.aiPM.workflow.timer, copy.aiPM.workflow.outbox, copy.aiPM.workflow.createDrafts]) {
      expect(screen.queryByRole('button', { name: hiddenNode })).toBeNull()
    }

    const aiNodeButton = await screen.findByRole('button', { name: copy.aiPM.workflow.aiNode })
    expect(screen.queryByRole('button', { name: copy.aiPM.workflow.aiCheck })).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflow.aiGenerate })).toBeNull()
    fireEvent.click(aiNodeButton)
    const capabilitySelect = await screen.findByRole('combobox', { name: copy.aiPM.workflow.aiCapability })
    await waitFor(() => expect(capabilitySelect.textContent).toContain('完整性检查'))
    expect(screen.getByText('检查背景、边界、异常和验收条件')).toBeTruthy()
    fireEvent.click(capabilitySelect)
    const completenessOption = await screen.findByRole('option', { name: '完整性检查' })
    expect(screen.getByRole('option', { name: '任务拆解' })).toBeTruthy()
    expect(screen.queryByRole('option', { name: '重复需求检查' })).toBeNull()
    expect(screen.queryByRole('option', { name: '交付风险' })).toBeNull()
    expect(screen.queryByRole('option', { name: '需求草拟' })).toBeNull()
    expect(screen.queryByRole('option', { name: 'AI 测试用例' })).toBeNull()
    expect(screen.queryByRole('option', { name: '工作流生成' })).toBeNull()
    fireEvent.pointerDown(completenessOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(completenessOption)

    fireEvent.click(aiNodeButton)
    const generateSelect = await screen.findByRole('combobox', { name: copy.aiPM.workflow.aiCapability })
    fireEvent.click(generateSelect)
    const taskPlanOption = await screen.findByRole('option', { name: '任务拆解' })
    for (const unopenedCapability of ['评审材料包', '会议决策提取', '验收用例', '提测说明', '修复计划', '测试总结', '验收汇总']) {
      expect(screen.queryByRole('option', { name: unopenedCapability })).toBeNull()
    }
    expect(screen.getByRole('option', { name: '完整性检查' })).toBeTruthy()
    expect(screen.queryByRole('option', { name: 'AI 测试用例' })).toBeNull()
    fireEvent.pointerDown(taskPlanOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(taskPlanOption)
    expect(await screen.findByText('生成任务、依赖和角色草稿')).toBeTruthy()

    fireEvent.click(generateSelect)
    const switchedBackOption = await screen.findByRole('option', { name: '完整性检查' })
    fireEvent.pointerDown(switchedBackOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(switchedBackOption)
    expect(await screen.findByText('检查背景、边界、异常和验收条件')).toBeTruthy()

    fireEvent.click(aiNodeButton)
    const finalCapabilitySelect = await screen.findByRole('combobox', { name: copy.aiPM.workflow.aiCapability })
    fireEvent.click(finalCapabilitySelect)
    const finalTaskPlanOption = await screen.findByRole('option', { name: '任务拆解' })
    fireEvent.pointerDown(finalTaskPlanOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(finalTaskPlanOption)

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflow.saveDraft }))
    await waitFor(() => expect(savedDefinition?.nodes.filter((node) => node.type === 'ai_check' && node.capability === 'requirement.completeness')).toHaveLength(2))
    expect(savedDefinition?.nodes.filter((node) => node.type === 'ai_generate' && node.capability === 'requirement.task_plan')).toHaveLength(1)
    expect(savedDefinition?.nodes.every((node) => node.config?.assignee_user_id === undefined)).toBe(true)
  })

  it('shows one AI node with only workflow-open plan-unit capabilities', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const template = { id: 5, name: '版本交付流程', description: '', subject_type: 'plan_unit', definition, published_version: 0, updated_at: new Date().toISOString() }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [
        { key: 'delivery.risk_monitor', name: '交付风险', description: '识别交付风险', subject_types: ['requirement', 'plan_unit'], workflow_subject_types: ['plan_unit'], allowed_fields: [], mode: 'analysis' },
        { key: 'version.scope_proposal', name: '版本范围建议', description: '生成版本范围建议', subject_types: ['plan_unit'], workflow_subject_types: [], allowed_fields: [], mode: 'proposal' },
      ] } })
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }

    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '版本交付流程' })).toBeTruthy()
    const aiNodeButton = await screen.findByRole('button', { name: copy.aiPM.workflow.aiNode })
    fireEvent.click(aiNodeButton)
    const capabilitySelect = await screen.findByRole('combobox', { name: copy.aiPM.workflow.aiCapability })
    expect(capabilitySelect.textContent).toContain('交付风险')
    fireEvent.click(capabilitySelect)
    expect(await screen.findByRole('option', { name: '交付风险' })).toBeTruthy()
    expect(screen.queryByRole('option', { name: '版本范围建议' })).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflow.aiCheck })).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflow.aiGenerate })).toBeNull()
  })

  it('shows a recoverable capability-directory error for an empty workflow template', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const template = { id: 5, name: '空白需求流程', description: '', subject_type: 'requirement', definition, published_version: 0, updated_at: new Date().toISOString() }
    let capabilityCalls = 0
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/ai/capabilities') {
        capabilityCalls += 1
        if (capabilityCalls === 1) throw new Error('capability directory unavailable')
        return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [
          { key: 'requirement.completeness', name: '完整性检查', description: '检查需求完整性', subject_types: ['requirement'], workflow_subject_types: ['requirement'], allowed_fields: [], mode: 'analysis' },
          { key: 'requirement.task_plan', name: '任务拆解', description: '拆解研发任务', subject_types: ['requirement'], workflow_subject_types: ['requirement'], allowed_fields: [], mode: 'artifact' },
        ] } })
      }
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }

    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '空白需求流程' })).toBeTruthy()
    const directoryError = await screen.findByRole('alert')
    expect(within(directoryError).getByText(copy.aiPM.workflow.aiCapabilityLoadFailed)).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflow.aiNode })).toBeNull()
    fireEvent.click(within(directoryError).getByRole('button', { name: copy.ui.retry }))
    expect(await screen.findByRole('button', { name: copy.aiPM.workflow.aiNode })).toBeTruthy()
  })

  it('keeps a historical AI capability readable when the directory fails and excludes it after retry', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'legacy-ai', type: 'ai_generate', name: '历史评审助手', capability: 'requirement.review_pack' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [{ id: 'e1', source: 'start', target: 'legacy-ai' }, { id: 'e2', source: 'legacy-ai', target: 'end' }],
    }
    const template = { id: 5, name: '历史 AI 流程', description: '', subject_type: 'requirement', definition, published_version: 1, updated_at: new Date().toISOString() }
    let capabilityCalls = 0
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/ai/capabilities') {
        capabilityCalls += 1
        if (capabilityCalls === 1) throw new Error('capability directory unavailable')
        return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [
          { key: 'requirement.task_plan', name: '任务拆解', description: '生成任务、依赖和角色草稿', subject_types: ['requirement'], workflow_subject_types: ['requirement'], allowed_fields: [], mode: 'artifact' },
          { key: 'requirement.review_pack', name: '评审材料包', description: '汇总评审材料', subject_types: ['requirement'], workflow_subject_types: [], allowed_fields: [], mode: 'artifact' },
        ] } })
      }
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }

    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '历史 AI 流程' })).toBeTruthy()
    fireEvent.click(await within(screen.getByRole('region', { name: copy.aiPM.workflow.canvas })).findByText('历史评审助手'))
    const directoryError = await screen.findByRole('alert')
    expect(within(directoryError).getByText(copy.aiPM.workflow.aiCapabilityLoadFailed)).toBeTruthy()
    expect(within(directoryError).getByText(copy.aiPM.workflow.aiCapabilityCurrent('历史评审助手'))).toBeTruthy()
    fireEvent.click(within(directoryError).getByRole('button', { name: copy.ui.retry }))

    expect(await screen.findByText(copy.aiPM.workflow.aiCapabilityLegacy('评审材料包'))).toBeTruthy()
    const capabilitySelect = screen.getByRole('combobox', { name: copy.aiPM.workflow.aiCapability })
    expect(capabilitySelect.textContent).toContain(copy.aiPM.workflow.aiCapabilityOpenedPlaceholder)
    fireEvent.click(capabilitySelect)
    expect(await screen.findByRole('option', { name: '任务拆解' })).toBeTruthy()
    expect(screen.queryByRole('option', { name: '评审材料包' })).toBeNull()
  })

  it('keeps historical hidden nodes on the canvas and preserves them when saving', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [
        { id: 'start', type: 'start', name: '开始' },
        { id: 'legacy-form', type: 'form', name: '历史表单', config: { requirement_input: true, assignee_user_id: 7 } },
        { id: 'legacy-timer', type: 'timer', name: '历史计时器', config: { delay_seconds: 3600 } },
        { id: 'legacy-outbox', type: 'outbox_action', name: '历史外部动作' },
        { id: 'legacy-drafts', type: 'create_drafts', name: '历史草稿动作' },
        { id: 'end', type: 'end', name: '结束' },
      ],
      edges: [
        { id: 'e1', source: 'start', target: 'legacy-form' },
        { id: 'e2', source: 'legacy-form', target: 'legacy-timer' },
        { id: 'e3', source: 'legacy-timer', target: 'legacy-outbox' },
        { id: 'e4', source: 'legacy-outbox', target: 'legacy-drafts' },
        { id: 'e5', source: 'legacy-drafts', target: 'end' },
      ],
    }
    let savedDefinition: WorkflowDefinition | undefined
    const template = { id: 5, name: '历史兼容流程', description: '', subject_type: 'requirement', definition, published_version: 1, updated_at: new Date().toISOString() }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/pm/workflows/templates/5' && config.method === 'patch') {
        const body = JSON.parse(String(config.data)) as { definition: WorkflowDefinition }
        savedDefinition = body.definition
        return response(config, { data: { ...template, definition: body.definition, updated_at: new Date(Date.now() + 1000).toISOString() } })
      }
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen3.6-max-preview', capabilities: [] } })
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }

    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '历史兼容流程' })).toBeTruthy()
    const canvas = screen.getByRole('region', { name: copy.aiPM.workflow.canvas })
    for (const name of ['历史表单', '历史计时器', '历史外部动作', '历史草稿动作']) {
      expect(within(canvas).getByText(name)).toBeTruthy()
    }
    for (const hiddenNode of [copy.aiPM.workflow.form, copy.aiPM.workflow.timer, copy.aiPM.workflow.outbox, copy.aiPM.workflow.createDrafts]) {
      expect(screen.queryByRole('button', { name: hiddenNode })).toBeNull()
    }

    fireEvent.click(within(canvas).getByText('历史表单'))
    fireEvent.change(await screen.findByLabelText(copy.aiPM.workflow.nodeName), { target: { value: '历史表单保留' } })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflow.saveDraft }))
    await waitFor(() => expect(savedDefinition?.nodes.find((node) => node.id === 'legacy-form')?.name).toBe('历史表单保留'))
    expect(savedDefinition?.nodes.map((node) => node.type)).toEqual(['start', 'form', 'timer', 'outbox_action', 'create_drafts', 'end'])
    expect(savedDefinition?.nodes.find((node) => node.id === 'legacy-form')?.config).toEqual({ requirement_input: true })
    expect(savedDefinition?.nodes.find((node) => node.id === 'legacy-timer')?.config).toEqual({ delay_seconds: 3600 })
  })

  it('lets a missing singleton end node be added once and saved from the catalog', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    const definition: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'work', type: 'human_task', name: '执行' }],
      edges: [{ id: 'e1', source: 'start', target: 'work' }],
    }
    let savedDefinition: WorkflowDefinition | undefined
    const template = { id: 5, name: '补齐结束节点', description: '', subject_type: 'plan_unit', definition, published_version: 0, updated_at: new Date().toISOString() }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/templates/5' && config.method === 'get') return response(config, { data: template })
      if (config.url === '/pm/workflows/templates/5' && config.method === 'patch') {
        const body = JSON.parse(String(config.data)) as { definition: WorkflowDefinition }
        savedDefinition = body.definition
        return response(config, { data: { ...template, definition: body.definition, updated_at: new Date(Date.now() + 1000).toISOString() } })
      }
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen3.6-max-preview', capabilities: [] } })
      if (config.url === '/ai/runs') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    }
    renderWorkflowEditorRoute()
    expect(await screen.findByRole('heading', { name: '补齐结束节点' })).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.workflow.startNode })).toHaveProperty('disabled', true)
    const endButton = screen.getByRole('button', { name: copy.aiPM.workflow.endNode })
    expect(endButton).toHaveProperty('disabled', false)
    fireEvent.click(endButton)
    expect(endButton).toHaveProperty('disabled', true)
    const saveButton = screen.getByRole('button', { name: copy.aiPM.workflow.saveDraft })
    expect(saveButton).toHaveProperty('disabled', false)
    fireEvent.click(saveButton)
    await waitFor(() => expect(savedDefinition?.nodes.filter((node) => node.type === 'end')).toHaveLength(1))
  })

  it('sends the latest canvas to AI, applies the proposal, and saves without validation', async () => {
    const current: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const proposal: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'review', type: 'approval', name: '人工审批', config: { critical_gate: true } }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'review' }, { id: 'e2', source: 'review', target: 'end', kind: 'approve' }],
    }
    let createBody: Record<string, unknown> | undefined
    let applied: WorkflowDefinition | undefined
    let validationCalls = 0
    const completedRun = { id: 21, capability: 'workflow.generate', subject_type: 'workflow_template', subject_id: 5, subject_version: 1, status: 'completed', model: 'qwen3.6-max-preview', output: { summary: '已生成人工审批流', changes: [], items: [], risks: [], humanReviewRequired: true, workflowProposal: { mode: 'replace', explanation: '在开始和结束间增加人工门禁', warnings: [], definition: proposal } }, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: completedRun.model, capabilities: [{ key: 'workflow.generate', name: '工作流生成', description: '生成流程', subject_types: ['workflow_template'], allowed_fields: [], mode: 'proposal' }] } })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') { createBody = JSON.parse(String(config.data)); return response(config, { data: completedRun }) }
      if (config.url === '/ai/runs/21') return response(config, { data: completedRun })
      if (config.url === '/pm/workflows/templates/5/validate-definition') validationCalls += 1
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowAIAssistant workspaceId={7} templateId={5} definition={current} dirty onApply={async (value) => { applied = value }} /></QueryClientProvider>)
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflow.promptLabel), { target: { value: '增加人工审批' } })
    const generateButton = screen.getByRole('button', { name: copy.aiPM.workflow.updateCanvas })
    await waitFor(() => expect(generateButton).toHaveProperty('disabled', false))
    fireEvent.click(generateButton)
    await waitFor(() => expect(createBody).toMatchObject({ capability: 'workflow.generate', user_prompt: '增加人工审批', workflow_definition: current }))
    expect(await screen.findByText(copy.aiPM.workflow.changePreview)).toBeTruthy()
    await waitFor(() => expect(applied).toEqual(proposal))
    expect(await screen.findByText(copy.aiPM.workflow.autoSaved)).toBeTruthy()
    expect(validationCalls).toBe(0)
    expect(screen.getByText('1', { selector: 'strong' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /应用到画布/ })).toBeNull()
  })

  it('keeps the applied canvas and reports an automatic save failure', async () => {
    const current: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const proposal: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [],
    }
    const completedRun = { id: 26, capability: 'workflow.generate', subject_type: 'workflow_template', subject_id: 5, subject_version: 1, status: 'completed', model: 'qwen3.6-max-preview', output: { summary: '生成了无效流程', changes: [], items: [], risks: [], humanReviewRequired: true, workflowProposal: { mode: 'replace', explanation: '缺少开始到结束的连线', warnings: [], definition: proposal } }, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    const onApply = vi.fn(async () => { throw new Error('save failed') })
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: completedRun.model, capabilities: [{ key: 'workflow.generate', name: '工作流生成', description: '生成流程', subject_types: ['workflow_template'], allowed_fields: [], mode: 'proposal' }] } })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: completedRun })
      if (config.url === '/ai/runs/26') return response(config, { data: completedRun })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowAIAssistant workspaceId={7} templateId={5} definition={current} dirty={false} onApply={onApply} /></QueryClientProvider>)
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflow.promptLabel), { target: { value: '生成无效流程' } })
    const generateButton = screen.getByRole('button', { name: copy.aiPM.workflow.generateDraft })
    await waitFor(() => expect(generateButton).toHaveProperty('disabled', false))
    fireEvent.click(generateButton)
    expect(await screen.findByText(copy.aiPM.workflow.autoSaveFailed)).toBeTruthy()
    expect(onApply).toHaveBeenCalledWith(proposal)
  })

  it('reports a semantic no-op without pretending the canvas was applied', async () => {
    const current: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'review', type: 'approval', name: '人工评审', config: { critical_gate: true } }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'review', loop: false }, { id: 'e2', source: 'review', target: 'end' }],
    }
    const equivalent: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始', config: {} }, { id: 'review', type: 'approval', name: '人工评审', capability: '', config: { critical_gate: true, delay_seconds: null } }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'review' }, { id: 'e2', source: 'review', target: 'end', kind: '', condition: '', loop: false }],
    }
    const completedRun = { id: 29, capability: 'workflow.generate', subject_type: 'workflow_template', subject_id: 5, subject_version: 1, status: 'completed', model: 'qwen3.6-max-preview', output: { summary: '保留现有流程', changes: [], items: [], risks: [], humanReviewRequired: true, workflowProposal: { mode: 'replace', explanation: '当前流程已经完整', warnings: [], definition: equivalent } }, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    const onApply = vi.fn(async () => undefined)
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: completedRun.model, capabilities: [{ key: 'workflow.generate', name: '工作流生成', description: '生成流程', subject_types: ['workflow_template'], allowed_fields: [], mode: 'proposal' }] } })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: completedRun })
      if (config.url === '/ai/runs/29') return response(config, { data: completedRun })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowAIAssistant workspaceId={7} templateId={5} definition={current} dirty={false} onApply={onApply} /></QueryClientProvider>)
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflow.promptLabel), { target: { value: '生成需求工作流' } })
    const generateButton = screen.getByRole('button', { name: copy.aiPM.workflow.generateDraft })
    await waitFor(() => expect(generateButton).toHaveProperty('disabled', false))
    fireEvent.click(generateButton)
    expect(await screen.findByText(copy.aiPM.workflow.noChanges)).toBeTruthy()
    expect(onApply).not.toHaveBeenCalled()
    expect(screen.queryByText(copy.aiPM.workflow.autoSaved)).toBeNull()
  })

  it('does not reapply a historical completed run when the editor opens', async () => {
    const current: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const historicalProposal: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'review', type: 'approval', name: '旧审批' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'review' }, { id: 'e2', source: 'review', target: 'end' }],
    }
    const completedRun = { id: 27, capability: 'workflow.generate', subject_type: 'workflow_template', subject_id: 5, subject_version: 1, status: 'completed', model: 'qwen3.6-max-preview', output: { summary: '历史方案', changes: [], items: [], risks: [], humanReviewRequired: true, workflowProposal: { mode: 'replace', explanation: '历史运行结果', warnings: [], definition: historicalProposal } }, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    const onApply = vi.fn(async () => undefined)
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: completedRun.model, capabilities: [] } })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [completedRun], total: 1 })
      if (config.url === '/ai/runs/27') return response(config, { data: completedRun })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowAIAssistant workspaceId={7} templateId={5} definition={current} dirty={false} onApply={onApply} /></QueryClientProvider>)
    expect(await screen.findByText(copy.aiPM.workflow.changePreview)).toBeTruthy()
    expect(onApply).not.toHaveBeenCalled()
  })

  it('resumes automatic save after the editor remounts while its AI run is active', async () => {
    const current: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const proposal: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'launch', type: 'approval', name: '上线确认' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'launch' }, { id: 'e2', source: 'launch', target: 'end' }],
    }
    const completedRun = { id: 37, capability: 'workflow.generate', subject_type: 'workflow_template', subject_id: 5, subject_version: 1, status: 'completed', model: 'qwen3.6-max-preview', output: { summary: '增加上线确认', changes: [], items: [], risks: [], humanReviewRequired: true, workflowProposal: { mode: 'replace', explanation: '恢复后保存方案', warnings: [], definition: proposal } }, created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    window.sessionStorage.setItem('pm.workflow-ai.pending-save.7.5', '37')
    const onApply = vi.fn(async () => undefined)
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: completedRun.model, capabilities: [] } })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [completedRun], total: 1 })
      if (config.url === '/ai/runs/37') return response(config, { data: completedRun })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowAIAssistant workspaceId={7} templateId={5} definition={current} dirty={false} onApply={onApply} /></QueryClientProvider>)
    await waitFor(() => expect(onApply).toHaveBeenCalledWith(proposal))
    expect(await screen.findByText(copy.aiPM.workflow.autoSaved)).toBeTruthy()
    expect(window.sessionStorage.getItem('pm.workflow-ai.pending-save.7.5')).toBeNull()
  })

  it('reports AI generation to the canvas and removes the duplicate loading card from the assistant', async () => {
    const current: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const queuedRun = { id: 22, capability: 'workflow.generate', subject_type: 'workflow_template', subject_id: 5, subject_version: 1, status: 'queued', model: 'qwen3.6-max-preview', created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    const onRunStateChange = vi.fn()
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: queuedRun.model, capabilities: [{ key: 'workflow.generate', name: '工作流生成', description: '生成流程', subject_types: ['workflow_template'], allowed_fields: [], mode: 'proposal' }] } })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: queuedRun })
      if (config.url === '/ai/runs/22') return response(config, { data: queuedRun })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowAIAssistant workspaceId={7} templateId={5} definition={current} dirty={false} onApply={async () => undefined} onRunStateChange={onRunStateChange} /></QueryClientProvider>)
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflow.promptLabel), { target: { value: '增加完整性检查' } })
    const generateButton = screen.getByRole('button', { name: copy.aiPM.workflow.generateDraft })
    await waitFor(() => expect(generateButton).toHaveProperty('disabled', false))
    fireEvent.click(generateButton)
    await waitFor(() => expect(onRunStateChange).toHaveBeenCalledWith({ busy: true, phase: 'queued', runId: 22 }))
    expect(screen.getByRole('button', { name: copy.aiPM.workflow.aiRunningButton })).toHaveProperty('disabled', true)
    expect(screen.getByRole('button', { name: copy.aiPM.workflow.explainIssues })).toHaveProperty('disabled', true)
    expect(screen.getByRole('button', { name: copy.aiPM.workflow.simulate })).toHaveProperty('disabled', true)
    expect(screen.queryByText(copy.aiPM.workflow.generating)).toBeNull()
  })

  it('lets a failed workflow run retry with the current prompt and canvas', async () => {
    const current: WorkflowDefinition = {
      nodes: [{ id: 'start', type: 'start', name: '开始' }, { id: 'end', type: 'end', name: '结束' }],
      edges: [{ id: 'e1', source: 'start', target: 'end' }],
    }
    const failedRun = { id: 24, capability: 'workflow.generate', subject_type: 'workflow_template', subject_id: 5, subject_version: 1, status: 'failed', model: 'qwen3.6-max-preview', error_message: 'AI runtime returned status 502', created_at: new Date().toISOString(), updated_at: new Date().toISOString() }
    const queuedRun = { ...failedRun, id: 25, status: 'queued', error_message: '' }
    let createBody: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: failedRun.model, capabilities: [{ key: 'workflow.generate', name: '工作流生成', description: '生成流程', subject_types: ['workflow_template'], allowed_fields: [], mode: 'proposal' }] } })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [failedRun], total: 1 })
      if (config.url === '/ai/runs/24') return response(config, { data: failedRun })
      if (config.url === '/ai/runs' && config.method === 'post') { createBody = JSON.parse(String(config.data)); return response(config, { data: queuedRun }) }
      if (config.url === '/ai/runs/25') return response(config, { data: queuedRun })
      return response(config, { data: [] })
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(<QueryClientProvider client={client}><WorkflowAIAssistant workspaceId={7} templateId={5} definition={current} dirty={false} onApply={async () => undefined} /></QueryClientProvider>)
    const retryButton = await screen.findByRole('button', { name: copy.aiPM.workflow.retryGeneration })
    expect(retryButton).toHaveProperty('disabled', true)
    fireEvent.change(screen.getByLabelText(copy.aiPM.workflow.promptLabel), { target: { value: '重新生成完整生命周期' } })
    await waitFor(() => expect(retryButton).toHaveProperty('disabled', false))
    fireEvent.click(retryButton)
    await waitFor(() => expect(createBody).toMatchObject({ capability: 'workflow.generate', user_prompt: '重新生成完整生命周期', workflow_definition: current }))
  })
})
