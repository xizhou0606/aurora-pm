import { BI_SCREEN_COPY } from '@/shared/copy'

import type { ChartModel, DashboardData } from '../types'
import { base, COLORS, itemTooltip, safeCount, withAlpha } from './core'

const clamp = (value: number) => Math.min(100, Math.max(0, Math.round(value)))

/**
 * 指挥健康度雷达：把总览指标归一成六维 0-100 态势多边形，
 * 形状即短板——凹进去的轴就是当前交付链的拖后腿环节。
 */
export function commandRadarModel(data: DashboardData): ChartModel {
  const copy = BI_SCREEN_COPY.charts.healthRadar
  const m = data.overview_metrics
  const openBase = Math.max(1, safeCount(m.done_tasks) + safeCount(m.open_bugs))
  const dims = [
    { name: copy.dims.delivery, value: clamp(m.completion_rate_pct) },
    {
      name: copy.dims.onTime,
      value: clamp(100 - (safeCount(m.overdue_tasks) / Math.max(1, safeCount(m.in_flight_tasks))) * 100),
    },
    {
      name: copy.dims.quality,
      value: clamp(100 - (safeCount(m.open_bugs) / openBase) * 100),
    },
    {
      name: copy.dims.blocker,
      value: clamp(100 - (safeCount(m.blocker_bugs) / Math.max(1, safeCount(m.open_bugs))) * 100),
    },
    {
      name: copy.dims.review,
      value: clamp(100 - (safeCount(m.pending_review_bugs) / Math.max(1, safeCount(m.open_bugs))) * 100),
    },
    {
      name: copy.dims.assignee,
      value: clamp(100 - (safeCount(m.unassigned_tasks) / Math.max(1, safeCount(m.total_tasks))) * 100),
    },
  ]
  const score = clamp(dims.reduce((total, dim) => total + dim.value, 0) / dims.length)
  return {
    id: 'health-radar',
    title: copy.title,
    description: copy.description,
    meta: copy.meta(score),
    ariaLabel: copy.ariaLabel,
    emptyTitle: copy.emptyTitle,
    emptyDescription: copy.emptyDescription,
    option: {
      ...base(copy.ariaLabel),
      tooltip: itemTooltip(),
      radar: {
        indicator: dims.map((dim) => ({ name: dim.name, max: 100 })),
        radius: '66%',
        center: ['50%', '52%'],
        splitNumber: 4,
        axisName: { color: COLORS.subtle, fontSize: 11 },
        axisLine: { lineStyle: { color: COLORS.grid } },
        splitLine: { lineStyle: { color: COLORS.grid } },
        splitArea: {
          areaStyle: { color: [withAlpha(COLORS.slate, 0.03), 'transparent'] },
        },
      },
      series: [{
        type: 'radar',
        symbol: 'circle',
        symbolSize: 5,
        cursor: 'default',
        data: [{
          value: dims.map((dim) => dim.value),
          name: copy.title,
        }],
        lineStyle: { color: COLORS.blue, width: 2 },
        itemStyle: { color: COLORS.blue, borderColor: COLORS.border, borderWidth: 1 },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: withAlpha(COLORS.blue, 0.38) },
              { offset: 1, color: withAlpha(COLORS.violet, 0.1) },
            ],
          },
        },
        emphasis: { focus: 'series' },
      }],
    },
    summaryCaption: copy.summaryCaption,
    summaryColumns: [BI_SCREEN_COPY.chart.dimension, BI_SCREEN_COPY.chart.value],
    summaryRows: dims.map((dim) => [dim.name, dim.value]),
    points: dims.map((dim) => ({
      label: dim.name,
      values: [{ series: copy.title, value: dim.value }],
    })),
  }
}
