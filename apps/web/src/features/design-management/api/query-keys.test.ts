import { describe, expect, it } from 'bun:test'

import { designReviewKeys } from './query-keys'

describe('design review query keys', () => {
  it('isolates every resource by workspace', () => {
    expect(designReviewKeys.detail(7, 'review-1')).not.toEqual(
      designReviewKeys.detail(8, 'review-1'),
    )
    expect(designReviewKeys.detail(7, 'review-1')).toEqual([
      'pm', 'workspace', 7, 'designReviews', 'detail', 'review-1',
    ])
  })

  it('keeps all list filters in the cache identity', () => {
    const base = {
      keyword: 'checkout',
      project_set_id: 3,
      plan_unit_id: 8,
      requirement_id: 13,
      status: 'analyzing' as const,
      conclusion: 'needs_fix' as const,
      page: 2,
      page_size: 50,
    }

    expect(designReviewKeys.list(7, base)).toEqual([
      'pm', 'workspace', 7, 'designReviews', 'list', base,
    ])
    expect(designReviewKeys.list(7, base)).not.toEqual(
      designReviewKeys.list(7, { ...base, requirement_id: 14 }),
    )
  })

  it('rejects an unselected workspace instead of sharing global data', () => {
    expect(() => designReviewKeys.root(0)).toThrow(/positive workspace ID/)
  })
})
