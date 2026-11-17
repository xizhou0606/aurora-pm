import { describe, expect, it } from 'bun:test'

import {
  reviewDetailQueryOptions,
  reviewListQueryOptions,
  reviewRequirementsQueryOptions,
} from './queries'

describe('design review query enablement', () => {
  it('does not request protected collections before view access resolves', () => {
    const input = { page: 1, page_size: 20 }

    expect(reviewRequirementsQueryOptions(7, false).enabled).toBe(false)
    expect(reviewListQueryOptions(7, input, false).enabled).toBe(false)
  })

  it('does not request a recheck source before create access resolves', () => {
    expect(reviewDetailQueryOptions(7, 'review-1', false).enabled).toBe(false)
    expect(reviewDetailQueryOptions(7, '', true).enabled).toBe(false)
  })
})
