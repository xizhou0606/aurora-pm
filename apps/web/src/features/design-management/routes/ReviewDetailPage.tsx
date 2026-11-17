import { useNavigate, useParams } from '@tanstack/react-router'
import { PhArrowLeft as ArrowLeft } from '@/shared/ui/icons'

import { copy } from '../copy'
import { useWorkspaceStore } from '@/shared/state'
import { AlertBanner, Button, EmptyState, LoadingSkeleton, OperationalPage } from '@/shared/ui'
import { ReviewWorkbench } from '../components/ReviewWorkbench'
import {
  useDesignReviewAccess,
  useDifferenceMutations,
  useObjectURL,
  useReviewAsset,
  useReviewDetail,
  useReviewLifecycleMutations,
} from '../hooks'
import { assetByKind, type DifferenceInput, type Region } from '../model'

type OperationState = 'idle' | 'pending' | 'error'

export function ReviewDetailPage() {
  const { reviewId } = useParams({ from: '/workspace/design-management/reviews/$reviewId' })
  const navigate = useNavigate({ from: '/workspace/design-management/reviews/$reviewId' })
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const detail = useReviewDetail(workspaceId, reviewId)
  const access = useDesignReviewAccess(detail.data?.capabilities)
  const lifecycle = useReviewLifecycleMutations(workspaceId, reviewId)
  const differences = useDifferenceMutations(workspaceId, reviewId)
  const designAsset = assetByKind(detail.data?.assets ?? [], ['design_normalized', 'design_original'])
  const implementationAsset = assetByKind(detail.data?.assets ?? [], ['implementation_normalized', 'implementation_original'])
  const heatmapAsset = assetByKind(detail.data?.assets ?? [], ['pixel_heatmap', 'pixel_heatmap_original'])
  const design = useReviewAsset(workspaceId, reviewId, designAsset?.id)
  const implementation = useReviewAsset(workspaceId, reviewId, implementationAsset?.id)
  const heatmap = useReviewAsset(workspaceId, reviewId, heatmapAsset?.id)
  const designURL = useObjectURL(design.data)
  const implementationURL = useObjectURL(implementation.data)
  const heatmapURL = useObjectURL(heatmap.data)
  const assetState: OperationState = (
    (designAsset && design.isPending) || (implementationAsset && implementation.isPending) || (heatmapAsset && heatmap.isPending)
  ) ? 'pending' : design.isError || implementation.isError || heatmap.isError ? 'error' : 'idle'
  const differenceMutations = [differences.create, differences.update, differences.remove, differences.verify, differences.resolve, differences.reopen, differences.resolveRecheckMatch, differences.overrideRecheckVerification]
  const analysisMutations = [lifecycle.retry, lifecycle.abandon, lifecycle.remove, lifecycle.exportFile, differences.deepReview, differences.retryDeepReview, differences.cancelDeepReview]
  const differenceState = operationState(differenceMutations)
  const analysisState = operationState(analysisMutations)
  const differenceMessage = operationMessage(differenceMutations)
  const analysisMessage = operationMessage(analysisMutations)
  const back = <Button variant="outline" onClick={() => void navigate({ to: '/workspace/design-management' })}><ArrowLeft />{copy.designManagement.navigation.back}</Button>

  if (detail.isPending) {
    return <OperationalPage title={copy.designManagement.detail.title} actions={back}><div className="p-5"><LoadingSkeleton rows={10} /></div></OperationalPage>
  }
  if (detail.isError || !detail.data) {
    return <OperationalPage title={copy.designManagement.detail.title} actions={back}><div className="grid gap-4 p-5"><AlertBanner tone="error">{detail.error instanceof Error ? detail.error.message : copy.designManagement.detail.loadFailed}</AlertBanner><EmptyState title={copy.designManagement.detail.notFound} action={<Button onClick={() => void detail.refetch()}>{copy.designManagement.common.retry}</Button>} /></div></OperationalPage>
  }
  const review = detail.data
  return (
    <OperationalPage
      title={review.requirement_title}
      description={copy.designManagement.format.scopeRound(review.project_set_name, review.plan_unit_title, copy.designManagement.detail.roundValue(review.round_number))}
      actions={back}
      refresh={detail.isFetching ? 'refreshing' : detail.isError ? 'stale_error' : 'idle'}
      bodyOverflow="hidden"
    >
      <ReviewWorkbench
        review={review}
        designURL={designURL}
        implementationURL={implementationURL}
        heatmapURL={heatmapURL}
        assetState={assetState}
        assetMessage={operationMessage([design, implementation, heatmap])}
        differenceState={differenceState}
        differenceMessage={differenceMessage}
        analysisState={analysisState}
        analysisMessage={analysisMessage}
        canCreate={access.canCreate && access.canCreateNow}
        canManageDifferences={access.canManageDifferences}
        canManageAnalysis={access.canManageAnalysis && access.canAnalyzeNow}
        canDelete={access.canDelete && review.capabilities.can_delete}
        onRoundChange={(id) => void navigate({ to: '/workspace/design-management/reviews/$reviewId', params: { reviewId: id } })}
        onRecheck={() => void navigate({ to: '/workspace/design-management/reviews/new', search: { requirement_id: review.requirement_id, source_review_id: review.id } })}
        onRetryAnalysis={() => lifecycle.retry.mutate()}
        onAbandonAnalysis={() => lifecycle.abandon.mutate()}
        onDeepReview={(designRegion: Region, implementationRegion: Region, focus: string) => differences.deepReview.mutate({ designRegion, implementationRegion, focus })}
        onRetryDeepReview={(id) => differences.retryDeepReview.mutate(id)}
        onCancelDeepReview={(id) => differences.cancelDeepReview.mutate(id)}
        onExport={(format) => lifecycle.exportFile.mutate(format)}
        onDelete={() => lifecycle.remove.mutate(undefined, { onSuccess: () => void navigate({ to: '/workspace/design-management', replace: true }) })}
        onCreateDifference={(input: DifferenceInput) => differences.create.mutate(input)}
        onUpdateDifference={(id: string, input: DifferenceInput) => differences.update.mutate({ id, input })}
        onDeleteDifference={(id: string) => differences.remove.mutate(id)}
        onVerifyDifference={(id, status) => differences.verify.mutate({ id, status })}
        onResolveDifference={(id: string) => differences.resolve.mutate(id)}
        onReopenDifference={(id: string) => differences.reopen.mutate(id)}
        onResolveRecheckMatch={(id, resolution) => differences.resolveRecheckMatch.mutate({ id, resolution })}
        onOverrideRecheckVerification={(id, status) => differences.overrideRecheckVerification.mutate({ id, status })}
      />
    </OperationalPage>
  )
}

type AsyncOperation = { isPending: boolean; isError: boolean; error: unknown }

function operationState(operations: readonly AsyncOperation[]): OperationState {
  if (operations.some((item) => item.isPending)) return 'pending'
  if (operations.some((item) => item.isError)) return 'error'
  return 'idle'
}

function operationMessage(operations: readonly { isError: boolean; error: unknown }[]): string | undefined {
  const error = operations.find((item) => item.isError)?.error
  return error instanceof Error ? error.message : undefined
}
