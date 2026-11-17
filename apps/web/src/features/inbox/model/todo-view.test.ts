import { describe, expect, it } from 'bun:test'

import {
  inboxCalendarDateKey,
  inboxCalendarDays,
  inboxDateKey,
  inboxMonthLabel,
  moveInboxCalendarMonth,
  workflowTodoItem,
} from './todo-view'

describe('Inbox calendar view model', () => {
  it('builds a Monday-first six-week August 2026 grid and marks today', () => {
    const days = inboxCalendarDays(new Date(2026, 7, 1), new Date(2026, 7, 9))

    expect(days).toHaveLength(42)
    expect(days[0]).toMatchObject({ key: '2026-07-27', day: 27, inMonth: false })
    expect(days[6]).toMatchObject({ key: '2026-08-02', day: 2, inMonth: true })
    expect(days.find((day) => day.today)?.key).toBe('2026-08-09')
    expect(days.find((day) => day.key === '2026-08-18')).toMatchObject({ inMonth: true })
  })

  it('moves calendar months without carrying an invalid day', () => {
    expect(inboxDateKey(moveInboxCalendarMonth(new Date(2026, 7, 31), 1)))
      .toBe('2026-09-01')
    expect(inboxMonthLabel(new Date(2026, 7, 1))).toContain('2026年8月')
  })

  it('keeps workflow action and context visible in the unified item', () => {
    const item = workflowTodoItem({
      id: 61,
      todoId: 'workflow:71:review:61',
      sourceType: 'workflow_node',
      sourceRevision: '61',
      title: '产品评审',
      status: 'active',
      actionKind: 'ai_confirm',
      allowedActions: ['complete'],
      detailPath: '/workspace/requirements/21?workflow_node=review',
      workflowInstanceId: 71,
      workflowNodeId: 'review',
      nodeType: 'ai_check',
      subjectTitle: '需求 A',
      projectSetName: '项目集 A',
      projectName: '项目 A',
      requirementTitle: '需求 A',
    })

    expect(item).toMatchObject({
      kind: 'workflow',
      title: '产品评审',
      context: '项目集 A / 项目 A / 需求 A',
      status: 'AI 结果待确认',
      statusTone: 'accent',
    })
  })

  it('places a workflow todo without a node due date on the day it became a todo', () => {
    const item = workflowTodoItem({
      id: 205,
      todoId: 'workflow:29:ai-plan-task-breakdown:205',
      sourceType: 'workflow_node',
      sourceRevision: '205:9:completed',
      title: 'AI任务拆解',
      status: 'active',
      actionKind: 'ai_confirm',
      allowedActions: ['complete'],
      detailPath: '/workspace/requirements/153?workflow_node=ai-plan-task-breakdown',
      workflowInstanceId: 29,
      workflowNodeId: 'ai-plan-task-breakdown',
      nodeType: 'ai_generate',
      subjectTitle: 'test',
      createdAt: '2026-08-16T02:39:14.521Z',
    })

    expect(item.dueAt).toBeUndefined()
    expect(inboxCalendarDateKey(item)).toBe('2026-08-16')
  })

  it('keeps an explicit due date ahead of the created date on the calendar', () => {
    const item = workflowTodoItem({
      id: 61,
      todoId: 'workflow:71:review:61',
      sourceType: 'workflow_node',
      sourceRevision: '61',
      title: '产品评审',
      status: 'active',
      actionKind: 'approval',
      allowedActions: ['approve'],
      detailPath: '/workspace/requirements/21?workflow_node=review',
      workflowInstanceId: 71,
      workflowNodeId: 'review',
      nodeType: 'approval',
      subjectTitle: '需求 A',
      dueAt: '2026-08-19',
      createdAt: '2026-08-16T02:39:14.521Z',
    })

    expect(inboxCalendarDateKey(item)).toBe('2026-08-19')
  })
})
