import { nativeModuleKeys, pmKeys } from '@/shared/query'
import type { QualityRouteState, QualitySceneOptionKind } from '../model/types'

export const QUALITY_ACTIVITY_PAGE_SIZE = 50

export function qualityKeys(workspaceId: number) {
  return {
    root: nativeModuleKeys.root(workspaceId, 'quality'),
    scope: nativeModuleKeys.resource(workspaceId, 'quality', 'scope'),
    userDirectory: pmKeys.resource(workspaceId, 'workspace', 'users', 'directory'),
    selectedDirectoryUsers: (ids: readonly number[]) => pmKeys.resource(
      workspaceId,
      'workspace',
      'users',
      'selected',
      ids,
    ),
    sceneOptions: (kind: QualitySceneOptionKind) => nativeModuleKeys.resource(
      workspaceId,
      'quality',
      'scene-options',
      kind,
    ),
    environments: nativeModuleKeys.resource(workspaceId, 'quality', 'scene-options', 'environment'),
    list: (route: QualityRouteState) => nativeModuleKeys.resource(
      workspaceId,
      'quality',
      'list',
      route.scope,
      route.filters,
      route.page,
      route.pageSize,
    ),
    summary: (route: QualityRouteState) => nativeModuleKeys.resource(
      workspaceId,
      'quality',
      'summary',
      route.scope,
      route.filters,
    ),
    dashboard: (route: QualityRouteState) => nativeModuleKeys.resource(
      workspaceId,
      'quality',
      'dashboard',
      route.scope,
      route.filters,
    ),
    detail: (id: number) => nativeModuleKeys.resource(workspaceId, 'quality', 'detail', id),
    activities: (id: number, pageSize = QUALITY_ACTIVITY_PAGE_SIZE) => nativeModuleKeys.resource(
      workspaceId,
      'quality',
      'detail',
      id,
      'activities',
      { limit: pageSize },
    ),
    comments: (id: number) => nativeModuleKeys.resource(
      workspaceId,
      'quality',
      'detail',
      id,
      'comments',
    ),
  }
}
