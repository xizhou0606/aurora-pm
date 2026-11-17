import { describe, expect, it } from 'bun:test'

import type { PMScopeOptions } from '@/shared/api'
import {
  applyQuickTaskFilledFields,
  createQuickTaskChildDraft,
  createQuickTaskDraft,
  quickTaskBatchPayload,
  quickTaskServerErrors,
  updateQuickTaskChild,
  updateQuickTaskParent,
  validateQuickTaskDrafts,
} from './quick-create'

const scope: PMScopeOptions = {
  projectSets: [{ id: 8, label: 'Set' }],
  projects: [{ id: 13, label: 'Project', parentId: 8 }],
  requirements: [{ id: 21, label: 'Requirement', parentId: 13 }],
}
const selected = { projectSetIds: [8], projectIds: [13], requirementIds: [21] }
const users = [{ id: 101, label: 'Teo' }]

describe('quick task drafts', () => {
  it('defaults the parent and inherits untouched child fields once', () => {
    let parent = createQuickTaskDraft({
      clientId: 'parent-1', selected, scope, assigneeId: 101,
    })
    parent = { ...parent, startsAt: '2026-08-07', dueAt: '2026-08-08' }
    let child = createQuickTaskChildDraft('child-1', parent)
    expect(child).toMatchObject({
      priority: 'normal', assigneeId: 101,
      startsAt: '2026-08-07', dueAt: '2026-08-08', estimatedHours: '',
    })

    child = updateQuickTaskChild(child, 'priority', 'high')
    parent = { ...parent, children: [child] }
    parent = updateQuickTaskParent(parent, 'priority', 'urgent')
    parent = updateQuickTaskParent(parent, 'dueAt', '2026-08-09')
    expect(parent.children[0]).toMatchObject({ priority: 'high', dueAt: '2026-08-09' })
  })

  it('applies only a selected task filled fields to existing rows without creating tasks', () => {
    const expandedScope: PMScopeOptions = {
      ...scope,
      requirements: [
        ...scope.requirements,
        { id: 22, label: 'Requirement 2', parentId: 13 },
      ],
    }
    const first = {
      ...createQuickTaskDraft({
        clientId: 'parent-1', selected, scope: expandedScope, assigneeId: 101,
      }),
      title: 'Keep first', dueAt: '2026-09-10', taskClientId: 9,
    }
    const second = {
      ...createQuickTaskDraft({
        clientId: 'parent-2',
        selected: { projectSetIds: [8], projectIds: [13], requirementIds: [22] },
        scope: expandedScope,
        assigneeId: 101,
      }),
      title: 'Copy me', priority: 'urgent', startsAt: '2026-09-01', estimatedHours: '4',
    }
    const child = { ...createQuickTaskChildDraft('child-1', first), title: 'Keep child' }
    const result = applyQuickTaskFilledFields([{ ...first, children: [child] }, second], 'parent-2', expandedScope)
    expect(result).toHaveLength(2)
    expect(result.map(({ requirementScope }) => requirementScope)).toEqual([
      { projectSetIds: [8], projectIds: [13], requirementIds: [22] },
      { projectSetIds: [8], projectIds: [13], requirementIds: [22] },
    ])
    expect(result[0]).toMatchObject({
      title: 'Copy me', priority: 'urgent', startsAt: '2026-09-01',
      dueAt: '2026-09-10', taskClientId: 9, estimatedHours: '4',
    })
    expect(result[0].children).toHaveLength(1)
    expect(result[0].children[0]).toMatchObject({
      clientId: 'child-1', title: 'Copy me', priority: 'urgent',
      startsAt: '2026-09-01', dueAt: '2026-09-10', taskClientId: 9,
      estimatedHours: '4',
      touched: { priority: true, taskClientId: false, assigneeId: true, startsAt: true, dueAt: false },
    })
  })

  it('enforces the backend 220-character title boundary for parents and children', () => {
    const parent = {
      ...createQuickTaskDraft({ clientId: 'parent-1', selected, scope, assigneeId: 101 }),
      title: 'x'.repeat(221),
    }
    const child = { ...createQuickTaskChildDraft('child-1', parent), title: 'y'.repeat(221) }
    const errors = validateQuickTaskDrafts({ tasks: [{ ...parent, children: [child] }], scope, users })
    expect(errors).toMatchObject({
      'parent-1:title': 'title',
      'child-1:title': 'title',
    })
  })

  it('maps dates to local day boundaries and omits child requirement ownership', () => {
    const parent = {
      ...createQuickTaskDraft({ clientId: 'parent-1', selected, scope, assigneeId: 101 }),
      title: 'Parent', startsAt: '2026-08-07', dueAt: '2026-08-08', estimatedHours: '2',
    }
    const child = {
      ...createQuickTaskChildDraft('child-1', parent), title: 'Child', estimatedHours: '0.5',
    }
    const payload = quickTaskBatchPayload([{ ...parent, children: [child] }])
    expect(payload.tasks[0]).toMatchObject({
      client_id: 'parent-1', requirement_id: 21, assignee_id: 101,
      estimated_minutes: 120,
      starts_at: new Date(2026, 7, 7, 0, 0, 0).toISOString(),
      due_at: new Date(2026, 7, 8, 23, 59, 59).toISOString(),
    })
    expect(payload.tasks[0].children[0]).toMatchObject({
      client_id: 'child-1', estimated_minutes: 30,
    })
    expect(payload.tasks[0].children[0]).not.toHaveProperty('requirement_id')
  })

  it('submits explicit null only when a child clears an inherited date', () => {
    const parent = {
      ...createQuickTaskDraft({ clientId: 'parent-1', selected, scope, assigneeId: 101 }),
      title: 'Parent', startsAt: '2026-08-07', dueAt: '2026-08-08',
    }
    let child = createQuickTaskChildDraft('child-1', parent)
    child = updateQuickTaskChild(child, 'startsAt', '')
    const payload = quickTaskBatchPayload([{ ...parent, children: [child] }])
    expect(payload.tasks[0].children[0]).toMatchObject({
      starts_at: null,
      due_at: new Date(2026, 7, 8, 23, 59, 59).toISOString(),
    })
  })

  it('submits explicit null when a child clears an inherited client', () => {
    const parent = {
      ...createQuickTaskDraft({ clientId: 'parent-1', selected, scope, assigneeId: 101 }),
      title: 'Parent', taskClientId: 7,
    }
    let child = createQuickTaskChildDraft('child-1', parent)
    child = updateQuickTaskChild(child, 'taskClientId', undefined)
    expect(quickTaskBatchPayload([{ ...parent, children: [child] }]).tasks[0].children[0]).toMatchObject({
      task_client_id: null,
    })
  })

  it('maps service field errors back to stable draft rows', () => {
    expect(quickTaskServerErrors({ details: { fields: [
      { client_id: 'parent-1', field: 'requirement_id' },
      { client_id: 'child-1', field: 'assignee_id' },
    ] } })).toEqual({
      'parent-1:requirement_id': 'requirement',
      'child-1:assignee_id': 'assignee',
    })
  })
})
