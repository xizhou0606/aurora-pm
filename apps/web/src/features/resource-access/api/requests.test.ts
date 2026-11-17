import { describe, expect, it } from 'bun:test'
import { normalizeRequestContext } from './normalizers'
const fixture = { resource_id: 'dr_1', owner_user_id: 7, owner_name: 'Owner', can_request: false, can_review: true, has_access: true, pending_count: 1, items: [{ id: 1, applicant_user_id: 8, applicant_name: 'Applicant', reason: 'edit', status: 'pending', read: true, created_at: '2026-09-09T01:00:00Z', expires_at: '2026-09-12T01:00:00Z' }] }
describe('access request contract', () => {
  it('permits an explicitly missing owner without enabling submission', () => {
    expect(normalizeRequestContext({ ...fixture, owner_user_id: 0, owner_name: '', can_request: true })).toMatchObject({ ownerUserId: 0, ownerName: '', canRequest: false })
    expect(() => normalizeRequestContext({ ...fixture, owner_name: '' })).toThrow()
  })
  it('keeps read status independent from processing and pending count', () => { const result = normalizeRequestContext(fixture); expect(result.items[0]).toMatchObject({ read: true, status: 'pending' }); expect(result.pendingCount).toBe(1) })
  it('accepts revocation and rejects unknown statuses or malformed permission facts', () => {
    expect(normalizeRequestContext({ ...fixture, items: [{ ...fixture.items[0], status: 'revoked' }] }).items[0].status).toBe('revoked')
    expect(() => normalizeRequestContext({ ...fixture, can_review: 'true' })).toThrow()
    expect(() => normalizeRequestContext({ ...fixture, items: [{ ...fixture.items[0], status: 'unexpected' }] })).toThrow()
  })
  it('normalizes table grant terms and rejects unknown levels and types', () => {
    const granted = { ...fixture, resource_type: 'bitable', items: [{ ...fixture.items[0], status: 'approved', grant_level: 'viewer', grant_expires_at: '2026-09-16T01:00:00Z' }] }
    expect(normalizeRequestContext(granted)).toMatchObject({ resourceType: 'bitable', items: [{ grantLevel: 'viewer', grantExpiresAt: '2026-09-16T01:00:00Z' }] })
    expect(() => normalizeRequestContext({ ...granted, resource_type: 'unknown' })).toThrow()
    expect(() => normalizeRequestContext({ ...granted, items: [{ ...granted.items[0], grant_level: 'owner' }] })).toThrow()
  })
})
