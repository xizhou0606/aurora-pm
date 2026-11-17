function pathId(value: string): string {
  const id = value.trim()
  if (!id || id.length > 200 || /[/?#\\]/.test(id)) throw new Error('Invalid resource access path ID')
  return encodeURIComponent(id)
}

const resourceRoot = (resourceId: string) => `/document-resources/${pathId(resourceId)}`

export const resourceAccessEndpoints = {
  bitableRequestContext: (resourceRef: string) => `/bitables/${pathId(resourceRef)}/access-request-context`,
  documentRequestContext: (documentId: number) => `/documents/${pathId(String(documentId))}/access-request-context`,
  requests: (resourceId: string) => `${resourceRoot(resourceId)}/access-requests`,
  request: (resourceId: string, id: number) => `${resourceRoot(resourceId)}/access-requests/${pathId(String(id))}`,
  policy: (resourceId: string) => `${resourceRoot(resourceId)}/access-policy`,
  subjects: '/document-resource-subjects',
  links: (resourceId: string) => `${resourceRoot(resourceId)}/share-links`,
  link: (resourceId: string, linkId: string) => (
    `${resourceRoot(resourceId)}/share-links/${pathId(linkId)}`
  ),
  rotateLink: (resourceId: string, linkId: string) => (
    `${resourceRoot(resourceId)}/share-links/${pathId(linkId)}/rotate`
  ),
  exchangeLink: '/document-share-links/exchange',
} as const
