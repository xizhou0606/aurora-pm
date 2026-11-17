import { describe, expect, it } from 'bun:test'
import * as echarts from 'echarts/core'
import { BarChart, LineChart, RadarChart, ScatterChart } from 'echarts/charts'
import {
  AriaComponent,
  GridComponent,
  LegendComponent,
  MarkPointComponent,
  PolarComponent,
  TooltipComponent,
} from 'echarts/components'
import { SVGRenderer } from 'echarts/renderers'

import { dashboardFixture } from '../utils/testFixtures'
import { overviewModels } from './main'
import { completionCapsuleModel, ownerLoadBarModel, versionQualityMatrixModel } from './ranking'
import { qualityTrendChart } from './quality'
import { trendChart } from './trend'

echarts.use([
  BarChart,
  LineChart,
  RadarChart,
  ScatterChart,
  AriaComponent,
  GridComponent,
  LegendComponent,
  MarkPointComponent,
  PolarComponent,
  TooltipComponent,
  SVGRenderer,
])

function renderHeadless(option: unknown): string {
  const instance = echarts.init(null, null, {
    renderer: 'svg',
    ssr: true,
    width: 900,
    height: 320,
  } as Parameters<typeof echarts.init>[2])
  try {
    instance.setOption(option as Record<string, unknown>)
    return instance.renderToSVGString()
  } finally {
    instance.dispose()
  }
}

describe('headless chart options render', () => {
  it('renders the version capsule option', () => {
    const svg = renderHeadless(overviewModels(dashboardFixture()).version.option)
    expect(svg.length).toBeGreaterThan(0)
  })

  it('renders the requirement capsule option', () => {
    const svg = renderHeadless(overviewModels(dashboardFixture()).requirement.option)
    expect(svg.length).toBeGreaterThan(0)
  })

  it('renders the owner load bar option', () => {
    const svg = renderHeadless(overviewModels(dashboardFixture()).owner.option)
    expect(svg.length).toBeGreaterThan(0)
  })

  it('renders the quality flow option', () => {
    const svg = renderHeadless(qualityTrendChart(dashboardFixture()).option)
    expect(svg.length).toBeGreaterThan(0)
  })

  it('renders the risk matrix option', () => {
    const svg = renderHeadless(versionQualityMatrixModel(dashboardFixture().version_quality_risks).option)
    expect(svg.length).toBeGreaterThan(0)
  })

  it('renders the command health radar option', () => {
    const svg = renderHeadless(overviewModels(dashboardFixture()).healthRadar.option)
    expect(svg.length).toBeGreaterThan(0)
  })

  it('renders the trend option', () => {
    const svg = renderHeadless(trendChart(dashboardFixture()).option)
    expect(svg.length).toBeGreaterThan(0)
  })
})
