import type { ColumnDef } from '@tanstack/react-table'
import { useEffect, useMemo, useState } from 'react'

import { qualitySeverityTone } from '@/features/quality-management'
import { BI_SCREEN_COPY, QUALITY_COPY, WORK_ITEMS_COPY } from '@/shared/copy'
import { workItemPriorityTone } from '@/shared/model'
import {
	DataTable,
	Checkbox,
	ResponsivePanel,
	StatusPill,
	ViewTabs,
} from '@/shared/ui'

import type { Environment } from '../components/environment'
import { useTeamMemberItemsQuery } from '../hooks/useQueries'
import {
	filterTeamMemberDrilldownItems,
	isTeamMemberTaskOverdue,
} from '../model/team-member-drilldown'
import type {
	RouteState,
	TeamMemberDrilldownItem,
	TeamMemberItemType,
	TeamMetricRow,
} from '../types'

export type TeamMemberDrilldownTarget = {
	userId: number
	userName: string
	role: TeamMetricRow['role_key']
}

function initialItemType(role: TeamMetricRow['role_key']): TeamMemberItemType {
	if (role === 'product') return 'requirement'
	if (role === 'testing') return 'bug'
	return 'task'
}

function itemTypeOptions(role: TeamMetricRow['role_key']) {
	const copy = BI_SCREEN_COPY.team.memberDrilldownTabs
	if (role !== 'development') {
		const id = initialItemType(role)
		return [{ id, label: copy[id] }]
	}
	return [
		{ id: 'task' as const, label: copy.task },
		{ id: 'bug' as const, label: copy.bug },
	]
}

function dateTime(value?: string) {
	if (!value) return '—'
	const parsed = new Date(value)
	return Number.isNaN(parsed.getTime()) ? '—' : new Intl.DateTimeFormat('zh-CN', {
		year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
	}).format(parsed)
}

function statusTone(status: string) {
	switch (status.toLowerCase()) {
	case 'completed':
	case 'done':
	case 'closed':
		return 'neutral' as const
	case 'in_progress':
	case 'in_dev':
		return 'info' as const
	case 'review':
	case 'pending_test':
		return 'accent' as const
	default:
		return 'warning' as const
	}
}

function statusLabel(status: string) {
	const labels = BI_SCREEN_COPY.team.memberDrilldownStatusLabels
	return labels[status as keyof typeof labels] || status || '—'
}

function importancePill(item: TeamMemberDrilldownItem, itemType: TeamMemberItemType) {
	if (itemType === 'bug') {
		const severity = String(item.severity || '').trim().toLowerCase()
		return severity ? (
			<StatusPill tone={qualitySeverityTone(severity)}>
				{QUALITY_COPY.enums.severity[severity] || item.severity}
			</StatusPill>
		) : '—'
	}
	return item.priority ? (
		<StatusPill tone={workItemPriorityTone(item.priority)}>
			{WORK_ITEMS_COPY.enums.priorities[
				(item.priority.toLowerCase() === 'medium' ? 'normal' : item.priority.toLowerCase())
			] || item.priority}
		</StatusPill>
	) : '—'
}

