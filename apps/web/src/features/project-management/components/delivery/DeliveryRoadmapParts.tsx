import { useEffect, useMemo, useState } from 'react'
import { PhCheck, PhCircle, PhClock, PhPencil } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, EmptyState, StatusPill, UserPreview, type StatusPillTone } from '@/shared/ui'
import type {
  DeliveryLaneModel,
  DeliveryNodeModel,
  DeliveryNodeState,
  DeliveryRoadmapState,
} from '../../model'
import type { DeliveryRoadmapActions } from './DeliveryRoadmapTypes'

const deliveryCopy = copy.projectManagement.delivery

const nodeTone: Record<DeliveryNodeState, string> = {
  // 交付节点状态为数据驱动取色（emerald=完成 / amber=延迟完成），无对应变体
  done: 'border-emerald-500/30 bg-emerald-50 dark:bg-emerald-950/400/[0.055]', // eslint-disable-line shadcn/no-restyle
  late_done: 'border-amber-500/35 bg-amber-50 dark:bg-amber-950/400/[0.055]', // eslint-disable-line shadcn/no-restyle
  current: 'border-primary/60 bg-primary/[0.06] ring-1 ring-primary/15',
  overdue: 'border-destructive/40 bg-destructive/[0.055]',
  upcoming: 'border-border bg-card',
  unscheduled: 'border-dashed border-border bg-muted/20',
}

const nodePillTone: Record<DeliveryNodeState, StatusPillTone> = {
  done: 'success',
  late_done: 'warning',
  current: 'info',
  overdue: 'danger',
  upcoming: 'neutral',
  unscheduled: 'neutral',
}

function NodeIcon({ state }: { state: DeliveryNodeState }) {
  if (state === 'done' || state === 'late_done') {
    return <PhCheck aria-hidden="true" className="size-3.5" />
  }
  if (state === 'current' || state === 'overdue') {
    return <PhClock aria-hidden="true" className="size-3.5" />
  }
  return <PhCircle aria-hidden="true" className="size-3.5" />
}

