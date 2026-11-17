import { PhArrowsOut, PhArrowsIn } from '@/shared/ui/icons'
import { useRef, useState } from 'react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import {
  Button,
  EmptyState,
  LoadingSkeleton,
  OperationalPage,
  ViewTabs,
} from '@/shared/ui'

import type { Environment } from '../components/environment'
import {
  presentGeneratedTime,
  presentScopeSummary,
  readyDashboardData,
} from '../model'
import { useFullscreen, useScreenModel } from '../hooks'
import { DashboardStateView } from './DashboardStateView'
import { ScopeControl } from './ScopeControl'
import { TeamControls } from './TeamControls'
import { TeamDataView } from './TeamDataView'
import { TeamMetricSettingsPanel } from './TeamMetricSettingsPanel'
import { TeamScoreSettingsPanel } from './TeamScoreSettingsPanel'

export function Screen({
  environment,
  onRuntimeFailure,
}: {
  environment: Environment
  onRuntimeFailure?: (error: unknown) => void
}) {
  const root = useRef<HTMLElement>(null)
  const fullscreen = useFullscreen(root)
  const model = useScreenModel(environment, onRuntimeFailure)
  const [metricSettingsOpen, setMetricSettingsOpen] = useState(false)
  const [scoreSettingsOpen, setScoreSettingsOpen] = useState(false)
  const copy = BI_SCREEN_COPY.page

  if (model.operationAccess.status === 'loading') {
    return (
      <OperationalPage title={copy.title}>
        <LoadingSkeleton rows={6} label={copy.loading} />
      </OperationalPage>
    )
  }

  if (model.operationAccess.status === 'error') {
    return (
      <OperationalPage title={copy.title}>
        <EmptyState
          role="alert"
          title={copy.errorTitle}
          action={<Button type="button" variant="outline" onClick={model.operationAccess.retry}>{copy.retry}</Button>}
        />
      </OperationalPage>
    )
  }

  if (!model.allowed) {
    return (
      <OperationalPage title={copy.title}>
        <EmptyState
          role="alert"
          title={copy.unavailableTitle}
        />
      </OperationalPage>
    )
  }

  if (model.scope.status === 'invalid') {
    return (
      <OperationalPage title={copy.title}>
        <EmptyState
          role="alert"
          title={copy.invalidScopeTitle}
          action={(
            <Button type="button" variant="outline" onClick={model.scope.reset}>
              {copy.resetInvalidScope}
            </Button>
          )}
        />
      </OperationalPage>
    )
  }

  const scopeSummary = presentScopeSummary({
    count: model.scopeCount,
    label: model.scopeLabel,
  })
  const data = readyDashboardData(model.dashboard)

  return (
    <section
      id="bi-screen-dashboard-surface"
      ref={root}
      data-testid="bi-screen-native-module"
      className={cn(
        'flex min-h-0 min-w-0 flex-1 bg-background',
        fullscreen.fallback && 'fixed inset-0 z-50 h-dvh w-screen',
      )}
    >
      <OperationalPage
        className="h-full"
        title={copy.title}
        refresh={model.refresh}
        actions={(
          <>
            <ScopeControl
              state={model.scope}
              value={model.route.scope}
              summary={scopeSummary}
              onChange={model.setScope}
            />
            {data ? (
              <span className="text-xs text-muted-foreground">
                {model.refresh === 'refreshing' ? copy.updating : copy.updatedAt}{' '}
                <time dateTime={data.generated_at}>{presentGeneratedTime(data.generated_at)}</time>
              </span>
            ) : null}
            <Button
              ref={fullscreen.triggerRef}
              type="button"
              variant="outline"
              size="sm"
              title={fullscreen.active ? copy.exitFullscreenTitle : copy.enterFullscreenTitle}
              aria-controls="bi-screen-dashboard-surface"
              aria-pressed={fullscreen.active}
              onClick={() => void fullscreen.toggle()}
            >
              {fullscreen.active ? <PhArrowsIn aria-hidden="true" /> : <PhArrowsOut aria-hidden="true" />}
              {fullscreen.active ? copy.exitFullscreen : copy.fullscreen}
            </Button>
          </>
        )}

      >
        <div className="grid min-w-0 gap-2 px-4 pt-3 pb-4 sm:px-5 sm:pt-3 sm:pb-5">
          <div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <ViewTabs
              value={model.route.tab}
              label={copy.tabsLabel}
              density="compact"
              tabs={[
                ...(model.operationAccess.viewOverview
                  ? [{ id: 'overview' as const, label: copy.overviewTab }]
                  : []),
                ...(model.operationAccess.viewOverview
                  ? [{ id: 'team' as const, label: copy.teamTab }]
                  : []),
              ]}
              onValueChange={(value) => model.setTab(value === 'team' ? 'team' : 'overview')}
            />
            {model.route.tab === 'team' ? (
              <TeamControls
                dimension={model.teamDimension}
                slice={model.teamSlice}
                onOpenMetricSettings={model.operationAccess.viewMetrics
                  ? () => setMetricSettingsOpen(true)
                  : undefined}
                onOpenScoreSettings={model.operationAccess.viewOverview && data
                  ? () => setScoreSettingsOpen(true)
                  : undefined}
              />
            ) : null}
          </div>
          {model.navigation.status === 'error' ? (
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive" role="alert">
              <span>{BI_SCREEN_COPY.team.navigationFailed}</span>
              <Button type="button" variant="ghost" size="xs" onClick={model.navigation.dismiss}>
                {BI_SCREEN_COPY.team.dismissNavigationError}
              </Button>
            </div>
          ) : null}
          {model.route.tab === 'team' ? (
            <TeamDataView state={model.dashboard} environment={environment} route={model.route} operationAccess={model.operationAccess} />
          ) : (
            <DashboardStateView state={model.dashboard} onDrilldown={model.drilldown} />
          )}
        </div>
      </OperationalPage>
      {model.operationAccess.viewMetrics ? <TeamMetricSettingsPanel
        open={metricSettingsOpen}
        onOpenChange={setMetricSettingsOpen}
        settings={model.metricSettingsQuery.data}
        loadState={model.metricSettingsQuery.isPending
          ? 'loading'
          : model.metricSettingsQuery.isError ? 'error' : 'ready'}
        onRetry={() => {
          model.metricSettingsMutation.reset()
          void model.metricSettingsQuery.refetch()
        }}
        onSave={model.metricSettingsMutation.mutate}
        saveState={model.metricSettingsMutation.isPending
          ? 'pending'
          : model.metricSettingsMutation.isSuccess
            ? 'success'
            : model.metricSettingsMutation.isError ? 'error' : 'idle'}
        canUpdate={model.operationAccess.updateMetrics}
      /> : null}
      {data ? <TeamScoreSettingsPanel
        open={scoreSettingsOpen}
        onOpenChange={setScoreSettingsOpen}
        profile={data.team_data.score_profile}
        onSave={model.scoreProfileMutation.mutate}
        onReset={model.scoreProfileMutation.mutate}
        saveState={model.scoreProfileMutation.isPending
          ? 'pending'
          : model.scoreProfileMutation.isSuccess
            ? 'success'
            : model.scoreProfileMutation.isError ? 'error' : 'idle'}
        canUpdate={model.operationAccess.updateScoring}
      /> : null}
    </section>
  )
}
