import type { ReactNode } from 'react'
import { PhCalendar, PhClipboardText, PhKanban, PhStack, PhShieldCheck } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { StatusPill, UserPreview, type StatusPillTone } from '@/shared/ui'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'
import {
  clampProgress,
  aggregatePlanUnitSchedule,
  formatPMDate,
  formatPMDateRange,
  healthLabel,
  projectItemTypeLabel,
  riskLabel,
  statusLabel,
  planUnitStatusTone,
  projectStatusTone,
  type ResourceHeaderModel,
} from '../model'

type ResourceHeroProps = {
  header: ResourceHeaderModel
  actions?: ReactNode
  title?: ReactNode
  /** Inline meta rendered in the banner row between the heading and the actions. */
  inlineMeta?: ReactNode
  footer?: ReactNode
  density?: 'default' | 'compact'
} & (
  | { kind: 'project'; project: ScrumProjectNew; planUnits: ScrumVersionTaskNew[] }
  | { kind: 'plan-unit'; project: ScrumProjectNew; unit: ScrumVersionTaskNew }
)

const healthTone = (health?: string): StatusPillTone =>
  health === 'on_track' ? 'success' : health === 'at_risk' ? 'warning' : health === 'off_track' ? 'danger' : 'neutral'

const riskTone = (risk?: string): StatusPillTone =>
  risk === 'high' ? 'danger' : risk === 'medium' ? 'warning' : risk === 'low' ? 'success' : 'neutral'

function Metric({
  icon,
  label,
  value,
  description,
  className,
  compact,
}: {
  icon: ReactNode
  label: string
  value: ReactNode
  description?: ReactNode
  className?: string
  compact?: boolean
}) {
  return (
    <div className={cn('pm-resource-metric grid min-w-0 bg-card', compact ? 'gap-2 px-3 py-2' : 'gap-2.5 px-4 py-3', className)}>
      <span className="mt-0.5 grid size-7 place-items-center rounded-md bg-primary/10 text-primary">
        {icon}
      </span>
      <span className="min-w-0">
        <span className="block text-xs font-medium text-muted-foreground">{label}</span>
        <span className="mt-0.5 block min-w-0 text-sm font-semibold text-foreground">{value}</span>
        {description ? <span className="mt-0.5 block truncate text-xs text-muted-foreground">{description}</span> : null}
      </span>
    </div>
  )
}

function ProgressMetric({ value, description, compact }: { value: number; description: string; compact?: boolean }) {
  return (
    <Metric
      icon={<PhShieldCheck aria-hidden="true" className="size-3.5" />}
      label={copy.projectManagement.planUnitDetail.progress}
      value={`${value}%`}
      compact={compact}
      description={(
        <span className="grid gap-1">
          <span role="progressbar" aria-label={copy.projectManagement.planUnitDetail.progress} aria-valuemin={0} aria-valuemax={100} aria-valuenow={value} className="inline-flex h-1.5 w-16 overflow-hidden rounded-full bg-muted align-middle">
            <span className="block h-full w-(--hero-progress-width) rounded-full bg-primary" style={{ '--hero-progress-width': `${value}%` }} />
          </span>
          <span>{description}</span>
        </span>
      )}
    />
  )
}

const inlineFact = (label: string, value: ReactNode) => (
  <span className="inline-flex min-w-0 max-w-full items-center gap-1">
    <span className="shrink-0 text-muted-foreground">{label}</span>
    <span className="min-w-0 truncate font-medium text-foreground">{value}</span>
  </span>
)

