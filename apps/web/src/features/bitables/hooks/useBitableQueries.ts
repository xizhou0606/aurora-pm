import { useQuery } from '@tanstack/react-query'
import { canRetainStaleApiData, PMSafeApiError } from '@/shared/api'
import {
  clearResourceAccessCredential,
  expectedResourceAccessId,
} from '@/features/resource-access'
import type { TableScope } from '../api/client'
import type { RecordQuery } from '../model/types'
import { bitableKey, currentEnvironment, type BitableEnvironment } from './environment'

export function useResourceList(env: BitableEnvironment, enabled: boolean) {
  return useQuery({ queryKey: bitableKey(env, undefined, 'list'), enabled: enabled && env.workspaceId > 0,
    queryFn: async ({ signal }) => {
      const resources = await env.client.list(signal)
      if (resources.some((resource) => resource.workspaceId !== env.workspaceId) || !currentEnvironment(env)) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      return resources
    }, retry: false })
}
export function useResource(env: BitableEnvironment, resourceId: string) {
  return useQuery({ queryKey: bitableKey(env, { resourceId, tableId: '' }, 'resource'), enabled: env.workspaceId > 0 && Boolean(resourceId),
    queryFn: async ({ signal }) => {
      const resource = await env.client.resource(resourceId, signal)
      const expected = expectedResourceAccessId('bitable')
      if (expected && resource.resourceId !== expected) {
        clearResourceAccessCredential('bitable')
        throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      }
      if (resource.workspaceId !== env.workspaceId || resource.id !== resourceId || !currentEnvironment(env)) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      return resource
    }, retry: false })
}
export function useBootstrap(env: BitableEnvironment, scope: TableScope, enabled: boolean) {
  return useQuery({ queryKey: bitableKey(env, scope, 'bootstrap'), enabled: enabled && env.workspaceId > 0 && Boolean(scope.tableId),
    queryFn: async ({ signal }) => {
      const b = await env.client.bootstrap(scope, signal)
      if (b.resource.workspaceId !== env.workspaceId || b.resource.id !== scope.resourceId || b.tableId !== scope.tableId || !currentEnvironment(env)) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      return b
    }, retry: false })
}
export function useRecords(env: BitableEnvironment, scope: TableScope, query: RecordQuery, enabled: boolean) {
  return useQuery({ queryKey: bitableKey(env, scope, 'records', query), enabled: enabled && Boolean(query.viewId),
    queryFn: async ({ signal }) => {
      const page = await env.client.query(scope, query, signal)
      if (!currentEnvironment(env) || page.offset !== query.offset) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      return page
    }, retry: false })
}
export function useSearch(env: BitableEnvironment, scope: TableScope, query: RecordQuery, text: string, enabled: boolean) {
  return useQuery({ queryKey: bitableKey(env, scope, 'search', query, text), enabled: enabled && Boolean(text.trim()) && Boolean(query.viewId),
    queryFn: async ({ signal }) => {
      const result = await env.client.search(scope, query, text, signal)
      if (!currentEnvironment(env)) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      return result
    }, retry: false })
}
export function usePeople(env: BitableEnvironment, scope: TableScope, enabled: boolean) {
  return useQuery({ queryKey: bitableKey(env, scope, 'people'), enabled: enabled && Boolean(scope.tableId),
    queryFn: async ({ signal }) => {
      const people = await env.client.people(scope, signal)
      if (!currentEnvironment(env)) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      return people
    }, retry: false })
}
export function visibleQueryData<T>(data: T | undefined, error: unknown): T | undefined {
  return error && !canRetainStaleApiData(error) ? undefined : data
}
