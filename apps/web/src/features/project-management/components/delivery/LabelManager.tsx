import { useEffect, useState } from 'react'

import { copy } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  EmptyState,
  Input,
  LoadingSkeleton,
  ResponsivePanel,
} from '@/shared/ui'
import { projectDeliveryNodeLabelIdentity } from '@/features/project-management/utils/projectDeliveryNodes'
import type { ScrumProjectDeliveryNodeLabel } from '../../api'
import type { DeliveryLabelManagerState } from '../../model'
import { WriteFormField } from '../WriteFormField'

const deliveryCopy = copy.projectManagement.delivery
const EMPTY_LABELS: readonly ScrumProjectDeliveryNodeLabel[] = []

type LabelCatalogState =
  | { status: 'loading'; writable: false }
  | { status: 'error'; retry: () => void; writable: false }
  | { status: 'empty'; refreshing: boolean; staleError?: { retry: () => void }; writable: boolean }
  | {
      status: 'ready'
      items: readonly ScrumProjectDeliveryNodeLabel[]
      refreshing: boolean
      staleError?: { retry: () => void }
      writable: boolean
    }

export type LabelManagerControl = {
  catalog: LabelCatalogState
  history: readonly ScrumProjectDeliveryNodeLabel[]
  allowDraftLabels?: boolean
  canCreate?: boolean
  canRename?: boolean
  canRemove?: boolean
  onNameChange: (name: string) => void
  onCreate: () => void
  onRename: (label: ScrumProjectDeliveryNodeLabel, name: string) => void
  onRemove: (label: ScrumProjectDeliveryNodeLabel) => void
  onClose: () => void
}

function actionLocked(state: DeliveryLabelManagerState): boolean {
  return state.status === 'open' && !['idle', 'error'].includes(state.action.status)
}

function actionError(state: DeliveryLabelManagerState): string | undefined {
  if (state.status !== 'open' || state.action.status !== 'error') return undefined
  if (state.action.errorCode === 'LABEL_REQUIRED') return deliveryCopy.labels.required
  if (state.action.errorCode === 'LABEL_DUPLICATE') return deliveryCopy.labels.duplicate
  if (state.action.errorCode === 'RESOURCE_CHANGED') return deliveryCopy.feedback.contextChanged
  return deliveryCopy.feedback.labelFailed
}

function LabelRows({
  items,
  history,
  names,
  locked,
  onNameChange,
  onRename,
  onRequestRemove,
  allowDraftLabels,
  canRename,
  canRemove,
}: {
  items: readonly ScrumProjectDeliveryNodeLabel[]
  history: readonly ScrumProjectDeliveryNodeLabel[]
  names: Readonly<Record<string, string>>
  locked: boolean
  onNameChange: (identity: string, name: string) => void
  onRename: LabelManagerControl['onRename']
  onRequestRemove: (label: ScrumProjectDeliveryNodeLabel) => void
  allowDraftLabels: boolean
  canRename: boolean
  canRemove: boolean
}) {
  if (items.length === 0 && history.length === 0) {
    return (
      <EmptyState
        title={deliveryCopy.labels.empty}
        className="border border-dashed border-border"
      />
    )
  }
  return (
    <div className="grid gap-2">
      {items.map((label) => {
        const identity = projectDeliveryNodeLabelIdentity(label)
        const draftName = names[identity] ?? label.name
        return (
          <div key={identity} className="grid gap-2 rounded-lg border border-border bg-card p-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
            <Input
              value={draftName}
              disabled={locked || (!label.id && !allowDraftLabels)}
              aria-label={deliveryCopy.labels.name}
              onChange={(event) => onNameChange(identity, event.target.value)}
            />
            <div className="flex flex-wrap gap-2">
              {label.id || allowDraftLabels ? (
                <>
                  {canRename ? <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    disabled={locked || draftName.trim() === label.name.trim()}
                    onClick={() => onRename(label, draftName)}
                  >
                    {deliveryCopy.labels.rename}
                  </Button> : null}
                  {canRemove ? <Button
                    type="button"
                    size="xs"
                    variant="destructive"
                    disabled={locked}
                    onClick={() => onRequestRemove(label)}
                  >
                    {deliveryCopy.labels.remove}
                  </Button> : null}
                </>
              ) : null}
            </div>
          </div>
        )
      })}
      {history.map((label) => (
        <div key={`history:${projectDeliveryNodeLabelIdentity(label)}`} className="flex items-center justify-between gap-3 rounded-lg border border-dashed border-border bg-muted/25 px-3 py-2">
          <span className="truncate text-sm text-foreground">{label.name}</span>
          <span className="shrink-0 rounded-full border border-border bg-muted px-2 py-0.5 text-xs text-muted-foreground">
            {deliveryCopy.deletedLabel}
          </span>
        </div>
      ))}
    </div>
  )
}

