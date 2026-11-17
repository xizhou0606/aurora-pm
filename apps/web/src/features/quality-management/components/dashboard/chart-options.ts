import { QUALITY_COPY } from '@/shared/copy'
import { readThemePaint, withAlpha, type ResolvedAppearance } from '@/shared/ui'
import type { QualityDashboard } from '../../model/types'

// Series colors come from the shared chart semantic tokens; the mapping below
// only assigns token roles per data dimension (see design-tokens.css). Long
// dimensions fade trailing entries with alpha instead of adding new hues.
function dataColors(paint: ReturnType<typeof readThemePaint>) {
  const [blue, green, amber, violet, red, slate] = paint.chartSeries
  return {
    submitted: blue,
    resolved: violet,
    closed: green,
    backlog: amber,
    status: [slate, amber, blue, green, violet, red, withAlpha(slate, 0.6), withAlpha(blue, 0.6)] as const,
    severity: [red, amber, withAlpha(amber, 0.6), green, slate] as const,
  }
}

const FONT = 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", sans-serif'

export type QualityChartPaint = ReturnType<typeof readThemePaint>

function baseOption(description: string, paint: QualityChartPaint) {
  return {
    animation: true,
    animationDuration: 900,
    animationEasing: 'cubicOut',
    textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    aria: { enabled: true, description, decal: { show: false } },
  }
}

function tooltip(trigger: 'axis' | 'item', paint: QualityChartPaint) {
  return {
    trigger,
    confine: true,
    backgroundColor: paint.card,
    borderColor: paint.border,
    borderWidth: 1,
    padding: [8, 10],
    textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    extraCssText: 'box-shadow: var(--design-shadow-popover);',
    ...(trigger === 'axis'
      ? { axisPointer: { type: 'shadow', shadowStyle: { color: paint.border } } }
      : {}),
  }
}

function dimensionOption(
  rows: QualityDashboard['byStatus'],
  colors: readonly string[],
  ariaLabel: string,
  paint: QualityChartPaint,
) {
  const visibleRows = rows.filter((row) => row.count > 0)
  if (!visibleRows.length) return null
  const total = visibleRows.reduce((sum, row) => sum + row.count, 0)

  return {
    ...baseOption(ariaLabel, paint),
    color: colors,
    tooltip: tooltip('item', paint),
    title: {
      text: total.toLocaleString('zh-CN'),
      subtext: QUALITY_COPY.dashboard.count,
      left: 'center',
      top: '31%',
      textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 24, fontWeight: 650 },
      subtextStyle: { color: paint.muted, fontFamily: FONT, fontSize: 11, lineHeight: 18 },
    },
    legend: {
      type: 'scroll',
      left: 'center',
      bottom: 0,
      itemWidth: 8,
      itemHeight: 8,
      itemGap: 14,
      icon: 'circle',
      textStyle: { color: paint.muted, fontSize: 11 },
    },
    series: [{
      type: 'pie',
      radius: ['55%', '76%'],
      center: ['50%', '42%'],
      startAngle: 90,
      minAngle: 3,
      itemStyle: { borderColor: paint.card, borderWidth: 3, borderRadius: 4 },
      label: { show: false },
      labelLine: { show: false },
      emphasis: {
        scale: true,
        scaleSize: 4,
      },
      data: visibleRows.map((row) => ({ name: row.label, value: row.count })),
    }],
  }
}

export function buildQualityDashboardChartOptions(
  data: QualityDashboard,
  paint: QualityChartPaint,
  appearance: ResolvedAppearance,
) {
  const people = data.byPerson.slice(0, 10)
  const DATA_COLORS = dataColors(paint)
  const submittedColor = DATA_COLORS.submitted
  const statusColors = DATA_COLORS.status
  const person = people.length ? {
    ...baseOption(QUALITY_COPY.dashboard.personAria, paint),
    tooltip: tooltip('axis', paint),
    legend: {
      top: 0,
      right: 0,
      itemWidth: 8,
      itemHeight: 8,
      itemGap: 12,
      icon: 'circle',
      textStyle: { color: paint.muted, fontSize: 11 },
    },
    grid: { left: 4, right: 18, top: 48, bottom: 8, containLabel: true },
    xAxis: {
      type: 'value',
      min: 0,
      minInterval: 1,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: paint.muted, fontSize: 11, hideOverlap: true },
      splitLine: { lineStyle: { color: paint.border } },
    },
    yAxis: {
      type: 'category',
      inverse: true,
      data: people.map((row) => row.userName),
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: paint.muted,
        fontSize: 11,
        width: 72,
        overflow: 'truncate',
        margin: 10,
      },
    },
    series: [
      { name: QUALITY_COPY.dashboard.defectCount, type: 'bar', barMaxWidth: 7, itemStyle: { color: submittedColor, borderRadius: 2 }, data: people.map((row) => row.submitted) },
      { name: QUALITY_COPY.dashboard.resolved, type: 'bar', barMaxWidth: 7, itemStyle: { color: DATA_COLORS.resolved, borderRadius: 2 }, data: people.map((row) => row.resolved) },
      { name: QUALITY_COPY.dashboard.closed, type: 'bar', barMaxWidth: 7, itemStyle: { color: DATA_COLORS.closed, borderRadius: 2 }, data: people.map((row) => row.closed) },
      { name: QUALITY_COPY.dashboard.backlog, type: 'bar', barMaxWidth: 7, itemStyle: { color: DATA_COLORS.backlog, borderRadius: 2 }, data: people.map((row) => row.backlog) },
    ],
  } : null

  return {
    person,
    status: dimensionOption(data.byStatus, statusColors, QUALITY_COPY.dashboard.statusAria, paint),
    severity: dimensionOption(data.bySeverity, DATA_COLORS.severity, QUALITY_COPY.dashboard.severityAria, paint),
  }
}
