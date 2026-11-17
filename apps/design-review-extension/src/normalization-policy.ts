import type { NormalizationControlState, NormalizationOperation, NormalizationPlan } from './types'

const ELEMENT_ID = /^el-[1-9]\d*$/
const FINGERPRINT = /^[a-f0-9]{16,64}$/
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

function record(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function boundedString(value: unknown, maximum: number, allowEmpty = false): value is string {
  return typeof value === 'string' && value.length <= maximum && (allowEmpty || value.trim().length > 0)
}

function nullableEnum(value: unknown, allowed: readonly string[]) {
  return value === null || (typeof value === 'string' && allowed.includes(value))
}

function controlState(value: unknown): value is NormalizationControlState {
  if (!record(value)) return false
  return (value.checked === null || typeof value.checked === 'boolean')
    && nullableEnum(value.aria_checked, ['true', 'false', 'mixed'])
    && nullableEnum(value.aria_pressed, ['true', 'false', 'mixed'])
    && nullableEnum(value.aria_selected, ['true', 'false'])
    && (value.data_state === null || boundedString(value.data_state, 120, true))
}

function operation(value: unknown): value is NormalizationOperation {
  if (!record(value) || !boundedString(value.figma_source, 500) || typeof value.confidence !== 'number' || value.confidence < 0.9 || value.confidence > 1) return false
  if (value.type === 'set_text') {
    return typeof value.target_id === 'string' && ELEMENT_ID.test(value.target_id)
      && typeof value.expected_fingerprint === 'string' && FINGERPRINT.test(value.expected_fingerprint)
      && boundedString(value.text, 1000, true)
  }
  if (value.type === 'set_control_state') {
    return typeof value.target_id === 'string' && ELEMENT_ID.test(value.target_id)
      && typeof value.expected_fingerprint === 'string' && FINGERPRINT.test(value.expected_fingerprint)
      && controlState(value.state)
  }
  if (value.type !== 'replace_repeated_items'
    || typeof value.container_id !== 'string' || !ELEMENT_ID.test(value.container_id)
    || typeof value.template_id !== 'string' || !ELEMENT_ID.test(value.template_id)
    || value.container_id === value.template_id
    || typeof value.expected_container_fingerprint !== 'string' || !FINGERPRINT.test(value.expected_container_fingerprint)
    || typeof value.expected_template_fingerprint !== 'string' || !FINGERPRINT.test(value.expected_template_fingerprint)
    || !Array.isArray(value.items) || value.items.length === 0 || value.items.length > 100) return false
  return value.items.every((item) => {
    if (!record(item) || !Array.isArray(item.values) || item.values.length === 0 || item.values.length > 30) return false
    const targets = new Set<string>()
    return item.values.every((candidate) => {
      if (!record(candidate) || typeof candidate.target_id !== 'string' || !ELEMENT_ID.test(candidate.target_id) || targets.has(candidate.target_id)) return false
      targets.add(candidate.target_id)
      const textValid = candidate.text === null || boundedString(candidate.text, 1000, true)
      const stateValid = candidate.control_state === null || controlState(candidate.control_state)
      return textValid && stateValid && (candidate.text !== null || candidate.control_state !== null)
    })
  })
}

export function parseNormalizationPlan(value: unknown): NormalizationPlan | null {
  if (!record(value)
    || typeof value.inventory_token !== 'string' || !UUID.test(value.inventory_token)
    || !boundedString(value.model, 120)
    || !boundedString(value.prompt_version, 160)
    || typeof value.already_aligned !== 'boolean'
    || typeof value.mismatch_detected !== 'boolean'
    || !Array.isArray(value.operations) || value.operations.length > 200
    || !Array.isArray(value.skipped) || value.skipped.length > 200
    || !boundedString(value.summary, 2000)) return null
  if (value.already_aligned && (value.mismatch_detected || value.operations.length > 0)) return null
  if (!value.operations.every(operation)) return null
  if (!value.skipped.every((item) => record(item)
    && boundedString(item.figma_source, 500)
    && (item.target_id === null || (typeof item.target_id === 'string' && ELEMENT_ID.test(item.target_id)))
    && boundedString(item.reason, 1000))) return null
  const targets = value.operations.map((item) => item.type === 'replace_repeated_items' ? item.template_id : item.target_id)
  if (new Set(targets).size !== targets.length) return null
  return value as NormalizationPlan
}
