import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi, mock } from 'bun:test'

const hookMocks = (() => ({
  overview: vi.fn(),
  assignment: vi.fn(),
  chartOption: vi.fn(),
}))()

vi.mock('../hooks/useProjectOverviewModel', () => ({
  useProjectOverviewModel: hookMocks.overview,
  useProjectAssignmentModel: hookMocks.assignment,
}))


vi.mock('./LatestProjectUpdate', () => ({
  LatestProjectUpdate: ({ projectId }: { projectId: number }) => (
    <div data-testid="latest-project-update">LATEST_UPDATE_{projectId}</div>
  ),
}))

mock.module('@/shared/ui', () => ({
    LazyEChart: ({
      ariaLabel,
      option,
      emptyTitle,
      summary,
    }: {
      ariaLabel: string
      option: unknown
      emptyTitle: string
      summary: {
        caption: string
        columns: readonly string[]
        rows: readonly (readonly ReactNode[])[]
      }
    }) => {
      hookMocks.chartOption(option)
      return option ? (
        <div data-testid="task-execution-chart">
          <div role="img" aria-label={ariaLabel} />
          <table>
            <caption>{summary.caption}</caption>
            <thead><tr>{summary.columns.map((column) => <th key={column}>{column}</th>)}</tr></thead>
            <tbody>{summary.rows.map((row, index) => (
              <tr key={index}>{row.map((cell, cellIndex) => <td key={cellIndex}>{cell}</td>)}</tr>
            ))}</tbody>
          </table>
        </div>
      ) : <div role="status">{emptyTitle}</div>
    },
  }))

import { copy } from '@/shared/copy'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'
import type { ProjectOverviewAggregateModel } from '../model'
import { AssignmentAggregate } from './AssignmentAggregate'
import { PlanUnitOverview, ProjectOverview } from './Overview'

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'PM React',
  status: 'active',
}

const unit: ScrumVersionTaskNew = {
  id: 31,
  team_id: 2,
  project_new_id: 9,
  title: 'V1.0',
  status: 'development',
}

