import { pmKeys } from '@/shared/query'
import type { ReviewListInput } from '../model'

export const designReviewKeys = {
  root: (workspaceId: number) => pmKeys.resource(workspaceId, 'designReviews'),
  capabilities: (workspaceId: number) => pmKeys.resource(workspaceId, 'designReviews', 'capabilities'),
  requirements: (workspaceId: number) => pmKeys.resource(workspaceId, 'designReviews', 'requirements'),
  list: (workspaceId: number, input: ReviewListInput) => pmKeys.resource(workspaceId, 'designReviews', 'list', input),
  detail: (workspaceId: number, id: string) => pmKeys.resource(workspaceId, 'designReviews', 'detail', id),
  asset: (workspaceId: number, id: string, assetId: string) => pmKeys.resource(workspaceId, 'designReviews', 'asset', id, assetId),
  collectors: (workspaceId: number) => pmKeys.resource(workspaceId, 'designReviews', 'collectors'),
  captures: (workspaceId: number) => pmKeys.resource(workspaceId, 'designReviews', 'captures'),
}
