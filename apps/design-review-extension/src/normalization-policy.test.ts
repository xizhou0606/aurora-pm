import { describe, expect, it } from 'bun:test'
import { parseNormalizationPlan } from './normalization-policy'

const base = {
  inventory_token: '00000000-0000-4000-8000-000000000001',
  model: 'vision-model',
  prompt_version: 'pm-design-review-dom-normalization-1.0',
  already_aligned: false,
  mismatch_detected: true,
  skipped: [],
  summary: '需要对齐动态业务数据',
}

describe('normalization plan policy', () => {
  it('accepts a bounded text operation', () => {
    expect(parseNormalizationPlan({
      ...base,
      operations: [{
        type: 'set_text', target_id: 'el-2', expected_fingerprint: '1234567890abcdef',
        text: '42', figma_source: '订单数量', confidence: 0.97,
      }],
    })).not.toBeNull()
  })

  it('rejects low-confidence and duplicate target operations', () => {
    const operation = {
      type: 'set_text', target_id: 'el-2', expected_fingerprint: '1234567890abcdef',
      text: '42', figma_source: '订单数量', confidence: 0.89,
    }
    expect(parseNormalizationPlan({ ...base, operations: [operation] })).toBeNull()
    expect(parseNormalizationPlan({ ...base, operations: [{ ...operation, confidence: 0.99 }, { ...operation, confidence: 0.99 }] })).toBeNull()
  })

  it('rejects empty or unsafe repeated-item mutations', () => {
    expect(parseNormalizationPlan({
      ...base,
      operations: [{
        type: 'replace_repeated_items', container_id: 'el-1', template_id: 'el-2',
        expected_container_fingerprint: '1234567890abcdef', expected_template_fingerprint: 'abcdef1234567890',
        items: [{ values: [{ target_id: 'el-3', text: null, control_state: null }] }],
        figma_source: '列表', confidence: 0.99,
      }],
    })).toBeNull()
  })

  it('rejects contradictory aligned results', () => {
    expect(parseNormalizationPlan({ ...base, already_aligned: true, operations: [] })).toBeNull()
  })
})
