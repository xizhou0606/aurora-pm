import { useCallback } from 'react'

import type { QualityEnvironment } from '../components/environment'
import { qualityDetailPath, qualityListPath } from '../model/route'
import type { QualityRouteState } from '../model/types'
import {
  useAssignMutation,
  useDeleteMutation,
  useEditMutation,
  useResolveMutation,
  useReturnMutation,
  useTransitionMutation,
  useUploadMutation,
  useSceneOptionMutations,
  useQualityCommentDeleteMutation,
  useQualityCommentMutation,
} from './useMutations'
import {
  useQualityActivitiesQuery,
  useQualityCommentsQuery,
  useQualityDetailQuery,
  useQualityUsersQuery,
  useQualitySceneOptionCatalogs,
} from './useQueries'
import { useQualityLifecycleWorkflow } from './useLifecycleWorkflow'
import { useQualityOverlayWorkflow } from './useOverlayWorkflow'
import { useQualityOperationAccess } from './useOperationAccess'

export function useQualityDetailWorkflow(
  environment: QualityEnvironment,
  route: QualityRouteState,
) {
  const access = useQualityOperationAccess()
  const detail = useQualityDetailQuery(environment, route, access.canViewBugs)
  const bug = detail.status === 'ready' ? detail.bug : null
  const canUpdate = access.canUpdateBug(bug)
  const canDelete = access.canDeleteBug(bug)
  const canAssign = access.canAssignBug(bug)
  const canResolve = access.canResolveBug(bug)
  const canReturn = access.canReturnBug(bug)
  const canTransition = access.canTransitionBug(bug)
  const canUpload = access.canUploadMaterial(Boolean(bug?.capabilities.canUpload))
  const activities = useQualityActivitiesQuery(
    environment,
    route,
    access.canViewBugs && detail.status === 'ready',
  )
  const comments = useQualityCommentsQuery(
    environment,
    route,
    access.canViewComments && detail.status === 'ready',
  )
  const users = useQualityUsersQuery(environment, bug ? [
    bug.assigneeUserId,
    bug.reporterUserId,
    bug.reviewerUserId,
  ].filter((id): id is number => Boolean(id)) : [])
  const id = route.bugId || 0
  const edit = useEditMutation(environment, id, {
    ...route,
    kind: 'list',
    bugId: undefined,
  }, canUpdate)
  const assign = useAssignMutation(environment, id, canAssign)
  const resolve = useResolveMutation(environment, id, canResolve)
  const returnBug = useReturnMutation(environment, id, canReturn)
  const transition = useTransitionMutation(environment, id, canTransition)
  const remove = useDeleteMutation(environment, access.canDeleteBug)
  const upload = useUploadMutation(environment, canUpload)
  const sceneOptions = useQualitySceneOptionCatalogs(environment, access.canViewSceneOptions)
  const sceneOptionMutations = useSceneOptionMutations(environment, {
    canCreate: access.canCreateSceneOption,
    canDelete: access.canDeleteSceneOption,
  })
  const comment = useQualityCommentMutation(environment, id, access.canCreateComment)
  const deleteComment = useQualityCommentDeleteMutation(environment, id, (commentId) => (
    comments.status === 'ready'
    && access.canDeleteComment(comments.rows.find((row) => row.id === commentId))
  ))
  const overlay = useQualityOverlayWorkflow({
    workspaceId: environment.workspaceId,
    bugId: route.bugId,
    edit,
    assign,
    resolve,
    returnBug,
    transition,
    remove,
    upload,
  })
  const lifecycle = useQualityLifecycleWorkflow({
    detail,
    transition,
    canAssign,
    canResolve,
    canReturn,
    canTransition,
    openAssign: () => overlay.setAssignOpen(true),
    openResolve: () => overlay.setResolveOpen(true),
    openReturn: () => overlay.setReturnOpen(true),
  })
  const close = useCallback(() => environment.navigate(
    route.from === 'inbox' ? '/workspace/inbox' : qualityListPath(route),
  ), [environment, route])
  const setMode = useCallback((mode: 'view' | 'edit') => {
    if (!route.bugId) return Promise.resolve()
    return environment.navigate(qualityDetailPath(route, route.bugId, mode), true)
  }, [environment, route])

  return {
    detail,
    activities,
    comments,
    users,
    lifecycle,
    edit,
    assign,
    resolve,
    returnBug,
    remove,
    upload,
    sceneOptions,
    sceneOptionMutations,
    comment,
    deleteComment,
    canViewComments: access.canViewComments,
    canCreateComment: access.canCreateComment,
    canDeleteComment: access.canDeleteComment,
    canUpdate,
    canDelete,
    canUpload,
    overlay,
    close,
    setMode,
  }
}
