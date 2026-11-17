import { useEffect, useState } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { Button, ResponsivePanel, richTextVisibleCharacterCount } from '@/shared/ui'
import type {
  QualityMaterial,
  QualityMutationState,
  QualityResolvePayload,
  QualityUsersState,
} from '../../model/types'
import { UserDirectoryStatus } from '../list/UserDirectoryStatus'
import { QualitySelectField } from '../controls'
import { MaterialEditor, RichTextNote, UserSelect } from './Fields'
import { QUALITY_NOTE_MAX_CHARACTERS } from '../../model/payloads'

export function ResolveDialog({
  open,
  targetStatus = 'review',
  users,
  initialReviewerId,
  requirementId,
  canUpload,
  state,
  uploadState,
  onOpenChange,
  onUpload,
  onSubmit,
}: {
  open: boolean
  targetStatus?: QualityResolvePayload['status']
  users: QualityUsersState
  initialReviewerId?: number
  requirementId?: number
  canUpload: boolean
  state: QualityMutationState
  uploadState: QualityMutationState
  onOpenChange: (open: boolean) => void
  onUpload: (requirementId: number, file: File) => Promise<QualityMaterial>
  onSubmit: (payload: QualityResolvePayload) => Promise<void>
}) {
  const [reason, setReason] = useState('')
  const [reviewerId, setReviewerId] = useState<number>()
  const [note, setNote] = useState('')
  const [materials, setMaterials] = useState<QualityMaterial[]>([])
  useEffect(() => {
    if (open) setReviewerId(initialReviewerId)
    else {
      setReason('')
      setReviewerId(undefined)
      setNote('')
      setMaterials([])
    }
  }, [initialReviewerId, open])
  const pending = state.status === 'pending'
  const uploading = uploadState.status === 'pending'
  const busy = pending || uploading
  const completing = targetStatus === 'done'
  const noteTooLong = richTextVisibleCharacterCount(note) > QUALITY_NOTE_MAX_CHARACTERS
  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      pending={busy}
      title={completing
        ? QUALITY_COPY.lifecycle.completeTitle
        : QUALITY_COPY.lifecycle.resolveTitle}
      description={completing
        ? QUALITY_COPY.lifecycle.completeDescription
        : QUALITY_COPY.lifecycle.resolveDescription}
      footer={(
        <Button
          type="button"
          disabled={busy || !reason || noteTooLong}
          onClick={() => void onSubmit({
            status: targetStatus,
            resolution_reason: reason,
            resolution_note: note.trim(),
            remark: note.trim(),
            note: note.trim(),
            resolution_materials: materials,
            reviewer_user_id: reviewerId || null,
            verified_by_user_id: reviewerId || null,
          }).catch(() => undefined)}
        >
          {busy
            ? QUALITY_COPY.mutation.pending
            : completing
              ? QUALITY_COPY.lifecycle.complete
              : QUALITY_COPY.lifecycle.resolve}
        </Button>
      )}
    >
      <div className="grid gap-4">
        <UserDirectoryStatus state={users} />
        <QualitySelectField
          id="quality-resolution-reason"
          label={QUALITY_COPY.lifecycle.resolutionReason}
          value={reason}
          options={Object.entries(QUALITY_COPY.enums.resolution).map(([value, label]) => ({ value, label }))}
          emptyLabel={QUALITY_COPY.none}
          onValueChange={setReason}
        />
        <UserSelect
          id="quality-resolution-reviewer"
          label={QUALITY_COPY.lifecycle.reviewer}
          value={reviewerId}
          users={users}
          onChange={setReviewerId}
        />
        <RichTextNote
          id="quality-resolution-note"
          label={QUALITY_COPY.lifecycle.resolutionNote}
          value={note}
          items={materials}
          requirementId={requirementId}
          canUpload={canUpload}
          disabled={pending}
          uploadPending={uploading}
          error={noteTooLong
            ? QUALITY_COPY.form.noteTooLong(
                QUALITY_COPY.lifecycle.resolutionNote,
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
