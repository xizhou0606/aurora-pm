import { copy } from '@/shared/copy'
import { AlertBanner, Button, EmptyState, LoadingSkeleton } from '@/shared/ui'
import type {
  PlatformUser,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import { useProjectOverviewModel } from '../hooks/useProjectOverviewModel'
import type { ProjectOperationAccess } from '../hooks'
import { LatestProjectUpdate } from './LatestProjectUpdate'
import { DeliveryProgressCard } from './DeliveryProgressCard'
import { ProjectCreatorCard } from './members/ProjectCreatorCard'
import { WorkScopeCharts } from './WorkScopeCharts'
import { WorkScopePanel } from './WorkScopePanel'

type OverviewProps = {
  project: ScrumProjectNew
  units: readonly ScrumVersionTaskNew[]
  users: readonly PlatformUser[]
  unit?: ScrumVersionTaskNew
  operationAccess?: ProjectOperationAccess
}

function Overview({
  project,
  units,
  users,
  unit,
  operationAccess,
}: OverviewProps) {
  const overview = copy.projectManagement.overview
  const state = useProjectOverviewModel({ project, unit, units })

  if (state.status === 'loading') {
    return <LoadingSkeleton rows={8} label={overview.loading} />
  }
  if (state.status === 'error') {
    return (
      <EmptyState
        role="alert"
        title={overview.errorTitle}
        action={(
          <Button type="button" size="sm" variant="outline" onClick={state.retry}>
            {copy.ui.retry}
          </Button>
        )}
      />
    )
  }

  return (
    <section aria-labelledby="pm-project-overview-heading" className="grid min-w-0 gap-3">
      <h2 id="pm-project-overview-heading" className="sr-only">
        {overview.heading}
      </h2>

      {state.staleError ? (
        <AlertBanner
          tone="error"
          action={(
            <Button type="button" size="xs" variant="outline" onClick={state.staleError.retry}>
              {copy.ui.retry}
            </Button>
          )}
        >
          {overview.staleError}
        </AlertBanner>
      ) : null}
      {state.refreshing ? (
        <p role="status" aria-live="polite" className="text-xs text-muted-foreground">
          {overview.refreshing}
        </p>
      ) : null}

      <div data-testid="pm-project-cockpit" className="grid min-w-0 gap-3">
        <WorkScopePanel work={state.model.work} />
        <WorkScopeCharts work={state.model.work} />
        {unit ? <DeliveryProgressCard unit={unit} /> : null}
        {!unit ? (
          <>
            <ProjectCreatorCard project={project} workspaceUsers={users} />
            <LatestProjectUpdate
              projectId={project.id}
              className="rounded-lg border border-border bg-card p-4 sm:p-5"
            />
          </>
        ) : null}
      </div>
    </section>
  )
}

export function ProjectOverview(props: Omit<OverviewProps, 'unit'>) {
  return <Overview {...props} />
}

export function PlanUnitOverview({
  project,
  unit,
  users,
  operationAccess,
}: {
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  users: readonly PlatformUser[]
  operationAccess?: ProjectOperationAccess
}) {
  return (
    <Overview
      project={project}
      unit={unit}
      units={[unit]}
      users={users}
      operationAccess={operationAccess}
    />
  )
}
