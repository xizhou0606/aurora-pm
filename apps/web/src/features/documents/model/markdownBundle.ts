// Shared import/export envelope. The asset limit follows the document upload contract.
export const DOCUMENT_MARKDOWN_LIMITS = {
  markdownBytes: 4 * 1024 * 1024,
  archiveBytes: 64 * 1024 * 1024,
  entries: 512,
  uncompressedBytes: 128 * 1024 * 1024,
  imageBytes: 32 * 1024 * 1024,
} as const

export const DOCUMENT_IMAGE_MIME_BY_EXTENSION: Readonly<Record<string, string>> = {
  '.gif': 'image/gif', '.jpeg': 'image/jpeg', '.jpg': 'image/jpeg',
  '.png': 'image/png', '.webp': 'image/webp',
}

export function matchesDocumentImageMime(bytes: Uint8Array, mime: string): boolean {
  if (mime === 'image/png') {
    return bytes.length >= 8 && [137, 80, 78, 71, 13, 10, 26, 10].every((value, index) => bytes[index] === value)
  }
  if (mime === 'image/jpeg') return bytes.length >= 3 && bytes[0] === 255 && bytes[1] === 216 && bytes[2] === 255
  if (mime === 'image/gif') {
    const signature = new TextDecoder('ascii').decode(bytes.slice(0, 6))
    return signature === 'GIF87a' || signature === 'GIF89a'
  }
  if (mime === 'image/webp') {
    return bytes.length >= 12
      && new TextDecoder('ascii').decode(bytes.slice(0, 4)) === 'RIFF'
      && new TextDecoder('ascii').decode(bytes.slice(8, 12)) === 'WEBP'
  }
  return false
}
