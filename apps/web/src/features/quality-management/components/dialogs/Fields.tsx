import { OPERATION_FEEDBACK_COPY, QUALITY_COPY } from '@/shared/copy'
import {
  FormField,
  LazyFileUploader,
  LazyRichTextField,
  notifyOperationError,
  operationFeedback,
  uploadPreviewKind,
  UserSelect as SharedUserSelect,
  type RichTextPastedImage,
} from '@/shared/ui'
import type {
  QualityMaterial,
  QualityMutationState,
  QualityUsersState,
} from '../../model/types'
import { QUALITY_MATERIAL_MAX_BYTES } from '../../model/payloads'
import { isQualityInlineImage, isQualityPasteMaterialFile } from '../../model'

export function UserSelect({
  id,
  label,
  value,
  users,
  onChange,
}: {
  id: string
  label: string
  value?: number
  users: QualityUsersState
  onChange: (value?: number) => void
}) {
  return (
    <FormField id={id} label={label}>
      <SharedUserSelect
        id={id}
        label={label}
        value={value}
        options={users.rows.map((user) => ({
          id: user.id,
          name: user.name,
          secondary: user.username,
          avatarUrl: user.avatarUrl,
        }))}
        emptyLabel={QUALITY_COPY.none}
        disabled={users.status !== 'ready' && users.rows.length === 0}
        onValueChange={onChange}
      />
    </FormField>
  )
}

export function MaterialEditor({
  items,
  label = QUALITY_COPY.form.materials,
  hint = QUALITY_COPY.upload.hint,
  requirementId,
  canUpload,
  state,
  onChange,
  onUpload,
}: {
  items: QualityMaterial[]
  label?: string
  hint?: string
  requirementId?: number
  canUpload: boolean
  state: QualityMutationState
  onChange: (items: QualityMaterial[]) => void
  onUpload: (requirementId: number, file: File) => Promise<QualityMaterial>
}) {
  return (
    <LazyFileUploader
      label={label}
      hint={requirementId ? hint : QUALITY_COPY.upload.missingRequirement}
      maxBytes={QUALITY_MATERIAL_MAX_BYTES}
      disabled={!canUpload || !requirementId}
      pending={state.status === 'pending'}
      items={items.map((item) => ({
        ...item,
        label: uploadPreviewKind(item) === 'image'
            ? QUALITY_COPY.upload.imageMaterial
          : uploadPreviewKind(item) === 'video'
            ? QUALITY_COPY.upload.videoMaterial
            : label,
      }))}
      onRemove={(item) => onChange(items.filter((entry) => entry.url !== item.url))}
      onUpload={async (file) => {
        if (!requirementId) return
        try {
          onChange([...items, await onUpload(requirementId, file)])
        } catch {
          // The upload mutation owns the Toast and this boundary prevents an
          // unhandled promise from the hidden file input.
        }
      }}
    />
  )
}

export function RichTextNote({
  id,
  label,
  value,
  items,
  requirementId,
  canUpload,
  disabled,
  uploadPending,
  error,
  onChange,
  onItemsChange,
  onUpload,
}: {
  id: string
  label: string
  value: string
  items: QualityMaterial[]
  requirementId?: number
  canUpload: boolean
  disabled: boolean
  uploadPending: boolean
  error?: string
  onChange: (value: string) => void
  onItemsChange: (items: QualityMaterial[]) => void
  onUpload: (requirementId: number, file: File) => Promise<QualityMaterial>
}) {
  const uploadPastedFiles = async (
    incomingFiles: readonly File[],
  ): Promise<readonly RichTextPastedImage[]> => {
    const files = incomingFiles.filter(isQualityPasteMaterialFile)
    if (!files.length) return []
    if (!canUpload || disabled || uploadPending) {
      notifyOperationError(
        operationFeedback(OPERATION_FEEDBACK_COPY.quality.upload),
        new Error('PM_QUALITY_PASTE_UPLOAD_UNAVAILABLE'),
      )
      return []
    }
    const images: RichTextPastedImage[] = []
    const attachments: QualityMaterial[] = []
    for (const file of files) {
      try {
        const material = await onUpload(requirementId || 0, file)
        if (isQualityInlineImage(file)) images.push({ src: material.url, alt: file.name })
        else attachments.push(material)
      } catch {
        // The upload mutation owns the failure Toast and remaining files can continue.
      }
    }
    if (attachments.length) onItemsChange([...items, ...attachments])
    return images
  }

  return (
    <LazyRichTextField
      label={label}
      value={value}
      serialization="json"
      documentId={id}
      scope="quality-lifecycle-note"
      minHeight={120}
      editorHeight={320}
      disabled={disabled}
      pending={uploadPending}
      error={error}
      onChange={onChange}
      onPasteFiles={uploadPastedFiles}
    />
  )
}
