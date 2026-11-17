import type { ColumnDef } from '@tanstack/react-table'
import { Tooltip } from '@base-ui/react/tooltip'
import { PhSortDescending, PhSortAscending, PhArrowsDownUp, PhQuestion } from '@/shared/ui/icons'
import { useEffect, useMemo, useState } from 'react'

import { BI_SCREEN_COPY } from '@/shared/copy'
import {
  AdaptiveFilterBar,
  Button,
  DataTable,
  DataTableSortButton,
  UserIdentity,
  UserSelect,
  ViewTabs,
  useOverlayPortalContainer,
} from '@/shared/ui'

import type { TeamMemberGroup } from '../types'
import type { Environment } from '../components/environment'
import type { RouteState, TeamMemberItemType } from '../types'
import {
  TEAM_ROLE_METRICS,
  TEAM_ROLE_ORDER,
  isSeverityShareMetric,
  sortTeamMembers,
  teamMetricValue,
  type TeamDisplayMember,
  type TeamDisplayMetric,
  type TeamDisplayMetricKey,
  type TeamRoleKey,
} from '../model/team-display'
import { TeamChartView } from './TeamChartView'
import {
  TeamMemberDrilldownPanel,
  type TeamMemberDrilldownTarget,
} from './TeamMemberDrilldownPanel'

function MetricCell({ row, metric }: { row: TeamDisplayMember; metric: TeamDisplayMetric }) {
  const copy = BI_SCREEN_COPY.team
  const result = teamMetricValue(row, metric)
  if (result.value === undefined) return <span>{copy.noSample}</span>
  const value = metric.unit === 'hours' ? copy.hours(result.value) : metric.unit === 'percent' ? copy.percent(result.value) : result.value.toLocaleString('zh-CN')
  const showSamples = result.samples !== undefined && metric.unit !== 'count' && metric.key !== 'overdue_rate'
  return (
    <span className="grid gap-0.5 tabular-nums">
      <span>{value}</span>
      {isSeverityShareMetric(metric) && result.supportingValue !== undefined
        ? <span className="text-xs text-muted-foreground">{copy.severityTeamShare(copy.percent(result.supportingValue))}</span>
        : showSamples ? <span className="text-xs text-muted-foreground">{copy.responseSamples(result.samples!)}</span> : null}
    </span>
  )
}

function MetricSortHeader({ metric, active, descending, onSort }: {
  metric: TeamDisplayMetric
  active: boolean
  descending: boolean
  onSort: (key: TeamDisplayMetricKey) => void
}) {
  const copy = BI_SCREEN_COPY.team
  return (
    <span className="flex items-center gap-0.5">
      <DataTableSortButton
        aria-label={copy.metricSortAria(metric.label, active ? (descending ? 'descending' : 'ascending') : undefined)}
        onClick={() => onSort(metric.key)}
      >
        {metric.label}
        {active ? descending ? <PhSortDescending aria-hidden="true" /> : <PhSortAscending aria-hidden="true" /> : <PhArrowsDownUp aria-hidden="true" />}
      </DataTableSortButton>
      {metric.key === 'overdue_rate' && <OverdueRuleTooltip />}
    </span>
  )
}

