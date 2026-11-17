import { describe, expect, it } from 'bun:test'

import type { QualityDashboard } from '../../model/types'
import { buildQualityDashboardChartOptions, type QualityChartPaint } from './chart-options'

const dashboard: QualityDashboard = {
  summary: { submitted: 10, resolved: 4, closed: 3, backlog: 3 },
  byPerson: [{
    userId: 1,
    userName: '测试负责人',
    submitted: 10,
    resolved: 4,
    closed: 3,
    backlog: 3,
  }],
  byStatus: [
    { key: 'triage', label: '待确认', count: 2 },
    { key: 'closed', label: '已关闭', count: 8 },
  ],
  bySeverity: [
    { key: 'p2', label: '一般（P2）', count: 5 },
    { key: 'p3', label: '轻微（P3）', count: 5 },
  ],
}

const darkPaint: QualityChartPaint = {
  background: '#131316',
  card: '#1a1a1f',
  foreground: '#e9e9ee',
  muted: '#a4a4b0',
  border: '#28282f',
  primary: '#2563eb',
  chartGrid: '#232329',
  chartSeries: ['#6f92f2', '#46bd85', '#d3a24a', '#9c90ea', '#e5766c', '#949cab'],
  chartPositive: '#46bd85',
  chartNegative: '#e5766c',
}

describe('quality dashboard chart options', () => {
  it('uses semantic dark paint for canvas text, axes, tooltip, and separators', () => {
    const options = buildQualityDashboardChartOptions(dashboard, darkPaint, 'dark')

    expect(options.person).toMatchObject({
      textStyle: { color: darkPaint.foreground },
      tooltip: {
        backgroundColor: darkPaint.card,
        borderColor: darkPaint.border,
        textStyle: { color: darkPaint.foreground },
        axisPointer: { shadowStyle: { color: darkPaint.border } },
      },
      legend: { textStyle: { color: darkPaint.muted } },
      xAxis: {
        axisLabel: { color: darkPaint.muted },
        splitLine: { lineStyle: { color: darkPaint.border } },
      },
      yAxis: { axisLabel: { color: darkPaint.muted } },
    })
    expect(options.status).toMatchObject({
      title: {
        textStyle: { color: darkPaint.foreground },
        subtextStyle: { color: darkPaint.muted },
      },
      legend: { textStyle: { color: darkPaint.muted } },
      tooltip: {
        backgroundColor: darkPaint.card,
        borderColor: darkPaint.border,
        textStyle: { color: darkPaint.foreground },
      },
      series: [{
        itemStyle: { borderColor: darkPaint.card },
      }],
    })
  })

  it('resolves series colors from the shared chart tokens of the given paint', () => {
    const lightPaint: QualityChartPaint = {
      background: '#ffffff',
      card: '#ffffff',
      foreground: '#151820',
      muted: '#5d6675',
      border: 'rgba(20, 28, 42, 0.1)',
      primary: '#2563eb',
      chartGrid: 'rgba(20, 28, 42, 0.07)',
      chartSeries: ['#4373d1', '#1e9e63', '#bd8a1a', '#7466d1', '#cd4f45', '#8a93a6'],
      chartPositive: '#1e9e63',
      chartNegative: '#cd4f45',
    }
    const light = buildQualityDashboardChartOptions(dashboard, lightPaint, 'light')
    const dark = buildQualityDashboardChartOptions(dashboard, darkPaint, 'dark')

    expect(dark.person?.series[0].itemStyle.color).toBe(darkPaint.chartSeries[0])
    expect(dark.person?.series[1].itemStyle.color).toBe(darkPaint.chartSeries[3])
    expect(dark.person?.series[2].itemStyle.color).toBe(darkPaint.chartSeries[1])
    expect(dark.person?.series[3].itemStyle.color).toBe(darkPaint.chartSeries[2])
    expect(dark.status?.color[2]).toBe(darkPaint.chartSeries[0])
    expect(dark.severity?.color[0]).toBe(darkPaint.chartNegative)
    expect(light.person?.series[0].itemStyle.color).toBe(lightPaint.chartSeries[0])
    expect(light.status?.color[2]).toBe(lightPaint.chartSeries[0])
  })
})
