import { isQualityTerminalStatus } from '@/features/quality-management'

import type {
	TeamMemberDrilldownItem,
	TeamMemberItemType,
	TeamMetricRow,
} from '../types'

const TERMINAL_TASK_STATUSES = new Set([
	'completed', 'done', 'released', 'archived', 'closed', 'cancelled', 'canceled',
])

function timestamp(value: string | undefined): number | undefined {
	if (!value) return undefined
	const result = new Date(value).getTime()
	return Number.isNaN(result) ? undefined : result
}

export function isTeamMemberTaskOverdue(
	item: TeamMemberDrilldownItem,
	now = Date.now(),
): boolean {
	if (typeof item.overdue === 'boolean') return item.overdue
	const dueAt = timestamp(item.effective_due_at || item.due_at)
	if (dueAt === undefined) return false
	const completedAt = timestamp(item.completed_at)
	if (completedAt !== undefined) return completedAt > dueAt
	return !TERMINAL_TASK_STATUSES.has(item.status.toLowerCase()) && now > dueAt
}

export function filterTeamMemberDrilldownItems({
	items,
	role,
	itemType,
	showAll,
	now = Date.now(),
}: {
	items: readonly TeamMemberDrilldownItem[]
	role: TeamMetricRow['role_key']
	itemType: TeamMemberItemType
	showAll: boolean
	now?: number
}): TeamMemberDrilldownItem[] {
	if (showAll || role === 'product') return [...items]
	if (role === 'testing') {
		return items.filter((item) => !isQualityTerminalStatus(item.status))
	}
	if (itemType === 'bug') {
		return items.filter((item) => ['p0', 'p1'].includes(String(item.severity || '').trim().toLowerCase()))
	}
	return items.filter((item) => (
		!TERMINAL_TASK_STATUSES.has(item.status.toLowerCase()) || isTeamMemberTaskOverdue(item, now)
	))
}
