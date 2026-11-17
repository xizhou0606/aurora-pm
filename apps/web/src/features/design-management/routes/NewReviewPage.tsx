import { useNavigate, useSearch } from '@tanstack/react-router'
import { PhArrowLeft as ArrowLeft } from '@/shared/ui/icons'

import { copy } from '../copy'
import { useWorkspaceStore } from '@/shared/state'
import { AlertBanner, Button, LoadingSkeleton, OperationalPage } from '@/shared/ui'
import { NewReviewWorkbench, type NewReviewSubmission } from '../components/NewReviewWorkbench'
import {
  useCaptureDrafts,
  useCollectorMutations,
  useCollectors,
  useCreateReview,
  useDesignReviewAccess,
  useDOMNormalization,
  useFigmaPreview,
  useReviewCapabilities,
  useReviewDetail,
  useReviewRequirements,
} from '../hooks'
import { designReviewErrorStatus, figmaPreviewErrorCopyKey, newReviewSubmitErrorCopyKey } from '../model'

export function NewReviewPage() {
  const search = useSearch({ from: '/workspace/design-management/reviews/new' })
  const navigate = useNavigate({ from: '/workspace/design-management/reviews/new' })
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const capabilities = useReviewCapabilities(workspaceId)
  const access = useDesignReviewAccess(capabilities.data)
  const createEnabled = access.canCreate && access.canCreateNow
  const requirements = useReviewRequirements(workspaceId, createEnabled)
  const source = useReviewDetail(workspaceId, search.source_review_id ?? '', createEnabled)
  const collectors = useCollectors(workspaceId, capabilities.data?.can_manage_collectors ?? false)
  const captures = useCaptureDrafts(workspaceId, capabilities.data?.can_manage_collectors ?? false)
  const collectorMutations = useCollectorMutations(workspaceId)
  const preview = useFigmaPreview()
  const normalization = useDOMNormalization()
  const create = useCreateReview(workspaceId)
  const sourceReview = search.source_review_id ? source.data : undefined
  const previewErrorKey = preview.isError
    ? figmaPreviewErrorCopyKey(designReviewErrorStatus(preview.error))
    : undefined
  const createErrorMessage = create.isError
    ? copy.designManagement.newReview[newReviewSubmitErrorCopyKey(designReviewErrorStatus(create.error))]
    : undefined
  const submit = (input: NewReviewSubmission) => {
    create.mutate(input, {
      onSuccess: (review) => void navigate({
        to: '/workspace/design-management/reviews/$reviewId',
        params: { reviewId: review.id },
        replace: true,
      }),
    })
  }
  const back = <Button variant="outline" onClick={() => void navigate({ to: '/workspace/design-management' })}><ArrowLeft />{copy.designManagement.navigation.back}</Button>
  if (capabilities.isPending || access.status === 'loading' || (search.source_review_id && source.isPending)) {
    return <OperationalPage title={copy.designManagement.newReview.title} actions={back}><LoadingSkeleton rows={8} /></OperationalPage>
  }
  if (capabilities.isError || access.status === 'error') {
    return <OperationalPage title={copy.designManagement.newReview.title} actions={back}><div className="p-5"><AlertBanner tone="error" action={<Button size="sm" variant="outline" onClick={() => { access.retry(); void capabilities.refetch() }}>{copy.designManagement.common.retry}</Button>}>{copy.designManagement.newReview.capabilityLoadFailed}</AlertBanner></div></OperationalPage>
  }
  if (search.source_review_id && (source.isError || !sourceReview)) {
    return <OperationalPage title={copy.designManagement.newReview.recheckTitle} actions={back}><div className="p-5"><AlertBanner tone="error" action={<Button size="sm" variant="outline" onClick={() => void source.refetch()}>{copy.designManagement.common.retry}</Button>}>{copy.designManagement.newReview.sourceLoadFailed}</AlertBanner></div></OperationalPage>
  }
  if (!access.canCreate) {
    return <OperationalPage title={copy.designManagement.newReview.title} actions={back}><div className="p-5"><AlertBanner tone="warning">{copy.designManagement.newReview.createForbidden}</AlertBanner></div></OperationalPage>
  }
  if (!access.canCreateNow) {
    return <OperationalPage title={copy.designManagement.newReview.title} actions={back}><div className="p-5"><AlertBanner tone="warning">{copy.designManagement.newReview.createUnavailable}</AlertBanner></div></OperationalPage>
  }
  return (
    <OperationalPage
      title={sourceReview ? copy.designManagement.newReview.recheckTitle : copy.designManagement.newReview.title}
      description={copy.designManagement.newReview.description}
      actions={back}
    >
      <NewReviewWorkbench
        requirements={requirements.data ?? []}
        requirementsState={requirements.isPending ? 'loading' : requirements.isError ? 'error' : 'ready'}
        onRetryRequirements={() => void requirements.refetch()}
        sourceReview={sourceReview}
        initialRequirementId={search.requirement_id}
        preview={preview.data}
        previewState={preview.isPending ? 'pending' : preview.isError ? 'error' : preview.data ? 'ready' : 'idle'}
        previewErrorKey={previewErrorKey}
        submitState={create.isPending ? 'pending' : create.isError ? 'error' : 'idle'}
        message={createErrorMessage ?? (normalization.error instanceof Error ? normalization.error.message : collectorMutations.createCapture.error instanceof Error ? collectorMutations.createCapture.error.message : source.error instanceof Error ? source.error.message : undefined)}
        collectors={collectors.data ?? []}
        captures={captures.data ?? []}
        mobileCaptureState={collectorMutations.createCapture.isPending ? 'pending' : collectorMutations.createCapture.isError ? 'error' : 'idle'}
        onPreview={(figmaUrl) => preview.mutate(figmaUrl)}
        onResetPreview={preview.reset}
        onNormalizeDOM={(figmaUrl, inventory) => normalization.mutateAsync({ figmaUrl, inventory })}
        onStartMobileCapture={(input) => collectorMutations.createCapture.mutate(input)}
        onLoadMobileCapture={(id) => collectorMutations.captureAsset.mutateAsync(id)}
        onSubmit={submit}
      />
    </OperationalPage>
  )
}
