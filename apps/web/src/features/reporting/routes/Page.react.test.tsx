import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi, type Mock } from 'bun:test'

import { REPORTING_COPY } from '@/shared/copy'

import { ReportingEnvironmentProvider, type ReportingEnvironment } from '../components'
import { ReportingPage } from './Page'

// The score radar lazy-loads ECharts; stub the runtime so the page test stays
// hermetic (jsdom has no canvas).
vi.mock('@/shared/ui/file-media/echart-runtime', () => ({
  EChartRuntime: () => null,
}))

function directoryFixture(includeDepartments = true) {
  return {
    groups: [
      {
        id: 21,
        code: 'dev',
        name: '研发组',
        members: [
          {
            id: 101, username: 'alpha', display_name: '张三',
            avatar_url: 'https://example.test/avatar.png', group_name: '研发组', role_names: ['后端'],
            department_id: 'od-root', department_name: '研发组',
          },
          {
            id: 102, username: 'lisi', display_name: '李四',
            avatar_url: '', group_name: '研发组', role_names: [],
            department_id: 'od-child', department_name: '平台组',
          },
        ],
      },
      {
        id: 22,
        code: 'dev-1',
        name: '技术一部',
        members: [],
      },
    ],
    ungrouped: [],
    total: 1,
    ...(includeDepartments
      ? {
        departments: [
          { id: 5, open_department_id: 'od-root', parent_department_id: '0', name: '研发组', order: 1, member_count: 1 },
          { id: 6, open_department_id: 'od-child', parent_department_id: 'od-root', name: '平台组', order: 2, member_count: 1 },
          { id: 7, open_department_id: 'od-empty', parent_department_id: 'od-child', name: '设计组', order: 3, member_count: 0 },
        ],
      }
      : {}),
  }
}

function reportBodyFixture() {
  return {
    target: {
      kind: 'member', id: 101, name: '张三', sub_label: 'alpha',
      role_names: ['后端'], job_title: '高级工程师', avatar_url: 'https://example.test/avatar.png',
    },
    period: { type: 'daily', start: '2026-09-04T00:00:00Z', end: '2026-09-05T00:00:00Z', anchor_date: '2026-09-04' },
    generated_at: '2026-09-04T08:00:00Z',
    tasks: {
      completed: 3,
      completed_items: [{ id: 1, key: 'T-1', title: '完成任务一', status: 'completed' }],
      completed_on_time: 3,
      created: 1,
      active: 2,
      overdue: 0,
    },
    bugs: { resolved: 2, resolved_items: [], resolved_on_time: 2, closed: 1, open: 3, overdue: 0 },
    scores: {
      overall: 72,
      dimensions: [
        { key: 'delivery', label: '交付完成', score: 62, basis: '完成 5 项，待办 3 项' },
        { key: 'punctuality', label: '按期交付', score: 100, basis: '按期完成 5 / 5 项' },
        { key: 'quality', label: '质量修复', score: 40, basis: '解决 2 / 5 个缺陷' },
        { key: 'overdue_control', label: '逾期控制', score: 100, basis: '无逾期事项' },
        { key: 'volume', label: '交付量', score: 56, basis: '完成 5 项（基线 3 项）' },
      ],
    },
  }
}

// 无生效角色的成员目标：头衔区应回退到飞书职务并带「职务」小标注。
// 预览响应形状：{ report, latest_run? } —— latest_run 是该视图最近一次
// run（定时生成没有前端回执，页面靠它自动带出评语与正文）。
function reportFixture() {
  return { report: reportBodyFixture() }
}

function reportFixtureWithoutRoles() {
  const report = reportBodyFixture()
  return {
    report: {
      ...report,
      target: { ...report.target, role_names: [], job_title: '高级工程师' },
    },
  }
}

function departmentReportFixture() {
  const report = reportBodyFixture()
  return {
    report: {
      ...report,
      // 组/部门目标不携带生效角色与职务（后端契约），头衔区不渲染 chips。
      target: { ...report.target, kind: 'department', name: '研发组', sub_label: '', role_names: [], job_title: '' },
    },
  }
}

