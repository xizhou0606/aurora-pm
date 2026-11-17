import {
  boundedText,
  enumValue,
  parsePositiveRouteId,
  positivePage,
  searchRecord,
  stringValue,
  supportedPageSize,
} from './search-utils'

export const designReviewStatuses = [
  'draft',
  'queued',
  'analyzing',
  'completed',
  'failed',
  'abandoned',
  'deleting',
] as const

export const designReviewConclusions = [
  'needs_fix',
  'needs_review',
  'passed',
] as const

export type DesignReviewListSearch = ReturnType<typeof parseDesignReviewListSearch>
export type NewDesignReviewSearch = ReturnType<typeof parseNewDesignReviewSearch>

export function parseDesignReviewListSearch(value: unknown) {
  const search = searchRecord(value)
  return {
    keyword: boundedText(search.keyword, 120),
    project_set_id: parsePositiveRouteId(search.project_set_id),
    plan_unit_id: parsePositiveRouteId(search.plan_unit_id),
    requirement_id: parsePositiveRouteId(search.requirement_id),
    status: enumValue(search.status, designReviewStatuses),
    conclusion: enumValue(search.conclusion, designReviewConclusions),
    page: positivePage(search.page),
    page_size: supportedPageSize(search.page_size),
  }
}

export function parseNewDesignReviewSearch(value: unknown) {
  const search = searchRecord(value)
  const sourceReviewId = stringValue(search.source_review_id)
  return {
    requirement_id: parsePositiveRouteId(search.requirement_id),
    source_review_id: sourceReviewId && isUUID(sourceReviewId)
      ? sourceReviewId
      : undefined,
  }
}

export function isUUID(value: unknown): value is string {
  return typeof value === 'string'
    && /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value)
}
