export type ReviewStatus = 'draft' | 'queued' | 'analyzing' | 'completed' | 'failed' | 'abandoned' | 'deleting'
export type ReviewConclusion = 'needs_fix' | 'needs_review' | 'passed'
export type DifferenceCategory = 'layout' | 'size' | 'typography' | 'color' | 'component' | 'icon_image' | 'content' | 'effect'
export type DifferenceSeverity = 'critical' | 'major' | 'minor'
export type VerificationStatus = 'pending' | 'confirmed' | 'false_positive'
export type HistoricalRecheckStatus = 'still_present' | 'resolved' | 'uncertain' | 'not_applicable'
export type CanvasMode = 'compare' | 'overlay' | 'difference'

export type Region = { x: number; y: number; width: number; height: number }

export type AnalysisScope = {
  enabled: boolean
  version?: string
  selection_side?: 'design' | 'implementation'
  design_region?: Region
  implementation_region?: Region
}

export type DesignReviewTarget = {
  kind: 'web' | 'mobile'
  original_url?: string
  actual_url?: string
  platform?: 'ios' | 'android'
  app_id?: string
  app_name?: string
  app_version?: string
  orientation?: 'portrait' | 'landscape'
  device?: Record<string, unknown>
  screen?: Record<string, unknown>
  collector_version?: string
  mobilewright_version?: string
}

export type EvidenceAsset = {
  id: string
  kind: string
  content_type: string
  byte_size: number
  sha256: string
  width?: number
  height?: number
  created_at: string
}

export type DesignDifference = {
  id: string
  review_id: string
  ordinal: number
  source: 'ai' | 'manual' | 'history_recovered' | 'manual_reopen'
  analysis_kind: 'full' | 'deep'
  created_by_user_id?: number
  category: DifferenceCategory
  severity: DifferenceSeverity
  title: string
  description: string
  recommendation: string
  confidence: number
  design_region?: Region
  implementation_region?: Region
  evidence_basis: string
  evidence_notes: string
  verification_status: VerificationStatus
  resolution_status: 'unresolved' | 'resolved'
  recheck_status?: 'new' | 'still_present' | 'uncertain' | 'resolved' | 'not_applicable'
  human_edited: boolean
  created_at: string
  updated_at: string
  can_edit: boolean
  can_delete: boolean
  can_verify: boolean
  can_resolve: boolean
  can_reopen: boolean
}

export type ReviewCapabilities = {
  can_view: boolean
  can_create: boolean
  can_manage_differences: boolean
  can_manage_analysis: boolean
  can_delete: boolean
  can_manage_collectors: boolean
  creation_available: boolean
  figma_preview_available: boolean
  analysis_available: boolean
  object_storage_available: boolean
  unavailable_reasons: string[]
  allowed_actions: string[]
}

export type ReviewRound = {
  id: string
  round_number: number
  status: ReviewStatus
  conclusion?: ReviewConclusion
  created_at: string
  completed_at?: string
}

export type DeepReviewJob = {
  id: string
  review_id: string
  retry_of_id?: string
  status: 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  created_by_user_id: number
  design_region: Region
  implementation_region: Region
  focus: string
  attempts: number
  model?: string
  reasoning_effort?: string
  prompt_version?: string
  response_id?: string
  duration_ms?: number
  generated_difference_count: number
  last_error?: string
  started_at?: string
  completed_at?: string
  created_at: string
  can_retry: boolean
  can_cancel: boolean
}

export type RecheckVerification = {
  id: string
  historical_difference_id: string
  historical_title: string
  lineage_id?: string
  current_difference_id?: string
  ai_status: HistoricalRecheckStatus
  manual_status_override?: HistoricalRecheckStatus
  effective_status: HistoricalRecheckStatus
  confidence: number
  reason: string
  evidence_summary: string
  current_design_region?: Region
  current_implementation_region?: Region
  can_override: boolean
}

