import { REPORTING_COPY } from '@/shared/copy'

import type { MemberReport, ReportingPeriodType } from './types'

export function periodTypeLabel(type: ReportingPeriodType): string {
  switch (type) {
    case 'weekly':
      return REPORTING_COPY.period.weekly
    case 'monthly':
      return REPORTING_COPY.period.monthly
    default:
      return REPORTING_COPY.period.daily
  }
}

export function isReportingPeriodType(value: string): value is ReportingPeriodType {
  return value === 'daily' || value === 'weekly' || value === 'monthly'
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

export function todayAnchorDate(): string {
  const now = new Date()
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}

/**
 * 日报默认锚点=昨天：定时报于每天 09:00 生成"前一日"日报并缓存，
 * 页面打开时默认落在昨天的锚点上即可直接读到已生成的汇报与 AI 评语。
 */
export function yesterdayAnchorDate(): string {
  const now = new Date()
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
  return `${yesterday.getFullYear()}-${pad(yesterday.getMonth() + 1)}-${pad(yesterday.getDate())}`
}

export function isValidAnchorDate(value: string): boolean {
  return /^\d{4}-\d{2}-\d{2}$/.test(value)
}

function shortDateTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return `${shortDate(iso)} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function shortDate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

export function periodRangeLabel(period: { start: string; end: string; type: ReportingPeriodType }): string {
  const start = shortDate(period.start)
  const end = shortDate(period.end)
  if (period.type === 'daily') return start
  // End is exclusive; the rendered range shows the covered day inclusive.
  const endDate = new Date(period.end)
  if (!Number.isNaN(endDate.getTime())) endDate.setDate(endDate.getDate() - 1)
  const inclusiveEnd = Number.isNaN(endDate.getTime()) ? end : shortDate(endDate.toISOString())
  return period.type === 'monthly' ? `${start.slice(0, 7)}` : `${start} ~ ${inclusiveEnd}`
}

function shortDay(iso?: string): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return `${date.getMonth() + 1}-${date.getDate()}`
}

/**
 * Deterministic professional report body in the standard weekly-review
 * structure (概览 / 已完成 / 进行中 / 阻碍与风险 / 下周计划 / 数据指标).
 * It renders instantly from PM facts — the AI narrative only augments it.
 */
const sectionNumber = (sections: string[], title: string): string =>
  `${['一', '二', '三', '四', '五', '六', '七', '八'][sections.length] ?? '•'}、${title}`

/**
 * Deterministic professional report body in the standard weekly-review
 * structure (概览 / 已完成 / 进行中 / 成员明细 / 阻碍与风险 / 下周计划 / 数据指标).
 * It renders instantly from PM facts — the AI narrative only augments it.
 * Group targets additionally carry a per-member section so 组内导出 covers
 * every member instead of only the aggregate.
 */
export function composeReportText(report: MemberReport): string {
  const c = REPORTING_COPY.summaryLines
  const headings: string[] = []
  const section = (title: string): string => {
    const heading = `## ${sectionNumber(headings, title)}`
    headings.push(heading)
    return heading
  }

  const isGroup = report.target.kind === 'group' || report.target.kind === 'department'
  const periodType = report.period.type
  const lines: string[] = [
    c.heading(
      periodTypeLabel(periodType),
      report.target.name,
      periodRangeLabel(report.period),
    ),
    '',
    c.cutoff(shortDateTime(report.generatedAt)),
    '',
    section(c.sectionOverview(periodType)),
    c.overviewAuto(
      report.tasks.completed, report.tasks.overdue, report.bugs.overdue, report.bugs.open,
    ),
    '',
  ]

  lines.push(section(c.sectionDone(periodType)))
  const completed = [
    ...report.tasks.completedItems.map((item) =>
      c.completedItem(item.key || `#${item.id}`, `${item.title}（${shortDay(item.completedAt)} 完成）`)),
    ...report.bugs.resolvedItems.map((item) =>
      c.resolvedItem(item.key || `#${item.id}`, item.title, item.severity)),
  ]
  lines.push(completed.length > 0 ? completed.join('\n') : c.none, '')

  lines.push(section('进行中'))
  const active = report.tasks.activeItems.map((item) =>
    c.activeItem(item.key || `#${item.id}`, item.title, item.status || '-'))
  lines.push(active.length > 0 ? active.join('\n') : c.none, '')

  if (isGroup && report.members && report.members.length > 0) {
    lines.push(section('成员明细'))
    lines.push(report.members.map((rollup) => c.memberRollup(
      rollup.member.displayName,
      rollup.tasks.completed,
      rollup.tasks.active,
      rollup.tasks.overdue,
      rollup.bugs.resolved,
      rollup.bugs.open,
    )).join('\n'))
    lines.push('')
  }

  lines.push(section('阻碍与风险'))
  const risks = [
    ...report.tasks.overdueItems.map((item) =>
      c.overdueItem(item.key || `#${item.id}`, item.title)),
    ...report.bugs.openItems.map((item) =>
      c.openBugItem(item.key || `#${item.id}`, item.title, item.severity)),
  ]
  lines.push(risks.length > 0 ? risks.join('\n') : c.riskFree, '')

  lines.push(section(c.sectionPlan(periodType)))
  lines.push(report.tasks.active > 0 ? c.planFromActive(report.tasks.active) : c.none)
  lines.push('', section('数据指标'))
  lines.push(c.tasks(
    report.tasks.completed, report.tasks.created, report.tasks.active, report.tasks.overdue,
  ))
  lines.push(c.bugs(
    report.bugs.resolved, report.bugs.closed, report.bugs.open, report.bugs.overdue,
  ))
  if (report.bugReports) {
    lines.push(c.bugReports(
      report.bugReports.total, report.bugReports.p0, report.bugReports.p1, report.bugReports.p2,
    ))
  }
  if (report.requirements) {
    lines.push(c.requirements(
      report.requirements.total, report.requirements.createdInPeriod, report.requirements.done,
      report.requirements.acceptance, report.requirements.development, report.requirements.pending,
    ))
  }
  if (report.uiBugs) {
    lines.push(c.uiBugs(report.uiBugs.resolved, report.uiBugs.open, report.uiBugs.overdue))
  }
  lines.push('', section(c.scoreTitle))
  const dimensions = report.scores.dimensions
  if (dimensions.length > 0) {
    lines.push(
      c.scoreOverall(report.scores.overall),
      '',
      ...dimensions.map((dimension) => c.scoreRow(dimension.label, dimension.score, dimension.basis)),
    )
  } else {
    lines.push(c.none)
  }
  return lines.join('\n')
}
