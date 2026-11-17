import { fireEvent, render } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  WorkItem,
  WorkItemBoardGroup,
  WorkItemBoardLane,
  WorkItemBoardTransition,
} from '../../model/types'
import { Board } from './Board'

function row(): WorkItem {
  return {
    id: 31,
    rowId: 'task:31',
    title: 'Board task',
    taskKey: 'PM-31',
    status: 'draft',
    priority: 'normal',
    assigneeId: 7,
    assigneeName: 'Teo',
    watched: false,
    capabilities: {
      canUpdate: true,
      canDelete: false,
      canComment: false,
      canAttach: false,
      canWatch: false,
      canClone: false,
      allowedTransitions: ['in_dev'],
    },
  }
}

describe('Work-items Board', () => {
  it('keeps every lane at one fixed height with an internal card scroller', () => {
    const task = row()
    const lanes: WorkItemBoardLane[] = [
      { id: 'draft', label: 'Draft', rows: [task] },
      { id: 'in_dev', label: 'In development', rows: [] },
    ]
    const groups: WorkItemBoardGroup[] = [{ id: 'all', label: '', lanes }]
    const transition: WorkItemBoardTransition = {
      state: { status: 'idle' },
      canDrag: vi.fn(() => true),
      canDrop: vi.fn(() => true),
      start: vi.fn(() => true),
      over: vi.fn(),
      leave: vi.fn(),
      drop: vi.fn(),
      end: vi.fn(),
      retry: vi.fn(),
      dismiss: vi.fn(),
      moveByKeyboard: vi.fn(),
    }
    const view = render(
      <Board
        state={{
          status: 'ready', rows: [task], total: 1, refresh: 'idle', truncated: false,
        }}
        groups={groups}
        users={{
          status: 'ready',
          data: [{ id: 7, label: 'Teo', username: 'teo', avatarUrl: '/avatar.png' }],
        }}
        transition={transition}
        onOpen={vi.fn()}
      />,
    )

    const laneSections = view.container.querySelectorAll('section[aria-labelledby^="work-items-lane-"]')
    expect(laneSections).toHaveLength(2)
    laneSections.forEach((lane) => expect(lane.className).toContain('h-full'))
    expect(view.container.querySelectorAll('.overflow-y-auto')).toHaveLength(1)
    expect(view.container.querySelector('.overflow-x-auto')?.className).toContain('100dvh')

    const card = view.getByTestId('work-items-card')
    expect(card.className).toContain('min-h-28')
    expect(card.querySelectorAll('[data-slot="status-pill"]')).toHaveLength(2)
    expect(card.textContent).toContain(WORK_ITEMS_COPY.enums.priorities.normal)
    expect(card.textContent).toContain(WORK_ITEMS_COPY.enums.statuses.draft)
    expect(card.textContent).toContain(task.title)
    expect(card.textContent).toContain('Teo')
    expect(view.getByRole('button', { name: WORK_ITEMS_COPY.list.openDetail(task.title) })).toBeTruthy()
    expect(view.getByRole('button', { name: WORK_ITEMS_COPY.list.openDetail(task.title) }).getAttribute('title')).toBe(task.title)
    expect(view.getByTestId('work-items-empty-lane').textContent).toBe(
      WORK_ITEMS_COPY.board.emptyLane,
    )
    const target = laneSections[1] as HTMLElement
    const dataTransfer = { effectAllowed: '', dropEffect: '', setData: vi.fn() }
    fireEvent.dragStart(card, { dataTransfer })
    fireEvent.dragOver(target, { dataTransfer })
    fireEvent.drop(target, { dataTransfer })
    fireEvent.keyDown(card, { altKey: true, key: 'ArrowRight' })

    expect(transition.start).toHaveBeenCalledWith(expect.objectContaining({ id: 31 }))
    expect(transition.over).toHaveBeenCalledWith(
      'in_dev',
      'work-items-lane-all-in_dev',
    )
    expect(transition.drop).toHaveBeenCalledWith('in_dev')
    expect(transition.moveByKeyboard).toHaveBeenCalledWith(
      expect.objectContaining({ id: 31 }),
      1,
    )
  })

  it('renders a complete lane set for every derived group', () => {
    const task = row()
    const transition: WorkItemBoardTransition = {
      state: { status: 'idle' },
      canDrag: vi.fn(() => true),
      canDrop: vi.fn(() => true),
      start: vi.fn(() => true),
      over: vi.fn(),
      leave: vi.fn(),
      drop: vi.fn(),
      end: vi.fn(),
      retry: vi.fn(),
      dismiss: vi.fn(),
      moveByKeyboard: vi.fn(),
    }
    const groups: WorkItemBoardGroup[] = ['需求 A', '需求 B'].map((label, index) => ({
      id: `requirement:${index + 1}`,
      label,
      lanes: [
        { id: 'draft', label: '待处理', rows: index === 0 ? [task] : [] },
        { id: 'in_dev', label: '进行中', rows: [] },
        { id: 'pending_test', label: '待测试', rows: [] },
        { id: 'completed', label: '已完成', rows: [] },
        { id: 'cancelled', label: '已取消', rows: [] },
      ],
    }))

    const view = render(
      <Board
        state={{ status: 'ready', rows: [task], total: 1, refresh: 'idle', truncated: false }}
        groups={groups}
        users={{ status: 'ready', data: [] }}
        transition={transition}
        onOpen={vi.fn()}
      />,
    )

    expect(view.getAllByRole('heading', { level: 3, name: '待处理' })).toHaveLength(2)
    expect(view.container.querySelectorAll('section[aria-labelledby^="work-items-lane-"]'))
      .toHaveLength(10)
    expect(view.getAllByTestId('work-items-empty-lane')).toHaveLength(9)
  })
})
