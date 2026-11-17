export type ViewportTarget = {
  width: number
  height: number
  devicePixelRatio: number
  pageZoom: number
}

export type CaptureWarning = { code: string; message: string }

export type VisualElement = {
  id: string
  parent_id: string | null
  tag: string
  role: string | null
  text: string | null
  bounds: { x: number; y: number; width: number; height: number }
  styles: Record<string, string>
  clipped: boolean
  structural_fingerprint: string
  control_state: {
    checked: boolean | null
    aria_checked: string | null
    aria_pressed: string | null
    aria_selected: string | null
    data_state: string | null
  } | null
  allowed_operations: Array<'set_text' | 'set_control_state' | 'repeat_template'>
}

export type NormalizationControlState = NonNullable<VisualElement['control_state']>
export type NormalizationOperation = {
  type: 'set_text'
  target_id: string
  expected_fingerprint: string
  text: string
  figma_source: string
  confidence: number
} | {
  type: 'set_control_state'
  target_id: string
  expected_fingerprint: string
  state: NormalizationControlState
  figma_source: string
  confidence: number
} | {
  type: 'replace_repeated_items'
  container_id: string
  template_id: string
  expected_container_fingerprint: string
  expected_template_fingerprint: string
  items: Array<{ values: Array<{ target_id: string; text: string | null; control_state: NormalizationControlState | null }> }>
  figma_source: string
  confidence: number
}

export type NormalizationPlan = {
  inventory_token: string
  model: string
  prompt_version: string
  already_aligned: boolean
  mismatch_detected: boolean
  operations: NormalizationOperation[]
  skipped: Array<{ figma_source: string; target_id: string | null; reason: string }>
  summary: string
}

export type NormalizationReport = {
  plan: Pick<NormalizationPlan, 'model' | 'prompt_version' | 'already_aligned' | 'mismatch_detected' | 'summary' | 'skipped'>
  operations: Array<{ type: NormalizationOperation['type']; target_id: string; result: 'applied' | 'skipped'; reason: string | null }>
  counts: { applied: number; skipped: number }
}

export type CaptureEvidence = {
  request_id: string
  original_url: string
  actual_url: string
  captured_at: string
  viewport: ViewportTarget
  screenshot: string
  original_screenshot?: string
  elements: VisualElement[]
  structure_truncated: boolean
  warnings: CaptureWarning[]
  inventory: {
    version: '1.0'
    token: string
    captured_at: string
    page_fingerprint: string
    elements: VisualElement[]
    truncated: boolean
  }
  dom_normalization: { requested: boolean; report?: NormalizationReport }
}

export type CaptureTask = {
  request_id: string
  workbench_tab_id: number
  target_tab_id: number
  original_url: string
  viewport: ViewportTarget
  dom_normalization: boolean
  state: 'ready' | 'awaiting_normalization'
  created_at: string
}
