export function documentPath(documentId?: number): string {
  return documentId && documentId > 0
    ? `/workspace/documents/${documentId}`
    : '/workspace/documents'
}

export function documentCommentPath(documentId: number, threadId: number): string {
  return `${documentPath(documentId)}#comment-${threadId}`
}

export function parseDocumentCommentHash(hash: unknown): number | undefined {
  if (typeof hash !== 'string') return undefined
  const match = hash.match(/^#?comment-([1-9]\d*)$/)
  if (!match) return undefined
  const threadId = Number(match[1])
  return Number.isSafeInteger(threadId) ? threadId : undefined
}
