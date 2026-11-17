import { fireEvent, render } from '@testing-library/react'
import { act } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

const chartHarness = (() => {
  const chart = {
    dispatchAction: vi.fn(),
    dispose: vi.fn(),
    off: vi.fn(),
    on: vi.fn(),
    resize: vi.fn(),
    setOption: vi.fn(),
  }
  return {
    chart,
    init: vi.fn(() => chart),
    use: vi.fn(),
  }
})()

vi.mock('echarts/core', () => ({
  init: chartHarness.init,
  use: chartHarness.use,
}))
vi.mock('echarts/charts', () => ({
  BarChart: { id: 'bar' },
  LineChart: { id: 'line' },
  PieChart: { id: 'pie' },
  RadarChart: { id: 'radar-chart' },
  ScatterChart: { id: 'scatter' },
}))
vi.mock('echarts/components', () => ({
  AriaComponent: { id: 'aria' },
  GridComponent: { id: 'grid' },
  LegendComponent: { id: 'legend' },
  MarkAreaComponent: { id: 'mark-area' },
  MarkPointComponent: { id: 'mark-point' },
  PolarComponent: { id: 'polar' },
  RadarComponent: { id: 'radar-component' },
  TitleComponent: { id: 'title' },
  TooltipComponent: { id: 'tooltip' },
}))
vi.mock('echarts/renderers', () => ({
  CanvasRenderer: { id: 'canvas' },
}))

// Bun does not hoist mock registration ahead of static imports.
const { EChartRuntime } = await import('./echart-runtime')


const __stubPrev_ResizeObserver = (globalThis as any).ResizeObserver as unknown
const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
describe('EChart runtime lifecycle', () => {
  let resizeCallbacks: ResizeObserverCallback[]
  const observe = vi.fn()
  const disconnect = vi.fn()

  beforeEach(() => {
    chartHarness.init.mockClear()
    for (const method of Object.values(chartHarness.chart)) method.mockClear()
    observe.mockClear()
    disconnect.mockClear()
    resizeCallbacks = []
    ;(globalThis as any).ResizeObserver = class {
      constructor(callback: ResizeObserverCallback) {
        resizeCallbacks.push(callback)
      }

      observe = observe
      disconnect = disconnect
    }
    ;(globalThis as any).matchMedia = vi.fn(() => ({ matches: true }))
  })

  afterEach(() => {
    ;(globalThis as any).ResizeObserver = __stubPrev_ResizeObserver; (globalThis as any).matchMedia = __stubPrev_matchMedia
  })

  it('registers only the chart capabilities used by the BI screen', () => {
    expect(chartHarness.use).toHaveBeenCalledTimes(1)
    // 指挥大屏改版新增散点/标记点/极坐标能力后，注册清单同步到 15 项。
    expect(chartHarness.use.mock.calls[0][0]).toHaveLength(15)
    expect(chartHarness.use.mock.calls[0][0]).toContainEqual({ id: 'title' })
    expect(chartHarness.use.mock.calls[0][0]).toContainEqual({ id: 'scatter' })
  })

  it('resizes, disables motion, supports keyboard activation, and disposes cleanly', () => {
    const activate = vi.fn()
    const view = render(
      <EChartRuntime
        option={{ series: [{ type: 'bar', data: [1, 2, 3] }] }}
        ariaLabel="chart"
        pointCount={3}
        onDataPointActivate={activate}
      />,
    )

    const chart = view.getByRole('application')
    expect(chartHarness.init).toHaveBeenCalledWith(chart, undefined, { renderer: 'canvas' })
    expect(observe).toHaveBeenCalledWith(chart)
    expect(chartHarness.chart.setOption).toHaveBeenCalledWith(
      expect.objectContaining({ animation: false }),
      { notMerge: true },
    )

    act(() => resizeCallbacks[0]([], {} as ResizeObserver))
    expect(chartHarness.chart.resize).toHaveBeenCalledTimes(1)

    fireEvent.keyDown(chart, { key: 'ArrowRight' })
    expect(chartHarness.chart.dispatchAction).toHaveBeenCalledWith({ type: 'highlight', dataIndex: 1 })
    fireEvent.keyDown(chart, { key: 'Enter' })
    expect(activate).toHaveBeenCalledWith({ dataIndex: 1 })

    view.unmount()
    expect(disconnect).toHaveBeenCalledTimes(1)
    expect(chartHarness.chart.off).toHaveBeenCalledWith('click', expect.any(Function))
    expect(chartHarness.chart.dispose).toHaveBeenCalledTimes(1)
  })

  it('keeps charts without an activation contract non-interactive', () => {
    const view = render(
      <EChartRuntime
        option={{ series: [{ type: 'line', data: [1, 2, 3] }] }}
        ariaLabel="static chart"
        pointCount={3}
      />,
    )

    const chart = view.getByRole('img', { name: 'static chart' })
    expect(chart.getAttribute('tabindex')).toBeNull()
    fireEvent.keyDown(chart, { key: 'Enter' })
    expect(chartHarness.chart.dispatchAction).not.toHaveBeenCalled()
  })

  it('updates chart data and activation intent without reinitializing the canvas', () => {
    const initialActivation = vi.fn()
    const nextActivation = vi.fn()
    const view = render(
      <EChartRuntime
        option={{ series: [{ type: 'bar', data: [1] }] }}
        ariaLabel="chart"
        pointCount={1}
        onDataPointActivate={initialActivation}
      />,
    )
    const click = chartHarness.chart.on.mock.calls.find(([event]) => event === 'click')?.[1]

    view.rerender(
      <EChartRuntime
        option={{ series: [{ type: 'bar', data: [2] }] }}
        ariaLabel="chart"
        pointCount={1}
        onDataPointActivate={nextActivation}
      />,
    )
    act(() => click({ dataIndex: 0 }))

    expect(chartHarness.init).toHaveBeenCalledTimes(1)
    expect(chartHarness.chart.setOption).toHaveBeenCalledTimes(2)
    expect(initialActivation).not.toHaveBeenCalled()
    expect(nextActivation).toHaveBeenCalledWith({ dataIndex: 0, seriesIndex: undefined, name: undefined })
  })

  it('releases ten dashboard chart instances and their resize observers', () => {
    const view = render(
      <>
        {Array.from({ length: 10 }, (_, index) => (
          <EChartRuntime
            key={index}
            option={{ series: [{ type: 'bar', data: [index] }] }}
            ariaLabel={`chart-${index}`}
            pointCount={1}
          />
        ))}
      </>,
    )

    expect(chartHarness.init).toHaveBeenCalledTimes(10)
    expect(resizeCallbacks).toHaveLength(10)
    act(() => resizeCallbacks.forEach((callback) => callback([], {} as ResizeObserver)))
    expect(chartHarness.chart.resize).toHaveBeenCalledTimes(10)

    view.unmount()
    expect(disconnect).toHaveBeenCalledTimes(10)
    expect(chartHarness.chart.dispose).toHaveBeenCalledTimes(10)
  })
})
