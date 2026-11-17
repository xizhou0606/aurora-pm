import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import { BI_SCREEN_COPY } from '@/shared/copy'

import { dashboardFixture } from '../utils/testFixtures'
import type { TeamMemberGroup } from '../types'
import { TeamMemberMetricsTable } from './TeamMemberMetricsTable'

vi.mock('./TeamChartView', () => ({
  TeamChartView: ({
    members,
    metric,
    descending,
  }: {
    members: readonly { user_name: string }[]
    metric: { key: string }
    descending: boolean
  }) => (
    <div
      data-testid="team-metric-chart"
      data-descending={String(descending)}
      data-members={members.map((member) => member.user_name).join(',')}
      data-metric={metric.key}
    />
  ),
}))

function testingGroup(): TeamMemberGroup {
  const fixture = dashboardFixture()
  const source = fixture.team_data.member_groups.find((group) => group.role_key === 'testing')
  if (!source) throw new Error('testing fixture missing')
  const base = source.members[0]
  return {
    ...source,
    members: [
      {
        ...base,
        user_id: 1,
        user_name: '慢响应',
        bug_count: 4,
        bug_close_avg_hours: 8,
        bug_close_sample_size: 2,
      },
      {
        ...base,
        user_id: 2,
        user_name: '无样本',
        bug_count: 9,
        bug_close_avg_hours: undefined,
        bug_close_sample_size: 0,
      },
      {
        ...base,
        user_id: 3,
        user_name: '快响应',
        bug_count: 2,
        bug_close_avg_hours: 2,
        bug_close_sample_size: 1,
      },
    ],
  }
}