function OverdueRuleTooltip() {
  const copy = BI_SCREEN_COPY.team.overdueRule
  const portalContainer = useOverlayPortalContainer()
  return (
    <Tooltip.Root>
      <Tooltip.Trigger render={<Button type="button" variant="ghost" size="icon-sm" className="cursor-help text-muted-foreground" />} delay={150} aria-label={copy.ariaLabel}>
        <PhQuestion aria-hidden="true" className="size-3.5" />
      </Tooltip.Trigger>
      <Tooltip.Portal container={portalContainer ?? undefined}>
        <Tooltip.Positioner side="bottom" sideOffset={6} className="z-80" positionMethod="fixed">
          <Tooltip.Popup className="w-[min(34rem,calc(100vw-2rem))] rounded-md border border-border bg-card p-3 text-card-foreground shadow-[var(--design-shadow-popover)] outline-none">
            <p className="m-0 text-sm font-semibold">{copy.title}</p>
            <p className="mb-0 mt-1 text-xs leading-5 text-muted-foreground">{copy.sampleRule}</p>
            <table className="mt-2 w-full table-fixed border-collapse text-left text-xs">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="w-[34%] px-2 py-1.5 font-medium">{copy.columns.path}</th>
                  <th className="w-[30%] px-2 py-1.5 font-medium">{copy.columns.time}</th>
                  <th className="px-2 py-1.5 font-medium">{copy.columns.result}</th>
                </tr>
              </thead>
              <tbody>
                {copy.rows.map((row) => (<tr key={row.path} className="border-b border-border/70 last:border-b-0">
                  <td className="px-2 py-1.5 align-top">{row.path}</td>
                  <td className="px-2 py-1.5 align-top">{row.time}</td>
                  <td className="px-2 py-1.5 align-top text-muted-foreground">{row.result}</td>
                </tr>))}
              </tbody>
            </table>
            <p className="mb-0 mt-2 text-xs text-muted-foreground">{copy.footer}</p>
          </Tooltip.Popup>
        </Tooltip.Positioner>
      </Tooltip.Portal>
    </Tooltip.Root>
  )
}

