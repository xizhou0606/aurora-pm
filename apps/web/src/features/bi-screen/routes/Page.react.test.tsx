import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'

import { BiScreenPage } from './Page'
import { EnvironmentProvider, type Environment } from '../components/environment'
import { dashboardFixture } from '../utils/testFixtures'

mock.module('@/shared/ui', () => ({
    LazyEChart: ({
      ariaLabel,
      option,
      emptyTitle,
      summary,
      onDataPointActivate,
    }: {
      ariaLabel: string
      option: unknown
      emptyTitle: string
      summary: { caption: string; columns: readonly string[]; rows: readonly (readonly ReactNode[])[] }
      onDataPointActivate?: (event: { dataIndex: number }) => void
    }) => option ? (
      <div data-testid="lazy-echart">
        {onDataPointActivate ? (
          <button type="button" aria-label={ariaLabel} onClick={() => onDataPointActivate({ dataIndex: 0 })} />
        ) : (
          <div role="img" aria-label={ariaLabel} />
        )}
        <table>
          <caption>{summary.caption}</caption>
          <thead><tr>{summary.columns.map((column) => <th key={column}>{column}</th>)}</tr></thead>
          <tbody>{summary.rows.map((row, index) => (
            <tr key={index}>{row.map((cell, cellIndex) => <td key={cellIndex}>{cell}</td>)}</tr>
          ))}</tbody>
        </table>
      </div>
    ) : <div role="status">{emptyTitle}</div>,
  }))

// NumberFlow 的 lit 自定义元素无法在 jsdom 中升级，测试里以纯文本替身渲染
vi.mock('@number-flow/react', () => ({
  default: ({ value, suffix }: { value: unknown; suffix?: string }) => (
    <span>{String(value ?? '')}{suffix ?? ''}</span>
  ),
}))

function environment(input: Partial<Environment> = {}) {
  return {
    workspaceId: 7,
    sessionRevision: 3,
    pathname: '/workspace/bi-screen',
    search: {},
    allowed: true,
    operationAccess: {
      status: 'ready',
      retry: vi.fn(),
      viewOverview: true,
      viewTeamDetails: true,
      updateScoring: true,
      viewMetrics: true,
      updateMetrics: true,
    },
    navigate: vi.fn().mockResolvedValue(undefined),
    client: {
      dashboard: vi.fn().mockResolvedValue(dashboardFixture()),
		teamMemberItems: vi.fn().mockResolvedValue({
			role: 'product', item_type: 'requirement', user_id: 7,
			dimension: 'version', total: 0, items: [],
		}),
      scopeOptions: vi.fn().mockResolvedValue({
        projectSets: [{ id: 1, label: 'Set A' }],
        projects: [{ id: 11, label: 'Version A', parentId: 1 }],
        requirements: [{ id: 21, label: 'Requirement A', parentId: 11 }],
      }),
		users: vi.fn().mockResolvedValue([
			{ id: 7, username: 'owner-a', display_name: 'Owner A' },
		]),
		putTeamScoreProfile: vi.fn().mockResolvedValue(dashboardFixture().team_data.score_profile),
		teamMetricSettings: vi.fn().mockResolvedValue({
			version: 0, can_manage: true, configured: false, valid: false, options: [],
		}),
		putTeamMetricSettings: vi.fn(),
    },
    ...input,
  } satisfies Environment
}

function renderPage(value: Environment, onRuntimeFailure = vi.fn()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
  const view = render(
    <QueryClientProvider client={queryClient}>
      <EnvironmentProvider value={value}>
        <BiScreenPage onRuntimeFailure={onRuntimeFailure} />
      </EnvironmentProvider>
    </QueryClientProvider>,
  )
  return { ...view, queryClient, onRuntimeFailure }
}

