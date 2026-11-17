import { ProjectAccessGuide } from '@/features/resource-access'
import { useMemo, useRef, useState } from 'react'
import { QUALITY_COPY } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  CopyLinkButton,
  EmptyState,
  LazyMediaPreview,
  LoadingSkeleton,
  ResponsivePanel,
  uploadPreviewKind,
} from '@/shared/ui'
import { draftFromBug, editPayload } from '../../model/payloads'
import { safeMaterialUrl } from '../../model/values'
import type {
  QualityAssignPayload,
  QualityBug,
  QualityComment,
  QualityDetailState,
  QualityEditPayload,
  QualitySceneOptionCatalogs,
  QualitySceneOptionMutations,
  QualityLifecycleAction,
  QualityLifecycleModel,
  QualityMaterial,
  QualityMutationState,
  QualityResolvePayload,
  QualityReturnPayload,
  QualityRouteState,
  QualityUsersState,
} from '../../model/types'
import type { QualityActivitiesState, QualityCommentsState } from '../../hooks'
import type { QualityScopeState } from '../../hooks'
import { Form } from '../form/Form'
import { ReadView } from './ReadView'
import { AssignmentDialog, ResolveDialog, ReturnDialog } from '../dialogs'

type Mutation<T> = {
  state: QualityMutationState
  run: (payload: T) => Promise<unknown>
}

const DETAIL_EDIT_FORM_ID = 'quality-detail-edit-form'

