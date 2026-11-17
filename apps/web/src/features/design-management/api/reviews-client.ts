import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import type { ReviewClient } from './contracts'
import { designReviewRequest } from './request'
import type { ReviewPage } from '../model'

export const reviewClient = Object.freeze({
  capabilities: (signal) => designReviewRequest('GET', '/design-reviews/capabilities', undefined, signal),
  requirements: (signal) => designReviewRequest('GET', '/design-reviews/requirements', undefined, signal),
  list: async (input, signal) => {
    try {
      const response = await platformApi.get('/design-reviews', { params: input, signal })
      return unwrapApiData<ReviewPage>(response) as ReviewPage
    } catch (error) {
      throw toSafeApiError(error, 'PM_DESIGN_REVIEW_REQUEST_FAILED')
    }
  },
  detail: (id, signal) => designReviewRequest('GET', `/design-reviews/${id}`, undefined, signal),
  create: (input) => designReviewRequest('POST', '/design-reviews', input),
  createRecheck: (id, input) => designReviewRequest('POST', `/design-reviews/${id}/rechecks`, input),
  figmaPreview: (figmaUrl) => designReviewRequest('POST', '/design-reviews/figma-preview', { figma_url: figmaUrl }),
  asset: async (id, assetId, signal) => {
    try {
      const response = await platformApi.get<Blob>(`/design-reviews/${id}/assets/${assetId}`, {
        responseType: 'blob', signal,
      })
      return response.data
    } catch (error) {
      throw toSafeApiError(error, 'PM_DESIGN_REVIEW_ASSET_FAILED')
    }
  },
  normalizeDOM: (figmaUrl, inventory) => designReviewRequest('POST', '/design-reviews/dom-normalize', { figma_url: figmaUrl, inventory }),
  uploadAsset: async (id, kind, file) => {
    const data = new FormData()
    data.append('kind', kind)
    data.append('file', file)
    await designReviewRequest('POST', `/design-reviews/${id}/assets`, data)
  },
  queueAnalysis: async (id) => { await designReviewRequest('POST', `/design-reviews/${id}/analysis`) },
  retryAnalysis: async (id) => { await designReviewRequest('POST', `/design-reviews/${id}/analysis/retry`) },
  abandonAnalysis: async (id) => { await designReviewRequest('POST', `/design-reviews/${id}/analysis/abandon`) },
  remove: async (id) => { await designReviewRequest('DELETE', `/design-reviews/${id}`) },
  exportFile: async (id, format) => {
    try {
      const response = await platformApi.get(`/design-reviews/${id}/export`, {
        params: { format }, responseType: 'blob',
      })
      return response.data as Blob
    } catch (error) {
      throw toSafeApiError(error, 'PM_DESIGN_REVIEW_EXPORT_FAILED')
    }
  },
} satisfies ReviewClient)