export function ResourceHero(props: ResourceHeroProps) {
  const compact = props.density === 'compact'
  const resource = props.kind === 'project' ? props.project : props.unit
  const itemType = props.kind === 'project'
    ? copy.projectManagement.projects
    : projectItemTypeLabel(props.unit.type)
  const progress = clampProgress(resource.progress)
  const Icon = props.kind === 'project'
    ? PhStack
    : props.unit.type === 'version' ? PhCalendar : PhKanban
  const owner = props.header.owner
  const projectAggregate = props.kind === 'project' && !props.project.starts_at && !props.project.due_at
    ? aggregatePlanUnitSchedule(props.planUnits)
    : undefined
  const aggregateAvailable = Boolean(projectAggregate?.startsAt || projectAggregate?.dueAt)
  const aggregateComplete = Boolean(projectAggregate?.startsAt && projectAggregate?.dueAt)
  const scheduleLabel = aggregateAvailable
    ? copy.projectManagement.overview.childScheduleRange
    : copy.projectManagement.planUnitDetail.dateRange
  const scheduleValue = aggregateComplete
    ? formatPMDateRange(projectAggregate?.startsAt, projectAggregate?.dueAt)
    : projectAggregate?.startsAt
      ? copy.projectManagement.overview.childScheduleStartOnly(formatPMDate(projectAggregate.startsAt))
      : projectAggregate?.dueAt
        ? copy.projectManagement.overview.childScheduleEndOnly(formatPMDate(projectAggregate.dueAt))
    : props.kind === 'project' && !props.project.starts_at && !props.project.due_at
      ? copy.projectManagement.overview.unscheduled
      : formatPMDateRange(resource.starts_at, resource.due_at)
  const scheduleDescription = aggregateComplete
    ? copy.projectManagement.overview.childScheduleDescription(projectAggregate?.contributingUnits ?? 0)
    : aggregateAvailable
      ? copy.projectManagement.overview.childSchedulePartialDescription(projectAggregate?.contributingUnits ?? 0)
    : props.kind === 'project' && !props.project.starts_at && !props.project.due_at
      ? copy.projectManagement.overview.unscheduledDescription
      : props.kind === 'project'
        ? `${Number(props.project.version_task_count || 0)} ${copy.projectManagement.planUnits}`
        : `${Number(props.unit.requirement_count || 0)} ${copy.projectManagement.tabs.requirements}`
  const hasSchedule = aggregateAvailable || Boolean(resource.starts_at || resource.due_at)
  const ownerLabel = owner?.label ?? copy.projectManagement.overview.owner
  const ownerValue = owner ? (
    <UserPreview
      name={owner.preview.name}
      secondary={owner.preview.secondary}
      avatarUrl={owner.preview.avatarUrl}
      size="sm"
      className="max-w-full"
    />
  ) : copy.projectManagement.overview.unset
  const progressDescription = props.kind === 'project'
    ? `${Number(props.project.requirement_count || 0)} ${copy.projectManagement.tabs.requirements}`
    : copy.projectManagement.planUnitList.taskProgress(
        Number(props.unit.completed_task_count || 0),
        Number(props.unit.total_task_count || 0),
      )
  const compactFacts = [
    owner ? inlineFact(ownerLabel, ownerValue) : null,
    hasSchedule ? inlineFact(scheduleLabel, scheduleValue) : null,
    props.kind === 'plan-unit' ? inlineFact(copy.projectManagement.tabs.tasks, progressDescription) : null,
    props.kind === 'plan-unit' ? inlineFact(copy.projectManagement.tabs.requirements, String(Number(props.unit.requirement_count || 0))) : null,
  ].filter(Boolean)
  const pills = (
    <>
      <StatusPill tone={props.kind === 'project'
        ? projectStatusTone(resource.status)
        : planUnitStatusTone(resource.status)}>{statusLabel(resource.status)}</StatusPill>
      <StatusPill tone={healthTone(resource.health_status)}>{healthLabel(resource.health_status)}</StatusPill>
      {props.kind === 'plan-unit' ? (
        <StatusPill tone={riskTone(props.unit.risk_level)}>{riskLabel(props.unit.risk_level)}</StatusPill>
      ) : null}
    </>
  )
  const metaAnnouncement = (
    <span className="sr-only" aria-hidden="true">
      {owner ? `${owner.label}：${owner.preview.name} ` : ''}
      {props.header.meta.join(' ')}
    </span>
  )

  return (
    <header
      data-slot="pm-resource-hero"
      data-resource-kind={props.kind}
      data-density={compact ? 'compact' : 'default'}
      className="overflow-hidden rounded-lg border border-border bg-card"
    >
      <div className={cn(
        'pm-resource-hero__banner flex min-w-0 flex-col',
        props.inlineMeta
          ? 'gap-2 px-4 py-2.5 lg:flex-row lg:items-center lg:justify-between'
          : compact
            ? 'gap-2 px-4 py-2.5 sm:flex-row sm:items-center sm:justify-between'
            : 'gap-4 px-4 py-4 sm:flex-row sm:items-start sm:justify-between sm:px-5',
      )}>
        <div className={cn(
          'pm-resource-hero__heading flex min-w-0 items-start',
          props.inlineMeta ? 'lg:flex-1' : undefined,
        )}>
          <span className="pm-resource-hero__icon grid shrink-0 place-items-center rounded-lg bg-primary text-primary-foreground">
            <Icon aria-hidden="true" className="size-5" />
          </span>
          <div className="min-w-0">
            {compact ? (
              <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                {props.title ?? (
                  <h2 className="m-0 truncate text-lg font-semibold tracking-tight text-foreground">
                    {props.header.title}
                  </h2>
                )}
                {compactFacts.length ? (
                  <div
                    data-slot="page-header-meta"
                    aria-label={copy.projectManagement.overview.metricsLabel}
                    className="pm-resource-hero__facts inline-flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs leading-4"
                  >
                    {metaAnnouncement}
                    {compactFacts.map((fact, index) => (
                      <span key={index} className="inline-flex min-w-0 items-center gap-x-2">
                        {index > 0 ? <span aria-hidden="true" className="select-none text-border">{copy.projectManagement.common.inlineSeparator}</span> : null}
                        {fact}
                      </span>
                    ))}
                  </div>
                ) : metaAnnouncement}
              </div>
            ) : (
              <>
                <p className="pm-resource-hero__kicker m-0 text-xs font-semibold text-primary uppercase">{itemType}</p>
                {props.title ?? (
                  <h2 className="mt-0.5 mb-0 truncate text-xl font-semibold tracking-tight text-foreground">
                    {props.header.title}
                  </h2>
                )}
              </>
            )}
            {compact ? null : (
              <>
                <p className="mt-1 mb-0 max-w-3xl text-xs leading-5 text-muted-foreground">{props.header.description}</p>
                <div className="mt-2.5 flex flex-wrap gap-1.5">{pills}</div>
              </>
            )}
          </div>
        </div>
        {props.inlineMeta ? (
          <div data-slot="pm-resource-hero-inline-meta" className="flex shrink-0 items-center">{props.inlineMeta}</div>
        ) : null}
        {props.actions ? <div data-slot="page-header-actions" className="flex shrink-0 flex-wrap items-center gap-2">{props.actions}</div> : null}
      </div>

      {compact ? null : (
        <div data-slot="page-header-meta" aria-label={copy.projectManagement.overview.metricsLabel} className="grid min-w-0 grid-cols-2 gap-px border-t border-border bg-border xl:grid-cols-4">
          {metaAnnouncement}
          <Metric
            icon={<PhKanban aria-hidden="true" className="size-3.5" />}
            label={ownerLabel}
            value={ownerValue}
          />
          <Metric
            icon={<PhCalendar aria-hidden="true" className="size-3.5" />}
            label={scheduleLabel}
            value={scheduleValue}
            description={scheduleDescription}
          />
          <ProgressMetric
            value={progress}
            description={progressDescription}
          />
          {props.kind === 'plan-unit' ? (
            <Metric
              icon={<PhClipboardText aria-hidden="true" className="size-3.5" />}
              label={copy.projectManagement.tabs.requirements}
              value={String(Number(props.unit.requirement_count || 0))}
              description={copy.projectManagement.planUnitList.taskProgress(
                Number(props.unit.completed_task_count || 0),
                Number(props.unit.total_task_count || 0),
              )}
            />
          ) : (
            <Metric
              icon={<PhShieldCheck aria-hidden="true" className="size-3.5" />}
              label={copy.projectManagement.overview.metricHealth}
              value={healthLabel(props.project.health_status)}
              description={props.project.health_reason || copy.projectManagement.overview.healthReasonUnset}
            />
          )}
        </div>
      )}
      {props.footer ? (
        <div data-slot="pm-resource-hero-footer" className={cn('min-w-0', !compact && 'border-t border-border')}>
          {props.footer}
        </div>
      ) : null}
    </header>
  )
}
