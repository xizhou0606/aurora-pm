import { copy } from '@/shared/copy'
import { Button, DateTimePicker, ResponsivePanel, UserDirectorySelect } from '@/shared/ui'
import type {
  DeliveryQuickEditDraft,
  DeliveryQuickEditState,
  OwnerDirectoryState,
} from '../../model'
import { draftDateValue } from '../../model/form-utils'
import { WriteFormField } from '../WriteFormField'

const deliveryCopy = copy.projectManagement.delivery

export type QuickEditControl = {
  owners: OwnerDirectoryState
  onChange: (draft: DeliveryQuickEditDraft) => void
  onCancel: () => void
  onSubmit: () => void
}

export function QuickEdit({
  state,
  target,
  control,
}: {
  state: DeliveryQuickEditState
  target?: { title: string; label: string } | null
  control: QuickEditControl
}) {
  const editing = state.status === 'editing' ? state : undefined
  const locked = editing?.submit.status === 'pending'
  const submitError = editing?.submit.status === 'error'
    ? editing.submit.errorCode === 'RESOURCE_CHANGED'
      ? deliveryCopy.feedback.contextChanged
      : deliveryCopy.feedback.saveFailed
    : undefined
  return (
    <ResponsivePanel
      open={Boolean(editing)}
      onOpenChange={(open) => { if (!open && !locked) control.onCancel() }}
      title={target?.label || deliveryCopy.quickEdit}
      description={target?.title || deliveryCopy.description}
      closeLabel={deliveryCopy.cancel}
      pending={locked}
      className="max-w-lg"
      footer={editing ? (
        <Button
          type="submit"
          form="pm-delivery-quick-edit-form"
          disabled={locked || !editing.dirty}
        >
          {locked ? deliveryCopy.saving : deliveryCopy.save}
        </Button>
      ) : undefined}
    >
      {editing ? (
        <form
          id="pm-delivery-quick-edit-form"
          className="grid gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (!locked && editing.dirty) control.onSubmit()
          }}
        >
          <div className="grid gap-3 sm:grid-cols-2">
            <WriteFormField id="pm-delivery-quick-due" label={deliveryCopy.dueAt}>
              <DateTimePicker
                id="pm-delivery-quick-due"
                value={draftDateValue(editing.draft.dueAt, true)}
                disabled={locked}
                onChange={(value) => control.onChange({
                  ...editing.draft,
                  dueAt: value || null,
                })}
              />
            </WriteFormField>
            <WriteFormField id="pm-delivery-quick-actual" label={deliveryCopy.actualCompletedAt}>
              <DateTimePicker
                id="pm-delivery-quick-actual"
                value={draftDateValue(editing.draft.actualCompletedAt, true)}
                defaultMonth={draftDateValue(editing.draft.dueAt, true)}
                disabled={locked}
                onChange={(value) => control.onChange({
                  ...editing.draft,
                  actualCompletedAt: value || null,
                })}
              />
            </WriteFormField>
          </div>
          <UserDirectorySelect
            id="pm-delivery-quick-owner"
            label={deliveryCopy.owner}
            value={editing.draft.ownerUserId}
            state={control.owners}
            disabled={locked}
            onValueChange={(ownerUserId) => control.onChange({
              ...editing.draft,
              ownerUserId: ownerUserId ?? null,
            })}
          />
          {submitError ? (
            <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/8 px-3 py-2 text-sm text-destructive">
              {submitError}
            </p>
          ) : null}
        </form>
      ) : null}
    </ResponsivePanel>
  )
}
