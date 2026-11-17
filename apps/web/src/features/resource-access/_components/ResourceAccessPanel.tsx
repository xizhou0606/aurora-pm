import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'
import { AlertBanner, Button, EmptyState, LoadingSkeleton, ResponsivePanel } from '@/shared/ui'

import type { ResourceAccessPanelView } from '../model'
import { PolicyForm } from './PolicyForm'
import { ResourceShareLinksSection } from './ResourceShareLinksSection'

export function ResourceAccessPanel({
  open,
  view,
  onOpenChange,
  onCloseRequest,
  purpose = 'permissions',
}: {
  open: boolean
  view: ResourceAccessPanelView
  onOpenChange: (open: boolean) => void
  onCloseRequest?: () => boolean | void
  purpose?: 'share' | 'permissions'
}) {
  const ready = view.status === 'ready' ? view.control : null
  const saving = ready?.state.status === 'submitting'
  const canSave = ready?.state.status === 'dirty'
    && ready.state.base.canManageAccess
    && ready.state.base.mode !== 'legacy'

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      onCloseRequest={onCloseRequest}
      title={purpose === 'share' ? copy.links.title : copy.panel.title}
      description={purpose === 'share' ? copy.links.description : copy.panel.description}
      closeLabel={copy.panel.close}
      pending={saving}
      className={purpose === 'share' ? 'max-w-xl' : 'max-w-3xl'}
      footer={ready && purpose === 'permissions' ? (
        <>
          <Button
            type="button"
            variant="outline"
            disabled={saving}
            onClick={() => {
              ready.onDiscard()
              onOpenChange(false)
            }}
          >
            {copy.panel.cancel}
          </Button>
          <Button type="button" disabled={!canSave || saving} onClick={ready.onSubmit}>
            {saving ? copy.panel.saving : copy.panel.save}
          </Button>
        </>
      ) : undefined}
    >
      {view.status === 'loading' ? (
        <LoadingSkeleton rows={5} label={copy.panel.loading} />
      ) : view.status === 'error' ? (
        <div className="grid gap-3">
          <EmptyState
            role="alert"
            title={copy.panel.errorTitle}
            action={<Button type="button" variant="outline" onClick={view.onRetry}>{copy.panel.retry}</Button>}
          />
          <AlertBanner tone="error">{copy.panel.errorReason}</AlertBanner>
        </div>
      ) : (
        <div className="grid gap-3">
          {view.refresh === 'stale_error' ? (
            <AlertBanner tone="warning">{copy.change.stale}</AlertBanner>
          ) : null}
          {purpose === 'permissions' ? <><p className="text-xs text-muted-foreground">{copy.panel.independentLinks}</p><PolicyForm
            state={view.control.state}
            subjects={view.control.subjects}
            error={view.control.error}
            onEdit={view.control.onEdit}
            onReload={view.control.onReload}
          /></> : (
            <ResourceShareLinksSection control={view.control.links} />
          )}
        </div>
      )}
    </ResponsivePanel>
  )
}