function groupReportFixture(withCode = false) {
  const report = reportBodyFixture()
  const rollupCode = (commits: number, additions: number, debugHits: number, todoHits: number) => ({
    repos: 2,
    commits,
    additions,
    deletions: 10,
    prs_merged: 1,
    reviews_completed: 0,
    source_additions: additions - 30,
    test_additions: 30,
    doc_additions: 0,
    config_additions: 0,
    prs_reviewed: 0,
    debug_hits: debugHits,
    todo_hits: todoHits,
    commit_reviews: [],
    freshness: { degraded: false },
  })
  return {
    ...report,
    target: { ...report.target, kind: 'group', name: '研发组', sub_label: 'dev', role_names: [], job_title: '' },
    ...(withCode
      ? {
        code: {
          repos: 1, commits: 21, additions: 420, deletions: 20, prs_merged: 4,
          reviews_completed: 3, source_additions: 300, test_additions: 80,
          doc_additions: 20, config_additions: 20, prs_reviewed: 2,
          debug_hits: 1, todo_hits: 2, commit_reviews: [], freshness: { degraded: false },
        },
      }
      : {}),
    members: [
      {
        member: { id: 101, username: 'alpha', display_name: '张三', group_name: '研发组', role_names: [] },
        tasks: {
          completed: 5, completed_items: [], completed_on_time: 4, created: 2,
          active: 1, active_items: [], overdue: 0, overdue_items: [],
        },
        bugs: {
          resolved: 2, resolved_items: [], resolved_on_time: 2, closed: 1,
          open: 1, open_items: [], overdue: 0,
        },
        ...(withCode ? { code: rollupCode(9, 300, 0, 2) } : {}),
      },
      {
        member: { id: 102, username: 'lisi', display_name: '李四', group_name: '研发组', role_names: [] },
        tasks: {
          completed: 2, completed_items: [], completed_on_time: 2, created: 1,
          active: 3, active_items: [], overdue: 1, overdue_items: [],
        },
        bugs: {
          resolved: 4, resolved_items: [], resolved_on_time: 3, closed: 2,
          open: 2, open_items: [], overdue: 1,
        },
        ...(withCode ? { code: rollupCode(12, 120, 3, 0) } : {}),
      },
    ],
  }
}

const COMPLETED_TEAM_RUN = {
  id: 77,
  status: 'completed',
  capability: 'report.daily',
  subject_type: 'group',
  subject_id: 21,
  summary: '本期团队完成 7 项任务。',
  overall_comment: '团队整体评语。',
  dimension_comments: {
    tasks: '任务维度短评。',
    quality: '质量维度短评。',
    code: '代码维度短评。',
  },
}

type Environment = ReportingEnvironment

function environment(input: Partial<Environment> = {}) {
  return {
    workspaceId: 7,
    userId: 9,
    sessionRevision: 1,
    pathname: '/workspace/reporting',
    allowed: true,
    navigate: vi.fn().mockResolvedValue(undefined),
    client: {
      directory: vi.fn().mockResolvedValue(directoryFixture()),
      memberPreview: vi.fn().mockResolvedValue(reportFixture()),
      memberGenerate: vi.fn().mockResolvedValue({ run_id: 77, status: 'queued', report: reportBodyFixture() }),
      groupPreview: vi.fn(),
      groupGenerate: vi.fn(),
      departmentPreview: vi.fn().mockResolvedValue(departmentReportFixture()),
      departmentGenerate: vi.fn(),
      runStatus: vi.fn().mockResolvedValue({
        id: 77,
        status: 'completed',
        capability: 'report.daily',
        subject_type: 'member',
        subject_id: 101,
        summary: '今日完成 3 项任务，解决 2 个缺陷。',
      }),
      exportFeishuDoc: vi.fn().mockResolvedValue({
        document_id: 'doc-1', url: 'https://example.test/docx/doc-1', title: '【日报】张三 2026-09-04',
      }),
    },
    ...input,
  } satisfies Environment
}

function renderPage(value: Environment) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ReportingEnvironmentProvider value={value}>
        <ReportingPage />
      </ReportingEnvironmentProvider>
    </QueryClientProvider>,
  )
}

