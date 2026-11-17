import { DayPilot, DayPilotScheduler } from '@daypilot/daypilot-lite-react'
import { useMemo } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  ganttDatePatch,
  type GanttOwnerGroup,
  type GanttWindow,
  type ScheduledWorkItem,
} from '../../model/gantt'
import type { WorkItem } from '../../model/types'
import type { WorkItemDetailPatch } from '../../model/mutation'

import './daypilot.css'

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

function taskResourceId(row: ScheduledWorkItem): string {
  return `task:${row.item.rowId}`
}

function triggerFromEvent(event: MouseEvent): HTMLElement | undefined {
  return event.target instanceof HTMLElement ? event.target : undefined
}

function taskHeaderHtml(index: number, item: WorkItem): string {
  const key = escapeHtml(item.taskKey || WORK_ITEMS_COPY.list.fallbackId(item.id))
  const title = escapeHtml(item.title || WORK_ITEMS_COPY.list.noTitle)
  return `<div class="work-items-daypilot-task-header"><span class="work-items-daypilot-index">${index}</span><span class="work-items-daypilot-description"><strong>${key}</strong><span>${title}</span></span></div>`
}

function userInitial(label: string): string {
  return [...label.trim()][0]?.toLocaleUpperCase() || '·'
}

function groupHeaderHtml(group: GanttOwnerGroup, collapsed: boolean): string {
  const avatar = group.avatarUrl
    ? `<img src="${escapeHtml(group.avatarUrl)}" alt="" class="size-full object-cover" />`
    : `<span data-slot="user-preview-initial">${escapeHtml(userInitial(group.label))}</span>`
  const secondary = group.secondary
    ? `<small class="truncate text-xs font-normal text-muted-foreground">${escapeHtml(group.secondary)}</small>`
    : ''
  return `<div class="work-items-daypilot-group-header"><span aria-hidden="true">${collapsed ? '&#9656;' : '&#9662;'}</span><span data-slot="user-preview" data-size="sm" class="inline-flex min-w-0 items-center gap-1.5"><span data-slot="user-preview-avatar" aria-hidden="true" class="grid size-5 shrink-0 place-items-center overflow-hidden rounded-full bg-muted text-xs font-semibold text-muted-foreground">${avatar}</span><span class="grid min-w-0 leading-tight"><strong class="truncate text-xs font-normal text-foreground">${escapeHtml(group.label)}</strong>${secondary}</span></span></div>`
}

function eventHtml(row: ScheduledWorkItem): string {
  const duration = escapeHtml(WORK_ITEMS_COPY.gantt.duration(row.durationDays))
  const title = escapeHtml(row.item.title || WORK_ITEMS_COPY.list.noTitle)
  return `<div class="work-items-daypilot-event-content"><span class="work-items-daypilot-duration">${duration}</span><span class="work-items-daypilot-event-title">${title}</span></div>`
}

function timeHeaders(window: GanttWindow): DayPilot.TimeHeaderData[] {
  return window.headers.map((groupBy) => ({
    groupBy,
    format: groupBy === 'Month' ? 'yyyy年M月' : groupBy === 'Day' ? 'd' : 'M/d',
    height: 38,
  }))
}

