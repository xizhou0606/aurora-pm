import { PMSafeApiError } from '@/shared/api'

export type BitableSurfaceState =
  | { status: 'ready' }
  | { status: 'loading' }
  | { status: 'failure'; reason: 'workspace' | 'session' | 'missing' | 'forbidden' | 'unavailable' | 'share' | 'table' | 'view'; checkAccess: boolean }

function failed(error: unknown, target: 'resource' | 'table'): BitableSurfaceState {
  if (!(error instanceof PMSafeApiError)) return { status: 'failure', reason: 'unavailable', checkAccess: false }
  if (['WORKSPACE_CONTEXT_REQUIRED', 'WORKSPACE_NOT_FOUND', 'WORKSPACE_FORBIDDEN', 'BITABLE_SCOPE_MISMATCH'].includes(error.code)) {
    return { status: 'failure', reason: 'workspace', checkAccess: false }
  }
  if (error.status === 401) return { status: 'failure', reason: 'session', checkAccess: false }
  if (error.status === 404) return { status: 'failure', reason: target === 'table' ? 'table' : 'missing', checkAccess: target === 'resource' }
  if (error.status === 403) return { status: 'failure', reason: 'forbidden', checkAccess: error.code !== 'ACCESS_INTERFACE_FORBIDDEN' }
  return { status: 'failure', reason: 'unavailable', checkAccess: false }
}

export function bitableSurfaceState(input: {
  workspaceId: number
  signedIn: boolean
  hasResourceId: boolean
  shareStatus: 'checking' | 'ready' | 'error'
  resourceError: unknown
  bootstrapError: unknown
  accessStatus: 'loading' | 'error' | 'ready'
  canList: boolean
  resourcePending: boolean
  bootstrapPending: boolean
  hasResource: boolean
  hasTable: boolean
  hasView: boolean
  canRead: boolean
}): BitableSurfaceState {
  if (!input.signedIn) return { status: 'failure', reason: 'session', checkAccess: false }
  if (input.workspaceId <= 0) return { status: 'failure', reason: 'workspace', checkAccess: false }
  if (!input.hasResourceId) return { status: 'failure', reason: 'missing', checkAccess: false }
  if (input.resourceError) return failed(input.resourceError, 'resource')
  if (input.bootstrapError) return failed(input.bootstrapError, 'table')
  if (input.shareStatus === 'error') return { status: 'failure', reason: 'share', checkAccess: false }
  if (input.accessStatus === 'error') return { status: 'failure', reason: 'unavailable', checkAccess: false }
  if (input.accessStatus === 'ready' && !input.canList) return { status: 'failure', reason: 'forbidden', checkAccess: false }
  if (input.shareStatus === 'checking' || input.resourcePending || input.accessStatus === 'loading') return { status: 'loading' }
  if (!input.hasResource) return { status: 'failure', reason: 'missing', checkAccess: true }
  if (!input.hasTable) return { status: 'failure', reason: 'table', checkAccess: false }
  if (input.bootstrapPending) return { status: 'loading' }
  if (!input.canRead) return { status: 'failure', reason: 'forbidden', checkAccess: true }
  if (!input.hasView) return { status: 'failure', reason: 'view', checkAccess: false }
  return { status: 'ready' }
}
