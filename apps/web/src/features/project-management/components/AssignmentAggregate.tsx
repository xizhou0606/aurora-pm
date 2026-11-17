import { copy } from '@/shared/copy'
import { AlertBanner, Button, EmptyState, LoadingSkeleton } from '@/shared/ui'
import type {
  PlatformUser,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import { useProjectAssignmentModel } from '../hooks/useProjectOverviewModel'
import { ProjectMembersCard } from './members/ProjectMembersCard'

export function AssignmentAggregate({
  project,
  unit,
  users,
}: {
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  users: readonly PlatformUser[]
}) {
  const state = useProjectAssignmentModel({ project, unit, users })
  const overview = copy.projectManagement.overview

  if (state.status === 'loading') {
    return <LoadingSkeleton rows={6} label={overview.loading} />
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
    <div className="grid min-w-0 gap-5">
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
      <ProjectMembersCard unit={unit} workspaceUsers={users} />
    </div>
  )
}