function DeliveryNodeCard({
  node,
  action,
}: {
  node: DeliveryNodeModel
  action?: () => void
}) {
  const dateLabel = node.actualCompletedAt
    ? deliveryCopy.actualCompletedAt
    : deliveryCopy.dueAt
  const dateValue = (node.actualCompletedAt ?? node.dueAt)?.slice(0, 10) ||
    copy.projectManagement.common.unknownDate
  const content = (
    <>
      {action ? <span className="sr-only">{deliveryCopy.quickEdit}</span> : null}
      <span className="flex min-w-0 items-start justify-between gap-2">
        <span className="flex min-w-0 items-start gap-2">
          <span className={cn(
            'mt-0.5 grid size-6 shrink-0 place-items-center rounded-full border bg-card',
            node.state === 'overdue' && 'border-destructive/40 text-destructive',
            node.state === 'current' && 'border-primary/40 text-primary',
            (node.state === 'done' || node.state === 'late_done') &&
              'border-emerald-500/35 text-emerald-700 dark:text-emerald-400',
          )}>
            <NodeIcon state={node.state} />
          </span>
          <span className="min-w-0">
            <span className="line-clamp-2 block text-xs font-semibold leading-4 text-foreground">
              {node.label}
            </span>
            <StatusPill tone={nodePillTone[node.state]} className="mt-1.5">
              {node.stateLabel}
            </StatusPill>
          </span>
        </span>
        {action ? <PhPencil aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" /> : null}
      </span>
      <span className="mt-3 grid gap-2 border-t border-border/70 pt-2.5 text-left">
        <span className="flex items-center justify-between gap-2 text-xs">
          <span className="text-muted-foreground">{dateLabel}</span>
          <span className="font-medium tabular-nums text-foreground">{dateValue}</span>
        </span>
        <span className="flex min-w-0 items-center justify-between gap-2">
          <UserPreview
            name={node.owner.name}
            secondary={node.owner.secondary}
            avatarUrl={node.owner.avatarUrl}
            size="sm"
            className="min-w-0 max-w-28"
          />
          <span className="shrink-0 text-xs text-muted-foreground">
            {node.scheduleOutcome}
          </span>
        </span>
      </span>
      {node.deleted ? (
        <span className="mt-2 inline-flex rounded-full border border-border bg-muted px-2 py-0.5 text-xs text-muted-foreground">
          {deliveryCopy.deletedLabel}
        </span>
      ) : null}
    </>
  )

  const className = cn(
    'block h-full w-[11.5rem] rounded-lg border p-3 text-left outline-none transition-[border-color,background-color,box-shadow,transform]',
    node.state === 'done'
      ? nodeTone.done
      : node.state === 'late_done'
        ? nodeTone.late_done
        : node.state === 'current'
          ? nodeTone.current
          : node.state === 'overdue'
            ? nodeTone.overdue
            : node.state === 'upcoming'
              ? nodeTone.upcoming
              : nodeTone.unscheduled,
    action && 'hover:-translate-y-0.5 hover:border-primary/50 focus-visible:ring-2 focus-visible:ring-ring',
  )
  return action ? (
    <Button type="button" variant="outline" className={className} onClick={action}>
      {content}
    </Button>
  ) : <div className={className}>{content}</div>
}

function preferredLaneId(lanes: readonly DeliveryLaneModel[]): number {
  const scored = lanes.map((lane, index) => ({
    id: lane.id,
    index,
    score: lane.nodes.reduce((sum, node) => (
      sum + (node.state === 'overdue' ? 100 : node.state === 'current' ? 10 : 0)
    ), lane.nodes.length > 0 ? 1 : 0),
  }))
  scored.sort((left, right) => right.score - left.score || left.index - right.index)
  return scored[0]?.id ?? 0
}

function LaneSelector({
  lanes,
  selectedLaneId,
  onSelect,
}: {
  lanes: readonly DeliveryLaneModel[]
  selectedLaneId: number
  onSelect: (laneId: number) => void
}) {
  if (lanes.length <= 1) return null
  return (
    <div
      role="group"
      aria-label={deliveryCopy.selectLane}
      className="flex min-w-0 gap-1.5 overflow-x-auto pb-1"
    >
      {lanes.map((lane) => {
        const selected = lane.id === selectedLaneId
        const delayed = lane.nodes.some((node) => node.state === 'overdue')
        return (
          <Button
            key={lane.id}
            type="button"
            size="xs"
            variant={selected ? 'default' : 'outline'}
            aria-pressed={selected}
            className={cn(
              'shrink-0 gap-1.5',
              !selected && delayed && 'border-destructive/30 text-destructive',
            )}
            onClick={() => onSelect(lane.id)}
          >
            {delayed ? <span aria-hidden="true" className="size-1.5 rounded-full bg-current" /> : null}
            {lane.title}
            <span className={selected ? 'text-primary-foreground/75' : 'text-muted-foreground'}>
              {deliveryCopy.progressValue(lane.progress)}
            </span>
          </Button>
        )
      })}
    </div>
  )
}

export function DeliveryLane({
  lane,
  actions,
}: {
  lane: DeliveryLaneModel
  actions: DeliveryRoadmapActions
}) {
  const editLane = lane.canEdit && actions.editLane
    ? () => actions.editLane?.(lane.id)
    : undefined
  return (
    <article className="min-w-0 rounded-lg border border-border bg-muted/[0.014] p-3">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <p className="m-0 text-xs font-medium text-muted-foreground">
            {deliveryCopy.selectedLane(lane.title)}
          </p>
          <div className="mt-1 flex flex-wrap items-center gap-2">
            <h3 className="m-0 text-sm font-semibold text-foreground">{lane.title}</h3>
            <StatusPill tone="neutral">{lane.status}</StatusPill>
          </div>
        </div>
        <span className="flex items-center gap-2">
          <span className="text-xs font-semibold tabular-nums text-foreground">
            {deliveryCopy.progressValue(lane.progress)}
          </span>
          {editLane ? (
            <Button type="button" size="xs" variant="outline" onClick={editLane}>
              {deliveryCopy.editNodes}
            </Button>
          ) : null}
        </span>
      </header>
      <div className="mt-3 flex items-center gap-2">
        <div
          role="progressbar"
          aria-label={lane.progressLabel}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={lane.progress}
          className="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-muted"
        >
          <span
            aria-hidden="true"
            className="block h-full w-(--lane-progress-width) rounded-full bg-primary transition-[width]"
            style={{ '--lane-progress-width': `${lane.progress}%` }}
          />
        </div>
      </div>
      {lane.nodes.length > 0 ? (
        <ol className="mt-4 flex min-w-0 items-stretch overflow-x-auto pb-2">
          {lane.nodes.map((node, index) => (
            <li key={node.key} className="flex shrink-0 items-center">
              <DeliveryNodeCard
                node={node}
                action={lane.canEdit && actions.quickEdit
                  ? () => actions.quickEdit?.(lane.id, node.key)
                  : undefined}
              />
              {index < lane.nodes.length - 1 ? (
                <span aria-hidden="true" className="relative h-px w-7 shrink-0 bg-border">
                  <span className="absolute -right-0.5 -top-0.5 size-1.5 rotate-45 border-r border-t border-border" />
                </span>
              ) : null}
            </li>
          ))}
        </ol>
      ) : (
        <EmptyState
          title={deliveryCopy.emptyTitle}
          className="mt-3 min-h-40 border border-dashed border-border bg-card"
        />
      )}
    </article>
  )
}

export function ReadyRoadmap({
  state,
  actions,
}: {
  state: Extract<DeliveryRoadmapState, { status: 'ready' }>
  actions: DeliveryRoadmapActions
}) {
  const fallbackLaneId = useMemo(() => preferredLaneId(state.lanes), [state.lanes])
  const [selectedLaneId, setSelectedLaneId] = useState(fallbackLaneId)
  useEffect(() => {
    if (!state.lanes.some((lane) => lane.id === selectedLaneId)) {
      setSelectedLaneId(fallbackLaneId)
    }
  }, [fallbackLaneId, selectedLaneId, state.lanes])
  const selectedLane = state.lanes.find((lane) => lane.id === selectedLaneId) ?? state.lanes[0]
  if (!selectedLane) return null

  return (
    <div className="grid min-w-0 gap-2.5">
      <LaneSelector
        lanes={state.lanes}
        selectedLaneId={selectedLane.id}
        onSelect={setSelectedLaneId}
      />
      <DeliveryLane lane={selectedLane} actions={actions} />
    </div>
  )
}