export function LabelManager({
  state,
  targetTitle,
  control,
}: {
  state: DeliveryLabelManagerState
  targetTitle?: string
  control: LabelManagerControl
}) {
  const opened = state.status === 'open'
  const locked = actionLocked(state)
  const canCreate = control.canCreate ?? true
  const canRename = control.canRename ?? true
  const canRemove = control.canRemove ?? true
  const writesDisabled = locked || !control.catalog.writable
  const [names, setNames] = useState<Record<string, string>>({})
  const [removeTarget, setRemoveTarget] = useState<ScrumProjectDeliveryNodeLabel | null>(null)
  const items = control.catalog.status === 'ready'
    ? control.catalog.items
    : EMPTY_LABELS

  useEffect(() => {
    if (!opened) {
      setNames({})
      setRemoveTarget(null)
      return
    }
    setNames(Object.fromEntries(items.map((label) => [
      projectDeliveryNodeLabelIdentity(label),
      label.name,
    ])))
  }, [items, opened])

  useEffect(() => {
    if (state.status === 'open' && state.action.status === 'idle') {
      setRemoveTarget(null)
    }
  }, [state])

  const message = actionError(state)
  const removeError = state.status === 'open' && state.action.status === 'error' &&
    state.action.action === 'delete'
    ? message
    : undefined
  return (
    <>
      <ResponsivePanel
        open={opened}
        onOpenChange={(open) => { if (!open) control.onClose() }}
        title={deliveryCopy.labels.title}
        description={(
          <span className="grid gap-0.5">
            <span>{deliveryCopy.labels.description}</span>
            {targetTitle ? <span>{targetTitle}</span> : null}
          </span>
        )}
        closeLabel={deliveryCopy.cancel}
        pending={locked}
        className="max-w-xl"
      >
        {opened ? (
          <div className="grid gap-4">
            {canCreate ? <form
              className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"
              onSubmit={(event) => {
                event.preventDefault()
                if (!writesDisabled) control.onCreate()
              }}
            >
              <WriteFormField id="pm-delivery-label-name" label={deliveryCopy.labels.name} error={state.action.status === 'error' && state.action.action === 'create' ? message : undefined}>
                <Input
                  id="pm-delivery-label-name"
                  autoFocus
                  value={state.draftName}
                  disabled={writesDisabled}
                  placeholder={deliveryCopy.labels.namePlaceholder}
                  aria-invalid={state.action.status === 'error' && state.action.action === 'create' || undefined}
                  onChange={(event) => control.onNameChange(event.target.value)}
                />
              </WriteFormField>
              <Button type="submit" disabled={writesDisabled}>
                {deliveryCopy.labels.create}
              </Button>
            </form> : null}

            {control.catalog.status === 'loading' ? (
              <LoadingSkeleton rows={3} label={deliveryCopy.labels.loading} />
            ) : null}
            {control.catalog.status === 'error' ? (
              <div role="alert" className="flex flex-wrap items-center gap-2 rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
                <span>{deliveryCopy.labels.loadFailed}</span>
                <Button type="button" size="xs" variant="outline" onClick={control.catalog.retry}>
                  {copy.projectManagement.common.retry}
                </Button>
              </div>
            ) : null}
            {(control.catalog.status === 'ready' || control.catalog.status === 'empty') && control.catalog.staleError ? (
              <div role="alert" className="flex flex-wrap items-center gap-2 rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
                <span>{deliveryCopy.labels.loadFailed}</span>
                <Button type="button" size="xs" variant="outline" onClick={control.catalog.staleError.retry}>
                  {copy.projectManagement.common.retry}
                </Button>
              </div>
            ) : null}
            {control.catalog.status === 'empty' && control.history.length === 0 ? (
              <EmptyState
                title={deliveryCopy.labels.empty}
                className="border border-dashed border-border"
              />
            ) : null}
            {control.catalog.status === 'ready' || control.history.length > 0 ? (
              <LabelRows
                items={items}
                history={control.history}
                names={names}
                locked={writesDisabled}
                onNameChange={(identity, name) => setNames((current) => ({
                  ...current,
                  [identity]: name,
                }))}
                onRename={control.onRename}
                onRequestRemove={setRemoveTarget}
                allowDraftLabels={control.allowDraftLabels === true}
                canRename={canRename}
                canRemove={canRemove}
              />
            ) : null}
            {message && state.action.status === 'error' && state.action.action !== 'create' && !removeTarget ? (
              <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
                {message}
              </p>
            ) : null}
          </div>
        ) : null}
      </ResponsivePanel>
      <ConfirmDialog
        open={Boolean(removeTarget)}
        onOpenChange={(open) => { if (!open && !locked) setRemoveTarget(null) }}
        onConfirm={() => { if (removeTarget) control.onRemove(removeTarget) }}
        title={deliveryCopy.labels.remove}
        description={deliveryCopy.labels.confirmRemove(removeTarget?.name || '')}
        cancelLabel={deliveryCopy.cancel}
        confirmLabel={deliveryCopy.labels.remove}
        pending={state.status === 'open' && state.action.status === 'deleting'}
        error={removeError}
      />
    </>
  )
}
