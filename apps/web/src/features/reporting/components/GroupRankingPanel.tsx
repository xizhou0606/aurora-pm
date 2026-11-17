import { useMemo, useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'

import { PhArrowsDownUp, PhSortAscending, PhSortDescending } from '@/shared/ui/icons'
import { REPORTING_COPY } from '@/shared/copy'
import { DataTable, DataTableSortButton } from '@/shared/ui'
import { cn } from '@/shared/lib/utils'

import type {
  BugReportMetrics,
  CodeActivity,
  ReportDimensionComments,
  ReportingMemberRollup,
  RequirementMetrics,
  UIBugMetrics,
} from '../model/types'

// 组/部门汇报的成员排行：一张两级表头的聚合表 —— 一级标题把列分成
// 任务/质量/岗位焦点三组，二级标题是各指标列，点击即按该指标排序（箭头
// 显示在标题旁）。排名列随当前排序动态计算（第 1 行为第 1 名）。全部按
// 本期统计事实确定性排序，不经过 AI。
//
// 岗位焦点组按团队类型切换：测试团队展示提出 Bug 分档，产品团队展示需求
// 数量，设计团队展示 UI 缺陷处理，其余团队保持代码活动列。
type TeamType = 'default' | 'testing' | 'product' | 'design'

type SortKey =
  | 'completed'
  | 'taskOverdue'
  | 'resolved'
  | 'bugOverdue'
  | 'commits'
  | 'additions'
  | 'debugHits'
  | 'todoHits'
  | 'bugTotal'
  | 'bugP0'
  | 'bugP1'
  | 'bugP2'
  | 'reqTotal'
  | 'reqCreated'
  | 'reqDone'
  | 'reqActive'
  | 'uiResolved'
  | 'uiOpen'
  | 'uiOverdue'

type SortState = { key: SortKey; descending: boolean }

const requirementActive = (metrics: RequirementMetrics | undefined): number =>
  metrics ? metrics.development + metrics.acceptance : 0

const SORT_FACTORS: Record<SortKey, (rollup: ReportingMemberRollup) => number> = {
  completed: (rollup) => rollup.tasks.completed,
  taskOverdue: (rollup) => rollup.tasks.overdue,
  resolved: (rollup) => rollup.bugs.resolved,
  bugOverdue: (rollup) => rollup.bugs.overdue,
  commits: (rollup) => rollup.code?.commits ?? 0,
  additions: (rollup) => rollup.code?.additions ?? 0,
  debugHits: (rollup) => rollup.code?.debugHits ?? 0,
  todoHits: (rollup) => rollup.code?.todoHits ?? 0,
  bugTotal: (rollup) => rollup.bugReports?.total ?? 0,
  bugP0: (rollup) => rollup.bugReports?.p0 ?? 0,
  bugP1: (rollup) => rollup.bugReports?.p1 ?? 0,
  bugP2: (rollup) => rollup.bugReports?.p2 ?? 0,
  reqTotal: (rollup) => rollup.requirements?.total ?? 0,
  reqCreated: (rollup) => rollup.requirements?.createdInPeriod ?? 0,
  reqDone: (rollup) => rollup.requirements?.done ?? 0,
  reqActive: (rollup) => requirementActive(rollup.requirements),
  uiResolved: (rollup) => rollup.uiBugs?.resolved ?? 0,
  uiOpen: (rollup) => rollup.uiBugs?.open ?? 0,
  uiOverdue: (rollup) => rollup.uiBugs?.overdue ?? 0,
}

// 各团队类型的默认排序：按岗位核心指标降序，其余按完成任务降序。
const DEFAULT_SORT: Record<TeamType, SortState> = {
  default: { key: 'completed', descending: true },
  testing: { key: 'bugTotal', descending: true },
  product: { key: 'reqTotal', descending: true },
  design: { key: 'uiResolved', descending: true },
}

function compareByName(left: ReportingMemberRollup, right: ReportingMemberRollup): number {
  const byName = left.member.displayName.localeCompare(right.member.displayName, 'zh-Hans-CN')
  if (byName !== 0) return byName
  return left.member.id - right.member.id
}

function sortRollups(
  members: readonly ReportingMemberRollup[],
  sort: SortState,
): ReportingMemberRollup[] {
  const factor = SORT_FACTORS[sort.key]
  return [...members].sort((left, right) => {
    const leftValue = factor(left)
    const rightValue = factor(right)
    if (leftValue !== rightValue) return sort.descending ? rightValue - leftValue : leftValue - rightValue
    return compareByName(left, right)
  })
}

// 右对齐的指标数值：DataTable 的单元格内容由组件自行排版。
function MetricValue({ value, muted }: { value: number | string; muted?: boolean }) {
  return (
    <span className={`block text-right text-sm tabular-nums ${muted ? 'text-muted-foreground' : 'text-foreground'}`}>
      {value}
    </span>
  )
}

function DeltaValue({ additions, deletions }: { additions: number; deletions: number }) {
  const copy = REPORTING_COPY.ranking
  return (
    <span className="block text-right text-sm tabular-nums">
      <span className="text-emerald-600 dark:text-emerald-400">{copy.deltaPositive}{additions}</span>
      <span className="text-muted-foreground">{copy.deltaSeparator}</span>
      <span className="text-red-500 dark:text-red-400">{copy.deltaNegative}{deletions}</span>
    </span>
  )
}

// 二级标题 = 排序按钮：标题 + 方向箭头并排，点击切换排序。
function SortHeader({
  label,
  sortKey,
  sort,
  onSort,
}: {
  label: string
  sortKey: SortKey
  sort: SortState
  onSort: (key: SortKey) => void
}) {
  const copy = REPORTING_COPY.ranking
  const active = sort.key === sortKey
  return (
    <span className="flex justify-end">
      <DataTableSortButton
        aria-label={copy.sortAria(label, active ? (sort.descending ? 'descending' : 'ascending') : undefined)}
        onClick={() => onSort(sortKey)}
      >
        {label}
        {active
          ? sort.descending
            ? <PhSortDescending aria-hidden="true" />
            : <PhSortAscending aria-hidden="true" />
          : <PhArrowsDownUp aria-hidden="true" />}
      </DataTableSortButton>
    </span>
  )
}

export function GroupRankingPanel({
  members,
  code,
  bugReports,
  requirements,
  uiBugs,
  comments,
}: {
  members: readonly ReportingMemberRollup[]
  code?: CodeActivity
  // 岗位焦点团队的聚合块：出现其一即切换排行列组，并与底部汇总行对应。
  bugReports?: BugReportMetrics
  requirements?: RequirementMetrics
  uiBugs?: UIBugMetrics
  comments?: ReportDimensionComments
}) {
  const copy = REPORTING_COPY.ranking
  const teamType: TeamType = bugReports ? 'testing' : requirements ? 'product' : uiBugs ? 'design' : 'default'
  const [sort, setSort] = useState<SortState>(DEFAULT_SORT[teamType])
  const onSort = (key: SortKey) => {
    setSort((current) => current.key === key
      ? { key, descending: !current.descending }
      : { key, descending: true })
  }
  const rows = useMemo(() => sortRollups(members, sort), [members, sort])
  const codeCollected = useMemo(() => members.some((rollup) => rollup.code !== undefined), [members])

  // 岗位焦点列组：组内标签随团队类型切换（测试贡献/需求/设计/代码贡献）。
  const extraGroupLabel = teamType === 'testing' ? copy.testingGroup : teamType === 'product' ? copy.productGroup : teamType === 'design' ? copy.designGroup : copy.codeGroup

  const columns = useMemo<ColumnDef<ReportingMemberRollup>[]>(() => [
    {
      id: 'group-member',
      header: () => null,
      columns: [
        {
          id: 'rank',
          header: copy.rankColumn,
          enableHiding: false,
          cell: (context) => {
            const rank = context.row.index + 1
            return (
              <span
                className={
                  rank === 1
                    ? 'text-sm font-semibold text-accent'
                    : rank <= 3
                      ? 'text-sm font-semibold text-foreground'
                      : 'text-sm text-muted-foreground'
                }
              >
                {rank}
              </span>
            )
          },
        },
        {
          id: 'member',
          header: copy.memberColumn,
          enableHiding: false,
          cell: (context) => {
            const rollup = context.row.original
            return (
              <span className="flex min-w-0 items-center gap-1.5">
                {rollup.member.avatarUrl ? (
                  <img
                    src={rollup.member.avatarUrl}
                    alt=""
                    aria-hidden="true"
                    className="size-5 shrink-0 rounded-full object-cover"
                  />
                ) : null}
                <span className="truncate text-sm text-foreground">{rollup.member.displayName}</span>
              </span>
            )
          },
        },
      ],
    },
    {
      id: 'group-tasks',
      header: () => <span className="block text-center text-xs font-medium text-muted-foreground">{copy.tasksGroup}</span>,
      columns: [
        {
          id: 'completed',
          header: () => <SortHeader label={copy.completedColumn} sortKey="completed" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.tasks.completed} />,
        },
        {
          id: 'taskOverdue',
          header: () => <SortHeader label={copy.taskOverdueColumn} sortKey="taskOverdue" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.tasks.overdue} muted />,
        },
      ],
    },
    {
      id: 'group-quality',
      header: () => <span className="block text-center text-xs font-medium text-muted-foreground">{copy.qualityGroup}</span>,
      columns: [
        {
          id: 'resolved',
          header: () => <SortHeader label={copy.resolvedColumn} sortKey="resolved" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.bugs.resolved} />,
        },
        {
          id: 'bugOverdue',
          header: () => <SortHeader label={copy.bugOverdueColumn} sortKey="bugOverdue" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.bugs.overdue} muted />,
        },
      ],
    },
    ...(teamType === 'testing' ? [{
      id: 'group-testing',
      header: () => <span className="block text-center text-xs font-medium text-muted-foreground">{copy.testingGroup}</span>,
      columns: [
        {
          id: 'bugTotal',
          header: () => <SortHeader label={copy.bugTotalColumn} sortKey="bugTotal" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.bugReports?.total ?? 0} />,
        },
        {
          id: 'bugP0',
          header: () => <SortHeader label={copy.bugP0Column} sortKey="bugP0" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.bugReports?.p0 ?? 0} />,
        },
        {
          id: 'bugP1',
          header: () => <SortHeader label={copy.bugP1Column} sortKey="bugP1" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.bugReports?.p1 ?? 0} />,
        },
        {
          id: 'bugP2',
          header: () => <SortHeader label={copy.bugP2Column} sortKey="bugP2" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.bugReports?.p2 ?? 0} />,
        },
      ],
    } satisfies ColumnDef<ReportingMemberRollup>] : []),
    ...(teamType === 'product' ? [{
      id: 'group-product',
      header: () => <span className="block text-center text-xs font-medium text-muted-foreground">{copy.productGroup}</span>,
      columns: [
        {
          id: 'reqTotal',
          header: () => <SortHeader label={copy.reqTotalColumn} sortKey="reqTotal" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.requirements?.total ?? 0} />,
        },
        {
          id: 'reqCreated',
          header: () => <SortHeader label={copy.reqCreatedColumn} sortKey="reqCreated" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.requirements?.createdInPeriod ?? 0} />,
        },
        {
          id: 'reqDone',
          header: () => <SortHeader label={copy.reqDoneColumn} sortKey="reqDone" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.requirements?.done ?? 0} />,
        },
        {
          id: 'reqActive',
          header: () => <SortHeader label={copy.reqActiveColumn} sortKey="reqActive" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={requirementActive(context.row.original.requirements)} />,
        },
      ],
    } satisfies ColumnDef<ReportingMemberRollup>] : []),
    ...(teamType === 'design' ? [{
      id: 'group-design',
      header: () => <span className="block text-center text-xs font-medium text-muted-foreground">{copy.designGroup}</span>,
      columns: [
        {
          id: 'uiResolved',
          header: () => <SortHeader label={copy.uiResolvedColumn} sortKey="uiResolved" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.uiBugs?.resolved ?? 0} />,
        },
        {
          id: 'uiOpen',
          header: () => <SortHeader label={copy.uiOpenColumn} sortKey="uiOpen" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.uiBugs?.open ?? 0} />,
        },
        {
          id: 'uiOverdue',
          header: () => <SortHeader label={copy.uiOverdueColumn} sortKey="uiOverdue" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.uiBugs?.overdue ?? 0} />,
        },
      ],
    } satisfies ColumnDef<ReportingMemberRollup>] : []),
    ...(teamType === 'default' ? [{
      id: 'group-code',
      header: () => <span className="block text-center text-xs font-medium text-muted-foreground">{copy.codeGroup}</span>,
      columns: [
        {
          id: 'commits',
          header: () => <SortHeader label={copy.commitsColumn} sortKey="commits" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.code?.commits ?? copy.noCodeMark} muted={!context.row.original.code} />,
        },
        {
          id: 'additions',
          header: () => <SortHeader label={copy.linesColumn} sortKey="additions" sort={sort} onSort={onSort} />,
          cell: (context) => {
            const rollup = context.row.original
            return rollup.code
              ? <DeltaValue additions={rollup.code.additions} deletions={rollup.code.deletions} />
              : <MetricValue value={copy.noCodeMark} muted />
          },
        },
        {
          id: 'debugHits',
          header: () => <SortHeader label={copy.debugHitsColumn} sortKey="debugHits" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.code?.debugHits ?? copy.noCodeMark} muted={!context.row.original.code} />,
        },
        {
          id: 'todoHits',
          header: () => <SortHeader label={copy.todoColumn} sortKey="todoHits" sort={sort} onSort={onSort} />,
          cell: (context) => <MetricValue value={context.row.original.code?.todoHits ?? copy.noCodeMark} muted={!context.row.original.code} />,
        },
      ],
    } satisfies ColumnDef<ReportingMemberRollup>] : []),
  ], [copy, sort, teamType])

  // 第三段维度短评的数据键固定为 code，标签随团队类型切换（测试贡献/需求
  // 贡献），与排行列组的语义对应。
  const commentLines: { label: string; text: string }[] = ([
    { label: copy.tasksGroup, text: comments?.tasks },
    { label: copy.qualityGroup, text: comments?.quality },
    { label: extraGroupLabel, text: comments?.code },
  ] as { label: string; text?: string }[]).flatMap((line) => (line.text ? [{ label: line.label, text: line.text }] : []))

  return (
    <div className="min-w-0 space-y-2" aria-label={copy.title}>
      {commentLines.length > 0 ? (
        <div className="space-y-1 rounded-md border border-border bg-card px-3 py-2" role="note">
          {commentLines.map((line) => (
            <p key={line.label} className="text-xs leading-5">
              <span className="font-medium text-muted-foreground">{copy.aiCommentLabel(line.label)}</span>
              <span className="text-foreground">{line.text}</span>
            </p>
          ))}
        </div>
      ) : null}
      <DataTable
        columns={columns}
        state={members.length === 0
          ? { status: 'empty', title: copy.emptyTitle }
          : { status: 'ready', rows }}
        getRowId={(rollup) => String(rollup.member.id)}
        aria-label={copy.title}
        className={cn(
          // 分组分隔线：一级标题与每组首列画左边框，让二级标题的归属一目了然；
          // 一级标题行加浅色底与居中文案区分层级。
          '[&_th[data-column=group-tasks]]:border-l',
          '[&_th[data-column=group-quality]]:border-l',
          teamType === 'testing'
            ? '[&_th[data-column=group-testing]]:border-l'
            : teamType === 'product'
              ? '[&_th[data-column=group-product]]:border-l'
              : teamType === 'design'
                ? '[&_th[data-column=group-design]]:border-l'
                : '[&_th[data-column=group-code]]:border-l',
          '[&_th[data-column=completed]]:border-l [&_td[data-column=completed]]:border-l',
          '[&_th[data-column=resolved]]:border-l [&_td[data-column=resolved]]:border-l',
          teamType === 'testing'
            ? '[&_th[data-column=bugTotal]]:border-l [&_td[data-column=bugTotal]]:border-l'
            : teamType === 'product'
              ? '[&_th[data-column=reqTotal]]:border-l [&_td[data-column=reqTotal]]:border-l'
              : teamType === 'design'
                ? '[&_th[data-column=uiResolved]]:border-l [&_td[data-column=uiResolved]]:border-l'
                : '[&_th[data-column=commits]]:border-l [&_td[data-column=commits]]:border-l',
          '[&_thead>tr:first-child>th]:bg-muted/40',
        )}
      />
      <div className="space-y-1">
        {teamType === 'default' && code && codeCollected ? (
          <p className="text-xs text-muted-foreground">
            {copy.codeSummary(code.repos, code.commits, code.additions, code.deletions)}
          </p>
        ) : null}
        {teamType === 'testing' && bugReports ? (
          <p className="text-xs text-muted-foreground">
            {copy.bugSummary(bugReports.total, bugReports.p0, bugReports.p1, bugReports.p2)}
          </p>
        ) : null}
        {teamType === 'product' && requirements ? (
          <p className="text-xs text-muted-foreground">
            {copy.requirementSummary(requirements.total, requirements.createdInPeriod, requirements.done)}
          </p>
        ) : null}
        {teamType === 'design' && uiBugs ? (
          <p className="text-xs text-muted-foreground">
            {copy.uiBugSummary(uiBugs.resolved, uiBugs.open, uiBugs.overdue)}
          </p>
        ) : null}
        {code?.freshness.degraded ? (
          <p className="text-xs text-muted-foreground">{REPORTING_COPY.code.degraded(code.freshness.error)}</p>
        ) : null}
        <p className="text-xs text-muted-foreground">{copy.footnote}</p>
      </div>
    </div>
  )
}
