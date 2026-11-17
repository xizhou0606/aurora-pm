import { BI_SCREEN_COPY } from '@/shared/copy'

import type { ChartModel, OwnerBoardItem, ScopeProgressItem, VersionQualityRiskItem } from '../types'
import { base, categoryAxis, COLORS, itemTooltip, safeCount, valueAxis, withAlpha } from './core'

const MAX_RANKED_ROWS = 10

function rankedRows<T>(items: readonly T[], workload: (item: T) => number, cap = MAX_RANKED_ROWS): T[] {
  return items
    .map((item, index) => ({ item, index }))
    .sort((left, right) => (
      safeCount(workload(right.item)) - safeCount(workload(left.item))
      || left.index - right.index
    ))
    .slice(0, cap)
    .map(({ item }) => item)
}

function truncate(value: string | number, maxLength = 10) {
  const characters = Array.from(String(value ?? ''))
  return characters.length > maxLength
    ? `${characters.slice(0, maxLength).join('')}…`
    : characters.join('')
}

function completionTone(pct: number) {
  if (pct >= 80) return COLORS.green
  if (pct >= 45) return COLORS.blue
  if (pct > 0) return COLORS.amber
  return COLORS.slate
}

/**
 * Completion capsules: one rounded track per scope with the percent rendered
 * inside the filled segment and a red diamond flagging overdue rows. Same
 * capsule language as the composition band, chart-native, no trailing text.
 */
export function completionCapsuleModel(
  id: 'version' | 'requirement',
  items: readonly ScopeProgressItem[],
): ChartModel {
  const copy = BI_SCREEN_COPY.charts
  const scoped = id === 'version' ? copy.version : copy.requirement
  const visible = rankedRows(items, (item) => item.total_tasks)
    .sort((left, right) => safeCount(right.done_rate_pct) - safeCount(left.done_rate_pct))
  const capsules = visible.map((item) => {
    const pct = Math.min(100, Math.max(0, Math.round(safeCount(item.done_rate_pct))))
    return { item, pct }
  })
  return {
    id,
    title: scoped.title,
    description: scoped.description,
    meta: scoped.meta(items.length),
    ariaLabel: scoped.ariaLabel,
    emptyTitle: scoped.emptyTitle,
    emptyDescription: scoped.emptyDescription,
    option: capsules.length ? {
      ...base(scoped.ariaLabel),
      tooltip: itemTooltip(),
      grid: { left: 4, right: 40, top: 10, bottom: 8, containLabel: true },
      xAxis: {
        type: 'value',
        max: 100,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { show: false },
        splitLine: { show: false },
      },
      yAxis: categoryAxis(capsules.map(({ item }) => item.label), 14),
      series: [
        {
          name: copy.series.remaining,
          type: 'bar',
          silent: true,
          cursor: 'default',
          barWidth: 12,
          itemStyle: { color: withAlpha(COLORS.slate, 0.18), borderRadius: 6 },
          data: capsules.map(() => 100),
          z: 1,
        },
        {
          name: copy.series.completionRate,
          type: 'bar',
          cursor: 'pointer',
          barWidth: 12,
          barGap: '-100%',
          itemStyle: { borderRadius: 6 },
          data: capsules.map(({ pct }) => ({
            value: pct,
            itemStyle: { color: completionTone(pct) },
            label: {
              show: true,
              position: pct >= 18 ? 'insideRight' : 'right',
              distance: 6,
              formatter: `${pct}%`,
              color: pct >= 18 ? '#ffffff' : COLORS.subtle,
              fontSize: 11,
              fontWeight: 600,
            },
          })),
          emphasis: { focus: 'series' },
          z: 2,
        },
        {
          name: copy.series.overdue,
          type: 'scatter',
          symbol: 'diamond',
          symbolSize: 9,
          silent: true,
          cursor: 'default',
          itemStyle: { color: COLORS.red },
          data: capsules
            .filter(({ item }) => safeCount(item.overdue_tasks) > 0)
            .map(({ item }, index) => ({
              value: [100, index],
              overdue: safeCount(item.overdue_tasks),
            })),
          z: 3,
        },
      ],
    } : null,
    summaryCaption: scoped.summaryCaption,
    summaryColumns: [
      BI_SCREEN_COPY.chart.category,
      copy.series.done,
      copy.series.remaining,
      copy.series.overdue,
    ],
    summaryRows: items.map((item) => [
      item.label,
      safeCount(item.done_tasks),
      Math.max(0, safeCount(item.total_tasks) - safeCount(item.done_tasks)),
      safeCount(item.overdue_tasks),
    ]),
    points: capsules.map(({ item }) => ({
      label: item.label,
      values: [
        { series: copy.series.done, value: safeCount(item.done_tasks) },
        { series: copy.series.remaining, value: Math.max(0, safeCount(item.total_tasks) - safeCount(item.done_tasks)) },
      ],
      drilldown: id === 'version'
        ? { domain: 'tasks', scope: { projectIds: [item.id], requirementIds: [] } }
        : { domain: 'tasks', scope: { requirementIds: [item.id] } },
    })),
  }
}

