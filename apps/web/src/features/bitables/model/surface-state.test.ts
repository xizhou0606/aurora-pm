import { describe, expect, it } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import { bitableSurfaceState } from './surface-state'

const ready = { workspaceId: 2, signedIn: true, hasResourceId: true, shareStatus: 'ready' as const, resourceError: null, bootstrapError: null,
  accessStatus: 'ready' as const, canList: true, resourcePending: false, bootstrapPending: false,
  hasResource: true, hasTable: true, hasView: true, canRead: true }

describe('bitable surface boundary', () => {
  it('settles disabled queries without spinning for a missing workspace or table', () => {
    expect(bitableSurfaceState({ ...ready, workspaceId: 0, resourcePending: true, bootstrapPending: true })).toEqual({ status: 'failure', reason: 'workspace', checkAccess: false })
    expect(bitableSurfaceState({ ...ready, hasTable: false, bootstrapPending: true })).toEqual({ status: 'failure', reason: 'table', checkAccess: false })
    expect(bitableSurfaceState({ ...ready, hasResourceId: false, resourcePending: true })).toEqual({ status: 'failure', reason: 'missing', checkAccess: false })
  })
  it.each([
    ['BITABLE_FORBIDDEN', 403, 'forbidden', true],
    ['ACCESS_INTERFACE_FORBIDDEN', 403, 'forbidden', false],
    ['WORKSPACE_FORBIDDEN', 403, 'workspace', false],
    ['BITABLE_SCOPE_MISMATCH', 403, 'workspace', false],
    ['UNAUTHORIZED', 401, 'session', false],
    ['BITABLE_NOT_FOUND', 404, 'missing', true],
    ['SERVER_ERROR', 500, 'unavailable', false],
  ] as const)('settles %s immediately even while dependent queries are pending', (code, status, reason, checkAccess) => {
    expect(bitableSurfaceState({ ...ready, resourceError: new PMSafeApiError(code, status), bootstrapPending: true, accessStatus: 'loading' })).toEqual({ status: 'failure', reason, checkAccess })
  })
  it('does not request object access for a missing child table or a missing global action', () => {
    expect(bitableSurfaceState({ ...ready, bootstrapError: new PMSafeApiError('BITABLE_NOT_FOUND', 404) })).toEqual({ status: 'failure', reason: 'table', checkAccess: false })
    expect(bitableSurfaceState({ ...ready, canList: false, bootstrapPending: true })).toEqual({ status: 'failure', reason: 'forbidden', checkAccess: false })
  })
  it('keeps true loading, view and readable states distinct', () => {
    expect(bitableSurfaceState({ ...ready, resourcePending: true })).toEqual({ status: 'loading' })
    expect(bitableSurfaceState({ ...ready, hasView: false })).toEqual({ status: 'failure', reason: 'view', checkAccess: false })
    expect(bitableSurfaceState(ready)).toEqual({ status: 'ready' })
  })
})