describe('TeamMemberMetricsTable', () => {
  it('keeps the person column static and exposes sort state only on metric headers', async () => {
    render(
      <TeamMemberMetricsTable
        groups={[testingGroup()]}
        role="testing"
        onRoleChange={vi.fn()}
      />,
    )

    const personHeader = screen.getByRole('columnheader', { name: '成员' })
    expect(within(personHeader).queryByRole('button')).toBeNull()

    const bugCountSort = await screen.findByRole('button', { name: 'Bug 数降序' })
    expect(bugCountSort.getAttribute('aria-sort')).toBeNull()
    expect(screen.getByRole('columnheader', { name: 'Bug 数' }).getAttribute('aria-sort')).toBe('descending')
    expect(screen.getByRole('columnheader', { name: '解决到关闭平均耗时' }).getAttribute('aria-sort')).toBe('none')

    const memberRow = screen.getByRole('row', { name: '测试 · 慢响应' })
    expect(memberRow.className).toContain('hover:!bg-primary/[0.06]')
    expect(memberRow.className).toContain('focus-visible:!bg-primary/[0.06]')
  })

  it('starts a lower-is-better metric ascending, keeps missing values last, and links the chart order', async () => {
    render(
      <TeamMemberMetricsTable
        groups={[testingGroup()]}
        role="testing"
        onRoleChange={vi.fn()}
      />,
    )

    fireEvent.click(await screen.findByRole('button', { name: '解决到关闭平均耗时排序' }))

    const durationSort = await screen.findByRole('button', { name: '解决到关闭平均耗时升序' })
    expect(durationSort.getAttribute('aria-sort')).toBeNull()
    expect(screen.getByRole('columnheader', { name: '解决到关闭平均耗时' }).getAttribute('aria-sort')).toBe('ascending')

    const table = screen.getByRole('table', { name: '团队人员指标数据表格' })
    const rowText = within(table).getAllByRole('row').slice(1).map((row) => row.textContent)
    expect(rowText[0]).toContain('快响应')
    expect(rowText[1]).toContain('慢响应')
    expect(rowText[2]).toContain('无样本')

    await waitFor(() => expect(screen.getByTestId('team-metric-chart')).toMatchObject({
      dataset: expect.objectContaining({
        descending: 'false',
        members: '快响应,慢响应,无样本',
        metric: 'bug_close_hours',
      }),
    }))
  })

  it('switches to the next role workload sort without rendering the previous direction', async () => {
    const fixture = dashboardFixture()
    function RoleFixture() {
      const [role, setRole] = useState<'product' | 'testing' | 'development'>('testing')
      return (
        <TeamMemberMetricsTable
          groups={fixture.team_data.member_groups}
          role={role}
          onRoleChange={setRole}
        />
      )
    }
    render(<RoleFixture />)

    fireEvent.click(await screen.findByRole('button', { name: '解决到关闭平均耗时排序' }))
    expect(screen.getByRole('columnheader', { name: '解决到关闭平均耗时' }).getAttribute('aria-sort')).toBe('ascending')
    fireEvent.click(screen.getByRole('tab', { name: '研发' }))

    expect(screen.getByRole('columnheader', { name: '任务量' }).getAttribute('aria-sort')).toBe('descending')
    expect(screen.getByTestId('team-metric-chart')).toMatchObject({
      dataset: expect.objectContaining({ descending: 'true', metric: 'task_count' }),
    })
  })

  it('exposes the centralized development overdue branches beside the metric header', () => {
    const fixture = dashboardFixture()
    render(
      <TeamMemberMetricsTable
        groups={fixture.team_data.member_groups}
        role="development"
        onRoleChange={vi.fn()}
      />,
    )

    const help = screen.getByRole('button', { name: '查看研发任务逾期率规则' })
    expect(help.className).toContain('cursor-help')
    expect(BI_SCREEN_COPY.team.overdueRule.rows.map((row) => row.path)).toEqual([
      '进入待验证',
      '待验证后退回研发',
      '未待验证直接完成',
      '尚未待验证或完成',
    ])
		expect(BI_SCREEN_COPY.team.overdueRule.sampleRule).toBe(
			'有效样本：任务自身设置截止时间，或子任务可继承最近父任务截止时间；缺少有效截止时间的任务不计入逾期率。',
		)
		expect(screen.queryByText('6 个有效样本')).toBeNull()
		expect(screen.getByText('有效样本：Bug 响应耗时需具备指派与首次处理节点；逾期率口径见表头问号。')).toBeTruthy()
  })

  it('uses the shared user selector to filter the table and chart by member', async () => {
    render(
      <TeamMemberMetricsTable
        groups={[testingGroup()]}
        role="testing"
        onRoleChange={vi.fn()}
      />,
    )

    const selector = screen.getByRole('combobox', { name: '筛选成员' }) as HTMLInputElement
    fireEvent.click(selector)
    const member = await screen.findByRole('option', { name: '快响应' })
    fireEvent.pointerDown(member, { button: 0, pointerType: 'mouse' })
    fireEvent.click(member)

    expect(selector.value).toBe('快响应')
    const table = screen.getByRole('table', { name: '团队人员指标数据表格' })
    const rows = within(table).getAllByRole('row')
    expect(rows).toHaveLength(2)
    expect(rows[1].textContent).toContain('快响应')
    expect(screen.getByTestId('team-metric-chart')).toMatchObject({
      dataset: expect.objectContaining({ members: '快响应' }),
    })

    fireEvent.click(selector)
    const allMembers = await screen.findByRole('option', { name: '全部成员' })
    fireEvent.pointerDown(allMembers, { button: 0, pointerType: 'mouse' })
    fireEvent.click(allMembers)
    expect(within(table).getAllByRole('row')).toHaveLength(4)
  })

  it('aligns the personnel table with the Top 10 card and scrolls overflowing rows inside the table card', () => {
    const { container } = render(
      <TeamMemberMetricsTable
        groups={[testingGroup()]}
        role="testing"
        onRoleChange={vi.fn()}
      />,
    )

    const layoutClass = container.querySelector('[data-slot="team-member-metrics-layout"]')?.getAttribute('class') || ''
    expect(layoutClass).toContain('xl:h-[28rem]')
    expect(layoutClass).toContain('xl:items-stretch')

    const scrollClass = container.querySelector('[data-slot="team-member-metrics-table-scroll"]')?.getAttribute('class') || ''
    expect(scrollClass).toContain('xl:h-full')
    expect(scrollClass).toContain('xl:overflow-hidden')

    const tableClass = container.querySelector('[data-slot="data-table"]')?.getAttribute('class') || ''
    expect(tableClass).toContain('xl:h-full')
    expect(tableClass).toContain('[&_[data-slot=table-scroll-shell]]:h-full')
    expect(tableClass).not.toContain('[&_[data-slot=table-container]]')

    const tableContainerClass = container.querySelector('[data-slot="table-container"]')?.getAttribute('class') || ''
    expect(tableContainerClass).toContain('pm-data-table-scroll-area')
    expect(tableContainerClass).toContain('xl:h-full')
  })
})
