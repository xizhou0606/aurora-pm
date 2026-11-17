import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import type { ReactNode } from 'react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { coreApi, platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'

import { seedActionPermissions } from '../utils/test-action-permissions'
import { WorkflowSubjectLauncher } from './WorkflowSubjectLauncher'
import { WorkflowSummaryChannel, WorkflowSummaryHost } from './WorkflowSummaryChannel'
import type { WorkflowInstance } from '../model/types'

const originalPlatformAdapter = platformApi.defaults.adapter
const originalCoreAdapter = coreApi.defaults.adapter

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data, status: 200, statusText: 'OK', headers: {}, config }
}

function instance(): WorkflowInstance {
  return {
    id: 91,
    template_id: 8,
    template_name: '项目交付流程',
    template_version: 2,
    status: 'running',
    subject_type: 'plan_unit',
    subject_id: 27,
    current_node_ids: ['delivery'],
    current_nodes: [{ id: 'delivery', type: 'human_task', name: '研发交付' }],
    timeline_nodes: [
      { id: 'start', type: 'start', name: '开始', status: 'completed' },
      { id: 'delivery', type: 'human_task', name: '研发交付', status: 'active' },
      { id: 'end', type: 'end', name: '结束', status: 'pending' },
    ],
    definition_edges: [
      { id: 'e1', source: 'start', target: 'delivery' },
      { id: 'e2', source: 'delivery', target: 'end' },
    ],
    selected_node_ids: ['start', 'delivery', 'end'],
    selected_edge_ids: ['e1', 'e2'],
    node_runs: [],
    approvals: [],
    completed_steps: 0,
    total_steps: 1,
    progress: 0,
    started_at: '2026-08-09T00:00:00Z',
    subject_schedule: {
      state: 'scheduled',
      can_confirm: false,
      subject: { planned_start_at: '2026-08-01', planned_end_at: '2026-08-31' },
      conflicts: [],
    },
  }
}

function renderLauncher(layout?: 'horizontal' | 'vertical', density?: 'default' | 'compact', wrap?: (node: ReactNode) => ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  seedActionPermissions([PM_ACTION.WorkflowsInstancesView])
  const current = instance()
  platformApi.defaults.adapter = async (config) => {
    if (config.url === '/pm/workflows/instances' && config.method === 'get') {
      return response(config, { data: [{ id: current.id }], total: 1 })
    }
    if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') {
      return response(config, { data: current })
    }
    return response(config, { data: [] })
  }
  const launcher = (
    <WorkflowSubjectLauncher
      workspaceId={7}
      subjectType="plan_unit"
      subjectId={current.subject_id}
      instanceId={current.id}
      layout={layout}
      density={density}
    />
  )
  return render(
    <QueryClientProvider client={client}>
      {wrap ? wrap(launcher) : launcher}
    </QueryClientProvider>,
  )
}

