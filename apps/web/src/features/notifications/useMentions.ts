import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useStore } from 'zustand'
import { pmKeys } from '@/shared/query'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime/state'
import { listInbox, markInboxRead, type NotificationCategory } from './api'
export type { Mention } from './api'
function useNotificationScope() {
 const workspaceId = useStore(runtimeWorkspaceStore, state => state.currentWorkspaceId)
 const revision = useStore(runtimeSessionStore, state => state.revision)
 const authenticated = useStore(runtimeSessionStore, state => Boolean(state.token))
 return {workspaceId, revision, enabled: authenticated && workspaceId > 0}
}
export function useMentions(open: boolean, category: NotificationCategory = '') {
 const scope = useNotificationScope()
 return useInfiniteQuery({
  queryKey: scope.workspaceId > 0 ? pmKeys.resource(scope.workspaceId, 'notifications', scope.revision, category) : [...pmKeys.session(), 'notifications-disabled'],
  enabled: open && scope.enabled,
  initialPageParam: 0,
  queryFn: ({signal, pageParam}) => listInbox(category, pageParam, signal),
  getNextPageParam: page => page.next_cursor || undefined,
  refetchInterval: open ? 15000 : false,
 })
}
export function useNotificationRead() {
 const scope = useNotificationScope()
 const client = useQueryClient()
 return useMutation({
  mutationFn: ({id, all = false, category = ''}: {id:number; all?:boolean; category?:NotificationCategory}) => {
   if (!scope.enabled) return Promise.reject(new Error('INBOX_SESSION_UNAVAILABLE'))
   return markInboxRead(id, all, category)
  },
  onSuccess: () => client.invalidateQueries({queryKey:pmKeys.resource(scope.workspaceId, 'notifications')}),
 })
}