describe('ReportingPage', () => {
  it('fails closed before requesting directory data without a workspace', async () => {
    const value = environment({ allowed: false, workspaceId: 0 })
    renderPage(value)
    expect(screen.getByRole('alert')).not.toBeNull()
    await waitFor(() => {
      expect(value.client.directory).not.toHaveBeenCalled()
    })
  })

  it('renders the role-group directory with avatars and defaults to the first member preview', async () => {
    const value = environment()
    renderPage(value)
    expect(await screen.findByTestId('reporting-dept-row-od-root')).not.toBeNull()
    // Nested departments render under their parent node.
    expect(screen.getByTestId('reporting-dept-row-od-child')).not.toBeNull()
    // Parent badges carry the recursive member total (直属 + 子部门).
    expect(screen.getByText('2 人')).not.toBeNull()
    expect(screen.getByText('1 人')).not.toBeNull()
    // Empty departments stay visible with the shared empty state.
    expect(screen.getAllByText('暂无成员').length).toBeGreaterThan(0)
    await waitFor(() => {
      const preview = value.client.memberPreview as unknown as Mock<(...args: any[]) => any>
      expect(preview).toHaveBeenCalled()
      expect(preview.mock.calls[0][0]).toBe(101)
      expect(preview.mock.calls[0][1].type).toBe('daily')
    })
    expect(screen.getAllByAltText('').length).toBeGreaterThan(0)
    // 成员头衔区：生效角色渲染为中性 chips；有角色时飞书职务不做兜底展示。
    const rolesRow = screen.getByTestId('reporting-target-roles')
    expect(within(rolesRow).getByText('后端')).not.toBeNull()
    expect(screen.queryByText('高级工程师')).toBeNull()
    expect(screen.getByText(REPORTING_COPY.metrics.tasksCompleted)).not.toBeNull()
    // The professional structured body renders instantly from facts, with
    // cadence-specific section wording (日报 uses 今日).
    expect(screen.getByText(/一、今日概览/)).not.toBeNull()
    expect(screen.getByText(/【已完成】T-1 完成任务一/)).not.toBeNull()
    expect(screen.getByText(/六、数据指标/)).not.toBeNull()
    expect(screen.getByText(/七、多维评分/)).not.toBeNull()
    // The score panels render next to the fact metrics: radar panel plus the
    // per-dimension rows with their auditable basis.
    expect(screen.getByText(REPORTING_COPY.scores.title)).not.toBeNull()
    expect(screen.getByText(REPORTING_COPY.code.overallCommentTitle)).not.toBeNull()
    expect(screen.getAllByText('交付完成').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/解决 2 \/ 5 个缺陷/).length).toBeGreaterThan(0)
  })

  it('falls back to the 用户组 grouping before the first org sync', async () => {
    const value = environment({
      client: {
        directory: vi.fn().mockResolvedValue(directoryFixture(false)),
        memberPreview: vi.fn().mockResolvedValue(reportFixture()),
        memberGenerate: vi.fn().mockResolvedValue({ run_id: 77, status: 'queued', report: reportBodyFixture() }),
        groupPreview: vi.fn(),
        groupGenerate: vi.fn(),
        departmentPreview: vi.fn().mockResolvedValue(reportFixture()),
        departmentGenerate: vi.fn(),
        runStatus: vi.fn().mockResolvedValue({
          id: 77, status: 'completed', capability: 'report.daily',
          subject_type: 'member', subject_id: 101, summary: '今日完成。',
        }),
        exportFeishuDoc: vi.fn(),
      },
    } as Partial<Environment>)
    renderPage(value)
    expect(await screen.findByTestId('reporting-group-row-21')).not.toBeNull()
    expect(screen.getByText('技术一部')).not.toBeNull()
    expect(screen.queryAllByTestId('reporting-dept-row-od-root')).toEqual([])
  })

  it('falls back to the synced job title with a badge when the member has no roles', async () => {
    const value = environment({
      client: {
        directory: vi.fn().mockResolvedValue(directoryFixture(false)),
        memberPreview: vi.fn().mockResolvedValue(reportFixtureWithoutRoles()),
        memberGenerate: vi.fn().mockResolvedValue({ run_id: 77, status: 'queued', report: reportFixtureWithoutRoles() }),
        groupPreview: vi.fn(),
        groupGenerate: vi.fn(),
        departmentPreview: vi.fn(),
        departmentGenerate: vi.fn(),
        runStatus: vi.fn(),
        exportFeishuDoc: vi.fn(),
      },
    } as Partial<Environment>)
    renderPage(value)
    // role_names 为空且 job_title 非空：职务内容 + 「职务」小标注区分来源。
    const rolesRow = await screen.findByTestId('reporting-target-roles')
    expect(within(rolesRow).getByText('高级工程师')).not.toBeNull()
    expect(within(rolesRow).getByText(REPORTING_COPY.directory.jobTitleBadge)).not.toBeNull()
  })

  it('adopts the preview latest run so scheduled comments surface automatically', async () => {
    // 定时任务生成的 run 没有前端回执：预览随行 latest_run，页面自动采纳
    // 并带出综合评语、AI 评分与汇报正文，无需手动点击生成。
    const value = environment()
    value.client.memberPreview = vi.fn().mockResolvedValue({
      report: reportBodyFixture(),
      latest_run: {
        id: 901, status: 'completed', capability: 'report.daily',
        subject_type: 'member', subject_id: 101,
      },
    })
    value.client.runStatus = vi.fn().mockResolvedValue({
      id: 901, status: 'completed', capability: 'report.daily',
      subject_type: 'member', subject_id: 101,
      summary: '定时生成的汇报正文。',
      overall_comment: '定时生成的综合评语。',
      overall_score: 88,
    })
    renderPage(value)
    await waitFor(() => {
      expect(screen.getByText('定时生成的综合评语。')).not.toBeNull()
    })
    expect(screen.getByText(`${REPORTING_COPY.code.overallScoreLabel} 88`)).not.toBeNull()
    expect(screen.getByText('定时生成的汇报正文。')).not.toBeNull()
  })

  it('selects the whole role group and previews the group report', async () => {
    const value = environment({
      client: {
        directory: vi.fn().mockResolvedValue(directoryFixture(false)),
        memberPreview: vi.fn().mockResolvedValue(reportFixture()),
        memberGenerate: vi.fn().mockResolvedValue({ run_id: 77, status: 'queued', report: reportBodyFixture() }),
        groupPreview: vi.fn(),
        groupGenerate: vi.fn(),
        departmentPreview: vi.fn(),
        departmentGenerate: vi.fn(),
        runStatus: vi.fn(),
        exportFeishuDoc: vi.fn(),
      },
    } as Partial<Environment>)
    renderPage(value)
    fireEvent.click(await screen.findByTestId('reporting-group-row-21'))
    await waitFor(() => {
      const preview = value.client.groupPreview as unknown as Mock<(...args: any[]) => any>
      expect(preview).toHaveBeenCalled()
      expect(preview.mock.calls[0][0]).toBe(21)
      expect(preview.mock.calls[0][1].type).toBe('daily')
    })
  })

  it('selects a department and previews its subtree report', async () => {
    const value = environment()
    renderPage(value)
    fireEvent.click(await screen.findByTestId('reporting-dept-row-od-root'))
    await waitFor(() => {
      const preview = value.client.departmentPreview as unknown as Mock<(...args: any[]) => any>
      expect(preview).toHaveBeenCalled()
      expect(preview.mock.calls[0][0]).toBe(5)
      expect(preview.mock.calls[0][1].type).toBe('daily')
    })
    // The 部门 badge marks the department report context.
    expect(await screen.findByText(REPORTING_COPY.report.departmentBadge)).not.toBeNull()
  })

  it('switches the period and re-requests the preview', async () => {
    const value = environment()
    renderPage(value)
    await screen.findByAltText('')
    fireEvent.click(screen.getByTestId('reporting-period-weekly'))
    await waitFor(() => {
      const calls = ((value.client.memberPreview) as unknown as Mock<(...args: any[]) => any>).mock.calls
      expect(calls[calls.length - 1][1].type).toBe('weekly')
    })
  })

  it('queues a report generation and renders the completed narrative', async () => {
    const value = environment()
    renderPage(value)
    const generateButton = await screen.findByRole('button', { name: REPORTING_COPY.report.generate })
    fireEvent.click(generateButton)
    await waitFor(() => {
      expect((value.client.memberGenerate)).toHaveBeenCalled()
      expect(((value.client.memberGenerate) as unknown as Mock<(...args: any[]) => any>).mock.calls[0][0]).toBe(101)
      expect(((value.client.memberGenerate) as unknown as Mock<(...args: any[]) => any>).mock.calls[0][1].type).toBe('daily')
    })
    expect(await screen.findByText('今日完成 3 项任务，解决 2 个缺陷。')).not.toBeNull()
  })

  it('uploads the facts report to Feishu without a completed run', async () => {
    const value = environment()
    renderPage(value)
    // The upload button sits next to 导出 .md and works before any generation.
    const feishuButton = await screen.findByRole('button', { name: REPORTING_COPY.report.feishuUpload })
    fireEvent.click(feishuButton)
    await waitFor(() => {
      expect(value.client.exportFeishuDoc).toHaveBeenCalled()
    })
    const [runId, facts] = ((value.client.exportFeishuDoc) as unknown as Mock<(...args: any[]) => any>).mock.calls[0]
    expect(runId).toBe(0)
    expect(facts).toMatchObject({ targetKind: 'member', targetId: 101, type: 'daily' })
    expect(await screen.findByRole('link', { name: REPORTING_COPY.report.feishuUploaded })).not.toBeNull()
  })

  it('exports a completed run to a Feishu document', async () => {
    const value = environment()
    renderPage(value)
    fireEvent.click(await screen.findByRole('button', { name: REPORTING_COPY.report.generate }))
    // Wait for the run to complete so the upload carries the full document.
    await screen.findByText('今日完成 3 项任务，解决 2 个缺陷。')
    const feishuButton = await screen.findByRole('button', { name: REPORTING_COPY.report.feishuUpload })
    fireEvent.click(feishuButton)
    await waitFor(() => {
      expect(value.client.exportFeishuDoc).toHaveBeenCalled()
    })
    const [runId] = ((value.client.exportFeishuDoc) as unknown as Mock<(...args: any[]) => any>).mock.calls[0]
    expect(runId).toBe(77)
    const link = await screen.findByRole('link', { name: REPORTING_COPY.report.feishuUploaded })
    expect(link.getAttribute('href')).toBe('https://example.test/docx/doc-1')
  })

  it('shows the empty directory state when no members exist', async () => {
    const value = environment({
      client: {
        directory: vi.fn().mockResolvedValue({ groups: [], ungrouped: [], total: 0 }),
        memberPreview: vi.fn(),
        memberGenerate: vi.fn(),
        groupPreview: vi.fn(),
        groupGenerate: vi.fn(),
        departmentPreview: vi.fn(),
        departmentGenerate: vi.fn(),
        runStatus: vi.fn(),
        exportFeishuDoc: vi.fn(),
      },
    } as Partial<Environment>)
    renderPage(value)
    expect(await screen.findByText(REPORTING_COPY.directory.emptyTitle)).not.toBeNull()
    await waitFor(() => {
      expect(value.client.memberPreview).not.toHaveBeenCalled()
    })
  })

  it('renders the aggregated group ranking table without code data', async () => {
    const value = environment({
      client: {
        directory: vi.fn().mockResolvedValue(directoryFixture(false)),
        memberPreview: vi.fn().mockResolvedValue(reportFixture()),
        memberGenerate: vi.fn().mockResolvedValue({ run_id: 77, status: 'queued', report: reportBodyFixture() }),
        groupPreview: vi.fn().mockResolvedValue({ report: groupReportFixture(false) }),
        groupGenerate: vi.fn(),
        departmentPreview: vi.fn(),
        departmentGenerate: vi.fn(),
        runStatus: vi.fn(),
        exportFeishuDoc: vi.fn(),
      },
    } as Partial<Environment>)
    const { container } = renderPage(value)
    fireEvent.click(await screen.findByTestId('reporting-group-row-21'))
    // 固定五段顺序：综合评语只渲染一次；排行表在综合评语之后，汇报正文固定在最下。
    const table = await screen.findByRole('table', { name: REPORTING_COPY.ranking.title })
    const commentHeading = screen.getByText(REPORTING_COPY.code.overallCommentTitle)
    expect(screen.getAllByText(REPORTING_COPY.code.overallCommentTitle).length).toBe(1)
    expect(
      commentHeading.compareDocumentPosition(table) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    const summaryHeading = screen.getByText(REPORTING_COPY.report.summaryTitle)
    expect(
      table.compareDocumentPosition(summaryHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    expect(within(table).getByText(REPORTING_COPY.ranking.tasksGroup)).not.toBeNull()
    expect(within(table).getByText(REPORTING_COPY.ranking.qualityGroup)).not.toBeNull()
    expect(within(table).getByText(REPORTING_COPY.ranking.codeGroup)).not.toBeNull()
    // 默认按完成任务排序：张三在前；未采集代码的成员以「—」显示。
    const bodyRows = () => Array.from(table.querySelectorAll('tbody tr'))
      .map((row) => Array.from(row.querySelectorAll('td')).map((cell) => cell.textContent))
    expect(bodyRows()).toEqual([
      ['1', '张三', '5', '0', '2', '0', '—', '—', '—', '—'],
      ['2', '李四', '2', '1', '4', '1', '—', '—', '—', '—'],
    ])
  })

  it('renders AI commentary and sortable ranking headers after generation', async () => {
    const value = environment({
      client: {
        directory: vi.fn().mockResolvedValue(directoryFixture(false)),
        memberPreview: vi.fn().mockResolvedValue(reportFixture()),
        memberGenerate: vi.fn().mockResolvedValue({ run_id: 77, status: 'queued', report: reportBodyFixture() }),
        groupPreview: vi.fn().mockResolvedValue({ report: groupReportFixture(true) }),
        groupGenerate: vi.fn().mockResolvedValue({ run_id: 77, status: 'queued', report: groupReportFixture(true) }),
        departmentPreview: vi.fn(),
        departmentGenerate: vi.fn(),
        runStatus: vi.fn().mockResolvedValue(COMPLETED_TEAM_RUN),
        exportFeishuDoc: vi.fn(),
      },
    } as Partial<Environment>)
    const { container } = renderPage(value)
    fireEvent.click(await screen.findByTestId('reporting-group-row-21'))
    const table = await screen.findByRole('table', { name: REPORTING_COPY.ranking.title })
    fireEvent.click(screen.getByRole('button', { name: REPORTING_COPY.report.generate }))
    // 生成完成后：任务/质量/代码三段 AI 点评显示在排行表上方；综合评语仍在
    // 排行表之前且只渲染一次，汇报正文固定在最下。
    expect(await screen.findByText('任务维度短评。')).not.toBeNull()
    expect(screen.getByText('质量维度短评。')).not.toBeNull()
    expect(screen.getByText('代码维度短评。')).not.toBeNull()
    const commentHeading = screen.getByText(REPORTING_COPY.code.overallCommentTitle)
    expect(screen.getAllByText(REPORTING_COPY.code.overallCommentTitle).length).toBe(1)
    expect(
      commentHeading.compareDocumentPosition(table) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    const summaryHeading = screen.getByText(REPORTING_COPY.report.summaryTitle)
    expect(
      table.compareDocumentPosition(summaryHeading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    const bodyRows = () => Array.from(table.querySelectorAll('tbody tr'))
      .map((row) => Array.from(row.querySelectorAll('td')).map((cell) => cell.textContent))
    // 默认按完成任务排序：张三在前。
    expect(bodyRows()[0]).toEqual(['1', '张三', '5', '0', '2', '0', '9', '+300 / -10', '0', '2'])
    // 点击二级标题「调试残留」：按命中数降序，李四（3 次命中）反超到第一。
    fireEvent.click(within(table).getByRole('button', { name: REPORTING_COPY.ranking.sortAria(REPORTING_COPY.ranking.debugHitsColumn) }))
    expect(bodyRows()[0]).toEqual(['1', '李四', '2', '1', '4', '1', '12', '+120 / -10', '3', '0'])
    // 再次点击切换升序：命中最少的张三回到第一。
    fireEvent.click(within(table).getByRole('button', { name: REPORTING_COPY.ranking.sortAria(REPORTING_COPY.ranking.debugHitsColumn, 'descending') }))
    expect(bodyRows()[0]).toEqual(['1', '张三', '5', '0', '2', '0', '9', '+300 / -10', '0', '2'])
    // 组级代码汇总行显示在表格下方。
    expect(screen.getByText('本期代码：1 个仓库 · 21 次提交 · +420/-20 行')).not.toBeNull()
  })
})
