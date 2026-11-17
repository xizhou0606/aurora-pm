import type { NativeModuleQueryResource } from '../query'
import type { PMBusinessModuleId } from '@/shared/config'

export type NativeModuleRouteDefinition = {
  id: PMBusinessModuleId
  path: string
  moduleKey: 'task_library'
  menuKey: string
  queryResource: NativeModuleQueryResource
}

export const NATIVE_MODULE_ROUTES = [
  {
    id: 'inbox',
    path: '/workspace/inbox',
    moduleKey: 'task_library',
    menuKey: 'menu.work_items',
    queryResource: 'inbox',
  },
  {
    id: 'workItems',
    path: '/workspace/work-items',
    moduleKey: 'task_library',
    menuKey: 'menu.work_items',
    queryResource: 'globalWorkItems',
  },
  {
    id: 'biScreen',
    path: '/workspace/bi-screen',
    moduleKey: 'task_library',
    menuKey: 'menu.executive_dashboard',
    queryResource: 'biScreen',
  },
  {
    id: 'quality',
    path: '/workspace/quality-management',
    moduleKey: 'task_library',
    menuKey: 'menu.testing_board',
    queryResource: 'quality',
  },
  {
    id: 'reporting',
    path: '/workspace/reporting',
    moduleKey: 'task_library',
    menuKey: 'menu.reporting',
    queryResource: 'reporting',
  },
] as const satisfies readonly NativeModuleRouteDefinition[]

export function nativeModuleRoute(id: PMBusinessModuleId): NativeModuleRouteDefinition {
  const route = NATIVE_MODULE_ROUTES.find((candidate) => candidate.id === id)
  if (!route) throw new Error(`Unknown native module: ${id}`)
  return route
}

export function nativeModuleIdForPath(rawPath: string): PMBusinessModuleId | undefined {
  let pathname = ''
  try {
    pathname = new URL(rawPath, window.location.origin).pathname
  } catch {
    return undefined
  }
  if (pathname === '/workspace/inbox') return 'inbox'
  if (/^\/workspace\/work-items(?:\/\d+)?$/.test(pathname)) return 'workItems'
  if (pathname === '/workspace/bi-screen') return 'biScreen'
  if (pathname === '/workspace/reporting') return 'reporting'
  if (
    pathname === '/workspace/quality-management' ||
    pathname === '/workspace/quality-management/bugs/new' ||
    /^\/workspace\/quality-management\/bugs\/\d+$/.test(pathname)
  ) {
    return 'quality'
  }
  return undefined
}