export function TeamMemberMetricsTable({
  groups, role, onRoleChange, environment, route, canViewDetails = true,
}: {
  groups: readonly TeamMemberGroup[]
  role: TeamRoleKey
  onRoleChange: (role: TeamRoleKey) => void
  environment?: Environment
  route?: RouteState
  canViewDetails?: boolean
}) {
  const copy = BI_SCREEN_COPY.team
  const availableRoles = useMemo(() => TEAM_ROLE_ORDER.filter((roleKey) => groups.some((g) => g.role_key === roleKey)), [groups])
  const [selectedMemberId, setSelectedMemberId] = useState<number | undefined>()
  const [sortMetricKey, setSortMetricKey] = useState<TeamDisplayMetricKey>('requirement_count')
  const [descending, setDescending] = useState(true)
  const [drilldownTarget, setDrilldownTarget] = useState<TeamMemberDrilldownTarget>()
  const [drilldownItemType, setDrilldownItemType] = useState<TeamMemberItemType>('requirement')
  useEffect(() => { if (!availableRoles.includes(role) && availableRoles[0]) onRoleChange(availableRoles[0]) }, [availableRoles, onRoleChange, role])
  const group = groups.find((entry) => entry.role_key === role)
  const memberOptions = useMemo(() => (group?.members || []).map((m) => ({ id: m.user_id, name: m.user_name, avatarUrl: m.user_avatar_url })), [group?.members])
  const metrics = TEAM_ROLE_METRICS[role]
  const selectedMetric = metrics.find((m) => m.key === sortMetricKey)
  const activeMetric = selectedMetric ?? metrics[0]
  const effectiveDescending = selectedMetric ? descending : activeMetric.defaultDescending
  useEffect(() => {
    if (!metrics.some((m) => m.key === sortMetricKey)) { setSortMetricKey(metrics[0].key); setDescending(metrics[0].defaultDescending) }
  }, [metrics, sortMetricKey])
  useEffect(() => {
    if (selectedMemberId !== undefined && !memberOptions.some((m) => m.id === selectedMemberId)) setSelectedMemberId(undefined)
  }, [memberOptions, selectedMemberId])
  const rows = useMemo(() => sortTeamMembers(
    (group?.members || []).filter((m) => selectedMemberId === undefined || m.user_id === selectedMemberId), activeMetric, effectiveDescending,
  ), [activeMetric, effectiveDescending, group?.members, selectedMemberId])
  const handleSort = (key: TeamDisplayMetricKey) => {
    if (key === sortMetricKey) { setDescending((v) => !v); return }
    setSortMetricKey(key); setDescending(metrics.find((m) => m.key === key)?.defaultDescending ?? true)
  }
  const columns = useMemo<ColumnDef<TeamDisplayMember>[]>(() => [
    { id: 'member', header: copy.member, cell: ({ row }) => <UserIdentity name={row.original.user_name} avatarUrl={row.original.user_avatar_url} /> },
    ...metrics.map<ColumnDef<TeamDisplayMember>>((metric) => ({
      id: metric.key,
      meta: { ariaSort: metric.key === activeMetric.key ? effectiveDescending ? 'descending' : 'ascending' : 'none' },
      header: () => <MetricSortHeader metric={metric} active={metric.key === activeMetric.key} descending={effectiveDescending} onSort={handleSort} />,
      cell: ({ row }) => <MetricCell row={row.original} metric={metric} />,
    })),
  ], [activeMetric.key, copy.member, effectiveDescending, metrics])
  const changeRole = (nextRole: TeamRoleKey) => {
    const firstMetric = TEAM_ROLE_METRICS[nextRole][0]
    setSortMetricKey(firstMetric.key); setDescending(firstMetric.defaultDescending); setSelectedMemberId(undefined); onRoleChange(nextRole)
  }
  const openDrilldown = (member: TeamDisplayMember) => {
    if (!canViewDetails) return
    setDrilldownItemType(role === 'product' ? 'requirement' : role === 'testing' ? 'bug' : 'task')
    setDrilldownTarget({ userId: member.user_id, userName: member.user_name, role })
  }
  const memberFilterActive = selectedMemberId !== undefined
  return (
    <section aria-labelledby="bi-screen-team-member-title" className="grid min-w-0 gap-3">
      <header className="grid gap-3">
        <div><h2 id="bi-screen-team-member-title" className="m-0 text-sm font-semibold">{copy.memberTitle}</h2></div>
        <AdaptiveFilterBar controls={[
          { id: 'role', label: copy.roleFilter, node: (<ViewTabs value={role} label={copy.roleFilter} tabs={availableRoles.map((roleKey) => ({ id: roleKey, label: copy.roleLabels[roleKey] }))} onValueChange={(v) => changeRole(v as TeamRoleKey)} />) },
          { id: 'member', label: copy.memberSearch, node: (<UserSelect id="bi-team-member-filter" label={copy.memberSearch} value={selectedMemberId} options={memberOptions} emptyLabel={copy.allMembers} placeholder={copy.memberSearchPlaceholder} clearable className="w-full" onValueChange={setSelectedMemberId} />) },
        ]} />
      </header>
      <div data-slot="team-member-metrics-layout" className="grid min-w-0 items-start gap-3 xl:h-[28rem] xl:grid-cols-[minmax(0,3fr)_minmax(22rem,2fr)] xl:items-stretch">
        <div data-slot="team-member-metrics-table-scroll" className="grid min-w-0 gap-1.5 xl:h-full xl:grid-rows-[auto_minmax(0,1fr)] xl:overflow-hidden xl:rounded-md">
          <p data-slot="team-member-effective-sample-note" className="m-0 px-1 text-xs leading-5 text-muted-foreground">{copy.effectiveSampleNotes[role]}</p>
          <DataTable
            aria-label={copy.memberAriaLabel}
            className="xl:h-full xl:min-h-0 xl:[&_[data-slot=table-scroll-shell]]:h-full"
            columns={columns}
            tableClassName="min-w-[640px]"
            scrollAreaClassName="xl:h-full"
            state={rows.length ? { status: 'ready', rows } : memberFilterActive ? { status: 'empty', title: copy.memberSearchEmptyTitle } : { status: 'empty', title: copy.emptyTitle }}
            getRowId={(row) => `${role}-${row.user_id}`}
            // 行级交互类名是静态字符串；规则无法穿透函数形式的 className，行内豁免
            // eslint-disable-next-line shadcn/require-static-classes
            getRowClassName={() => 'cursor-pointer hover:!bg-primary/[0.06] focus-visible:!bg-primary/[0.06]'}
            rowAction={canViewDetails ? { label: (row) => copy.memberDrilldownTitle(copy.roleLabels[role], row.user_name), onActivate: openDrilldown } : undefined}
          />
        </div>
        <TeamChartView role={role} roleLabel={group?.role_label || copy.roleLabels[role]} members={rows} metric={activeMetric} descending={effectiveDescending} />
      </div>
      {environment && route && <TeamMemberDrilldownPanel environment={environment} route={route} target={drilldownTarget} itemType={drilldownItemType} onItemTypeChange={setDrilldownItemType} onOpenChange={(open) => { if (!open) setDrilldownTarget(undefined) }} canView={canViewDetails} />}
    </section>
  )
}
