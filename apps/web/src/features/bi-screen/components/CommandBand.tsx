import { BI_SCREEN_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import NumberFlow from '@number-flow/react'
import { useEffect, useState } from 'react'

import { commandOverview } from '../charts'
import { presentNumber } from '../model'
import type { CommandLamp, DashboardData } from '../types'

const RING_RADIUS = 40
const RING_CIRCUMFERENCE = 2 * Math.PI * RING_RADIUS
const ENTRANCE_MS = 1100

const LAMP_ENTRANCE_DELAY = [
  '[animation-delay:160ms]',
  '[animation-delay:220ms]',
  '[animation-delay:280ms]',
  '[animation-delay:340ms]',
  '[animation-delay:400ms]',
] as const

/** Eases 0 → target once on mount; reduced motion snaps straight to the value. */
function useCountUp(target: number, durationMs = ENTRANCE_MS): number {
  const [value, setValue] = useState(target)
  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      setValue(target)
      return
    }
    let frame = 0
    const start = performance.now()
    const tick = (now: number) => {
      const progress = Math.min(1, (now - start) / durationMs)
      const eased = 1 - Math.pow(1 - progress, 3)
      setValue(Math.round(target * eased))
      if (progress < 1) frame = requestAnimationFrame(tick)
    }
    setValue(0)
    frame = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frame)
  }, [target, durationMs])
  return value
}

const LAMP_DOT: Readonly<Record<CommandLamp['tone'], string>> = {
  negative: 'bg-red-500 dark:bg-red-400',
  warning: 'bg-orange-500 dark:bg-orange-400',
}

const DELTA_TONE: Readonly<Record<'up' | 'down' | 'flat', string>> = {
  up: 'text-emerald-700 dark:text-emerald-400',
  down: 'text-destructive',
  flat: 'text-muted-foreground',
}

export function CommandBand({ data }: { data: DashboardData }) {
  const copy = BI_SCREEN_COPY.command
  const overview = commandOverview(data)
  const ringPct = useCountUp(overview.completionPct)
  const dash = (RING_CIRCUMFERENCE * overview.completionPct) / 100
  const deltaLabel = overview.delta
    ? overview.delta.direction === 'up'
      ? copy.deltaUp(presentNumber(overview.delta.value))
      : overview.delta.direction === 'down'
        ? copy.deltaDown(presentNumber(overview.delta.value))
        : copy.deltaFlat
    : null
  return (
    <section
      data-testid="bi-screen-command-band"
      aria-label={copy.ariaLabel}
      className="grid min-w-0 animate-[pm-panel-rise_0.5s_cubic-bezier(0.22,1,0.36,1)_both] gap-4 rounded-lg bg-card p-4 lg:grid-cols-[auto_minmax(0,1fr)_minmax(0,auto)] lg:items-center"
    >
      <div className="flex min-w-0 items-center gap-4">
        <div
          className="relative size-24 shrink-0"
          role="img"
          aria-label={copy.ringAria(overview.completionPct)}
        >
          <svg viewBox="0 0 96 96" className="size-full -rotate-90" aria-hidden="true">
            <circle cx="48" cy="48" r={RING_RADIUS} fill="none" stroke="var(--border)" strokeWidth="8" />
            <circle
              cx="48"
              cy="48"
              r={RING_RADIUS}
              fill="none"
              stroke="var(--design-chart-positive)"
              strokeWidth="8"
              strokeLinecap="round"
              strokeDasharray={`${dash} ${RING_CIRCUMFERENCE}`}
              className="animate-[pm-ring-draw_1s_cubic-bezier(0.33,1,0.68,1)_0.25s_both]"
            />
          </svg>
          <span className="absolute inset-0 grid place-items-center text-2xl font-semibold tabular-nums text-foreground">
            <NumberFlow value={ringPct} suffix="%" />
          </span>
        </div>
        <div className="grid min-w-0 gap-1.5">
          <p className="m-0 flex min-w-0 flex-wrap items-baseline gap-x-2 text-sm">
            <NumberFlow
              className="text-lg font-semibold tabular-nums text-foreground"
              value={overview.totalTasks}
              locales="zh-CN"
            />
            <span className="text-muted-foreground">{BI_SCREEN_COPY.metrics.totalTasks}</span>
            <span className="truncate text-xs text-muted-foreground">
              {BI_SCREEN_COPY.metrics.doneTasks(presentNumber(overview.doneTasks))}
            </span>
          </p>
          <p className="m-0 flex min-w-0 flex-wrap items-baseline gap-x-2 text-xs text-muted-foreground">
            <span>{copy.recentCompleted(presentNumber(overview.recentCompleted))}</span>
            {deltaLabel && overview.delta ? (
              <span className={cn('font-medium tabular-nums', DELTA_TONE[overview.delta.direction])}>
                {deltaLabel}
                {overview.delta.direction !== 'flat' ? (
                  <span aria-hidden="true">{overview.delta.direction === 'up' ? ' ▲' : ' ▼'}</span>
                ) : null}
              </span>
            ) : null}
          </p>
          <p className="m-0 text-xs text-muted-foreground">{copy.inFlight(presentNumber(overview.inFlightTasks))}</p>
        </div>
      </div>
      <ul
        className="grid min-w-0 grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-5"
        aria-label={copy.lampsLabel}
      >
        {overview.lamps.map((lamp, index) => (
          <li
            key={lamp.id}
            title={lamp.hint}
            className={cn(
              'flex min-w-0 animate-[pm-panel-rise_0.5s_cubic-bezier(0.22,1,0.36,1)_both] items-center gap-2 rounded-lg bg-muted/50 px-2.5 py-2',
              LAMP_ENTRANCE_DELAY[index] ?? '',
              lamp.value > 0 ? '' : 'opacity-55',
            )}
          >
            <span
              aria-hidden="true"
              className={cn(
                'size-2 shrink-0 rounded-full',
                lamp.value > 0 ? LAMP_DOT[lamp.tone] : 'bg-muted-foreground/40',
                lamp.id === 'blocker' && lamp.value > 0 && 'animate-pulse',
              )}
            />
            <span className="flex min-w-0 items-baseline gap-1.5">
              <NumberFlow
                className="text-base font-semibold tabular-nums text-foreground"
                value={lamp.value}
                locales="zh-CN"
              />
              <span className="truncate text-xs text-muted-foreground">{lamp.label}</span>
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}
