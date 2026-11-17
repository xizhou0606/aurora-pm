import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import type { GanttOwnerGroup, GanttWindow } from '../../model/gantt'
import type { WorkItem } from '../../model/types'
import { GanttScheduler } from './GanttScheduler'

type SchedulerProps = {
  eventClickHandling?: string
  eventMoveHandling?: string
  eventResizeHandling?: string
  onEventClick?: () => void
  onEventMove?: (args: {
    e: { resource: () => string }
    newStart: { toString: () => string }
    newEnd: { toString: () => string }
    newResource: string
    preventDefault: () => void
  }) => void
  onEventResize?: (args: {
    e: { resource: () => string }
    newStart: { toString: () => string }
    newEnd: { toString: () => string }
    preventDefault: () => void
  }) => void
  onRowClick?: (args: {
    row: { id: string }
    originalEvent: MouseEvent
  }) => void
}

let schedulerProps: SchedulerProps | undefined

vi.mock('@daypilot/daypilot-lite-react', () => ({
  DayPilot: {},
  DayPilotScheduler: (props: SchedulerProps) => {
    schedulerProps = props
    return <div data-testid="daypilot-stub" />
  },
}))

const task: WorkItem = {
  id: 71,
  rowId: 'task:71',
  taskKey: 'GLOBAL-71',
  title: 'Verify native Gantt',
  startsAt: '2026-08-02',
  dueAt: '2026-08-10',
  watched: false,
  capabilities: {
    canUpdate: true,
    canDelete: true,
    canComment: true,
    canAttach: true,
    canWatch: true,
    canClone: false,
    allowedTransitions: [],
  },
}

const groups: readonly GanttOwnerGroup[] = [{
  id: 'owner:7',
  label: 'Teo',
  rows: [{
    item: task,
    start: '2026-08-02',
    endExclusive: '2026-08-11',
    durationDays: 9,
  }],
}]

const window: GanttWindow = {
  startDate: '2026-08-01',
  days: 31,
  cellScale: 'Day',
  cellWidth: 40,
  headers: ['Month', 'Day'],
}

function clickRow(target: HTMLElement) {
  const originalEvent = new MouseEvent('click', { bubbles: true })
  Object.defineProperty(originalEvent, 'target', { value: target })
  schedulerProps?.onRowClick?.({ row: { id: 'task:task:71' }, originalEvent })
}

describe('GanttScheduler interactions', () => {
  beforeEach(() => {
    schedulerProps = undefined
  })

  it('opens details only from the task title and keeps timeline bars inert', () => {
    const onOpen = vi.fn()
    render(
      <GanttScheduler
        groups={groups}
        window={window}
        narrow={false}
        collapsedGroupIds={new Set()}
        onToggleGroup={vi.fn()}
        onOpen={onOpen}
        onUpdate={vi.fn().mockResolvedValue(true)}
      />,
    )

    expect(screen.getByTestId('daypilot-stub')).toBeTruthy()
    expect(schedulerProps?.eventClickHandling).toBe('Disabled')
    expect(schedulerProps?.onEventClick).toBeUndefined()

    const timelineCell = document.createElement('div')
    clickRow(timelineCell)
    expect(onOpen).not.toHaveBeenCalled()

    const title = document.createElement('span')
    const header = document.createElement('div')
    header.className = 'work-items-daypilot-task-header'
    header.append(title)
    clickRow(title)

    expect(onOpen).toHaveBeenCalledTimes(1)
    expect(onOpen).toHaveBeenCalledWith(task, header)
  })

  it('routes horizontal move and resize changes through the controlled task update action', () => {
    const onUpdate = vi.fn().mockResolvedValue(true)
    render(
      <GanttScheduler
        groups={groups}
        window={window}
        narrow={false}
        collapsedGroupIds={new Set()}
        onToggleGroup={vi.fn()}
        onOpen={vi.fn()}
        onUpdate={onUpdate}
      />,
    )

    expect(schedulerProps?.eventMoveHandling).toBe('Update')
    expect(schedulerProps?.eventResizeHandling).toBe('Update')
    const preventMove = vi.fn()
    schedulerProps?.onEventMove?.({
      e: { resource: () => 'task:task:71' },
      newStart: { toString: () => '2026-08-05' },
      newEnd: { toString: () => '2026-08-14' },
      newResource: 'task:task:71',
      preventDefault: preventMove,
    })
    expect(preventMove).toHaveBeenCalledTimes(1)
    expect(onUpdate).toHaveBeenLastCalledWith(task, {
      startsAt: '2026-08-05',
      dueAt: '2026-08-13',
    })

    const preventResize = vi.fn()
    schedulerProps?.onEventResize?.({
      e: { resource: () => 'task:task:71' },
      newStart: { toString: () => '2026-08-02' },
      newEnd: { toString: () => '2026-08-16' },
      preventDefault: preventResize,
    })
    expect(preventResize).toHaveBeenCalledTimes(1)
    expect(onUpdate).toHaveBeenLastCalledWith(task, {
      startsAt: '2026-08-02',
      dueAt: '2026-08-15',
    })
  })

  it('rejects moving a task into another owner row', () => {
    const onUpdate = vi.fn().mockResolvedValue(true)
    render(
      <GanttScheduler
        groups={groups}
        window={window}
        narrow={false}
        collapsedGroupIds={new Set()}
        onToggleGroup={vi.fn()}
        onOpen={vi.fn()}
        onUpdate={onUpdate}
      />,
    )

    schedulerProps?.onEventMove?.({
      e: { resource: () => 'task:task:71' },
      newStart: { toString: () => '2026-08-05' },
      newEnd: { toString: () => '2026-08-14' },
      newResource: 'task:task:72',
      preventDefault: vi.fn(),
    })
    expect(onUpdate).not.toHaveBeenCalled()
  })
})
