import {
  Button,
  EmptyState,
  LoadingSkeleton,
  StatusPill,
  UserIdentity,
} from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { workItemPriorityTone, workItemStatusTone } from '@/shared/model'
import { workItemDetailTriggerId } from '../../model/utils'
import type {
  CollectionState,
  UserOptionsState,
  WorkItem,
  WorkItemBoardGroup,
  WorkItemBoardTransition,
} from '../../model/types'
import { priorityLabel, resolveWorkItemAssignee, statusLabel } from '../../model/utils'
import { VirtualRows } from './VirtualRows'

const BOARD_VIRTUAL_TOTAL_THRESHOLD = 500
const BOARD_VIRTUAL_LANE_THRESHOLD = 50

export function Board({
  state,
  groups,
  users,
  transition,
  onOpen,
}: {
  state: CollectionState<WorkItem>
  groups: readonly WorkItemBoardGroup[]
  users: UserOptionsState
  transition: WorkItemBoardTransition
  onOpen?: (row: WorkItem, trigger: HTMLElement) => void
}) {
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={8} label={WORK_ITEMS_COPY.board.loading} />
  }
  if (state.status === 'error') {
    return (
      <EmptyState
        role="alert"
        title={WORK_ITEMS_COPY.board.error}
        action={<Button type="button" onClick={state.retry}>{WORK_ITEMS_COPY.actions.retry}</Button>}
      />
    )
  }
  const hasLanes = groups.some((group) => group.lanes.length > 0)
  if (state.status === 'empty' && !hasLanes) {
    return (
      <section aria-label={WORK_ITEMS_COPY.board.label}>
        {state.refresh === 'refreshing' ? (
          <p role="status">{WORK_ITEMS_COPY.page.updating}</p>
        ) : null}
        {state.refresh === 'stale_error' ? (
          <p role="alert">
            {WORK_ITEMS_COPY.page.staleError}{' '}
            <Button type="button" size="sm" variant="outline" onClick={state.retry}>
              {WORK_ITEMS_COPY.actions.retry}
            </Button>
          </p>
        ) : null}
        <EmptyState title={WORK_ITEMS_COPY.board.empty} />
      </section>
    )
  }
  const totalRows = groups.reduce(
    (groupTotal, group) => groupTotal + group.lanes.reduce(
      (laneTotal, lane) => laneTotal + lane.rows.length,
      0,
    ),
    0,
  )
  const laneThreshold = totalRows >= BOARD_VIRTUAL_TOTAL_THRESHOLD
    ? BOARD_VIRTUAL_LANE_THRESHOLD
    : 100
  const pendingRowId = transition.state.status === 'pending' ? transition.state.rowId : undefined
  return (
    <section className="grid min-w-0 gap-2" aria-label={WORK_ITEMS_COPY.board.label}>
      {transition.state.status === 'pending' ? (
        <p className="m-0 text-sm text-muted-foreground" role="status">
          {WORK_ITEMS_COPY.mutation.pending}
        </p>
      ) : null}
      {transition.state.status === 'error' ? (
        <div className="flex flex-wrap items-center gap-2 text-sm text-destructive" role="alert">
          <span>{WORK_ITEMS_COPY.mutation.failed}</span>
          <Button type="button" size="sm" variant="outline" onClick={transition.retry}>
            {WORK_ITEMS_COPY.actions.retry}
          </Button>
          <Button type="button" size="sm" variant="ghost" onClick={transition.dismiss}>
            {WORK_ITEMS_COPY.mutation.cancel}
          </Button>
        </div>
      ) : null}
      <div className="grid min-w-0 gap-4">
        {groups.map((group) => (
          <section key={group.id} className="grid min-w-0 gap-2" aria-label={group.label || undefined}>
            {group.label ? (
              <h2 className="m-0 truncate text-sm font-semibold text-foreground" title={group.label}>
                {group.label}
              </h2>
            ) : null}
            <div className="h-[min(32rem,calc(100dvh-22rem))] min-h-80 min-w-0 overflow-x-auto pb-2">
              <div className="grid h-full min-w-max auto-cols-[minmax(17rem,20rem)] grid-flow-col items-stretch gap-3 max-[390px]:auto-cols-[minmax(15rem,calc(100vw-3rem))]">
                {group.lanes.map((lane) => {
                  const laneDomId = `work-items-lane-${group.id}-${lane.id}`.replace(/[^a-zA-Z0-9_-]/g, '-')
                  return (
                    <section
                      key={lane.id}
                      className="grid h-full min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] gap-2 rounded-lg border border-border bg-muted p-3 transition-[border-color,box-shadow] data-[drop-target=true]:border-primary data-[drop-target=true]:ring-2 data-[drop-target=true]:ring-primary/25"
                      aria-labelledby={laneDomId}
                      data-drop-target={transition.state.status === 'dragging'
                        && transition.state.targetStatus === lane.id
                        && transition.state.targetLaneId === laneDomId
                        ? 'true'
                        : undefined}
                      onDragOver={(event) => {
                        if (!transition.canDrop(lane.id)) return
                        event.preventDefault()
                        event.dataTransfer.dropEffect = 'move'
                        transition.over(lane.id, laneDomId)
                      }}
                      onDragLeave={(event) => {
                        const nextTarget = event.relatedTarget
                        if (nextTarget instanceof Node && event.currentTarget.contains(nextTarget)) return
                        transition.leave(lane.id, laneDomId)
                      }}
                      onDrop={(event) => {
                        if (!transition.canDrop(lane.id)) return
                        event.preventDefault()
                        transition.drop(lane.id)
                      }}
                    >
                      <header className="flex items-center justify-between gap-2">
                        <h3 id={laneDomId} className="m-0 truncate text-sm font-medium text-foreground">
                          {lane.label}
                        </h3>
                        <span className="shrink-0 text-xs text-muted-foreground">
                          {WORK_ITEMS_COPY.board.count(lane.rows.length)}
                        </span>
                      </header>
                      {lane.rows.length === 0 ? (
                        <div
                          data-testid="work-items-empty-lane"
                          className="grid h-full min-h-0 place-items-center rounded-md border border-dashed border-border/80 bg-background/35 px-4 text-center text-xs text-muted-foreground"
                        >
                          <span>{WORK_ITEMS_COPY.board.emptyLane}</span>
                        </div>
                      ) : (
                        <VirtualRows
                          rows={lane.rows}
                          threshold={laneThreshold}
                          ariaLabelledBy={laneDomId}
                          rowKey={(row) => row.rowId}
                          className="grid h-full min-h-0 max-h-none content-start gap-2 overflow-y-auto overscroll-contain pr-1"
                          renderRow={(row) => {
                            const movable = transition.canDrag(row)
                            const assignee = resolveWorkItemAssignee(
                              row,
                              users.status === 'ready' ? users.data : [],
                            )
                            return (
                              <article
                                data-testid="work-items-card"
                                className={`grid min-h-28 min-w-0 content-between gap-3 rounded-md border border-border bg-card p-4 data-[dragging=true]:opacity-50 data-[drop-pending=true]:opacity-60 ${movable ? 'cursor-grab active:cursor-grabbing' : ''}`}
                                draggable={movable}
                                tabIndex={movable ? 0 : undefined}
                                data-dragging={transition.state.status === 'dragging' && transition.state.rowId === row.rowId ? 'true' : undefined}
                                data-drop-pending={pendingRowId === row.rowId ? 'true' : undefined}
                                aria-busy={pendingRowId === row.rowId || undefined}
                                aria-keyshortcuts={movable ? 'Alt+ArrowLeft Alt+ArrowRight' : undefined}
                                onDragStart={(event) => {
                                  if (!transition.start(row)) {
                                    event.preventDefault()
                                    return
                                  }
                                  event.dataTransfer.effectAllowed = 'move'
                                  event.dataTransfer.setData('text/plain', row.rowId)
                                }}
                                onDragEnd={transition.end}
                                onKeyDown={(event) => {
                                  if (!event.altKey || !['ArrowLeft', 'ArrowRight'].includes(event.key)) return
                                  event.preventDefault()
                                  transition.moveByKeyboard(row, event.key === 'ArrowLeft' ? -1 : 1)
                                }}
                              >
                                <div className="flex min-w-0 items-start justify-between gap-2">
                                  {onOpen ? <Button
                                    id={workItemDetailTriggerId(row)}
                                    type="button"
                                    variant="link"
                                    className="h-auto min-w-0 flex-1 justify-start p-0 text-left"
                                    title={row.title || WORK_ITEMS_COPY.list.noTitle}
                                    aria-label={WORK_ITEMS_COPY.list.openDetail(row.title || WORK_ITEMS_COPY.list.noTitle)}
                                    onClick={(event) => onOpen(row, event.currentTarget)}
                                  >
                                    <span className="min-w-0 truncate">
                                      {row.title || WORK_ITEMS_COPY.list.noTitle}
                                    </span>
                                  </Button> : <span className="min-w-0 flex-1 truncate">{row.title || WORK_ITEMS_COPY.list.noTitle}</span>}
                                  <span className="flex shrink-0 items-center gap-1">
                                    <StatusPill
                                      tone={workItemPriorityTone(row.priority)}
                                      className="min-w-0"
                                    >
                                      {priorityLabel(row.priority)}
                                    </StatusPill>
                                    <StatusPill tone={workItemStatusTone(row.status)} className="min-w-0">
                                      {statusLabel(row.status)}
                                    </StatusPill>
                                  </span>
                                </div>
                                <div className="flex min-w-0 items-center justify-between gap-2">
                                  <span className="shrink-0 font-mono text-xs text-muted-foreground">
                                    {row.taskKey || WORK_ITEMS_COPY.list.fallbackId(row.id)}
                                  </span>
                                  <UserIdentity
                                    name={assignee.name}
                                    avatarUrl={assignee.user?.avatarUrl}
                                    size="sm"
                                    className="max-w-[55%] justify-end"
                                    title={assignee.name}
                                  />
                                </div>
                              </article>
                            )
                          }}
                        />
                      )}
                    </section>
                  )
                })}
              </div>
            </div>
          </section>
        ))}
      </div>
    </section>
  )
}
