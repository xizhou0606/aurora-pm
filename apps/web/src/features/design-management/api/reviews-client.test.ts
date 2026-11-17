import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'
import { reviewClient } from './reviews-client'

describe('design review lifecycle client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('keeps initial analysis and retry on their distinct permission routes', async () => {
    const request = vi.spyOn(platformApi, 'request').mockResolvedValue({ data: { data: { queued: true } } })

    await reviewClient.queueAnalysis('review-1')
    await reviewClient.retryAnalysis('review-1')

    expect(request).toHaveBeenNthCalledWith(1, {
      method: 'POST', url: '/design-reviews/review-1/analysis', data: undefined, signal: undefined,
    })
    expect(request).toHaveBeenNthCalledWith(2, {
      method: 'POST', url: '/design-reviews/review-1/analysis/retry', data: undefined, signal: undefined,
    })
  })
})
