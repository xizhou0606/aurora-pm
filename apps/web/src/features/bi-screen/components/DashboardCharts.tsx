import { useEffect, useMemo } from 'react'
import { gsap } from 'gsap'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { useAppearance } from '@/shared/ui'

import { overviewModels } from '../charts'
import type { DashboardData, Drilldown } from '../types'
import { ChartPanel } from './ChartPanel'
import { CompositionPanel } from './CompositionPanel'
import { VersionMatrixPanel } from './VersionMatrixPanel'

export function DashboardCharts({
  data,
  onDrilldown,
}: {
  data: DashboardData
  onDrilldown: (target: Drilldown) => void
}) {
  // Theme flips re-resolve the chart tokens, so models rebuild (and the
  // runtime replays their entrance) whenever the active ui-mode changes.
  const { resolved } = useAppearance()
  const models = useMemo(() => overviewModels(data), [data, resolved])

  // GSAP 编排：面板标题按序浮入，与各面板自身的弹簧/图表入场叠加成一条叙事。
  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    if (!document.querySelector('[data-bi-panel-title]')) return
    const tween = gsap.fromTo(
      '[data-bi-panel-title]',
      { y: -10, autoAlpha: 0 },
      { y: 0, autoAlpha: 1, stagger: 0.07, duration: 0.5, ease: 'power3.out', delay: 0.25, overwrite: 'auto' },
    )
    return () => {
      tween.kill()
      gsap.set('[data-bi-panel-title]', { clearProps: 'all' })
    }
  }, [models])

  return (
    <section
      aria-label={BI_SCREEN_COPY.charts.sectionLabel}
      className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3"
    >
      <ChartPanel
        model={models.trend}
        onDrilldown={onDrilldown}
        entranceDelayMs={80}
        className="md:col-span-2 xl:col-span-2"
      />
      <ChartPanel model={models.version} onDrilldown={onDrilldown} entranceDelayMs={160} />
      <VersionMatrixPanel
        model={models.matrix}
        onDrilldown={onDrilldown}
        entranceDelayMs={230}
        className="md:col-span-2 xl:col-span-2"
      />
      <ChartPanel model={models.requirement} onDrilldown={onDrilldown} entranceDelayMs={300} />
      <ChartPanel model={models.owner} onDrilldown={onDrilldown} entranceDelayMs={370} />
      <CompositionPanel model={models.composition} onDrilldown={onDrilldown} entranceDelayMs={440} />
      <ChartPanel model={models.healthRadar} onDrilldown={onDrilldown} entranceDelayMs={510} />
    </section>
  )
}
