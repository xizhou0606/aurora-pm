import type {
  AnalysisScope,
  DesignReviewTarget,
  ReviewConclusion,
  ReviewStatus,
} from './types'

export type CreateMobileCaptureInput = {
  collector_id: string
  platform: 'ios' | 'android'
  device_id: string
}

export type ReviewListInput = {
  keyword?: string
  project_set_id?: number
  plan_unit_id?: number
  requirement_id?: number
  status?: ReviewStatus
  conclusion?: ReviewConclusion
  page: number
  page_size: number
}

export type CreateReviewInput = {
  requirement_id: number
  source_review_id?: string
  figma_url: string
  implementation_target: DesignReviewTarget
  viewport_width: number
  viewport_height: number
  device_pixel_ratio: number
  page_zoom: number
  analysis_scope: AnalysisScope
  mobile_topbar: Record<string, unknown>
  dom_normalization: Record<string, unknown>
}
