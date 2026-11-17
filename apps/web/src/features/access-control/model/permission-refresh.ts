import { runtimeWorkspaceStore } from '@/shared/runtime'
import { currentPermissionsScopeKey, permissionsStore } from './permissions-store'

// All mounted permission consumers share one refresh loop. A change made by a
// different administrator/session must not leave this tab's UI permissions stale.
export const PERMISSION_REFRESH_INTERVAL_MS = 30_000
let consumers = 0
let timer: ReturnType<typeof setInterval> | undefined
function refreshVisiblePermissions() {
  if (document.visibilityState === 'hidden' || runtimeWorkspaceStore.getState().currentWorkspaceId <= 0) return
  void permissionsStore.getState().reload(currentPermissionsScopeKey(), true)
}

export function subscribePermissionRefresh(): () => void {
  consumers++
  if (consumers === 1) {
    timer = setInterval(refreshVisiblePermissions, PERMISSION_REFRESH_INTERVAL_MS)
    window.addEventListener('focus', refreshVisiblePermissions)
    document.addEventListener('visibilitychange', refreshVisiblePermissions)
  }
  let subscribed = true
  return () => {
    if (!subscribed) return
    subscribed = false
    consumers--
    if (consumers !== 0) return
    clearInterval(timer)
    timer = undefined
    window.removeEventListener('focus', refreshVisiblePermissions)
    document.removeEventListener('visibilitychange', refreshVisiblePermissions)
  }
}
