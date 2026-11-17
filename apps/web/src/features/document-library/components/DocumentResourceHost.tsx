import { DOCUMENT_LIBRARY_COPY as copy } from '@/shared/copy'
import { AlertBanner, EmptyState, LoadingSkeleton } from '@/shared/ui'

import type {
  DocumentResourceDescriptor,
  DocumentResourceRendererRegistry,
} from '../model'

export type DocumentResourceHostView =
  | { status: 'loading' }
  | { status: 'ready'; resource: DocumentResourceDescriptor; readOnly: boolean }
  | { status: 'unavailable'; reason: 'invalid_resource' | 'unknown_type' }

export function DocumentResourceHost({
  view,
  registry,
}: {
  view: DocumentResourceHostView
  registry: DocumentResourceRendererRegistry
}) {
  if (view.status === 'loading') {
    return <LoadingSkeleton rows={6} label={copy.resource.loading} />
  }
  if (view.status === 'unavailable') {
    const invalid = view.reason === 'invalid_resource'
    return (
      <div className="grid gap-3">
        <EmptyState title={invalid ? copy.resource.invalidTitle : copy.resource.unavailableTitle} />
        <AlertBanner tone="error">
          {invalid ? copy.resource.invalidReason : copy.resource.unavailableReason}
        </AlertBanner>
      </div>
    )
  }
  if (!view.resource.resourceId || !view.resource.payloadRef) {
    return (
      <div className="grid gap-3">
        <EmptyState title={copy.resource.invalidTitle} />
        <AlertBanner tone="error">{copy.resource.invalidReason}</AlertBanner>
      </div>
    )
  }
  const adapter = registry.resolve(view.resource.typeKey)
  if (!adapter) {
    return (
      <div className="grid gap-3">
        <EmptyState title={copy.resource.unavailableTitle} />
        <AlertBanner tone="error">{copy.resource.unavailableReason}</AlertBanner>
      </div>
    )
  }
  const Renderer = adapter.Renderer
  return <Renderer resource={view.resource} readOnly={view.readOnly} />
}