describe('workflow subject launcher track layout', () => {
  afterEach(() => {
    platformApi.defaults.adapter = originalPlatformAdapter
    coreApi.defaults.adapter = originalCoreAdapter
  })

  it('keeps the workflow loading skeleton mounted while the instance request is pending', () => {
    const current = instance()
    platformApi.defaults.adapter = async (config) => {
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') {
        return new Promise(() => {})
      }
      return response(config, { data: [{ id: current.id }], total: 1 })
    }

    const view = renderLauncher()

    expect(screen.getByTestId('plan_unit-workflow-workbench')).toBeTruthy()
    expect(view.container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(3)
  })

  it('keeps the default runtime track horizontal', async () => {
    renderLauncher()
    const stages = await screen.findByTestId('workflow-runtime-stages')
    expect(stages.dataset.layout).toBe('compact-track')
    expect(stages.className).toContain('overflow-x-auto')
    expect(screen.getByTestId('workflow-stage-delivery').className).toContain('w-[9.25rem]')
    expect(screen.getByTestId('plan_unit-workflow-workbench').className).toContain('rounded-lg')
    expect(screen.getByTestId('plan_unit-workflow-workbench').className).toMatch(/(?:^|\s)border(?:\s|$)/)
    expect(screen.getByTestId('workflow-schedule-tag').getAttribute('data-schedule-state')).toBe('scheduled')
    expect(screen.queryByTestId('workflow-schedule-summary')).toBeNull()
    expect(screen.queryByText(copy.aiPM.workflowSubject.scheduleScheduledDescription)).toBeNull()
  })

  it('tightens the horizontal track when compact density is requested', async () => {
    renderLauncher('horizontal', 'compact')
    const stages = await screen.findByTestId('workflow-runtime-stages')
    expect(screen.getByTestId('plan_unit-workflow-workbench').dataset.density).toBe('compact')
    expect(screen.getByTestId('plan_unit-workflow-workbench').className).not.toContain('rounded-lg')
    expect(screen.getByTestId('plan_unit-workflow-workbench').className).not.toMatch(/(?:^|\s)border(?:\s|$)/)
    expect(stages.dataset.layout).toBe('compact-track')
    expect(screen.getByTestId('workflow-node-action-delivery').className).toContain('min-h-9')
    expect(screen.getByTestId('workflow-node-action-delivery').className).not.toContain('min-h-11')
    expect(screen.getByTestId('workflow-node-action-delivery').className).not.toContain('min-h-[3.75rem]')
    expect(screen.queryByText('项目交付流程')).toBeNull()
    expect(screen.queryByTestId('workflow-schedule-summary')).toBeNull()
    expect(screen.getByText('0%')).toBeTruthy()
    expect(screen.getByText('0/1')).toBeTruthy()
  })

  it('renders the compact progress summary inside a summary host so the track keeps its own row', async () => {
    renderLauncher('horizontal', 'compact', (launcher) => (
      <WorkflowSummaryChannel>
        <div>
          <header data-testid="hero-banner">
            <WorkflowSummaryHost />
          </header>
          {launcher}
        </div>
      </WorkflowSummaryChannel>
    ))

    const stages = await screen.findByTestId('workflow-runtime-stages')
    const host = screen.getByTestId('workflow-summary-host')
    expect(within(host).getByText('0%')).toBeTruthy()
    expect(within(host).getByText('0/1')).toBeTruthy()
    expect(host.contains(stages)).toBe(false)
  })

  it('stacks the existing node track vertically when asked', async () => {
    renderLauncher('vertical')
    const stages = await screen.findByTestId('workflow-runtime-stages')
    expect(screen.getByTestId('plan_unit-workflow-workbench').dataset.layout).toBe('vertical')
    expect(stages.dataset.layout).toBe('vertical-track')
    expect(stages.className).toContain('overflow-y-auto')
    expect(stages.className).not.toContain('overflow-x-auto')
    expect(screen.getByTestId('workflow-stage-delivery').className).toContain('w-full')
    expect(screen.getByTestId('workflow-stage-delivery').className).not.toContain('w-[9.25rem]')
    expect(screen.getByTestId('workflow-node-action-delivery').className).toContain('pm-workflow-node--rail')
    expect(screen.getByTestId('workflow-node-action-delivery').className).not.toContain('min-h-[3.75rem]')
    expect(screen.queryByText(copy.aiPM.workflowSubject.nodePlanUnset)).toBeNull()
  })

  it('places vertical parallel lanes side by side', async () => {
    const current = {
      ...instance(),
      id: 93,
      current_node_ids: ['qa'],
      current_nodes: [{ id: 'qa', type: 'human_task', name: 'QA 测试' }],
      timeline_nodes: [
        { id: 'start', type: 'start', name: '开始', status: 'completed' },
        { id: 'split', type: 'parallel_split', name: '研发测试并行', status: 'completed' },
        { id: 'develop', type: 'human_task', name: '研发实现', status: 'completed' },
        { id: 'qa', type: 'human_task', name: 'QA 测试', status: 'active' },
        { id: 'join', type: 'parallel_join', name: '交付汇聚', status: 'waiting', parallel_completed: 1, parallel_total: 2 },
        { id: 'end', type: 'end', name: '结束', status: 'pending' },
      ],
      definition_edges: [
        { id: 'e1', source: 'start', target: 'split' },
        { id: 'e2', source: 'split', target: 'develop' },
        { id: 'e3', source: 'split', target: 'qa' },
        { id: 'e4', source: 'develop', target: 'join' },
        { id: 'e5', source: 'qa', target: 'join' },
        { id: 'e6', source: 'join', target: 'end' },
      ],
      selected_node_ids: ['start', 'split', 'develop', 'qa', 'join', 'end'],
      selected_edge_ids: ['e1', 'e2', 'e3', 'e4', 'e5', 'e6'],
    }
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    seedActionPermissions([PM_ACTION.WorkflowsInstancesView])
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') {
        return response(config, { data: [{ id: current.id }], total: 1 })
      }
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') {
        return response(config, { data: current })
      }
      return response(config, { data: [] })
    }
    render(
      <QueryClientProvider client={client}>
        <WorkflowSubjectLauncher
          workspaceId={7}
          subjectType="plan_unit"
          subjectId={current.subject_id}
          instanceId={current.id}
          layout="vertical"
        />
      </QueryClientProvider>,
    )
    const group = await screen.findByTestId('workflow-parallel-split')
    const lanes = screen.getByTestId('workflow-fork-lanes-split')
    expect(group.dataset.layout).toBe('vertical-parallel-columns')
    expect(lanes.className).toContain('grid-cols-2')
    expect(screen.getByTestId('workflow-fork-lane-split-0')).toBeTruthy()
    expect(screen.getByTestId('workflow-fork-lane-split-1')).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.parallelBranch(1))).toBeTruthy()
    expect(screen.getByText(copy.aiPM.workflowSubject.parallelBranch(2))).toBeTruthy()
    expect(within(group).queryByText(copy.aiPM.workflowSubject.nodePlanUnset)).toBeNull()
  })
})