export function GanttScheduler({
  groups,
  window,
  narrow,
  collapsedGroupIds,
  onToggleGroup,
  onOpen,
  onUpdate,
}: {
  groups: readonly GanttOwnerGroup[]
  window: GanttWindow
  narrow: boolean
  collapsedGroupIds: ReadonlySet<string>
  onToggleGroup: (groupId: string) => void
  onOpen?: (row: WorkItem, trigger: HTMLElement) => void
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  const model = useMemo(() => {
    const resources: DayPilot.ResourceData[] = []
    const events: DayPilot.EventData[] = []
    const rowByResource = new Map<string, ScheduledWorkItem>()
    for (const group of groups) {
      const collapsed = collapsedGroupIds.has(group.id)
      resources.push({
        id: group.id,
        name: group.label,
        html: groupHeaderHtml(group, collapsed),
        ariaLabel: (collapsed ? WORK_ITEMS_COPY.gantt.expandGroup : WORK_ITEMS_COPY.gantt.collapseGroup)(group.label),
        cssClass: 'work-items-daypilot-group',
        tags: { kind: 'group' },
      })
      if (collapsed) continue
      group.rows.forEach((row, index) => {
        const resource = taskResourceId(row)
        rowByResource.set(resource, row)
        resources.push({
          id: resource,
          name: row.item.title,
          html: taskHeaderHtml(index + 1, row.item),
          ariaLabel: WORK_ITEMS_COPY.gantt.openTask(row.item.title || WORK_ITEMS_COPY.list.noTitle),
          cssClass: 'work-items-daypilot-task',
          tags: { kind: 'task' },
        })
        events.push({
          id: row.item.rowId,
          resource,
          start: row.start,
          end: row.endExclusive,
          text: row.item.title || WORK_ITEMS_COPY.list.noTitle,
          html: eventHtml(row),
          cssClass: 'work-items-daypilot-event',
          backColor: 'transparent',
          borderColor: 'transparent',
          barHidden: true,
          toolTip: `${row.item.title || WORK_ITEMS_COPY.list.noTitle} · ${WORK_ITEMS_COPY.format.dateRange(row.item.startsAt ?? row.start, row.item.dueAt ?? row.start)}`,
          tags: { resource },
        })
      })
    }
    return { events, resources, rowByResource }
  }, [collapsedGroupIds, groups])

  const updateDates = (
    row: ScheduledWorkItem,
    start: DayPilot.Date,
    endExclusive: DayPilot.Date,
  ) => {
    const patch = ganttDatePatch(
      start.toString('yyyy-MM-dd'),
      endExclusive.toString('yyyy-MM-dd'),
    )
    if (patch && row.item.capabilities.canUpdate) void onUpdate(row.item, patch)
  }

  return (
    <div className="work-items-daypilot min-w-0" data-testid="work-items-daypilot-scheduler">
      <DayPilotScheduler
        startDate={window.startDate}
        days={window.days}
        scale={window.cellScale}
        cellWidth={window.cellWidth}
        timeHeaders={timeHeaders(window)}
        resources={model.resources}
        events={model.events}
        rowHeaderWidth={narrow ? 280 : 390}
        cornerHtml={`<div class="work-items-daypilot-corner"><span></span><strong>${escapeHtml(WORK_ITEMS_COPY.gantt.taskDescription)}</strong></div>`}
        locale="zh-cn"
        height={narrow ? 430 : 520}
        heightSpec="Max"
        eventHeight={30}
        rowMarginTop={8}
        rowMarginBottom={8}
        eventMoveHandling="Update"
        eventResizeHandling="Update"
        eventDeleteHandling="Disabled"
        timeRangeSelectedHandling="Disabled"
        eventClickHandling="Disabled"
        eventRightClickHandling="Disabled"
        showToolTip
        xssProtection="Enabled"
        onEventMove={(args) => {
          args.preventDefault()
          const resource = String(args.e.resource())
          if (String(args.newResource) !== resource) return
          const row = model.rowByResource.get(resource)
          if (row) updateDates(row, args.newStart, args.newEnd)
        }}
        onEventResize={(args) => {
          args.preventDefault()
          const row = model.rowByResource.get(String(args.e.resource()))
          if (row) updateDates(row, args.newStart, args.newEnd)
        }}
        onRowClick={(args) => {
          const resource = String(args.row.id)
          const group = groups.find((entry) => entry.id === resource)
          if (group) {
            onToggleGroup(group.id)
            return
          }
          const item = model.rowByResource.get(resource)?.item
          const trigger = triggerFromEvent(args.originalEvent)
          const taskHeader = trigger?.closest<HTMLElement>('.work-items-daypilot-task-header')
          if (item && taskHeader) onOpen?.(item, taskHeader)
        }}
      />
    </div>
  )
}
