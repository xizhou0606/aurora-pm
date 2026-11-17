import { BI_SCREEN_COPY } from '@/shared/copy'

import type { ChartModel, DashboardData } from '../types'
import { axisTooltip, base, COLORS, safeCount, valueAxis, withAlpha } from './core'

function areaGradient(color: string) {
  return {
    type: 'linear',
    x: 0,
    y: 0,
    x2: 0,
    y2: 1,
    colorStops: [
      { offset: 0, color: withAlpha(color, 0.16) },
      { offset: 1, color: withAlpha(color, 0.02) },
    ],
  }
}

export function trendChart(data: DashboardData): ChartModel {
  const copy = BI_SCREEN_COPY.charts.trend
  const seriesCopy = BI_SCREEN_COPY.charts.series
  const trend = data.completion_trend
  const maximum = Math.max(
    1,
    ...trend.flatMap((item) => [safeCount(item.created), safeCount(item.completed)]),
  )
  return {
    id: 'trend',
    title: copy.title,
    description: copy.description,
    meta: copy.meta(trend.reduce((total, item) => total + item.completed, 0).toLocaleString('zh-CN')),
    ariaLabel: copy.ariaLabel,
    emptyTitle: copy.emptyTitle,
    emptyDescription: copy.emptyDescription,
    option: trend.length ? {
      ...base(copy.ariaLabel),
      tooltip: {
        ...axisTooltip(),
        axisPointer: { type: 'line', lineStyle: { color: COLORS.border } },
      },
      legend: {
        top: 0,
        right: 0,
        itemWidth: 12,
        itemHeight: 3,
        textStyle: { color: COLORS.subtle, fontSize: 11 },
      },
      grid: { left: 8, right: 58, top: 38, bottom: 12, containLabel: true },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: trend.map((item) => item.label),
        axisLine: { lineStyle: { color: COLORS.border } },
        axisTick: { show: false },
        axisLabel: { color: COLORS.subtle, fontSize: 11, hideOverlap: true },
      },
      yAxis: valueAxis(maximum),
      series: [
        {
          name: seriesCopy.created,
          type: 'line',
          smooth: 0.2,
          showSymbol: false,
          data: trend.map((item) => safeCount(item.created)),
          lineStyle: { color: COLORS.blue, width: 2.5 },
          itemStyle: { color: COLORS.blue },
          areaStyle: areaGradient(COLORS.blue),
          markPoint: trend.some((item) => safeCount(item.created) > 0) ? {
            silent: true,
            symbol: 'circle',
            symbolSize: 7,
            itemStyle: { color: COLORS.blue },
            label: {
              show: true,
              position: 'top',
              distance: 7,
              formatter: (params: { value: number }) => BI_SCREEN_COPY.charts.peakLabel(safeCount(params.value)),
              color: COLORS.subtle,
              fontSize: 11,
              fontWeight: 600,
            },
            data: [{ type: 'max' }],
          } : undefined,
          emphasis: { focus: 'series' },
        },
        {
          name: seriesCopy.completed,
          type: 'line',
          smooth: 0.2,
          showSymbol: false,
          data: trend.map((item) => safeCount(item.completed)),
          lineStyle: { color: COLORS.teal, width: 2.5 },
          itemStyle: { color: COLORS.teal },
          areaStyle: areaGradient(COLORS.teal),
          endLabel: {
            show: true,
            formatter: (params: { value: number }) => (safeCount(params.value) > 0 ? String(safeCount(params.value)) : ''),
            color: COLORS.teal,
            fontSize: 12,
            fontWeight: 650,
            distance: 8,
          },
          emphasis: { focus: 'series' },
        },
      ],
    } : null,
    summaryCaption: copy.summaryCaption,
    summaryColumns: [BI_SCREEN_COPY.chart.category, seriesCopy.created, seriesCopy.completed],
    summaryRows: trend.map((item) => [item.label, item.created, item.completed]),
    points: trend.map((item) => ({
      label: item.label,
      values: [
        { series: seriesCopy.created, value: item.created },
        { series: seriesCopy.completed, value: item.completed },
      ],
    })),
    layout: 'wide',
  }
}
