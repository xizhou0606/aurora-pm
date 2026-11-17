import { describe, expect, it } from 'bun:test'

import { QUALITY_COPY } from '@/shared/copy'

import {
  qualityAllowedTransitionOptions,
  qualityCurrentOwnerUserId,
  mergeQualityActivityPages,
  qualityLifecycleModel,
  qualityStatusFilterOptions,
} from './selectors'
import { isQualityTerminalStatus, matchesQualityRelationFilter } from './filters'
import type { QualityBug, QualityMutationState } from './types'

function bug(overrides: Partial<QualityBug> = {}): QualityBug {
  return {
    id: 31,
    workspaceId: 7,
    title: 'Checkout crash',
    status: 'triage',
    reproductionMaterials: [],
    resolutionMaterials: [],
    capabilities: {
      canUpdate: false,
      canDelete: false,
      canAssign: false,
      canResolve: false,
      canReturn: false,
      canTransition: false,
      canUpload: false,
      allowedTransitions: [],
    },
    ...overrides,
  }
}

const idleTransition: QualityMutationState = { status: 'idle', action: 'transition' }

describe('quality selectors', () => {
  it('keeps terminal status semantics shared by list editing and summaries', () => {
    expect(['done', 'closed', 'canceled'].every(isQualityTerminalStatus)).toBe(true)
    expect(isQualityTerminalStatus('review')).toBe(false)
    expect(isQualityTerminalStatus('duplicate')).toBe(false)
  })

  it('resolves the current follower from the lifecycle stage without changing server facts', () => {
    expect(qualityCurrentOwnerUserId(bug({
      status: 'in_progress',
      assigneeUserId: 7,
      reporterUserId: 8,
      reviewerUserId: 9,
    }))).toBe(7)
    expect(qualityCurrentOwnerUserId(bug({
      status: 'review',
      assigneeUserId: 7,
      reporterUserId: 8,
      reviewerUserId: 9,
    }))).toBe(9)
    expect(qualityCurrentOwnerUserId(bug({
      status: 'done',
      assigneeUserId: 7,
      reporterUserId: 8,
    }))).toBe(8)
    expect(qualityCurrentOwnerUserId(bug({
      status: 'canceled',
      assigneeUserId: 7,
      reporterUserId: 8,
      reviewerUserId: 9,
    }))).toBe(9)
  })

  it('owns the actor relation filters in one selector', () => {
    const input = {
      userId: 7,
      status: 'review',
      assigneeUserId: 7,
      reporterUserId: 8,
      currentOwnerUserId: 9,
    }
    expect(matchesQualityRelationFilter({ ...input, relation: '' })).toBe(true)
    expect(matchesQualityRelationFilter({ ...input, relation: 'reported' })).toBe(false)
    expect(matchesQualityRelationFilter({ ...input, relation: 'assigned' })).toBe(false)
    expect(matchesQualityRelationFilter({ ...input, relation: 'responsible' })).toBe(true)
    expect(matchesQualityRelationFilter({ ...input, relation: 'resolved' })).toBe(true)
    expect(matchesQualityRelationFilter({ ...input, relation: 'resolved', status: 'in_progress' }))
      .toBe(false)
  })
  it('exposes one filter option per user-visible status label', () => {
    const options = qualityStatusFilterOptions()
    expect(new Set(options.map((option) => option.label)).size).toBe(options.length)
    expect(options.filter((option) => option.label === '待处理')).toHaveLength(1)
    expect(options).toContainEqual({ value: 'reopened', label: '重新打开' })
    expect(options.some((option) => ['canceled', 'cancelled'].includes(option.value))).toBe(false)
    expect(options.filter((option) => option.label === '已关闭')).toHaveLength(1)
    expect(qualityStatusFilterOptions('todo')).toContainEqual({ value: 'todo', label: '待处理' })
    expect(qualityStatusFilterOptions('todo').filter((option) => option.label === '待处理'))
      .toHaveLength(1)
  })

  it('exposes only direct API-supported status transitions for quick actions', () => {
    expect(qualityAllowedTransitionOptions(bug({
      status: 'review',
      capabilities: {
        ...bug().capabilities,
        canTransition: true,
        allowedTransitions: ['done', 'triage', 'review', 'closed'],
      },
    }))).toEqual([
      { value: 'done', label: '已关闭' },
      { value: 'triage', label: '待确认' },
    ])
    expect(qualityAllowedTransitionOptions(bug({
      capabilities: {
        ...bug().capabilities,
        canTransition: true,
        allowedTransitions: ['in_progress', 'canceled'],
      },
    }))).toEqual([{ value: 'in_progress', label: '处理中' }])
  })

  it('derives lifecycle actions only from API capabilities and transitions', () => {
    const model = qualityLifecycleModel(bug({
      capabilities: {
        ...bug().capabilities,
        canAssign: true,
        canTransition: true,
        allowedTransitions: ['in_progress', 'blocked', 'done', 'closed'],
      },
    }), idleTransition)

    expect(model.actions.map((action) => action.kind === 'transition'
      ? `${action.kind}:${action.status}`
      : action.kind)).toEqual([
      'transition:in_progress',
      'assign',
      'transition:done',
      'transition:blocked',
    ])
    expect(model.actions.find((action) => action.id === 'transition:blocked')?.label)
      .toBe(QUALITY_COPY.lifecycle.transitionTo('blocked'))
  })

  it('deduplicates overlapping activity pages by stable activity id', () => {
    expect(mergeQualityActivityPages([
      { rows: [{ id: 1, summary: 'created' }], total: 3, nextOffset: 2 },
      { rows: [{ id: 1, summary: 'duplicate' }, { id: 2, summary: 'assigned' }], total: 3 },
    ])).toEqual([
      { id: 1, summary: 'created' },
      { id: 2, summary: 'assigned' },
    ])
  })

  it('marks API transition actions pending without disabling independent workflow actions', () => {
    const pending: QualityMutationState = { status: 'pending', action: 'transition' }
    const model = qualityLifecycleModel(bug({
      capabilities: {
        ...bug().capabilities,
        canResolve: true,
        canTransition: true,
        allowedTransitions: ['done'],
      },
    }), pending)
    expect(model.actions.find((action) => action.kind === 'transition')?.disabled).toBe(true)
    expect(model.actions.find((action) => action.kind === 'resolve')?.disabled).toBe(false)
  })
})
