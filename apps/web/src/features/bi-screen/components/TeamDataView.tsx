import { useState } from 'react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton } from '@/shared/ui'

import type {
  DashboardState,
	RouteState,
  TeamMetricRow,
} from '../types'
import type { Environment } from '../components/environment'
import type { BiScreenOperationAccess } from '../hooks'
import { TeamTableView } from './TeamTableView'

export function TeamDataView({
  state,
  environment,
  route,
  operationAccess,
}: {
  state: DashboardState
  environment: Environment
  route: RouteState
  operationAccess: BiScreenOperationAccess
}) {
  const copy = BI_SCREEN_COPY.team
  const [role, setRole] = useState<TeamMetricRow['role_key']>('product')

  if (state.status === 'loading') {
    return <LoadingSkeleton rows={10} label={BI_SCREEN_COPY.page.loading} className="min-h-72 rounded-lg border border-border bg-card p-4" />
  }
  if (state.status === 'error') {
    return <div role="alert"><EmptyState title={BI_SCREEN_COPY.page.errorTitle} action={<Button type="button" variant="outline" onClick={state.retry}>{BI_SCREEN_COPY.page.retry}</Button>} /></div>
  }
  if (state.status === 'empty' || (!state.data.team_data.summary_rows.length && !state.data.team_data.member_groups.length)) {
		return <EmptyState role="status" title={copy.emptyTitle} />
  }

  return (
	<div className="grid min-w-0 gap-4">
		<TeamTableView data={state.data.team_data} role={role} onRoleChange={setRole} environment={environment} route={route} canViewDetails={operationAccess.viewTeamDetails} />
	</div>
  )
}
