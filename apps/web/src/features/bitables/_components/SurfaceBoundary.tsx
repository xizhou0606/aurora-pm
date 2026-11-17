import { DeniedDocumentAccess } from '@/features/resource-access'
import { AlertBanner, Button, EmptyState, LoadingSkeleton } from '@/shared/ui'
import { BITABLE_COPY as copy } from '@/shared/copy/bitables'
import type { BitableSurfaceState } from '../model/surface-state'

export function SurfaceBoundary({ state, bitableId, onRetry }: {
  state: BitableSurfaceState
  bitableId: string
  onRetry: () => void
}) {
  if (state.status === 'ready') return null
  if (state.status === 'loading') return <LoadingSkeleton rows={6} label={copy.loading} />
  const message = copy.loadFailures[state.reason]
  const fallback = <div className="flex h-full min-h-48 items-center justify-center p-6">
    <div className="flex w-full max-w-md flex-col items-center gap-3 text-center">
      <EmptyState className="flex-none" title={message.title} />
      <AlertBanner tone="error">{message.description}</AlertBanner>
      <Button variant="outline" onClick={onRetry}>{copy.retry}</Button>
    </div>
  </div>
  return state.checkAccess
    ? <DeniedDocumentAccess resourceType="bitable" bitableId={bitableId} fallback={fallback} />
    : fallback
}
