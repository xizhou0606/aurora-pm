import { describe, expect, it } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'

import { normalizePolicy, normalizeShareLinkSecret, normalizeSubjectPage } from './normalizers'

describe('resource access API contracts', () => {
  it('normalizes the frozen policy DTO and defaults missing subject capabilities closed', () => {
    const result = normalizePolicy({
      resource_id: 'resource-1',
      mode: 'restricted',
      workspace_level: null,
      rules: [{
        subject_kind: 'user',
        subject_id: '9',
        level: 'editor',
        subject: { id: '9', kind: 'user', display_name: 'User 9' },
      }],
      policy_revision: 4,
      can_manage_access: true,
      effective_level: 'editor',
      effective_sources: ['owner', 'rule:user:9'],
      subject_capabilities: {
        user: { available: true },
        role: { available: true },
      },
    })

    expect(result.rules[0]).toMatchObject({
      subjectKind: 'user', subjectId: '9', level: 'editor', includeDescendants: false,
    })
    expect(result.subjectCapabilities.user.available).toBe(true)
    expect(result.subjectCapabilities.organization_unit.available).toBe(false)
  })

  it('rejects malformed policy fields instead of widening access', () => {
    expect(() => normalizePolicy({
      resource_id: 'resource-1',
      mode: 'workspace',
      workspace_level: null,
      rules: [],
      policy_revision: 1,
      can_manage_access: true,
      effective_level: 'viewer',
      effective_sources: [],
      subject_capabilities: {},
    })).toThrow(PMSafeApiError)
  })

  it('keeps one-time link secrets only on create or rotate responses', () => {
    expect(normalizeShareLinkSecret({
      id: 'link-1',
      level: 'viewer',
      expires_at: null,
      created_at: '2026-09-08T00:00:00Z',
      revoked_at: null,
      secret: 'one-time-secret',
    }).secret).toBe('one-time-secret')
  })

  it('propagates unavailable organization capability from subject search', () => {
    const page = normalizeSubjectPage({
      items: [],
      total: 0,
      has_more: false,
      capabilities: { organization_unit: { available: false }, user: { available: true } },
    })
    expect(page.capabilities.organization_unit.available).toBe(false)
    expect(page.capabilities.user.available).toBe(true)
  })

  it('accepts a rule update without a new secret but rejects a missing secret contract', () => {
    const link = { id: 'link-1', level: 'editor', expires_at: null,
      created_at: '2026-09-08T00:00:00Z', updated_at: '2026-09-09T00:00:00Z', revoked_at: null }
    expect(normalizeShareLinkSecret({ ...link, secret: '' })).toMatchObject({ secret: '', updatedAt: link.updated_at })
    expect(() => normalizeShareLinkSecret(link)).toThrow(PMSafeApiError)
  })
})