describe('BI screen page', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    document.body.classList.remove('overflow-hidden')
  })

  it('fails closed before requesting scope or dashboard data', () => {
    const value = environment({ allowed: false, workspaceId: 0 })
    renderPage(value)
    expect(screen.getByRole('alert')).not.toBeNull()
    expect(value.client.dashboard).not.toHaveBeenCalled()
    expect(value.client.scopeOptions).not.toHaveBeenCalled()
  })

  it('fails closed before requesting business data when overview action is denied', () => {
    const value = environment({
      operationAccess: {
        status: 'ready',
        retry: vi.fn(),
        viewOverview: false,
        viewTeamDetails: false,
        updateScoring: false,
        viewMetrics: false,
        updateMetrics: false,
      },
    })
    renderPage(value)

    expect(screen.getByRole('alert')).not.toBeNull()
    expect(value.client.dashboard).not.toHaveBeenCalled()
    expect(value.client.scopeOptions).not.toHaveBeenCalled()
    expect(value.client.users).not.toHaveBeenCalled()
  })

  it('renders command band, chart panels, DOM summaries and owner data', async () => {
    renderPage(environment())
    expect(await screen.findByTestId('bi-screen-native-module')).not.toBeNull()
    await waitFor(() => expect(screen.getAllByTestId('lazy-echart')).toHaveLength(7))
    expect(screen.getByTestId('bi-screen-command-band')).not.toBeNull()
    expect(screen.getByTestId('bi-screen-matrix-panel')).not.toBeNull()

    expect(screen.queryByText(/故事点/)).toBeNull()
    expect(screen.getAllByRole('table').length).toBeGreaterThanOrEqual(8)
    expect(screen.getAllByText('Owner A').length).toBeGreaterThanOrEqual(2)
    expect(screen.getByText('质量风险脉冲')).not.toBeNull()
    expect(screen.getByText('版本质量风险')).not.toBeNull()
    expect(screen.getByText('缺陷负责人负载')).not.toBeNull()
    expect(screen.getByText('交付趋势')).not.toBeNull()
    expect(screen.getByText('任务构成')).not.toBeNull()
    expect(within(screen.getByTestId('bi-screen-composition-panel')).getAllByText('任务状态').length).toBeGreaterThan(0)
    expect(screen.getByTestId('bi-screen-trend-panel').className).toContain('xl:col-span-2')

    const trend = screen.getByTestId('bi-screen-trend-panel')
    const version = screen.getByTestId('bi-screen-version-panel')
    const requirement = screen.getByTestId('bi-screen-requirement-panel')
    const owner = screen.getByTestId('bi-screen-owner-panel')
    const composition = screen.getByTestId('bi-screen-composition-panel')
    const quality = screen.getByTestId('bi-screen-quality-trend-panel')
    expect(trend.compareDocumentPosition(version) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0)
    expect(version.compareDocumentPosition(requirement) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0)
    expect(requirement.compareDocumentPosition(owner) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0)
    expect(owner.compareDocumentPosition(composition) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0)
    expect(composition.compareDocumentPosition(quality) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0)
  })

  it('renders explicit role cards, personnel metrics, and the current Top 10 chart', async () => {
    renderPage(environment({ search: { bi_view: 'team' } }))
    expect(await screen.findByRole('heading', { name: '岗位指标总览' })).not.toBeNull()
    expect(screen.getByRole('table', { name: '团队人员指标数据表格' })).not.toBeNull()
    expect(screen.queryAllByTestId('lazy-echart')).toHaveLength(1)
    expect(screen.queryAllByRole('term')).toHaveLength(11)
    expect(screen.getAllByText('产品').length).toBeGreaterThanOrEqual(2)
    expect(screen.getAllByText('研发').length).toBeGreaterThanOrEqual(2)
    expect(screen.getAllByText('测试').length).toBeGreaterThanOrEqual(2)
    const sliceControl = screen.getByRole('combobox', { name: '数据切片' })
    const metricSettings = screen.getByRole('button', { name: '产品评审口径' })
    const scoreSettings = screen.getByRole('button', { name: '指标权重' })
    expect(sliceControl.compareDocumentPosition(metricSettings) & Node.DOCUMENT_POSITION_FOLLOWING)
      .not.toBe(0)
    expect(metricSettings.compareDocumentPosition(scoreSettings) & Node.DOCUMENT_POSITION_FOLLOWING)
      .not.toBe(0)
    const memberHeading = screen.getByRole('heading', { name: '人员指标数据' })
    const summaryHeading = screen.getByRole('heading', { name: '岗位指标总览' })
    expect(memberHeading.compareDocumentPosition(summaryHeading) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0)

    const memberSection = memberHeading.closest('section')
    expect(memberSection).not.toBeNull()
    const memberView = within(memberSection as HTMLElement)
    expect(memberView.getByRole('button', { name: '负责需求数降序' })).not.toBeNull()
    expect(memberView.getByRole('button', { name: '评审通过率排序' })).not.toBeNull()
    expect(memberView.getByRole('button', { name: '一次通过率排序' })).not.toBeNull()

    fireEvent.click(memberView.getByRole('tab', { name: '测试' }))
    expect(memberView.getByRole('button', { name: 'Bug 数降序' })).not.toBeNull()
    expect(memberView.getByRole('button', { name: '解决到关闭平均耗时排序' })).not.toBeNull()
    expect(memberView.getByRole('button', { name: '无效 Bug 率排序' })).not.toBeNull()
    expect(screen.queryByText('综合分')).toBeNull()
    fireEvent.click(scoreSettings)
    expect(screen.getByRole('dialog', { name: '团队综合指标权重' })).not.toBeNull()
  })

	it('opens role-specific member details within the active dashboard scope', async () => {
		const value = environment({ search: { bi_view: 'team' } })
		const teamMemberItems = vi.fn().mockImplementation((_, input: { itemType: string }) => Promise.resolve({
			role: input.itemType === 'requirement' ? 'product' : 'development',
			item_type: input.itemType,
			user_id: 7,
			dimension: 'version',
			total: 1,
			items: [{
				id: 88,
				key: input.itemType === 'bug' ? 'BUG-88' : input.itemType === 'task' ? 'TASK-88' : undefined,
				title: `Scoped ${input.itemType}`,
				status: 'in_progress',
				priority: input.itemType === 'requirement' ? 'normal' : input.itemType === 'task' ? 'high' : undefined,
				severity: input.itemType === 'bug' ? 'p1' : undefined,
				created_at: '2026-08-20T08:00:00Z',
			}],
		}))
		value.client.teamMemberItems = teamMemberItems
		renderPage(value)

		fireEvent.click(await screen.findByRole('row', { name: /产品 ·/ }))
		expect(await screen.findByText('Scoped requirement')).not.toBeNull()
		expect(screen.getByRole('columnheader', { name: '优先级' })).not.toBeNull()
		expect(screen.getByText('普通')).not.toBeNull()
		expect(teamMemberItems).toHaveBeenCalledWith(
			expect.objectContaining({ teamDimension: 'version' }),
			expect.objectContaining({ role: 'product', itemType: 'requirement', userId: 7 }),
			expect.any(AbortSignal),
		)
		fireEvent.click(screen.getByRole('button', { name: '关闭' }))

		fireEvent.click(screen.getByRole('tab', { name: '研发' }))
		fireEvent.click(await screen.findByRole('row', { name: /研发 ·/ }))
		expect(await screen.findByText('Scoped task')).not.toBeNull()
		expect(screen.getByText('高')).not.toBeNull()
		fireEvent.click(screen.getByRole('tab', { name: 'Bug' }))
		expect(await screen.findByText('Scoped bug')).not.toBeNull()
		expect(screen.getByRole('columnheader', { name: '严重级别' })).not.toBeNull()
		expect(screen.getByText('严重（P1）')).not.toBeNull()
		expect(teamMemberItems).toHaveBeenLastCalledWith(
			expect.anything(),
			expect.objectContaining({ role: 'development', itemType: 'bug', userId: 8 }),
			expect.any(AbortSignal),
		)
	})

  it('renders missing raw metrics as unavailable instead of a score eligibility message', async () => {
    const fixture = dashboardFixture()
    const product = fixture.team_data.member_groups.find((group) => group.role_key === 'product')
    if (!product) throw new Error('product fixture missing')
    product.members[0] = {
      ...product.members[0],
      workload: 1,
      score_eligible: false,
      composite_score: undefined,
      score_coverage_pct: 50,
      score_ineligible_reasons: ['workload_below_minimum', 'coverage_below_minimum'],
      score_missing_metric_keys: ['on_time_rate'],
    }
    const value = environment({ search: { bi_view: 'team' } })
    value.client.dashboard = vi.fn().mockResolvedValue(fixture)
    renderPage(value)

    expect(await screen.findByRole('table', { name: '团队人员指标数据表格' })).not.toBeNull()
    expect(screen.queryByText(/有效权重覆盖/)).toBeNull()
    expect(screen.queryByText(/综合分/)).toBeNull()
  })

  it('canonicalizes a historical chart-only link to the combined team view', async () => {
    renderPage(environment({ search: { bi_view: 'team', team_view: 'chart' } }))
    expect(await screen.findByRole('heading', { name: '岗位指标总览' })).not.toBeNull()
    expect(screen.getByRole('table', { name: '团队人员指标数据表格' })).not.toBeNull()
    expect(screen.getAllByTestId('lazy-echart')).toHaveLength(1)
    expect(screen.queryByRole('tab', { name: '图表' })).toBeNull()
  })

  it('updates the selected slice immediately and hides the previous dashboard while loading', async () => {
    let resolveSlice: ((value: ReturnType<typeof dashboardFixture>) => void) | undefined
    const value = environment({ search: { bi_view: 'team' } })
    value.client.dashboard = vi.fn()
      .mockResolvedValueOnce(dashboardFixture())
      .mockImplementationOnce(() => new Promise((resolve) => { resolveSlice = resolve }))
    renderPage(value)

    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()
    const select = screen.getByRole('combobox', { name: '数据切片' })
    fireEvent.click(select)
    const version = await screen.findByRole('option', { name: 'Version A' })
    fireEvent.pointerDown(version, { button: 0, pointerType: 'mouse' })
    fireEvent.click(version)

    expect(select.textContent).toContain('Version A')
    expect(screen.queryByRole('heading', { name: '人员指标数据' })).toBeNull()
    expect(screen.getAllByRole('status').length).toBeGreaterThan(0)
    await waitFor(() => expect(value.navigate).toHaveBeenCalledWith(
      '/workspace/bi-screen?bi_view=team&team_dimension_key=11',
    ))
    await waitFor(() => expect(value.client.dashboard).toHaveBeenLastCalledWith(
      7,
      expect.objectContaining({ teamDimension: 'version', teamDimensionKey: '11' }),
      expect.any(AbortSignal),
    ))

    resolveSlice?.(dashboardFixture({
      team_data: { ...dashboardFixture().team_data, dimension_key: '11' },
    }))

    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()
    await waitFor(() => expect(select.getAttribute('aria-busy')).not.toBe('true'))
  })

  it('keeps the latest dimension selected when navigation requests overlap', async () => {
    const value = environment({ search: { bi_view: 'team' } })
    value.navigate = vi.fn(() => new Promise<void>(() => undefined))
    renderPage(value)
    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: '月份' }))
    expect(screen.getByRole('tab', { name: '月份' }).getAttribute('aria-selected')).toBe('true')
    fireEvent.click(screen.getByRole('tab', { name: '版本' }))
    expect(screen.getByRole('tab', { name: '版本' }).getAttribute('aria-selected')).toBe('true')
    // Returning to a slice already present in the query cache should not hide
    // valid matching data while the stale navigation request is discarded.
    expect(screen.getByRole('heading', { name: '人员指标数据' })).not.toBeNull()

    await waitFor(() => expect(value.navigate).toHaveBeenCalledTimes(1))
    expect(value.navigate).toHaveBeenNthCalledWith(
      1,
      '/workspace/bi-screen?bi_view=team',
    )
  })

  it('exposes week as a peer team dimension and routes it canonically', async () => {
    const value = environment({ search: { bi_view: 'team' } })
    value.navigate = vi.fn(() => new Promise<void>(() => undefined))
    renderPage(value)
    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: '周' }))

    expect(screen.getByRole('tab', { name: '周' }).getAttribute('aria-selected')).toBe('true')
    expect(screen.getByRole('combobox', { name: '数据切片' }).textContent).toContain('全部周')
    await waitFor(() => expect(value.navigate).toHaveBeenCalledWith(
      '/workspace/bi-screen?bi_view=team&team_dimension=week',
    ))
  })

  it('expands the weekly slice popup so date ranges remain readable', async () => {
    const value = environment({ search: { bi_view: 'team', team_dimension: 'week' } })
    renderPage(value)
    const select = await screen.findByRole('combobox', { name: '数据切片' })

    expect(select.className).toContain('sm:w-72')

    fireEvent.click(select)

    const popup = document.querySelector('[data-slot="select-content"]')
    expect(popup?.className).toContain('w-max')
    expect(popup?.className).toContain('min-w-[var(--anchor-width)]')
  })

  it('restores the stable dimension and keeps an ordinary navigation failure on the page', async () => {
    let rejectNavigation: ((error: Error) => void) | undefined
    const error = new Error('navigation failed')
    const value = environment({ search: { bi_view: 'team' } })
    value.navigate = vi.fn(() => new Promise<void>((_resolve, reject) => {
      rejectNavigation = reject
    }))
    const { onRuntimeFailure } = renderPage(value)
    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: '月份' }))
    expect(screen.getByRole('tab', { name: '月份' }).getAttribute('aria-selected')).toBe('true')
    expect(screen.queryByRole('heading', { name: '人员指标数据' })).toBeNull()
    await waitFor(() => expect(value.navigate).toHaveBeenCalledTimes(1))
    rejectNavigation?.(error)

    expect((await screen.findByRole('alert')).textContent).toContain('筛选切换失败')
    expect(onRuntimeFailure).not.toHaveBeenCalled()
    expect(screen.getByRole('tab', { name: '版本' }).getAttribute('aria-selected')).toBe('true')
    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '知道了' }))
    expect(screen.queryByText(/筛选切换失败/)).toBeNull()
  })

  it('invalidates a queued team filter navigation when a newer same-page tab navigation starts', async () => {
    const value = environment({ search: { bi_view: 'team' } })
    renderPage(value)
    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: '月份' }))
    fireEvent.click(screen.getByRole('tab', { name: '交付总览' }))

    await waitFor(() => expect(value.navigate).toHaveBeenCalled())
    expect(value.navigate).toHaveBeenLastCalledWith('/workspace/bi-screen?team_dimension=month')
    expect(value.navigate).not.toHaveBeenCalledWith(
      '/workspace/bi-screen?bi_view=team&team_dimension=month',
    )
  })

  it('still escalates a true navigation transport failure to the runtime boundary', async () => {
    let rejectNavigation: ((error: PMSafeApiError) => void) | undefined
    const error = new PMSafeApiError('NAVIGATION_TRANSPORT_FAILED', 503)
    const value = environment({ search: { bi_view: 'team' } })
    value.navigate = vi.fn(() => new Promise<void>((_resolve, reject) => {
      rejectNavigation = reject
    }))
    const { onRuntimeFailure } = renderPage(value)
    expect(await screen.findByRole('heading', { name: '人员指标数据' })).not.toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: '月份' }))
    await waitFor(() => expect(value.navigate).toHaveBeenCalledTimes(1))
    rejectNavigation?.(error)

    await waitFor(() => expect(onRuntimeFailure).toHaveBeenCalledWith(error))
    expect(screen.getByRole('alert').textContent).toContain('筛选切换失败')
  })

  it('keeps task and quality chart activation on typed PM routes', async () => {
    const value = environment({
      search: { project_set_ids: '1', project_ids: '11' },
    })
    renderPage(value)
    const version = await screen.findByTestId('bi-screen-version-panel')
    fireEvent.click(within(version).getByRole('button'))
    await waitFor(() => expect(value.navigate).toHaveBeenCalledWith(
      '/workspace/work-items?project_set_ids=1&project_ids=11&view=list',
    ))
    const quality = screen.getByTestId('bi-screen-quality-trend-panel')
    fireEvent.click(within(quality).getByRole('button'))
    await waitFor(() => expect(value.navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?project_set_ids=1&project_ids=11',
    ))
  })

  it('exposes keyboard activation only on charts with real drilldowns', async () => {
    renderPage(environment())
    const trend = await screen.findByTestId('bi-screen-trend-panel')
    const version = screen.getByTestId('bi-screen-version-panel')

    expect(within(trend).queryByRole('button')).toBeNull()
    expect(within(trend).getByRole('img')).not.toBeNull()
    expect(within(version).getByRole('button')).not.toBeNull()
  })

  it('shows initial loading and empty states without rendering chart canvases', async () => {
    let resolveDashboard: ((value: unknown) => void) | undefined
    const loading = environment()
    loading.client.dashboard = vi.fn(() => new Promise((resolve) => { resolveDashboard = resolve }))
    const view = renderPage(loading)
    expect(screen.getAllByRole('status').length).toBeGreaterThan(0)
    resolveDashboard?.(dashboardFixture({
      overview_metrics: {
        ...dashboardFixture().overview_metrics,
        total_tasks: 0,
        open_bugs: 0,
      },
      completion_trend: [],
      version_progress: [],
      requirement_progress: [],
      owner_board: [],
      quality_trend: [],
      version_quality_risks: [],
      quality_owner_load: [],
    }))
    await waitFor(() => expect(screen.queryAllByTestId('lazy-echart')).toHaveLength(0))
    view.unmount()
  })

  it('keeps ready data visible after a refetch 5xx instead of escalating to iframe fallback', async () => {
    const value = environment()
    value.client.dashboard = vi.fn()
      .mockResolvedValueOnce(dashboardFixture())
      .mockRejectedValueOnce(new PMSafeApiError('BI_REFRESH_FAILED', 503))
    const { onRuntimeFailure } = renderPage(value)
    await waitFor(() => expect(screen.getAllByTestId('lazy-echart')).toHaveLength(7))
    fireEvent(document, new Event('visibilitychange'))
    await waitFor(() => expect(value.client.dashboard).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.getAllByTestId('lazy-echart')).toHaveLength(7))
    expect(screen.getByRole('alert')).not.toBeNull()
    expect(onRuntimeFailure).not.toHaveBeenCalled()
  })

  it('reports only transport failures to the module fallback boundary', async () => {
    const value = environment()
    value.client.dashboard = vi.fn().mockRejectedValue(new Error('transport'))
    const { onRuntimeFailure } = renderPage(value)
    await waitFor(() => expect(onRuntimeFailure).toHaveBeenCalledTimes(1))
  })

  it('uses native or fallback fullscreen and exits the CSS fallback with Escape', async () => {
    const value = environment()
    renderPage(value)
    const root = await screen.findByTestId('bi-screen-native-module')
    const button = screen.getByRole('button', { pressed: false })
    fireEvent.click(button)
    await waitFor(() => expect(root.classList.contains('fixed')).toBe(true))
    expect(button.getAttribute('aria-pressed')).toBe('true')
    expect(button.getAttribute('aria-controls')).toBe('bi-screen-dashboard-surface')
    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => expect(root.classList.contains('fixed')).toBe(false))
    expect(button.getAttribute('aria-pressed')).toBe('false')
    expect(document.activeElement).toBe(button)
  })
})