export function Detail({
  route,
  sharePath,
  state,
  scope,
  users,
  sceneOptions,
  sceneOptionMutations,
  activities,
  comments,
  currentUserId,
  comment,
  deleteComment,
  canViewComments,
  canCreateComment,
  canDeleteComment,
  canUpdate,
  canDelete,
  canUpload,
  lifecycle,
  edit,
  assign,
  resolve,
  returnBug,
  remove,
  upload,
  assignOpen,
  resolveOpen,
  returnOpen,
  previewUrl,
  onAssignOpenChange,
  onResolveOpenChange,
  onReturnOpenChange,
  onPreviewChange,
  onLifecycleAction,
  onModeChange,
  onOpenRequirement,
  onClose,
}: {
  route: QualityRouteState
  sharePath?: string
  state: QualityDetailState
  scope: QualityScopeState
  users: QualityUsersState
  sceneOptions: QualitySceneOptionCatalogs
  sceneOptionMutations: QualitySceneOptionMutations
  activities: QualityActivitiesState
  comments: QualityCommentsState
  currentUserId: number
  lifecycle: QualityLifecycleModel | null
  edit: Mutation<QualityEditPayload>
  assign: Mutation<QualityAssignPayload>
  resolve: Mutation<QualityResolvePayload>
  returnBug: Mutation<QualityReturnPayload>
  remove: Mutation<QualityBug>
  upload: {
    state: QualityMutationState
    run: (payload: { requirementId: number; file: File }) => Promise<QualityMaterial>
  }
  comment: Mutation<string>
  deleteComment: Mutation<number>
  canViewComments: boolean
  canCreateComment: boolean
  canDeleteComment: (comment: QualityComment) => boolean
  canUpdate: boolean
  canDelete: boolean
  canUpload: boolean
  assignOpen: boolean
  resolveOpen: boolean
  returnOpen: boolean
  previewUrl: string | null
  onAssignOpenChange: (open: boolean) => void
  onResolveOpenChange: (open: boolean) => void
  onReturnOpenChange: (open: boolean) => void
  onPreviewChange: (url: string | null) => void
  onLifecycleAction: (action: QualityLifecycleAction) => void
  onModeChange: (mode: 'view' | 'edit') => void
  onOpenRequirement: (requirementId: number) => void
  onClose: () => void
}) {
  const [deleteOpen, setDeleteOpen] = useState(false)
  const previewTriggerRef = useRef<HTMLElement>(null)
  const bug = state.status === 'ready' ? state.bug : null
  const draft = useMemo(() => bug ? draftFromBug(bug) : null, [bug])
  const reproductionPreviewUrls = useMemo(() => {
    const seen = new Set<string>()
    return (bug?.reproductionMaterials || []).flatMap((material) => {
      const url = safeMaterialUrl(material.url)
      if (!url || seen.has(url) || uploadPreviewKind({ url }) === 'file') return []
      seen.add(url)
      return [url]
    })
  }, [bug?.reproductionMaterials])
  const previewIndex = previewUrl ? reproductionPreviewUrls.indexOf(previewUrl) : -1
  const previewNavigation = previewIndex >= 0 && reproductionPreviewUrls.length > 1
    ? {
        current: previewIndex + 1,
        total: reproductionPreviewUrls.length,
        onPrevious: () => onPreviewChange(
          reproductionPreviewUrls[(previewIndex - 1 + reproductionPreviewUrls.length) % reproductionPreviewUrls.length],
        ),
        onNext: () => onPreviewChange(
          reproductionPreviewUrls[(previewIndex + 1) % reproductionPreviewUrls.length],
        ),
      }
    : undefined
  const pending = edit.state.status === 'pending' || remove.state.status === 'pending'

  return (
    <>
      <ResponsivePanel
        open
        onOpenChange={(nextOpen) => { if (!nextOpen) onClose() }}
        pending={pending}
        title={bug?.title || QUALITY_COPY.detail.title}
        description={bug?.bugKey}
        closeLabel={route.mode === 'edit' ? QUALITY_COPY.form.cancel : QUALITY_COPY.detail.close}
        headerActions={sharePath ? <CopyLinkButton path={sharePath} /> : undefined}
        height="viewport"
        className="max-w-[90rem]"
        contentClassName="bg-muted/20"
        footer={bug ? (
          <>
            {route.mode === 'view' && canUpdate ? <Button type="button" variant="outline" onClick={() => onModeChange('edit')}>{QUALITY_COPY.detail.edit}</Button> : null}
            {canDelete ? <Button type="button" variant="destructive" className="sm:mr-auto" onClick={() => setDeleteOpen(true)}>{QUALITY_COPY.confirm.delete}</Button> : null}
            {route.mode === 'edit' && canUpdate && scope.status === 'ready' ? (
              <Button type="submit" form={DETAIL_EDIT_FORM_ID} className="sm:order-3" disabled={pending}>
                {edit.state.status === 'pending' ? QUALITY_COPY.mutation.pending : QUALITY_COPY.form.save}
              </Button>
            ) : null}
          </>
        ) : undefined}
      >
        {state.status === 'loading' ? <LoadingSkeleton rows={6} label={QUALITY_COPY.detail.loading} /> : null}
        {state.status === 'not_found' || state.status === 'access_required' ? <ProjectAccessGuide target={{ kind: 'quality_bug', id: route.bugId || 0 }} title={QUALITY_COPY.detail.notFound} denied={state.status === 'access_required'} sharePath={sharePath} /> : null}
        {state.status === 'error' ? <EmptyState role="alert" title={QUALITY_COPY.detail.error} action={<Button type="button" variant="outline" onClick={state.retry}>{QUALITY_COPY.detail.retry}</Button>} /> : null}
        {state.status === 'ready' && state.refresh === 'stale_error' ? (
          <p role="status" className="m-0 rounded-lg border border-amber-300/50 bg-amber-50 dark:bg-amber-950/40 p-3 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-200">
            {QUALITY_COPY.detail.staleError}
          </p>
        ) : null}
        {bug && route.mode === 'edit' && scope.status === 'loading' ? (
          <LoadingSkeleton rows={5} label={QUALITY_COPY.list.loading} />
        ) : null}
        {bug && route.mode === 'edit' && scope.status === 'error' ? (
          <EmptyState
            role="alert"
            title={QUALITY_COPY.list.error}
            action={<Button type="button" variant="outline" onClick={() => void scope.retry()}>{QUALITY_COPY.detail.retry}</Button>}
          />
        ) : null}
        {bug && route.mode === 'edit' && canUpdate && draft && scope.status === 'ready' ? (
          <Form
            formId={DETAIL_EDIT_FORM_ID}
            actions="external"
            initial={draft}
            scope={scope.data}
            users={users}
            sceneOptions={sceneOptions}
            sceneOptionMutations={sceneOptionMutations}
            mutation={edit.state}
            uploadState={upload.state}
            canUpload={canUpload}
            allowStatusEdit
            submitLabel={QUALITY_COPY.form.save}
            onSubmit={async (next) => { await edit.run(editPayload(next)); onClose() }}
            onUpload={(requirementId, file) => upload.run({ requirementId, file })}
          />
        ) : null}
        {bug && (route.mode === 'view' || !canUpdate) && lifecycle ? (
          <ReadView
            bug={bug}
            users={users}
            scopeOptions={scope.status === 'ready' ? scope.data : null}
            activities={activities}
            comments={comments}
            currentUserId={currentUserId}
            commentMutation={comment.state}
            canViewComments={canViewComments}
            canCreateComment={canCreateComment}
            canDeleteComment={canDeleteComment}
            canUpload={canUpload}
            onComment={async (body) => {
              const normalized = body.trim()
              if (!normalized) return false
              try {
                await comment.run(normalized)
                return true
              } catch {
                return false
              }
            }}
            onDeleteComment={async (id) => {
              try {
                await deleteComment.run(id)
                return true
              } catch {
                return false
              }
            }}
            onUploadCommentImage={async (file) => {
              if (!canUpload) return null
              try {
                const material = await upload.run({ requirementId: bug.requirementId || 0, file })
                return safeMaterialUrl(material.url) || null
              } catch {
                return null
              }
            }}
            lifecycle={lifecycle}
            onLifecycleAction={onLifecycleAction}
            onOpenRequirement={onOpenRequirement}
            onPreview={(url, trigger) => {
              previewTriggerRef.current = trigger
              onPreviewChange(url)
            }}
          />
        ) : null}
      </ResponsivePanel>

      {bug ? (
        <>
          <AssignmentDialog open={assignOpen} users={users} initialAssigneeId={bug.assigneeUserId} requirementId={bug.requirementId} canUpload={canUpload} state={assign.state} uploadState={upload.state} onOpenChange={onAssignOpenChange} onUpload={(requirementId, file) => upload.run({ requirementId, file })} onSubmit={async (payload) => { await assign.run(payload); onAssignOpenChange(false) }} />
          <ResolveDialog open={resolveOpen} users={users} initialReviewerId={bug.reporterUserId} requirementId={bug.requirementId} canUpload={canUpload} state={resolve.state} uploadState={upload.state} onOpenChange={onResolveOpenChange} onUpload={(requirementId, file) => upload.run({ requirementId, file })} onSubmit={async (payload) => { await resolve.run(payload); onResolveOpenChange(false) }} />
          <ReturnDialog open={returnOpen} requirementId={bug.requirementId} canUpload={canUpload} state={returnBug.state} uploadState={upload.state} onOpenChange={onReturnOpenChange} onUpload={(requirementId, file) => upload.run({ requirementId, file })} onSubmit={async (payload) => { await returnBug.run(payload); onReturnOpenChange(false) }} />
          <ConfirmDialog
            open={deleteOpen}
            onOpenChange={setDeleteOpen}
            pending={remove.state.status === 'pending'}
            title={QUALITY_COPY.confirm.deleteTitle}
            description={QUALITY_COPY.confirm.deleteDescription(bug.bugKey || bug.title)}
            cancelLabel={QUALITY_COPY.confirm.cancel}
            confirmLabel={QUALITY_COPY.confirm.delete}
            error={remove.state.status === 'error' ? QUALITY_COPY.mutation.deleteFailed : undefined}
            onConfirm={() => { void remove.run(bug).then(onClose).catch(() => undefined) }}
          />
          <LazyMediaPreview
            open={Boolean(previewUrl)}
            url={previewUrl || ''}
            title={QUALITY_COPY.upload.previewTitle}
            closeLabel={QUALITY_COPY.upload.close}
            finalFocus={previewTriggerRef}
            navigation={previewNavigation}
            onOpenChange={(open: boolean) => { if (!open) onPreviewChange(null) }}
          />
        </>
      ) : null}
    </>
  )
}
