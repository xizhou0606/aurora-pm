import { Component, lazy, Suspense, type ErrorInfo, type ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'

import { Button } from '../primitives/button'
import { EmptyState } from '../feedback/empty-state'
import { LoadingSkeleton } from '../feedback/loading-skeleton'
import type { EChartActivation } from './echart-runtime'

const EChartRuntime = lazy(() => import('./echart-runtime').then((mod) => ({ default: mod.EChartRuntime })))

export interface EChartSummary {
  caption: string
  columns: readonly string[]
  rows: readonly (readonly (string | number)[])[]
}

export interface LazyEChartProps {
  option: Readonly<Record<string, unknown>> | null
  ariaLabel: string
  emptyTitle: string
  emptyDescription?: string
  loadingLabel: string
  loadErrorLabel: string
  retryLabel: string
  size?: 'small' | 'compact' | 'medium'
  fluid?: boolean
  summary: EChartSummary
  onDataPointActivate?: (event: EChartActivation) => void
}

class ChartErrorBoundary extends Component<
  { resetKey: number; fallback: (retry: () => void) => ReactNode; children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(_error: Error, _info: ErrorInfo) {}

  componentDidUpdate(previous: Readonly<{ resetKey: number }>) {
    if (previous.resetKey !== this.props.resetKey && this.state.failed) {
      this.setState({ failed: false })
    }
  }

  render() {
    return this.state.failed
      ? this.props.fallback(() => this.setState({ failed: false }))
      : this.props.children
  }
}

export function EChartSummaryDetails({ summary }: { summary: EChartSummary }) {
  return (
    <details className="min-h-0 min-w-0 overflow-hidden text-xs text-muted-foreground">
      <summary className="cursor-pointer select-none font-medium">{summary.caption}</summary>
      <div className="mt-2 max-h-36 max-w-full overflow-auto overscroll-contain">
        <table className="w-full min-w-max border-collapse text-left">
          <caption className="sr-only">{summary.caption}</caption>
          <thead>
            <tr>
              {summary.columns.map((column) => <th key={column} className="sticky top-0 border-b border-border bg-card px-2 py-1.5 font-medium">{column}</th>)}
            </tr>
          </thead>
          <tbody>
            {summary.rows.map((row, rowIndex) => (
              <tr key={rowIndex}>
                {row.map((cell, cellIndex) => <td key={cellIndex} className="border-b border-border/60 px-2 py-1.5 tabular-nums">{cell}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </details>
  )
}

export function LazyEChart({
  option,
  ariaLabel,
  emptyTitle,
  emptyDescription,
  loadingLabel,
  loadErrorLabel,
  retryLabel,
  size = 'compact',
  fluid = false,
  summary,
  onDataPointActivate,
}: LazyEChartProps) {
  if (!option) {
    return <EmptyState title={emptyTitle} />
  }

  return (
    <div data-slot="lazy-echart" className={cn(
      'grid min-h-0 min-w-0 grid-rows-[minmax(0,1fr)_auto] gap-2 p-3',
      fluid && 'h-full overflow-hidden',
    )}>
      <div
        data-slot="lazy-echart-stage"
        className={cn(
          'min-h-0 overflow-hidden',
          fluid && 'h-full',
          !fluid && size === 'small' && 'h-32',
          !fluid && size === 'compact' && 'h-64',
          !fluid && size === 'medium' && 'h-80',
        )}
      >
        <ChartErrorBoundary
          resetKey={summary.rows.length}
          fallback={(retry) => (
            <div role="alert" className="grid h-full place-items-center gap-3 text-sm text-destructive">
              <span>{loadErrorLabel}</span>
              <Button type="button" size="sm" variant="outline" onClick={retry}>{retryLabel}</Button>
            </div>
          )}
        >
          <Suspense fallback={<LoadingSkeleton rows={size === 'small' ? 3 : 6} label={loadingLabel} className="h-full" />}>
            <EChartRuntime
              option={option}
              ariaLabel={ariaLabel}
              pointCount={summary.rows.length}
              onDataPointActivate={onDataPointActivate}
              className="h-full min-h-0"
            />
          </Suspense>
        </ChartErrorBoundary>
      </div>
      <EChartSummaryDetails summary={summary} />
    </div>
  )
}
