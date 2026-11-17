import { BI_SCREEN_COPY } from '@/shared/copy'

import type { ChartModel, DashboardData, QualityTrendPoint } from '../types'
import { base, axisTooltip, COLORS, safeCount, withAlpha } from './core'

/**
 * Defect flow: thin columns for daily created/resolved near the baseline with
 * the open-bug stock as a clean line above. No filled areas — a high stock
 * must never paint the panel red.
 */
function qualityTrendOption(items: readonly QualityTrendPoint[], description: string) {
  if (!items.length) return null
  const dailyCreated = items.map((item, index) => {
    if (index === 0) return 0
    return Math.max(
      0,
      safeCount(item.open_bugs)
        - safeCount(items[index - 1].open_bugs)
        + safeCount(item.resolved_bugs),
    )
  })
  const stockMaximum = Math.max(1, ...items.map((item) => safeCount(item.open_bugs)))
  return {
    ...base(description),
    tooltip: axisTooltip(),
    legend: {
      top: 8,
      right: 12,
      itemWidth: 10,
      itemHeight: 7,
      itemGap: 18,
      icon: 'roundRect',
      textStyle: { color: COLORS.subtle, fontSize: 11 },
    },
    grid: { left: 42, right: 58, top: 54, bottom: 32 },
    xAxis: {
      type: 'category',
      boundaryGap: true,
      data: items.map((item) => item.label),
      axisLine: { lineStyle: { color: COLORS.border } },
      axisTick: { show: false },
      axisLabel: { color: COLORS.subtle, fontSize: 11, hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: Math.ceil(stockMaximum * 1.15),
      minInterval: 1,
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: { color: COLORS.subtle, fontSize: 11 },
      splitLine: { lineStyle: { color: COLORS.grid } },
    },
    series: [
      {
        name: BI_SCREEN_COPY.charts.series.bugCreated,
        type: 'bar',
        barWidth: 5,
        cursor: 'pointer',
        itemStyle: { color: withAlpha(COLORS.red, 0.65), borderRadius: [3, 3, 0, 0] },
        data: dailyCreated,
      },
      {
        name: BI_SCREEN_COPY.charts.series.bugResolved,
        type: 'bar',
        barWidth: 5,
        cursor: 'pointer',
        itemStyle: { color: COLORS.green, borderRadius: [3, 3, 0, 0] },
        data: items.map((item) => safeCount(item.resolved_bugs)),
      },
      {
        name: BI_SCREEN_COPY.charts.series.bugStock,
        type: 'line',
        smooth: 0.25,
        symbol: 'none',
        cursor: 'pointer',
        lineStyle: { color: COLORS.red, width: 2.5 },
        itemStyle: { color: COLORS.red },
        endLabel: {
          show: true,
          formatter: (params: { value: number }) => (safeCount(params.value) > 0 ? String(safeCount(params.value)) : ''),
          color: COLORS.red,
          fontSize: 12,
          fontWeight: 650,
          distance: 8,
        },
        data: items.map((item) => safeCount(item.open_bugs)),
      },
    ],
  }
}

export function qualityRiskDelta(items: readonly QualityTrendPoint[]) {
  const value = items.length > 1
    ? safeCount(items[0].open_bugs) - safeCount(items[items.length - 1].open_bugs)
    : 0
  const copy = BI_SCREEN_COPY.charts.qualityTrend
  return value > 0
    ? { value: Math.abs(value), tone: 'positive' as const, label: copy.riskReducedLabel, meta: copy.riskReduced(Math.abs(value)) }
    : value < 0
      ? { value: Math.abs(value), tone: 'negative' as const, label: copy.riskIncreasedLabel, meta: copy.riskIncreased(Math.abs(value)) }
      : { value: 0, tone: 'neutral' as const, label: copy.riskUnchangedLabel, meta: copy.riskUnchanged(0) }
}

export function qualityTrendChart(data: DashboardData): ChartModel {
  const copy = BI_SCREEN_COPY.charts
  const trend = data.quality_trend
  return {
    id: 'quality-trend',
    title: copy.qualityTrend.title,
    description: copy.qualityTrend.description,
    meta: qualityRiskDelta(trend).meta,
    ariaLabel: copy.qualityTrend.ariaLabel,
    emptyTitle: copy.qualityTrend.emptyTitle,
    emptyDescription: copy.qualityTrend.emptyDescription,
    option: qualityTrendOption(trend, copy.qualityTrend.ariaLabel),
    summaryCaption: copy.qualityTrend.summaryCaption,
    summaryColumns: [
      BI_SCREEN_COPY.chart.category,
      copy.series.bugStock,
      copy.series.highSeverityBugs,
      copy.series.bugResolved,
    ],
    summaryRows: trend.map((item) => [
      item.label, item.open_bugs, item.high_severity_bugs, item.resolved_bugs,
    ]),
    points: trend.map((item) => ({
      label: item.label,
      values: [
        { series: copy.series.bugStock, value: item.open_bugs },
        { series: copy.series.highSeverityBugs, value: item.high_severity_bugs },
        { series: copy.series.bugResolved, value: item.resolved_bugs },
      ],
      drilldown: { domain: 'quality' },
    })),
    size: 'medium',
  }
}
