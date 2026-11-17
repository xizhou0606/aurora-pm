import { describe, expect, it } from 'bun:test'
import { normalizeAuthorizationRecords } from './authorization-records'

const record = {
  id: 'device:1', user_code: 'TEST-CODE', channel: 'cli', client_name: 'PM CLI',
  principal_id: null, principal_name: null, grant_tools: ['user.me'],
  created_at: '2026-09-18T01:00:00Z', decided_at: null, expires_at: '2026-09-18T01:10:00Z',
  status: 'pending', code_valid: true, credential_valid: false, can_revoke: false,
}
const page = { items: [record], page: 1, page_size: 20, total: 1 }
describe('authorization record contract', () => {
  it('preserves pending requests without an authorized user', () => {
    expect(normalizeAuthorizationRecords(page)?.items[0].principal_id).toBeNull()
  })
  it('keeps consumed code validity independent of approved grant status', () => {
    const data = normalizeAuthorizationRecords({ ...page, items: [{ ...record, principal_id: 109, principal_name: 'Admin', decided_at: record.created_at, status: 'approved', code_valid: false, credential_valid: true, can_revoke: true }] })
    expect(data?.items[0].status).toBe('approved')
    expect(data?.items[0].code_valid).toBe(false)
    expect(data?.items[0].credential_valid).toBe(true)
    expect(data?.items[0].can_revoke).toBe(true)
  })
  it('fails closed on malformed pages and permission flags', () => {
    expect(normalizeAuthorizationRecords({ ...page, page: 0 })).toBeUndefined()
    expect(normalizeAuthorizationRecords({ ...page, items: [{ ...record, can_revoke: 'true' }] })).toBeUndefined()
    expect(normalizeAuthorizationRecords({ ...page, items: [{ ...record, status: 'unknown' }] })).toBeUndefined()
  })
})
