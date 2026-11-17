import type { TeamData, TeamMetricRow } from '../types'
import type { Environment } from '../components/environment'
import type { RouteState } from '../types'
import { TeamMemberMetricsTable } from './TeamMemberMetricsTable'
import { TeamSummaryTable } from './TeamSummaryTable'

export function TeamTableView({ data, role, onRoleChange, environment, route, canViewDetails }: {
	data: TeamData
	role: TeamMetricRow['role_key']
	onRoleChange: (role: TeamMetricRow['role_key']) => void
	environment: Environment
	route: RouteState
	canViewDetails: boolean
}) {
	return (
		<div className="grid min-w-0 gap-5">
			<TeamMemberMetricsTable groups={data.member_groups} role={role} onRoleChange={onRoleChange} environment={environment} route={route} canViewDetails={canViewDetails} />
			<TeamSummaryTable rows={data.summary_rows} />
		</div>
	)
}