const zeroModel: ProjectOverviewAggregateModel = {
  metrics: [
    { id: 'status', label: '项目状态', value: '进行中' },
    { id: 'progress', label: '进度', value: '0%', description: '暂无任务' },
    { id: 'health', label: '健康状态', value: '未评估' },
    { id: 'risk', label: '风险', value: '低' },
    { id: 'dates', label: '关键日期', value: '未设置' },
  ],
  attention: [],
  riskRequirements: [],
  evidence: { requirements: 0, tasks: 0, bugs: 0 },
  delivery: {
    verdict: '待补充交付节点',
    currentNode: '未配置',
    nextDue: '—',
    completedNodes: 0,
    totalNodes: 0,
    delayedNodes: 0,
  },
  work: {
    requirements: { total: 0, done: 0, active: 0, backlog: 0, highPriority: 0 },
    tasks: { total: 0, done: 0, active: 0, todo: 0, cancelled: 0, unassigned: 0 },
    bugs: { open: 0, critical: 0, normal: 0, minor: 0 },
  },
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('project overview aggregate components', () => {
  it('shows the numeric summary and three charts before project-set delivery', () => {
    hookMocks.overview.mockReturnValue({
      status: 'empty',
      model: zeroModel,
      refreshing: false,
    })

    render(
      <ProjectOverview
        project={project}
        units={[unit]}
        users={[]}
      />,
    )

    expect(screen.getByTestId('pm-work-scope')).toBeTruthy()
    expect(screen.getByTestId('pm-work-charts')).toBeTruthy()
    expect(screen.getByTestId('pm-work-chart-requirements')).toBeTruthy()
    expect(screen.getByTestId('pm-work-chart-tasks')).toBeTruthy()
    expect(screen.getByTestId('pm-work-chart-bugs')).toBeTruthy()
    expect(screen.getByTestId('pm-project-creator-card')).toBeTruthy()
    expect(screen.getByTestId('latest-project-update').textContent).toBe('LATEST_UPDATE_9')
    expect(screen.queryByTestId('pm-risk-requirements')).toBeNull()
    expect(screen.getAllByRole('status').some((node) => (
      node.textContent?.includes(copy.projectManagement.overview.workBreakdown.charts.empty)
    ))).toBe(true)

    const workScope = screen.getByTestId('pm-work-scope')
    const charts = screen.getByTestId('pm-work-charts')
    const creatorCard = screen.getByTestId('pm-project-creator-card')
    const latestUpdate = screen.getByTestId('latest-project-update')
    expect(workScope.dataset.layout).toBe('strip')
    expect(workScope.className).not.toContain('text-center')
    expect(workScope.compareDocumentPosition(charts) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(charts.compareDocumentPosition(creatorCard) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(creatorCard.compareDocumentPosition(latestUpdate) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('exposes stale retry without owning navigation', () => {
    const retry = vi.fn()
    hookMocks.overview.mockReturnValue({
      status: 'ready',
      model: zeroModel,
      refreshing: true,
      staleError: { retry },
    })

    render(
      <ProjectOverview
        project={project}
        units={[unit]}
        users={[]}
      />,
    )

    expect(screen.getByText(copy.projectManagement.overview.refreshing).getAttribute('role')).toBe('status')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('renders requirement, task and bug donuts from the work breakdown', () => {
    hookMocks.overview.mockReturnValue({
      status: 'ready',
      model: {
        ...zeroModel,
        work: {
          requirements: { total: 12, done: 4, active: 5, backlog: 3, highPriority: 2 },
          tasks: { total: 15, todo: 1, active: 2, done: 8, cancelled: 4, unassigned: 2 },
          bugs: { open: 3, critical: 1, normal: 2, minor: 0 },
        },
      },
      refreshing: false,
    })

    render(
      <ProjectOverview
        project={project}
        units={[unit]}
        users={[]}
      />,
    )

    const workCopy = copy.projectManagement.overview.workBreakdown
    const taskCopy = copy.projectManagement.overview.taskExecution
    expect(within(screen.getByTestId('pm-work-scope')).getByText('12')).toBeTruthy()
    expect(within(screen.getByTestId('pm-work-scope')).getByText('15')).toBeTruthy()
    expect(within(screen.getByTestId('pm-work-scope')).getByText('3')).toBeTruthy()
    expect(within(screen.getByTestId('pm-work-scope')).getByText(workCopy.taskDoneValue(53))).toBeTruthy()
    expect(screen.getByRole('img', {
      name: workCopy.charts.requirementAria(4, 5, 3),
    })).toBeTruthy()
    expect(screen.getByRole('img', {
      name: taskCopy.ariaLabel(1, 2, 8, 4),
    })).toBeTruthy()
    expect(screen.getByRole('img', {
      name: workCopy.charts.bugAria(1, 2, 0),
    })).toBeTruthy()
    expect(screen.queryByTestId('pm-risk-requirements')).toBeNull()
  })

  it('keeps version overview to the summary and charts', () => {
    hookMocks.overview.mockReturnValue({
      status: 'empty',
      model: zeroModel,
      refreshing: false,
    })

    render(
      <PlanUnitOverview
        project={project}
        unit={unit}
        users={[]}
      />,
    )
    expect(screen.getByTestId('pm-work-scope')).toBeTruthy()
    expect(screen.getByTestId('pm-work-charts')).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.overview.workBreakdown.requirements)).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.overview.workBreakdown.tasks)).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.overview.workBreakdown.bugs)).toBeTruthy()
    expect(screen.queryByTestId('delivery-roadmap')).toBeNull()
    expect(screen.queryByTestId('latest-project-update')).toBeNull()
    expect(screen.queryByTestId('pm-risk-requirements')).toBeNull()
  })

  it('renders a fatal boundary and retry for fail-closed aggregate errors', () => {
    const retry = vi.fn()
    hookMocks.overview.mockReturnValue({ status: 'error', retry })

    render(
      <ProjectOverview
        project={project}
        units={[unit]}
        users={[]}
      />,
    )

    expect(screen.getByRole('alert').textContent).toContain(
      copy.projectManagement.overview.errorTitle,
    )
    fireEvent.click(screen.getByRole('button', { name: copy.ui.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
  })
})

describe('delivery assignment aggregate', () => {
  it('renders the member summary (owner first, then participants) before the roadmap', () => {
    hookMocks.assignment.mockReturnValue({
      status: 'empty',
      summary: {
        status: 'empty',
        projectName: project.name,
        planUnitTitle: unit.title,
      },
      refreshing: false,
    })

    const { container } = render(
      <QueryClientProvider client={new QueryClient()}>
      <AssignmentAggregate
        project={project}
        unit={unit}
        users={[]}
      />
      </QueryClientProvider>,
    )

    // The aggregate people table and the phases-and-milestones roadmap are
    // retired: member display is the owner-first participants card only.
    expect(screen.queryByTestId('assignment-summary')).toBeNull()
    expect(screen.queryByTestId('delivery-roadmap')).toBeNull()
    expect(screen.getByTestId('pm-project-members-card')).toBeTruthy()
  })
})
