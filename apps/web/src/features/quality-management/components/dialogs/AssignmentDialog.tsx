import { useEffect, useState } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import {
  Button,
  Label,
  MultiUserSelect,
  ResponsivePanel,
  richTextVisibleCharacterCount,
} from '@/shared/ui'
import type {
  QualityAssignPayload,
  QualityMaterial,
  QualityMutationState,
  QualityUsersState,
} from '../../model/types'
import { UserDirectoryStatus } from '../list/UserDirectoryStatus'
import { MaterialEditor, RichTextNote, UserSelect } from './Fields'
import { QUALITY_NOTE_MAX_CHARACTERS } from '../../model/payloads'

export function AssignmentDialog({
  open,
  users,
  initialAssigneeId,
  selectionCount = 1,
  requirementId,
  canUpload,
  state,
  uploadState,
  onOpenChange,
  onUpload,
  onSubmit,
}: {
  open: boolean
  users: QualityUsersState
  initialAssigneeId?: number
  selectionCount?: number
  requirementId?: number
  canUpload: boolean
  state: QualityMutationState
  uploadState: QualityMutationState
  onOpenChange: (open: boolean) => void
  onUpload: (requirementId: number, file: File) => Promise<QualityMaterial>
  onSubmit: (payload: QualityAssignPayload) => Promise<void>
}) {
  const [assigneeId, setAssigneeId] = useState<number>()
  const [ccUserIds, setCCUserIds] = useState<number[]>([])
  const [note, setNote] = useState('')
  const [materials, setMaterials] = useState<QualityMaterial[]>([])
  useEffect(() => {
    if (open) setAssigneeId(initialAssigneeId)
    else {
      setAssigneeId(undefined)
      setCCUserIds([])
      setNote('')
      setMaterials([])
    }
  }, [initialAssigneeId, open])
  const pending = state.status === 'pending'
  const noteTooLong = richTextVisibleCharacterCount(note) > QUALITY_NOTE_MAX_CHARACTERS
  const changed = Boolean(
    assigneeId && (
      assigneeId !== initialAssigneeId ||
      ccUserIds.length > 0 ||
      note.trim() ||
      materials.length > 0
    ),
  )
  return (
    <ResponsivePanel
      open={open}
      onOpenChange={onOpenChange}
      pending={pending}
      title={selectionCount > 1
        ? QUALITY_COPY.bulk.assignmentTitle(selectionCount)
        : QUALITY_COPY.lifecycle.assignmentTitle}
      description={selectionCount > 1
        ? QUALITY_COPY.bulk.assignmentDescription
        : QUALITY_COPY.lifecycle.assignmentDescription}
      footer={(
        <Button
          type="button"
          disabled={pending || !changed || noteTooLong}
          onClick={() => {
            if (!assigneeId) return
            const trimmedNote = note.trim()
            void onSubmit({
              assignee_user_id: assigneeId,
              defect_owner_user_id: assigneeId,
              cc_user_ids: ccUserIds,
              copy_user_ids: ccUserIds,
              assignment_note: trimmedNote,
              assignment_materials: materials,
              remark: trimmedNote,
              note: trimmedNote,
            }).catch(() => undefined)
          }}
        >
          {pending ? QUALITY_COPY.mutation.pending : QUALITY_COPY.lifecycle.assign}
        </Button>
      )}
    >
      <div className="grid gap-4">
        <UserDirectoryStatus state={users} />
        <UserSelect
          id="quality-assignment-assignee"
          label={QUALITY_COPY.lifecycle.assignee}
          value={assigneeId}
          users={users}
          onChange={setAssigneeId}
        />
        <div className="grid min-w-0 gap-1.5">
          <Label htmlFor="quality-assignment-cc">{QUALITY_COPY.lifecycle.cc}</Label>
          <MultiUserSelect
            id="quality-assignment-cc"
            label={QUALITY_COPY.lifecycle.cc}
            value={users.rows.filter((user) => ccUserIds.includes(user.id))}
            options={users.rows}
            placeholder={QUALITY_COPY.none}
            disabled={users.status !== 'ready' && users.rows.length === 0}
            onValueChange={(next) => setCCUserIds(next.map((user) => user.id))}
          />
        </div>
        <RichTextNote
          id="quality-assignment-note"
          label={QUALITY_COPY.lifecycle.assignmentNote}
          value={note}
          items={materials}
          requirementId={requirementId}
          canUpload={canUpload}
          disabled={pending}
          uploadPending={uploadState.status === 'pending'}
          error={noteTooLong
            ? QUALITY_COPY.form.noteTooLong(
                QUALITY_COPY.lifecycle.assignmentNote,
                QUALITY_NOTE_MAX_CHARACTERS,
              )
            : undefined}
          onChange={setNote}
          onItemsChange={setMaterials}
          onUpload={onUpload}
        />
        <MaterialEditor
          items={materials}
          label={QUALITY_COPY.lifecycle.assignmentMaterials}
          hint={QUALITY_COPY.lifecycle.assignmentMaterialsHint}
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
