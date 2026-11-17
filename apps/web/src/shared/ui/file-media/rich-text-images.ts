export const RICH_TEXT_IMAGE_MAX_BYTES = 8 * 1024 * 1024

const RICH_TEXT_IMAGE_TYPES = new Set([
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
])

export function isRichTextImageFile(file: File): boolean {
  return RICH_TEXT_IMAGE_TYPES.has(file.type) && file.size > 0 && file.size <= RICH_TEXT_IMAGE_MAX_BYTES
}

export function clipboardImageFiles(data: DataTransfer | null | undefined, accepts: (file: File) => boolean = isRichTextImageFile): File[] {
  if (!data) return []
  return Array.from(data.files).filter(accepts)
}

export function clipboardFilesWithImage(data: DataTransfer | null | undefined): File[] {
  if (!data) return []
  const files = Array.from(data.files)
  return files.some(isRichTextImageFile) ? files : []
}
