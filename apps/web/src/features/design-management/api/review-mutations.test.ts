import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'
import type { DesignReviewDetail, NewReviewSubmission } from '../model'
import { executeReviewCreation } from './review-mutations'

const reviewID = '22222222-2222-4222-8222-222222222222'

function response(data: unknown) {
  return Promise.resolve({ data: { data } })
}

function review(status: DesignReviewDetail['status'] = 'draft') {
  return { id: reviewID, status } as DesignReviewDetail
}

function submission(): NewReviewSubmission {
  return {
    requirement_id: 101,
    figma_url: 'https://www.figma.com/design/file?node-id=1-2',
    implementation_target: { kind: 'web', original_url: 'https://example.test', actual_url: 'https://example.test' },
    viewport_width: 1440,
    viewport_height: 900,
    device_pixel_ratio: 1,
    page_zoom: 1,
    analysis_scope: { enabled: false },
    mobile_topbar: { enabled: false },
    dom_normalization: { enabled: false },
    implementationFile: new File(['implementation'], 'implementation.png', { type: 'image/png' }),
    implementationStructureFile: new File(['{}'], 'implementation.json', { type: 'application/json' }),
  }
}

describe('recoverable design review creation', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('reuses the known draft after an intermediate upload failure', async () => {
    let uploadCount = 0
    const request = vi.spyOn(platformApi, 'request').mockImplementation((config) => {
      if (config.method === 'POST' && config.url === '/design-reviews') return response(review()) as never
      if (config.method === 'GET' && config.url === `/design-reviews/${reviewID}`) return response(review()) as never
      if (config.method === 'POST' && config.url === `/design-reviews/${reviewID}/assets`) {
        uploadCount++
        if (uploadCount === 2) return Promise.reject(new Error('structure upload failed'))
        return response({ ok: true }) as never
      }
      if (config.method === 'POST' && config.url === `/design-reviews/${reviewID}/analysis`) return response({ queued: true }) as never
      return Promise.reject(new Error(`unexpected request ${config.method} ${config.url}`))
    })
    const resume = { current: undefined }
    const input = submission()

    await expect(executeReviewCreation(input, resume)).rejects.toThrow()
    await expect(executeReviewCreation(input, resume)).resolves.toMatchObject({ id: reviewID })

    expect(request.mock.calls.filter(([config]) => config.url === '/design-reviews')).toHaveLength(1)
    expect(request.mock.calls.filter(([config]) => config.url === `/design-reviews/${reviewID}`)).toHaveLength(1)
    expect(request.mock.calls.filter(([config]) => config.url === `/design-reviews/${reviewID}/analysis`)).toHaveLength(1)
    expect(resume.current).toBeUndefined()
  })

  it('does not overwrite evidence when an ambiguous queue response already advanced the review', async () => {
    let queued = false
    let uploadCount = 0
    const request = vi.spyOn(platformApi, 'request').mockImplementation((config) => {
      if (config.method === 'POST' && config.url === '/design-reviews') return response(review()) as never
      if (config.method === 'GET' && config.url === `/design-reviews/${reviewID}`) return response(review('queued')) as never
      if (config.method === 'POST' && config.url === `/design-reviews/${reviewID}/assets`) {
        uploadCount++
        return response({ ok: true }) as never
      }
      if (config.method === 'POST' && config.url === `/design-reviews/${reviewID}/analysis`) {
        queued = true
        return Promise.reject(new Error('response connection lost'))
      }
      return Promise.reject(new Error(`unexpected request ${config.method} ${config.url}`))
    })
    const resume = { current: undefined }
    const input = submission()

    await expect(executeReviewCreation(input, resume)).rejects.toThrow()
    expect(queued).toBe(true)
    expect(uploadCount).toBe(2)
    await expect(executeReviewCreation(input, resume)).resolves.toMatchObject({ id: reviewID, status: 'queued' })
    expect(uploadCount).toBe(2)
    expect(request.mock.calls.filter(([config]) => config.url === '/design-reviews')).toHaveLength(1)
  })
})
