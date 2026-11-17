import { useMemo } from 'react'
import { QUALITY_COPY } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton } from '@/shared/ui'
import type { QualityScopeState } from '../../hooks'
import { emptyDraft } from '../../model/payloads'
import type {
  QualityBugDraft,
  QualitySceneOptionCatalogs,
  QualitySceneOptionMutations,
  QualityMaterial,
  QualityMutationState,
  QualityRouteState,
  QualityUsersState,
} from '../../model/types'
import { Form } from './Form'

export function Create({
  formId,
  route,
  access,
  scope,
  users,
  mutation,
  uploadState,
  canUpload,
  sceneOptions,
  sceneOptionMutations,
  onSubmit,
  onUpload,
  onCancel,
}: {
  formId: string
  route: QualityRouteState
  access: 'loading' | 'denied' | 'allowed'
  scope: QualityScopeState
  users: QualityUsersState
  mutation: QualityMutationState
  uploadState: QualityMutationState
  canUpload: boolean
  sceneOptions: QualitySceneOptionCatalogs
  sceneOptionMutations: QualitySceneOptionMutations
  onSubmit: (draft: QualityBugDraft) => Promise<void>
  onUpload: (requirementId: number, file: File) => Promise<QualityMaterial>
  onCancel: () => void
}) {
  const initial = useMemo(() => ({
    ...emptyDraft({
      actualResult: route.actualResult,
      sourceExecutionId: route.sourceExecutionId,
      sourceTestId: route.sourceTestId,
    }),
    requirementId: route.scope.requirementIds.length === 1
      ? route.scope.requirementIds[0]
      : undefined,
  }), [route.actualResult, route.scope.requirementIds, route.sourceExecutionId, route.sourceTestId])

  if (access === 'loading' || scope.status === 'loading') {
    return <LoadingSkeleton rows={5} label={QUALITY_COPY.list.loading} />
  }
  if (access === 'denied') {
    return <EmptyState role="alert" title={QUALITY_COPY.page.contractTitle} />
  }
  if (scope.status === 'error') {
    return (
      <EmptyState
        role="alert"
        title={QUALITY_COPY.list.error}
        action={<Button type="button" variant="outline" onClick={() => void scope.retry()}>{QUALITY_COPY.detail.retry}</Button>}
      />
    )
  }
  if (scope.status === 'unavailable') {
    return (
      <EmptyState
        role="alert"
        title={QUALITY_COPY.page.workspaceUnavailableTitle}
      />
    )
  }
  return (
    <Form
      formId={formId}
      actions="external"
      initial={initial}
      scope={scope.data}
      users={users}
      mutation={mutation}
      uploadState={uploadState}
      canUpload={canUpload}
      sceneOptions={sceneOptions}
      sceneOptionMutations={sceneOptionMutations}
      submitLabel={QUALITY_COPY.form.submit}
      onSubmit={onSubmit}
      onUpload={onUpload}
      onCancel={onCancel}
    />
  )
}
