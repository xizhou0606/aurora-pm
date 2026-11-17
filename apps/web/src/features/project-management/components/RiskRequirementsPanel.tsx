import { useState } from 'react'
import { PhWarning, PhArrowUpRight, PhBug, PhCaretDown, PhCaretUp, PhClipboardText, PhListChecks, PhShieldCheck } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, IconBadge, StatusPill } from '@/shared/ui'
import {
  projectItemTypeLabel,
  statusLabel,
  type ProjectRiskRequirement,
  type ProjectRiskRequirementDimension,
} from '../model'

const MAX_VISIBLE_REQUIREMENTS = 5

function limitedRiskRequirements(items: readonly ProjectRiskRequirement[]) {
  if (items.length <= MAX_VISIBLE_REQUIREMENTS) return [...items]
  const critical = items.filter((item) => item.severity === 'critical')
  const high = items.filter((item) => item.severity === 'high')
  if (critical.length === 0 || high.length === 0) return [...items.slice(0, MAX_VISIBLE_REQUIREMENTS)]
  const criticalSlots = Math.min(critical.length, MAX_VISIBLE_REQUIREMENTS - 1)
  return [
    ...critical.slice(0, criticalSlots),
    ...high.slice(0, MAX_VISIBLE_REQUIREMENTS - criticalSlots),
  ]
}

function dimensionTone(dimension: ProjectRiskRequirementDimension) {
  return dimension === 'task_unassigned' ? 'warning' as const : 'danger' as const
}

function dimensionLabel(
  item: ProjectRiskRequirement,
  dimension: ProjectRiskRequirementDimension,
) {
  const risk = copy.projectManagement.overview.riskRequirements
  if (dimension === 'task_overdue') return risk.dimensionTaskOverdue(item.overdueTasks)
  if (dimension === 'task_unassigned') return risk.dimensionTaskUnassigned(item.unassignedTasks)
  return risk.dimensionBugOpen(item.openBugs)
}

function RiskRequirementRow({
  item,
  onOpen,
}: {
  item: ProjectRiskRequirement
  onOpen: (item: ProjectRiskRequirement) => void
}) {
  const risk = copy.projectManagement.overview.riskRequirements
  return (
    <li>
      <Button
        type="button"
        variant="ghost"
        aria-label={risk.openLabel(item.title)}
        className={cn(
          'group grid h-auto w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2 rounded-none border-l-2 px-3 py-3 text-left whitespace-normal shadow-none transition-colors sm:px-4',
          'hover:bg-muted/25 focus-visible:z-10 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring',
          'lg:grid-cols-[minmax(12rem,0.85fr)_minmax(16rem,1.15fr)_auto] lg:items-center',
          item.severity === 'critical' ? 'border-l-destructive' : 'border-l-amber-500', // eslint-disable-line shadcn/no-restyle -- 数据驱动的风险等级左边框色，无对应变体
        )}
        onClick={() => onOpen(item)}
      >
        <span className="min-w-0">
          <strong className="line-clamp-2 block text-sm font-semibold leading-5 text-foreground">
            {item.title}
          </strong>
          <span className="mt-1 block truncate text-xs text-muted-foreground">
            {risk.planUnit(
              item.planUnitType ? projectItemTypeLabel(item.planUnitType) : '',
              item.planUnitTitle,
            )}
          </span>
        </span>

        <span className="col-span-2 grid min-w-0 gap-2 lg:col-span-1 lg:col-start-2 lg:row-start-1">
          <span className="grid min-w-0 grid-cols-3 gap-2">
            <span className="min-w-0">
              <span className="flex items-center gap-1 text-xs text-muted-foreground">
                <PhClipboardText aria-hidden="true" className="size-3 shrink-0" />
                {risk.metricRequirement}
              </span>
              <strong className="mt-0.5 block truncate text-sm font-semibold tabular-nums text-foreground">
                {statusLabel(item.requirementStatus)}
              </strong>
            </span>
            <span className="min-w-0">
              <span className="flex items-center gap-1 text-xs text-muted-foreground">
                <PhListChecks aria-hidden="true" className="size-3 shrink-0" />
                {risk.metricTask}
              </span>
              <strong className="mt-0.5 block truncate text-sm font-semibold tabular-nums text-foreground">
                {risk.taskProgress(item.completedTasks, item.totalTasks)}
              </strong>
            </span>
            <span className="min-w-0">
              <span className="flex items-center gap-1 text-xs text-muted-foreground">
                <PhBug aria-hidden="true" className="size-3 shrink-0" />
                {risk.metricBug}
              </span>
              <strong className="mt-0.5 block truncate text-sm font-semibold tabular-nums text-foreground">
                {risk.bugCount(item.openBugs)}
              </strong>
            </span>
          </span>

          <span className="flex min-w-0 flex-wrap gap-1.5">
            {item.dimensions.map((dimension) => (
              <StatusPill
                key={dimension}
                tone={dimensionTone(dimension)}
                className="min-w-0 px-2 py-0.5 text-xs"
              >
                {dimensionLabel(item, dimension)}
              </StatusPill>
            ))}
          </span>
        </span>

        <PhArrowUpRight
          aria-hidden="true"
          className="col-start-2 row-start-1 size-4 shrink-0 text-muted-foreground transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-primary lg:col-start-3"
        />
      </Button>
    </li>
  )
}