/**
 * Owner load as ranked load bars: red segment = overdue, soft green = the
 * rest of the in-hand load, done/total rendered inside the bar. Duplicate
 * display names are aggregated so every owner owns exactly one row.
 */
export function ownerLoadBarModel(items: readonly OwnerBoardItem[]): ChartModel {
  const copy = BI_SCREEN_COPY.charts
  const assigned = rankedRows(
    items.filter((item) => Boolean(item.owner_id)),
    (item) => item.total_tasks,
  )
  const unassigned = items.find((item) => !item.owner_id)
  const ordered = [...assigned, ...(unassigned ? [unassigned] : [])]
  const byName = new Map<string, { total: number; overdue: number; done: number; unassigned: boolean }>()
  for (const item of ordered) {
    const key = item.owner_name
    const merged = byName.get(key) ?? { total: 0, overdue: 0, done: 0, unassigned: !item.owner_id }
    merged.total += safeCount(item.total_tasks)
    merged.overdue += safeCount(item.overdue_tasks)
    merged.done += safeCount(item.done_tasks)
    byName.set(key, merged)
  }
  const rows = [...byName.entries()].map(([name, load]) => ({ name, ...load }))
  const maximum = Math.max(1, ...rows.map((row) => row.total))
  return {
    id: 'owner',
    title: copy.owner.title,
    description: copy.owner.description,
    meta: copy.owner.meta(items.length),
    ariaLabel: copy.owner.ariaLabel,
    emptyTitle: copy.owner.emptyTitle,
    emptyDescription: copy.owner.emptyDescription,
    option: rows.length ? {
      ...base(copy.owner.ariaLabel),
      tooltip: {
        ...itemTooltip(),
        formatter: (params: { name: string; value: number; seriesName: string }) => {
          const row = rows.find((candidate) => candidate.name === params.name)
          if (!row) return params.name
          const metric = params.seriesName === copy.series.overdue ? row.overdue : row.total - row.overdue
          return `${params.name}<br/>${copy.series.total} ${row.total} · ${copy.series.overdue} ${row.overdue} · ${copy.series.done} ${row.done} · ${params.seriesName} ${metric}`
        },
      },
      legend: {
        top: 0,
        right: 0,
        itemWidth: 8,
        itemHeight: 8,
        icon: 'circle',
        selectedMode: false,
        data: [copy.series.overdue, copy.series.normal],
        textStyle: { color: COLORS.subtle, fontSize: 11 },
      },
      grid: { left: 4, right: 40, top: 30, bottom: 8, containLabel: true },
      xAxis: {
        type: 'value',
        max: Math.ceil(maximum * 1.04),
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { show: false },
        splitLine: { show: false },
      },
      yAxis: categoryAxis(rows.map((row) => row.name), 8),
      series: [
        {
          name: copy.series.overdue,
          type: 'bar',
          stack: 'load',
          cursor: 'default',
          barWidth: 12,
          itemStyle: { color: COLORS.red, borderRadius: [6, 0, 0, 6] },
          data: rows.map((row) => row.overdue),
          z: 2,
        },
        {
          name: copy.series.normal,
          type: 'bar',
          stack: 'load',
          cursor: 'default',
          barWidth: 12,
          itemStyle: { color: withAlpha(COLORS.green, 0.45), borderRadius: [0, 6, 6, 0] },
          data: rows.map((row) => {
            const inside = row.total >= maximum * 0.34
            return {
              value: Math.max(0, row.total - row.overdue),
              itemStyle: row.unassigned ? { color: withAlpha(COLORS.slate, 0.35) } : undefined,
              label: {
                show: true,
                position: inside ? 'insideRight' : 'right',
                distance: 6,
                formatter: copy.race.doneCaption(
                  row.done.toLocaleString('zh-CN'),
                  row.total.toLocaleString('zh-CN'),
                ),
                color: row.unassigned
                  ? COLORS.subtle
                  : inside ? '#ffffff' : COLORS.subtle,
                fontSize: 11,
              },
            }
          }),
          z: 1,
        },
      ],
    } : null,
    summaryCaption: copy.owner.summaryCaption,
    summaryColumns: [BI_SCREEN_COPY.chart.category, copy.series.overdue, copy.series.normal],
    summaryRows: items.map((item) => [
      item.owner_name,
      safeCount(item.overdue_tasks),
      safeCount(item.normal_tasks),
    ]),
    points: ordered.map((item) => ({
      label: item.owner_name,
      values: [
        { series: copy.series.overdue, value: safeCount(item.overdue_tasks) },
        { series: copy.series.normal, value: safeCount(item.normal_tasks) },
      ],
    })),
  }
}

