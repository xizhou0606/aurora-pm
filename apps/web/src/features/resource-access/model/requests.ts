import type { ResourceAccessLevel, ShareLinkExpiry } from './types'
export type AccessRequestResourceType = 'rich_text' | 'bitable'
export type RequestGrantTerms = { grantLevel: ResourceAccessLevel; grantDuration: ShareLinkExpiry }
export type AccessRequestStatus = 'pending' | 'approved' | 'rejected' | 'revoked' | 'expired'
export type AccessRequest = {
  id: number
  applicantUserId: number
  applicantName: string
  reason: string
  status: AccessRequestStatus
  read: boolean
  createdAt: string
  expiresAt: string
  grantLevel: ResourceAccessLevel
  grantExpiresAt: string | null
}
export type AccessRequestContext = {
  resourceId: string
  resourceType: AccessRequestResourceType
  ownerUserId: number
  ownerName: string
  canRequest: boolean
  canReview: boolean
  hasAccess: boolean
  hasMore: boolean
  pendingCount: number
  items: readonly AccessRequest[]
}
export type AccessRequestUpdate = { id: number; decision?: 'approved' | 'rejected' | 'revoked'; read?: boolean; grantLevel?: ResourceAccessLevel; grantDuration?: ShareLinkExpiry }
