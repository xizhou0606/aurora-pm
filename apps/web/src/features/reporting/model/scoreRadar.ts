import { readThemePaint, withAlpha } from '@/shared/ui'

import type { ReportScores } from './types'

const FONT = 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", sans-serif'

/**
 * 汇报评分雷达：五维 0-100 态势多边形，形状即短板——凹进去的轴就是
 * 本期交付的拖后腿环节。颜色走共享主题解析，light/dark 自动跟随。
 */
export function scoreRadarOption(scores: ReportScores): Readonly<Record<string, unknown>> | null {
  const dimensions = scores.dimensions
  if (dimensions.length === 0) return null
  const paint = readThemePaint()
  const series = paint.chartSeries[0]
  return {
    animationDuration: 450,
    textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    aria: { enabled: true, description: `${scores.overall} 分`, decal: { show: false } },
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: paint.card,
      borderColor: paint.border,
      borderWidth: 1,
      padding: [8, 10],
      textStyle: { color: paint.foreground, fontFamily: FONT, fontSize: 12 },
    },
    radar: {
      center: ['50%', '52%'],
      radius: '68%',
      splitNumber: 4,
      indicator: dimensions.map((dimension) => ({ name: dimension.label, max: 100 })),
      axisName: { color: paint.muted, fontSize: 12, fontWeight: 600 },
      axisLine: { lineStyle: { color: paint.chartGrid } },
      splitLine: { lineStyle: { color: paint.chartGrid } },
      splitArea: { show: false },
    },
    series: [{
      type: 'radar',
      symbol: 'circle',
      symbolSize: 6,
      cursor: 'default',
      data: [{
        name: '综合',
        value: dimensions.map((dimension) => dimension.score),
      }],
      lineStyle: { color: series, width: 2 },
      itemStyle: { color: series, borderColor: paint.border, borderWidth: 1 },
      areaStyle: { color: withAlpha(series, 0.16) },
      emphasis: { focus: 'series' },
    }],
  }
}
