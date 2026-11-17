import { describe, expect, it } from 'bun:test'

import { unavailableSubjectCapabilities } from './capabilities'
import type { ResourceAccessPolicy } from './types'
import {
  createResourceAccessEditorState,
  resourceAccessChangeImpact,
  resourceAccessEditorReducer,
} from './reducer'

function policy(overrides: Partial<ResourceAccessPolicy> = {}): ResourceAccessPolicy {
  const subjectCapabilities = unavailableSubjectCapabilities()
  subjectCapabilities.user = { available: true }
  return {
    resourceId: 'resource-1',
    mode: 'private',
    workspaceLevel: null,
    rules: [],
    policyRevision: 3,
    canManageAccess: true,
    effectiveLevel: 'editor',
    effectiveSources: ['owner'],
    subjectCapabilities,
    ...overrides,
  }
}

describe('resource access editor reducer', () => {
  it('keeps a local dirty draft when a background policy hydrate arrives', () => {
    const initial = createResourceAccessEditorState(policy())
    const dirty = resourceAccessEditorReducer(initial, { type: 'set_mode', mode: 'workspace' })
    const hydrated = resourceAccessEditorReducer(dirty, {
      type: 'hydrate',
      policy: policy({ policyRevision: 4 }),
    })

    expect(hydrated.status).toBe('dirty')
    expect(hydrated.draft.mode).toBe('workspace')
    expect(hydrated.base.policyRevision).toBe(3)
  })

  it('preserves the draft on conflict and discards against the latest server policy', () => {
    const original = policy()
    const latest = policy({ mode: 'workspace', workspaceLevel: 'viewer', policyRevision: 4 })
    const dirty = resourceAccessEditorReducer(
      createResourceAccessEditorState(original),
      { type: 'set_mode', mode: 'restricted' },
    )
    const submitting = resourceAccessEditorReducer(dirty, { type: 'submit' })
    const conflicted = resourceAccessEditorReducer(submitting, {
      type: 'submit_conflicted',
      latest,
    })

    expect(conflicted.status).toBe('conflict')
    expect(conflicted.draft.mode).toBe('restricted')
    const rebased = resourceAccessEditorReducer(conflicted, { type: 'rebase' })
    expect(rebased.status).toBe('dirty')
    expect(rebased.base.policyRevision).toBe(4)
    expect(rebased.draft.mode).toBe('restricted')
    const discarded = resourceAccessEditorReducer(conflicted, { type: 'discard' })
    expect(discarded.status).toBe('pristine')
    expect(discarded.base.policyRevision).toBe(4)
    expect(discarded.draft.mode).toBe('workspace')
  })

  it('marks uncertain writes separately from ordinary failures', () => {
    const dirty = resourceAccessEditorReducer(
      createResourceAccessEditorState(policy()),
      { type: 'set_mode', mode: 'link' },
    )
    const submitting = resourceAccessEditorReducer(dirty, { type: 'submit' })

    expect(resourceAccessEditorReducer(submitting, { type: 'submit_unknown' }).status)
      .toBe('unknown_result')
    expect(resourceAccessEditorReducer(submitting, { type: 'submit_failed' }).status)
      .toBe('dirty')
    const unknown = resourceAccessEditorReducer(submitting, { type: 'submit_unknown' })
    const resolved = resourceAccessEditorReducer(unknown, {
      type: 'resolve_unknown',
      latest: policy({ mode: 'link', policyRevision: 4 }),
    })
    expect(resolved.status).toBe('pristine')
    expect(resolved.base.policyRevision).toBe(4)
  })

  it('ignores a late submit result after the editor identity was reset', () => {
    const original = policy()
    const dirty = resourceAccessEditorReducer(
      createResourceAccessEditorState(original),
      { type: 'set_mode', mode: 'workspace' },
    )
    const submitting = resourceAccessEditorReducer(dirty, { type: 'submit' })
    const reset = resourceAccessEditorReducer(submitting, {
      type: 'hydrate',
      policy: policy({ resourceId: 'resource-2' }),
      force: true,
    })
    const late = resourceAccessEditorReducer(reset, {
      type: 'submit_succeeded',
      policy: policy({ resourceId: 'resource-1', mode: 'workspace' }),
    })
    expect(late.base.resourceId).toBe('resource-2')
    expect(late.status).toBe('pristine')
  })

  it('fails closed when an unavailable subject kind is injected into the draft', () => {
    const initial = resourceAccessEditorReducer(
      createResourceAccessEditorState(policy()),
      { type: 'set_mode', mode: 'restricted' },
    )
    const next = resourceAccessEditorReducer(initial, {
      type: 'add_rule',
      rule: {
        subjectKind: 'organization_unit',
        subjectId: '42',
        level: 'editor',
        includeDescendants: true,
      },
    })

    expect(next.draft.rules).toEqual([])
  })

  it('classifies broad workspace access as expansion and private as reduction', () => {
    const base = policy({ mode: 'restricted' })
    expect(resourceAccessChangeImpact(base, {
      mode: 'workspace', workspaceLevel: 'viewer', rules: [],
    })).toBe('expanded')
    expect(resourceAccessChangeImpact(base, {
      mode: 'private', workspaceLevel: 'viewer', rules: [],
    })).toBe('reduced')
  })
})
