import { useState } from 'react'

import {
  DetailSection,
  LazyRichTextField,
  type RichTextPastedImage,
} from '@/shared/ui'
import { WORK_ITEMS_COPY, copy } from '@/shared/copy'

import type { WorkItem } from '../../model/types'
import { isWorkItemAttachmentFile, isWorkItemInlineImage } from '../../model/attachments'

export function DetailDescription({
  item,
  value,
  editable,
  saveState,
  canUploadBinary,
  onChange,
  onUploadImage,
  onUploadAttachment,
}: {
  item: WorkItem
  value: string
  editable: boolean
  saveState: 'idle' | 'pending' | 'error'
  canUploadBinary: boolean
  onChange: (value: string) => void
  onUploadImage: (file: File) => Promise<string | null>
  onUploadAttachment: (file: File) => Promise<boolean>
}) {
  const [pasteError, setPasteError] = useState<string>()
  const pending = saveState === 'pending'
  const uploadPastedFiles = async (
    files: readonly File[],
  ): Promise<readonly RichTextPastedImage[]> => {
    setPasteError(undefined)
    const images: RichTextPastedImage[] = []
    let failed = false
    for (const file of files) {
      try {
        if (isWorkItemInlineImage(file)) {
          const src = await onUploadImage(file)
          if (src) images.push({ src, alt: file.name })
          else failed = true
        } else if (!isWorkItemAttachmentFile(file) || !await onUploadAttachment(file)) {
          failed = true
        }
      } catch {
        failed = true
      }
    }
    if (failed) setPasteError(copy.ui.richText.pasteFailed)
    return images
  }
  return (
    <DetailSection id="work-items-detail-description" title={WORK_ITEMS_COPY.detail.descriptionLabel}>
      <LazyRichTextField
        label={WORK_ITEMS_COPY.detail.descriptionLabel}
        value={value}
        documentId={item.id}
        scope={`${item.teamId || 0}:${item.projectId || 0}`}
        serialization="json"
        visuallyHiddenLabel
        readOnly={!editable}
        pending={pending}
        minHeight={220}
        placeholder={WORK_ITEMS_COPY.detail.noDescription}
        error={pasteError}
        onPasteFiles={editable && canUploadBinary ? uploadPastedFiles : undefined}
        onChange={editable ? onChange : undefined}
      />
    </DetailSection>
  )
}
