import { useEffect, useState } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { Button, ResponsivePanel, richTextVisibleCharacterCount } from '@/shared/ui'
import type {
  QualityMaterial,
  QualityMutationState,
  QualityReturnPayload,
} from '../../model/types'
import { MaterialEditor, RichTextNote } from './Fields'
import { QUALITY_NOTE_MAX_CHARACTERS } from '../../model/payloads'

export function ReturnDialog({
  open,
  requirementId,
  canUpload,
  state,
  uploadState,
  onOpenChange,
  onUpload,
  onSubmit,
}: {
  open: boolean
  requirementId?: number
  canUpload: boolean
  state: QualityMutationState
  uploadState: QualityMutationState
  onOpenChange: (open: boolean) => void
  onUpload: (requirementId: number, file: File) => Promise<QualityMaterial>
  onSubmit: (payload: QualityReturnPayload) => Promise<void>
}) {
  const [note, setNote] = useState('')
  const [materials, setMaterials] = useState<QualityMaterial[]>([])
  useEffect(() => {
    if (!open) {
      setNote('')
      setMaterials([])
    }
  }, [open])
  const pending = state.status === 'pending'
  const uploading = uploadState.status === 'pending'
  const busy = pending || uploading
  const noteTooLong = richTextVisibleCharacterCount(note) > QUALITY_NOTE_MAX_CHARACTERS
  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      pending={busy}
      title={QUALITY_COPY.lifecycle.returnTitle}
      description={QUALITY_COPY.lifecycle.returnDescription}
      footer={(
        <Button
          type="button"
          disabled={busy || noteTooLong}
          onClick={() => void onSubmit({
            status: 'reopened',
            regression_note: note.trim(),
            regression_materials: materials,
          }).catch(() => undefined)}
        >
          {busy ? QUALITY_COPY.mutation.pending : QUALITY_COPY.lifecycle.return}
        </Button>
      )}
    >
      <div className="grid gap-4">
        <RichTextNote
          id="quality-return-note"
          label={QUALITY_COPY.lifecycle.returnReason}
          value={note}
          items={materials}
          requirementId={requirementId}
          canUpload={canUpload}
          disabled={pending}
          uploadPending={uploading}
          error={noteTooLong
            ? QUALITY_COPY.form.noteTooLong(
                QUALITY_COPY.lifecycle.returnReason,
                QUALITY_NOTE_MAX_CHARACTERS,
              )
            : undefined}
          onChange={setNote}
          onItemsChange={setMaterials}
          onUpload={onUpload}
        />
        <MaterialEditor
          items={materials}
          requirementId={requirementId}
          canUpload={canUpload}
          state={uploadState}
          onChange={setMaterials}
          onUpload={onUpload}
        />
      </div>
    </ResponsivePanel>
  )
}
