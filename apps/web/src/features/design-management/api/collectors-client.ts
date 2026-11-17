import { platformApi, toSafeApiError } from '@/shared/api'
import type { CollectorClient } from './contracts'
import { designReviewRequest } from './request'

export const collectorClient = Object.freeze({
  list: (signal) => designReviewRequest('GET', '/design-review-mobile-collectors', undefined, signal),
  captures: (signal) => designReviewRequest('GET', '/design-review-mobile-captures', undefined, signal),
  createPairingCode: () => designReviewRequest('POST', '/design-review-mobile-collectors/pairing-codes'),
  createCapture: (input) => designReviewRequest('POST', '/design-review-mobile-captures', input),
  captureAsset: async (id) => {
    try {
      const response = await platformApi.get(`/design-review-mobile-captures/${id}/asset`, { responseType: 'blob' })
      return response.data as Blob
    } catch (error) {
      throw toSafeApiError(error, 'PM_DESIGN_REVIEW_MOBILE_ASSET_FAILED')
    }
  },
  consumeCapture: (id) => designReviewRequest('POST', `/design-review-mobile-captures/${id}/consume`),
  revoke: async (id) => { await designReviewRequest('DELETE', `/design-review-mobile-collectors/${id}`) },
  removeCapture: async (id) => { await designReviewRequest('DELETE', `/design-review-mobile-captures/${id}`) },
} satisfies CollectorClient)