function RiskRequirementGroup({
  severity,
  items,
  total,
  onOpen,
}: {
  severity: ProjectRiskRequirement['severity']
  items: readonly ProjectRiskRequirement[]
  total: number
  onOpen: (item: ProjectRiskRequirement) => void
}) {
  const risk = copy.projectManagement.overview.riskRequirements
  if (items.length === 0) return null
  const critical = severity === 'critical'
  return (
    <section aria-label={critical ? risk.critical : risk.high}>
      <header className={cn(
        'flex min-w-0 items-center justify-between gap-3 border-y border-border px-3 py-2 first:border-t-0 sm:px-4',
        critical ? 'bg-destructive/[0.045]' : 'bg-amber-50 dark:bg-amber-950/400/[0.055]',
      )}>
        <span className="min-w-0">
          <strong className={cn('text-xs font-semibold', critical ? 'text-destructive' : 'text-amber-700 dark:text-amber-400')}>
            {critical ? risk.critical : risk.high}
          </strong>
          <span className="ml-2 text-xs text-muted-foreground">
            {critical ? risk.criticalDescription : risk.highDescription}
          </span>
        </span>
        <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
          {risk.groupCount(items.length, total)}
        </span>
      </header>
      <ol className="divide-y divide-border">
        {items.map((item) => (
          <RiskRequirementRow key={item.id} item={item} onOpen={onOpen} />
        ))}
      </ol>
    </section>
  )
}

export function RiskRequirementsPanel({
  items,
  checkedRequirements,
  checkedTasks,
  checkedBugs,
  onOpen,
}: {
  items: readonly ProjectRiskRequirement[]
  checkedRequirements: number
  checkedTasks: number
  checkedBugs: number
  onOpen: (item: ProjectRiskRequirement) => void
}) {
  const risk = copy.projectManagement.overview.riskRequirements
  const [showAll, setShowAll] = useState(false)
  const visible = showAll ? items : limitedRiskRequirements(items)
  const criticalTotal = items.filter((item) => item.severity === 'critical').length
  const highTotal = items.length - criticalTotal
  const remaining = Math.max(0, items.length - visible.length)

  return (
    <section
      aria-labelledby="pm-risk-requirements-heading"
      data-testid="pm-risk-requirements"
      className="grid min-w-0 content-start gap-3 rounded-lg border border-border bg-card p-3 sm:p-4"
    >
      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <span className="flex min-w-0 items-start gap-2.5">
          <span className={cn(
            'mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg',
            items.length > 0
              ? 'bg-destructive/10 text-destructive'
              : 'bg-emerald-50 dark:bg-emerald-950/400/10 text-emerald-700 dark:text-emerald-400',
          )}>
            {items.length > 0
              ? <PhWarning aria-hidden="true" className="size-4" />
              : <PhShieldCheck aria-hidden="true" className="size-4" />}
          </span>
          <span className="min-w-0">
            <h3 id="pm-risk-requirements-heading" className="text-sm font-semibold text-foreground">
              {risk.heading}
            </h3>
            <p className="mt-0.5 text-xs leading-5 text-muted-foreground">{risk.description}</p>
            <p className="mt-1 text-xs text-muted-foreground">
              {risk.evidence(checkedRequirements, checkedTasks, checkedBugs)}
            </p>
          </span>
        </span>
        {items.length > 0 ? (
          <span className="flex shrink-0 flex-wrap gap-1.5 sm:justify-end">
            {criticalTotal > 0 ? (
              <StatusPill tone="danger" className="min-w-0">
                {risk.critical} {criticalTotal}
              </StatusPill>
            ) : null}
            {highTotal > 0 ? (
              <StatusPill tone="warning" className="min-w-0">
                {risk.high} {highTotal}
              </StatusPill>
            ) : null}
          </span>
        ) : null}
      </header>

      {visible.length > 0 ? (
        <>
          <div id="pm-risk-requirement-list" className="overflow-hidden rounded-lg border border-border">
            <RiskRequirementGroup
              severity="critical"
              items={visible.filter((item) => item.severity === 'critical')}
              total={criticalTotal}
              onOpen={onOpen}
            />
            <RiskRequirementGroup
              severity="high"
              items={visible.filter((item) => item.severity === 'high')}
              total={highTotal}
              onOpen={onOpen}
            />
          </div>
          {items.length > MAX_VISIBLE_REQUIREMENTS ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              className="justify-self-center"
              aria-expanded={showAll}
              aria-controls="pm-risk-requirement-list"
              onClick={() => setShowAll((current) => !current)}
            >
              {showAll ? <PhCaretUp aria-hidden="true" /> : <PhCaretDown aria-hidden="true" />}
              {showAll ? risk.showLess : risk.showRemaining(remaining)}
            </Button>
          ) : null}
        </>
      ) : (
        <div className="flex items-center gap-3 rounded-lg border border-emerald-200 dark:border-emerald-800/80 bg-emerald-50/55 dark:bg-emerald-950/40 px-3 py-3 text-left">
          <IconBadge icon={PhShieldCheck} tone="emerald" variant="soft" size="md" />
          <span className="min-w-0">
            <strong className="block text-sm font-medium text-foreground">{risk.emptyTitle}</strong>
            <span className="mt-0.5 block text-xs leading-5 text-muted-foreground">
              {risk.emptyDescription}
            </span>
          </span>
        </div>
      )}
    </section>
  )
}
