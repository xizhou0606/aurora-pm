import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { ResourceAccessPanel } from '@/features/resource-access'
import { Button, ResponsivePanel, UserDirectorySelect } from '@/shared/ui'
import type { DocumentTreeControl, DocumentTreeViewState } from '../../hooks'
import { DocumentTreeDestinationField } from './DocumentTreeDestinationField'

function Footer({
  pending,
  cancel,
  submit,
  cancelLabel,
  submitLabel,
}: {
  pending: boolean
  cancel: () => void
  submit: () => void
  cancelLabel: string
  submitLabel: string
}) {
  return (
    <>
      <Button type="button" variant="outline" disabled={pending} onClick={cancel}>{cancelLabel}</Button>
      <Button type="button" disabled={pending} onClick={submit}>{submitLabel}</Button>
    </>
  )
}

export function DocumentTreeContextDialogs({ state, control }: {
  state: Extract<DocumentTreeViewState, { status: 'ready' }>['context']
  control: DocumentTreeControl['context']
}) {
  const { dialog } = state
  const destination = dialog.kind === 'destination' ? dialog : undefined
  const destinationCopy = destination?.action === 'copy' ? DOCUMENTS_COPY.copyNode : DOCUMENTS_COPY.shortcut
  const owner = dialog.kind === 'owner' ? dialog : undefined
  const share = dialog.kind === 'share' ? dialog : undefined

  return (
    <>
      <ResponsivePanel
        open={Boolean(destination)}
        onOpenChange={(open) => { if (!open) control.close() }}
        title={destinationCopy.title}
        description={destinationCopy.description}
        pending={destination?.submit === 'pending'}
        className="max-w-md"
        footer={destination ? (
          <Footer
            pending={destination.submit === 'pending'}
            cancel={control.close}
            submit={() => { void control.submit() }}
            cancelLabel={destinationCopy.cancel}
            submitLabel={destinationCopy.submit}
          />
        ) : undefined}
      >
        {destination ? (
          <>
            <DocumentTreeDestinationField
              id="document-context-destination"
              label={destinationCopy.fieldLabel}
              parentId={destination.parentId}
              folders={destination.folders}
              onChange={control.updateDestination}
            />
            {destination.submit === 'error' ? (
              <p role="alert" className="m-0 text-xs text-destructive">{destinationCopy.failed}</p>
            ) : null}
          </>
        ) : null}
      </ResponsivePanel>

      <ResourceAccessPanel
        purpose={share?.purpose ?? 'share'}
        open={Boolean(share)}
        onOpenChange={(open) => { if (!open) control.close() }}
        onCloseRequest={state.access.onCloseRequest}
        view={state.access.view}
      />

      <ResponsivePanel
        open={Boolean(owner)}
        onOpenChange={(open) => { if (!open) control.close() }}
        title={DOCUMENTS_COPY.owner.title}
        description={DOCUMENTS_COPY.owner.description}
        pending={owner?.submit === 'pending'}
        className="max-w-md"
        footer={owner ? (
          <Footer
            pending={owner.submit === 'pending'}
            cancel={control.close}
            submit={() => { void control.submit() }}
            cancelLabel={DOCUMENTS_COPY.owner.cancel}
            submitLabel={DOCUMENTS_COPY.owner.submit}
          />
        ) : undefined}
      >
        {owner ? (
          <UserDirectorySelect
            id="document-owner"
            label={DOCUMENTS_COPY.owner.fieldLabel}
            value={owner.ownerUserId}
            state={state.owners}
            disabled={owner.submit === 'pending'}
            error={owner.required
              ? DOCUMENTS_COPY.owner.required
              : owner.submit === 'error' ? DOCUMENTS_COPY.owner.failed : undefined}
            onValueChange={control.updateOwner}
          />
        ) : null}
      </ResponsivePanel>
    </>
  )
}
