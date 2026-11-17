import { BI_SCREEN_COPY } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton } from '@/shared/ui'

import type { DashboardState, Drilldown } from '../types'
import { CommandBand } from './CommandBand'
import { DashboardCharts } from './DashboardCharts'
import { QualityOverview } from './QualityOverview'

export function DashboardStateView({
  state,
  onDrilldown,
}: {
  state: DashboardState
  onDrilldown: (target: Drilldown) => void
}) {
  const copy = BI_SCREEN_COPY.page

  if (state.status === 'loading') {
    return (
      <div className="grid min-w-0 gap-3">
        <LoadingSkeleton rows={3} label={copy.loading} className="min-h-28 rounded-lg bg-card p-4" />
        <div className="grid min-w-0 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 5 }, (_, index) => (
            <LoadingSkeleton
              key={index}
              rows={8}
              label={copy.loading}
              className="min-h-72 rounded-lg bg-card p-4"
            />
          ))}
        </div>
      </div>
    )
  }

  if (state.status === 'error') {
    return (
      <div role="alert">
        <EmptyState
          title={copy.errorTitle}
          action={(
            <Button type="button" variant="outline" onClick={state.retry}>
              {copy.retry}
            </Button>
          )}
        />
      </div>
    )
  }

  if (state.status === 'empty') {
    return (
      <EmptyState
        role="status"
        title={copy.emptyTitle}
        action={(
          <Button type="button" variant="outline" onClick={state.retry}>
            {copy.retry}
          </Button>
        )}
      />
    )
  }

  return (
    <div className="grid min-w-0 gap-3">
      <CommandBand data={state.data} />
      <DashboardCharts data={state.data} onDrilldown={onDrilldown} />
      <QualityOverview data={state.data} onDrilldown={onDrilldown} />
    </div>
  )
}