/**
 * Version quality as a risk matrix: x = open bugs, y = high severity, dot
 * size = bug volume, color = risk level. The top-right corner is the danger
 * zone, so severity reads as position instead of stacked bar text.
 */
export function versionQualityMatrixModel(risks: readonly VersionQualityRiskItem[]): ChartModel {
  const copy = BI_SCREEN_COPY.charts
  const toneFor = (level: string) => {
    if (level === 'high') return COLORS.red
    if (level === 'medium') return COLORS.amber
    return COLORS.slate
  }
  const xMaximum = Math.max(1, ...risks.map((item) => safeCount(item.open_bugs)))
  const yMaximum = Math.max(1, ...risks.map((item) => safeCount(item.high_severity_bugs)))
  return {
    id: 'version-quality',
    title: copy.versionQuality.title,
    description: copy.versionQuality.description,
    meta: copy.versionQuality.meta(risks.length),
    ariaLabel: copy.versionQuality.ariaLabel,
    emptyTitle: copy.versionQuality.emptyTitle,
    emptyDescription: copy.versionQuality.emptyDescription,
    option: risks.length ? {
      ...base(copy.versionQuality.ariaLabel),
      tooltip: itemTooltip(),
      grid: { left: 8, right: 30, top: 30, bottom: 8, containLabel: true },
      xAxis: {
        ...valueAxis(xMaximum),
        name: copy.series.open,
        nameGap: 6,
        nameTextStyle: { color: COLORS.subtle, fontSize: 11 },
      },
      yAxis: {
        type: 'value',
        min: 0,
        max: yMaximum + 1,
        minInterval: 1,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { color: COLORS.subtle, fontSize: 11 },
        splitLine: { lineStyle: { color: COLORS.grid } },
      },
      series: [{
        name: copy.versionQuality.title,
        type: 'scatter',
        cursor: 'pointer',
        symbolSize: (value: unknown, params: { data: { volume: number } }) =>
          12 + Math.min(10, Math.round(params.data.volume * 0.9)),
        data: risks.map((item) => ({
          name: item.version_name,
          value: [safeCount(item.open_bugs), safeCount(item.high_severity_bugs)],
          volume: safeCount(item.open_bugs),
          itemStyle: { color: toneFor(item.risk_level), opacity: 0.85 },
        })),
        label: {
          show: true,
          position: 'top',
          formatter: (params: { name: string }) => truncate(params.name, 12),
          color: COLORS.subtle,
          fontSize: 11,
        },
        emphasis: { focus: 'series' },
      }],
    } : null,
    summaryCaption: copy.versionQuality.summaryCaption,
    summaryColumns: [BI_SCREEN_COPY.chart.category, copy.series.open, copy.series.highSeverity],
    summaryRows: risks.map((item) => [
      item.version_name,
      safeCount(item.open_bugs),
      safeCount(item.high_severity_bugs),
    ]),
    points: risks.map((item) => ({
      label: item.version_name,
      values: [
        { series: copy.series.open, value: safeCount(item.open_bugs) },
        { series: copy.series.highSeverity, value: safeCount(item.high_severity_bugs) },
      ],
      drilldown: { domain: 'quality', scope: { projectIds: [item.version_id], requirementIds: [] } },
    })),
  }
}