export function TeamMemberDrilldownPanel({
	environment,
	route,
	target,
	itemType,
	onItemTypeChange,
	onOpenChange,
	canView = true,
}: {
	environment: Environment
	route: RouteState
	target?: TeamMemberDrilldownTarget
	itemType: TeamMemberItemType
	onItemTypeChange: (value: TeamMemberItemType) => void
	onOpenChange: (open: boolean) => void
	canView?: boolean
}) {
	const copy = BI_SCREEN_COPY.team
	const [showAll, setShowAll] = useState(target?.role === 'product')
	const query = useTeamMemberItemsQuery(environment, route, target ? {
		userId: target.userId,
		role: target.role,
		itemType,
	} : undefined, canView)
	useEffect(() => {
		setShowAll(target?.role === 'product')
	}, [itemType, target?.role, target?.userId])
	const visibleItems = useMemo(() => filterTeamMemberDrilldownItems({
		items: query.data?.items ?? [],
		role: target?.role ?? 'product',
		itemType,
		showAll,
	}), [itemType, query.data?.items, showAll, target?.role])
	const columns: ColumnDef<TeamMemberDrilldownItem>[] = [
		{
			id: 'key',
			size: 120,
			header: copy.memberDrilldownColumns.key,
			cell: ({ row }) => <span className="font-mono text-xs text-muted-foreground">{row.original.key || `#${row.original.id}`}</span>,
		},
		{
			id: 'title',
			size: 360,
			header: copy.memberDrilldownColumns.title,
			cell: ({ row }) => <span className="block max-w-[30rem] truncate font-medium" title={row.original.title}>{row.original.title}</span>,
		},
		{
			id: 'status',
			size: 160,
			header: copy.memberDrilldownColumns.status,
			cell: ({ row }) => (
				<span className="flex flex-wrap items-center gap-1.5">
					<StatusPill tone={statusTone(row.original.status)}>{statusLabel(row.original.status)}</StatusPill>
					{itemType === 'task' && isTeamMemberTaskOverdue(row.original) ? (
						<StatusPill tone="danger">{copy.memberDrilldownOverdue}</StatusPill>
					) : null}
				</span>
			),
		},
		{
			id: 'severity',
			size: 120,
			header: itemType === 'bug'
				? copy.memberDrilldownColumns.severity
				: copy.memberDrilldownColumns.priority,
			cell: ({ row }) => importancePill(row.original, itemType),
		},
		...(itemType === 'task' ? [
			{
				id: 'effective_due_at',
				size: 180,
				header: copy.memberDrilldownColumns.dueAt,
				cell: ({ row }: { row: { original: TeamMemberDrilldownItem } }) => (
					<span className="grid gap-0.5">
						<time className="whitespace-nowrap tabular-nums" dateTime={row.original.effective_due_at}>
							{dateTime(row.original.effective_due_at)}
						</time>
						{row.original.due_at_source === 'parent' ? (
							<span className="whitespace-nowrap text-xs text-muted-foreground">
								{copy.memberDrilldownInheritedDueAt}
							</span>
						) : null}
					</span>
				),
			},
			{
				id: 'delivery_at',
				size: 180,
				header: copy.memberDrilldownColumns.deliveryAt,
				cell: ({ row }: { row: { original: TeamMemberDrilldownItem } }) => (
					<time className="whitespace-nowrap tabular-nums" dateTime={row.original.delivery_at}>
						{dateTime(row.original.delivery_at)}
					</time>
				),
			},
		] : []),
		{
			id: 'created_at',
			size: 180,
			header: copy.memberDrilldownColumns.createdAt,
			cell: ({ row }) => <time className="whitespace-nowrap tabular-nums" dateTime={row.original.created_at}>{dateTime(row.original.created_at)}</time>,
		},
	]
	const state = query.isPending
		? { status: 'loading' as const, label: copy.memberDrilldownLoading, skeletonRows: 8 }
		: query.isError
			? {
				status: 'error' as const,
				title: copy.memberDrilldownError,
				description: copy.memberDrilldownErrorDescription,
				onRetry: () => void query.refetch(),
				retryLabel: copy.memberDrilldownRetry,
			}
			: visibleItems.length
				? { status: 'ready' as const, rows: visibleItems }
				: {
					status: 'empty' as const,
					title: copy.memberDrilldownEmpty,
					description: copy.memberDrilldownEmptyDescription,
				}

	return (
		<ResponsivePanel
			open={Boolean(target)}
			onOpenChange={onOpenChange}
			title={target ? copy.memberDrilldownTitle(copy.roleLabels[target.role], target.userName) : ''}
			description={copy.memberDrilldownDescription}
			height="viewport"
			className="max-w-6xl"
			contentClassName="grid min-h-0 grid-rows-[auto_minmax(0,1fr)] gap-3 overflow-hidden"
		>
			<div className="flex min-h-9 items-center justify-between gap-3">
				{target?.role === 'development' ? (
					<ViewTabs
						value={itemType}
						label={copy.roleFilter}
						tabs={itemTypeOptions(target.role)}
						onValueChange={(value) => onItemTypeChange(value as TeamMemberItemType)}
					/>
				) : <span className="text-sm font-medium">{copy.memberDrilldownTabs[itemType]}</span>}
				<span className="flex items-center gap-4">
					<label className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
						<Checkbox
							checked={showAll}
							onCheckedChange={(checked) => setShowAll(checked === true)}
						/>
						{copy.memberDrilldownShowAll}
					</label>
					<span className="text-xs text-muted-foreground">{copy.memberDrilldownCount(visibleItems.length)}</span>
				</span>
			</div>
			<DataTable
				aria-label={copy.memberDrilldownAria(target?.userName || '', copy.memberDrilldownTabs[itemType])}
				columns={columns}
				state={state}
				className="h-full min-h-0 [&_[data-slot=table-scroll-shell]]:h-full"
				tableClassName={itemType === 'task' ? 'min-w-[1260px] table-fixed' : 'min-w-[940px] table-fixed'}
				scrollAreaClassName="h-full"
				getRowId={(row) => `${itemType}-${row.id}`}
			/>
		</ResponsivePanel>
	)
}
