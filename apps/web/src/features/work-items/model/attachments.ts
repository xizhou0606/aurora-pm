export const WORK_ITEM_ATTACHMENT_MAX_BYTES = 32 * 1024 * 1024
export const WORK_ITEM_INLINE_IMAGE_MAX_BYTES = 8 * 1024 * 1024

export function isWorkItemAttachmentFile(file: File): boolean {
  return file.size > 0 && file.size <= WORK_ITEM_ATTACHMENT_MAX_BYTES
}

export function isWorkItemInlineImage(file: File): boolean {
  return file.type.startsWith('image/')
    && file.size > 0
    && file.size <= WORK_ITEM_INLINE_IMAGE_MAX_BYTES
}

export function appendPendingAttachments(
  current: readonly File[],
  incoming: readonly File[],
): File[] {
  const next = [...current]
  for (const file of incoming) {
    if (!isWorkItemAttachmentFile(file)) continue
    if (next.some((entry) => (
      entry.name === file.name
      && entry.size === file.size
      && entry.lastModified === file.lastModified
    ))) continue
    next.push(file)
  }
  return next
}
