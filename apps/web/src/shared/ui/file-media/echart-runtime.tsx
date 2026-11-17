import { BarChart, LineChart, PieChart, RadarChart, ScatterChart } from 'echarts/charts'
import {
  AriaComponent,
  GridComponent,
  LegendComponent,
  MarkAreaComponent,
  MarkPointComponent,
  PolarComponent,
  RadarComponent,
  TitleComponent,
  TooltipComponent,
} from 'echarts/components'
import { init, use, type EChartsCoreOption, type EChartsType } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { useEffect, useRef, useState } from 'react'

import { cn } from '@/shared/lib/utils'

use([
  BarChart,
  LineChart,
  PieChart,
  RadarChart,
  ScatterChart,
  AriaComponent,
  GridComponent,
  LegendComponent,
  MarkAreaComponent,
  MarkPointComponent,
  PolarComponent,
  RadarComponent,
  TitleComponent,
  TooltipComponent,
  CanvasRenderer,
])

export interface EChartActivation {
  dataIndex: number
  seriesIndex?: number
  name?: string
}

export interface EChartRuntimeProps {
  option: Readonly<Record<string, unknown>>
  ariaLabel: string
  pointCount: number
  className?: string
  onDataPointActivate?: (event: EChartActivation) => void
}

export function EChartRuntime({
  option,
  ariaLabel,
  pointCount,
  className,
  onDataPointActivate,
}: EChartRuntimeProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<EChartsType | null>(null)
  const activationRef = useRef(onDataPointActivate)
  const optionRef = useRef(option)
  const applyRef = useRef<(playEntrance: boolean) => void>(() => undefined)
  const revealedRef = useRef(false)
  const [activeIndex, setActiveIndex] = useState(0)
  activationRef.current = onDataPointActivate
  optionRef.current = option

  // Charts mount with animation off and replay their full entrance the first
  // time they scroll into view, so below-the-fold panels never look static.
  applyRef.current = (playEntrance: boolean) => {
    const chart = chartRef.current
    if (!chart) return
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    chart.setOption(
      {
        ...optionRef.current,
        ...(!playEntrance || reducedMotion ? { animation: false } : {}),
      } as EChartsCoreOption,
      { notMerge: true },
    )
  }

  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const chart = init(container, undefined, { renderer: 'canvas' })
    chartRef.current = chart

    const handleClick = (params: { dataIndex?: number; seriesIndex?: number; name?: string }) => {
      if (typeof params.dataIndex !== 'number') return
      setActiveIndex(params.dataIndex)
      activationRef.current?.({
        dataIndex: params.dataIndex,
        seriesIndex: params.seriesIndex,
        name: params.name,
      })
    }
    chart.on('click', handleClick)

    const observer = new ResizeObserver(() => chart.resize())
    observer.observe(container)
    const visibility = typeof IntersectionObserver === 'undefined'
      ? null
      : new IntersectionObserver((entries) => {
        if (!entries.some((entry) => entry.isIntersecting && entry.intersectionRatio > 0)) return
        if (!revealedRef.current) {
          revealedRef.current = true
          chart.resize()
          applyRef.current(true)
          return
        }
        chart.resize()
      })
    visibility?.observe(container)
    return () => {
      observer.disconnect()
      visibility?.disconnect()
      chart.off('click', handleClick)
      chart.dispose()
      chartRef.current = null
    }
  }, [])

  useEffect(() => {
    applyRef.current(revealedRef.current)
  }, [option])

  const moveSelection = (nextIndex: number) => {
    if (!pointCount) return
    const normalized = (nextIndex + pointCount) % pointCount
    setActiveIndex(normalized)
    chartRef.current?.dispatchAction({ type: 'downplay' })
    chartRef.current?.dispatchAction({ type: 'highlight', dataIndex: normalized })
    chartRef.current?.dispatchAction({ type: 'showTip', dataIndex: normalized })
  }

  return (
    <div
      ref={containerRef}
      data-slot="echart-runtime"
      role={onDataPointActivate ? 'application' : 'img'}
      aria-label={ariaLabel}
      tabIndex={onDataPointActivate ? 0 : undefined}
      className={cn('min-h-0 min-w-0 outline-none focus-visible:ring-2 focus-visible:ring-ring', className)}
      onKeyDown={(event) => {
        if (!onDataPointActivate || !pointCount) return
        if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
          event.preventDefault()
          moveSelection(activeIndex + 1)
        } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
          event.preventDefault()
          moveSelection(activeIndex - 1)
        } else if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onDataPointActivate({ dataIndex: activeIndex })
        }
      }}
    />
  )
}
