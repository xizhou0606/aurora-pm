import { describe, expect, it } from 'bun:test'

import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'
import { appendPendingAttachments } from './attachments'
import {
  buildRequirementOptions,
  createPayload,
  initialCreateDraft,
  validateCreateDraft,
  type WorkItemCreateDraft,
} from './create'

const scope: PMScopeOptions = {
  projectSets: [{ id: 8, label: '项目集' }],
  projects: [{ id: 13, label: '版本', parentId: 8 }],
  requirements: [
    { id: 21, label: '需求', parentId: 13 },
    { id: 22, label: '孤立需求' },
  ],
}

const selected: PMScopeSelection = {
  projectSetIds: [8],
  projectIds: [13],
  requirementIds: [21],
}

function readyDraft(patch: Partial<WorkItemCreateDraft> = {}): WorkItemCreateDraft {
  return {
    ...initialCreateDraft(selected, scope),
    title: '  对齐 PM 创建契约  ',
    ...patch,
  }
}

describe('Work-items create state model', () => {
  it('only presents requirements with a complete workspace hierarchy', () => {
    expect(buildRequirementOptions(scope)).toEqual([{
      value: '21',
      label: '项目集 / 版本 / 需求',
      selection: selected,
    }])
    expect(initialCreateDraft(selected, scope).requirementScope).toEqual(selected)
    expect(initialCreateDraft(selected, scope, { assigneeId: 9 })).toMatchObject({
      status: 'draft',
      priority: 'normal',
      assigneeId: 9,
      estimatedHours: '',
      attachments: [],
    })
    expect(initialCreateDraft({ ...selected, requirementIds: [21, 22] }, scope).requirementScope)
      .toEqual({ projectSetIds: [], projectIds: [], requirementIds: [] })
  })

  it('fails invalid workspace, hierarchy, directories and dates closed', () => {
    const common = {
      workspaceId: 7,
      scope,
      users: [{ id: 9, label: 'Teo' }],
      statuses: [{ key: 'draft', label: '待处理', sortOrder: 10 }],
    }
    expect(validateCreateDraft({ ...common, workspaceId: 0, draft: readyDraft() })).toBe('workspace')
    expect(validateCreateDraft({
      ...common,
      draft: readyDraft({
        requirementScope: { projectSetIds: [8], projectIds: [13], requirementIds: [999] },
      }),
    })).toBe('requirement')
    expect(validateCreateDraft({ ...common, draft: readyDraft({ title: ' ' }) })).toBe('title')
    expect(validateCreateDraft({ ...common, draft: readyDraft({ title: 'x'.repeat(257) }) })).toBe('title')
    expect(validateCreateDraft({ ...common, draft: readyDraft({ status: '' }) })).toBe('status')
    expect(validateCreateDraft({ ...common, draft: readyDraft({ status: 'unknown' }) })).toBe('status')
    expect(validateCreateDraft({ ...common, draft: readyDraft({ priority: '' }) })).toBe('priority')
    expect(validateCreateDraft({ ...common, draft: readyDraft({ assigneeId: 99 }) })).toBe('assignee')
    expect(validateCreateDraft({
      ...common,
      draft: readyDraft({ startsAt: '2026-08-05', dueAt: '2026-08-04' }),
    }))
      .toBe('date-range')
    expect(validateCreateDraft({ ...common, draft: readyDraft({ startsAt: '2026-08-05T00:00:00' }) }))
      .toBe('date-range')
  })

  it('maps only fields accepted by latest PM POST /tasks', () => {
    expect(createPayload(readyDraft({
      description: '  状态驱动  ',
      status: 'draft',
      priority: 'high',
      assigneeId: 9,
      estimatedHours: '1.5',
      startsAt: '2026-08-04',
      dueAt: '2026-08-05',
    }), 101)).toEqual({
      requirement_id: 21,
      kind: 'task',
      title: '对齐 PM 创建契约',
      description: '状态驱动',
      status: 'draft',
      priority: 'high',
      reporter_user_id: 101,
      assignee_id: 9,
      estimated_minutes: 90,
      starts_at: new Date(2026, 7, 4, 0, 0, 0).toISOString(),
      due_at: new Date(2026, 7, 5, 23, 59, 59).toISOString(),
    })
  })

  it('does not carry the removed story-points field into the task create contract', () => {
    const payload = createPayload(readyDraft(), 101) as Record<string, unknown>
    expect(payload).not.toHaveProperty('story_points')
    expect(payload).toMatchObject({ description: '', status: 'draft', priority: 'normal' })
  })

  it('stages unique non-empty attachments without uploading them', () => {
    const first = new File(['first'], 'scope.png', { lastModified: 1 })
    const duplicate = new File(['first'], 'scope.png', { lastModified: 1 })
    const second = new File(['second'], 'notes.txt', { lastModified: 2 })
    expect(appendPendingAttachments([first], [duplicate, second, new File([], 'empty.txt')]))
      .toEqual([first, second])
  })
})
