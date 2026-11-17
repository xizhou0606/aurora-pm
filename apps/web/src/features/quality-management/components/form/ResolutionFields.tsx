import { QUALITY_COPY } from '@/shared/copy'
import {
  LazyRichTextField,
  UserSelect,
  type RichTextPastedImage,
} from '@/shared/ui'

import type { QualityBugDraft, QualityMutationState } from '../../model/types'
import { QUALITY_NOTE_MAX_CHARACTERS } from '../../model/payloads'
import { QualitySelectField } from '../controls'
import { Field } from './Field'
import type { FormValidationField } from './validation'

interface UserOption {
  id: number
  name: string
  secondary?: string
  avatarUrl?: string
}

export function ResolutionFields({
  draft,
  userOptions,
  usersUnavailable,
  mutationPending,
  uploadPending,
  validation,
  resolutionNoteTooLong,
  onUpdate,
  onClearValidation,
  onPasteFiles,
}: {
  draft: QualityBugDraft
  userOptions: readonly UserOption[]
  usersUnavailable: boolean
  mutationPending: boolean
  uploadPending: boolean
  validation: FormValidationField
  resolutionNoteTooLong: boolean
  onUpdate: <TKey extends keyof QualityBugDraft>(key: TKey, value: QualityBugDraft[TKey]) => void
  onClearValidation: () => void
  onPasteFiles: (files: readonly File[]) => Promise<readonly RichTextPastedImage[]>
}) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <QualitySelectField
        id="quality-form-resolution-reason"
        label={QUALITY_COPY.lifecycle.resolutionReason}
        value={draft.resolutionReason}
        options={Object.entries(QUALITY_COPY.enums.resolution).map(([value, label]) => ({ value, label }))}
        emptyLabel={QUALITY_COPY.none}
        error={validation === 'resolution' ? QUALITY_COPY.form.resolutionReasonRequired : undefined}
        onValueChange={(value) => onUpdate('resolutionReason', value)}
      />
      <Field id="quality-form-reviewer" label={QUALITY_COPY.lifecycle.reviewer}>
        <UserSelect
          id="quality-form-reviewer"
          label={QUALITY_COPY.lifecycle.reviewer}
          value={draft.reviewerUserId}
          options={userOptions}
          emptyLabel={QUALITY_COPY.none}
          disabled={usersUnavailable}
          onValueChange={(value) => onUpdate('reviewerUserId', value)}
        />
      </Field>
      <div className="sm:col-span-2">
        <LazyRichTextField
          label={QUALITY_COPY.lifecycle.resolutionNote}
          value={draft.resolutionNote}
          serialization="json"
          documentId="quality-form-resolution-note"
          scope="quality-resolution-note"
          minHeight={140}
          disabled={mutationPending}
          pending={uploadPending}
          error={validation === 'resolutionNote' || resolutionNoteTooLong
            ? QUALITY_COPY.form.noteTooLong(
                QUALITY_COPY.lifecycle.resolutionNote,
                QUALITY_NOTE_MAX_CHARACTERS,
              )
            : undefined}
          onPasteFiles={onPasteFiles}
          onChange={(value) => {
            onClearValidation()
            onUpdate('resolutionNote', value)
          }}
        />
      </div>
    </div>
  )
}