export type PendingRecheckMatch = {
  id: string
  candidate_id: string
  suggested_lineage_id: string
  suggested_lineage_title: string
  candidate: DifferenceInput
  discovery_source: 'full_page' | 'history_recovered'
  confidence: number
  reason: string
  can_resolve: boolean
}

export type DesignReviewSummary = {
  id: string
  chain_root_id: string
  source_review_id?: string
  round_number: number
  requirement_id?: number
  requirement_title: string
  project_set_id: number
  project_set_name: string
  plan_unit_id: number
  plan_unit_title: string
  implementation_kind: 'web' | 'mobile'
  implementation_target: DesignReviewTarget
  figma_url: string
  figma_node_name: string
  status: ReviewStatus
  conclusion?: ReviewConclusion
  difference_count: number
  unresolved_difference_count: number
  created_by_user_id: number
  created_by_name: string
  created_at: string
  updated_at: string
  can_delete: boolean
}

export type DesignReviewDetail = DesignReviewSummary & {
  viewport_width: number
  viewport_height: number
  device_pixel_ratio: number
  page_zoom: number
  design_width: number
  design_height: number
  dimensions_match: boolean
  design_structure_available: boolean
  implementation_structure_complete: boolean
  capture_warnings: unknown[]
  analysis_scope: AnalysisScope
  mobile_topbar: Record<string, unknown>
  dom_normalization: Record<string, unknown>
  analysis_summary?: string
  model?: string
  reasoning_effort?: string
  prompt_version?: string
  response_id?: string
  analysis_duration_ms?: number
  analysis_attempts: number
  last_error_stage?: string
  last_error?: string
  assets: EvidenceAsset[]
  differences: DesignDifference[]
  rounds: ReviewRound[]
  deep_review_jobs: DeepReviewJob[]
  recheck_verifications: RecheckVerification[]
  pending_recheck_matches: PendingRecheckMatch[]
  capabilities: ReviewCapabilities
}

export type ReviewPage = {
  items: DesignReviewSummary[]
  page: number
  page_size: number
  total: number
}

export type RequirementOption = {
  id: number
  title: string
  project_set_id: number
  project_set_name: string
  plan_unit_id: number
  plan_unit_title: string
}

export type FigmaPreview = {
  file_key: string
  node_id: string
  node_name: string
  image_url: string
  width: number
  height: number
  expires_at?: string
}

export type MobileCollector = {
  id: string
  name: string
  version: string
  mobilewright_version: string
  hostname: string
  architecture: 'arm64' | 'x64'
  inventory?: {
    devices?: Array<{
      id: string
      name: string
      platform: 'ios' | 'android'
      type: 'real' | 'simulator' | 'emulator'
      model?: string | null
      os_version?: string | null
    }>
    doctor?: { ready: boolean; summary: string }
  }
  doctor_ready: boolean
  doctor_summary: string
  last_seen_at?: string
  revoked_at?: string
  created_at: string
}

export type MobileCaptureDraft = {
  id: string
  collector_id: string
  status: 'queued' | 'claimed' | 'succeeded' | 'failed' | 'cancelled' | 'consumed' | 'expired'
  platform: 'ios' | 'android'
  device_id: string
  app_id?: string
  target?: Record<string, unknown>
  elements?: unknown[]
  structure_truncated?: boolean
  warnings?: unknown[]
  screenshot_available: boolean
  expires_at: string
  captured_at?: string
  last_error?: string
}

export type DifferenceInput = Pick<DesignDifference,
  'category' | 'severity' | 'title' | 'description' | 'recommendation' | 'evidence_notes' | 'confidence'
> & Pick<DesignDifference, 'design_region' | 'implementation_region'>

export type DifferenceFilters = {
  category: 'all' | DifferenceCategory
  severity: 'all' | DifferenceSeverity
  verification: 'all' | VerificationStatus
  recheck: 'all' | NonNullable<DesignDifference['recheck_status']>
}
