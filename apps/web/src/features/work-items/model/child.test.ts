import { describe, expect, it } from 'bun:test'

import { childCreatePayload, createEmptyChildDraft, validateChildDraft } from './child'

const users = [{ id: 9, label: 'Teo' }]
const statuses = [{ value: 'draft', label: '待处理' }]

describe('Work-items child draft model', () => {
  it('keeps every optional PM main field absent until explicitly selected', () => {
    const draft = { ...createEmptyChildDraft('one'), title: ' Child ' }
    expect(validateChildDraft({ draft, users, statuses })).toBeUndefined()
    expect(childCreatePayload(draft)).toEqual({ clientId: 'one', title: 'Child' })
  })

  it('validates users, dates and estimated time before any write', () => {
    expect(validateChildDraft({
      draft: {
        ...createEmptyChildDraft('one'), title: 'Child', assigneeId: 99,
      },
      users,
      statuses,
    })).toBe('assignee')
    expect(validateChildDraft({
      draft: {
        ...createEmptyChildDraft('two'), title: 'Child',
        startsAt: '2026-08-09', dueAt: '2026-08-08',
      },
      users,
      statuses,
    })).toBe('date-range')
    expect(validateChildDraft({
      draft: {
        ...createEmptyChildDraft('three'), title: 'Child', estimatedHours: '-1',
      },
      users,
      statuses,
    })).toBe('estimated-hours')
  })

  it('maps contract-backed optional fields and never creates an owner field', () => {
    const payload = childCreatePayload({
      ...createEmptyChildDraft('one'), title: 'Child', reporterUserId: 9, assigneeId: 9,
      priority: 'high', startsAt: '2026-08-06', dueAt: '2026-08-08',
      estimatedHours: '1.5', status: 'draft',
    })
    expect(payload).toEqual({
      clientId: 'one', title: 'Child', reporterUserId: 9, assigneeId: 9,
      priority: 'high', startsAt: '2026-08-06', dueAt: '2026-08-08',
      estimatedMinutes: 90, status: 'draft',
    })
    expect(payload).not.toHaveProperty('ownerUserId')
  })
})
