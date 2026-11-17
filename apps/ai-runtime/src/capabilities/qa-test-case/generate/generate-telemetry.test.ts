import { describe, expect, it } from 'bun:test';
import {
  qaTestCaseGenerationSuccessTelemetry,
  qaTestCaseModelCallSuccessTelemetry,
  qaTestCaseStageSuccessTelemetry,
} from './runtime/generate-telemetry.js';
import { TEST_CASE_REQUEST_TIMEOUT_MS } from './runtime/generate-runtime.js';

describe('QA success telemetry', () => {
  it('keeps only bounded allowlisted model-call metadata', () => {
    const call = qaTestCaseModelCallSuccessTelemetry(99, 'draft_coverage_patch', {
      text: '敏感模型原文',
      usage: {
        inputTokens: 11.9,
        outputTokens: Number.POSITIVE_INFINITY,
        totalTokens: 99_999_999,
        providerMetadata: { prompt: '敏感 prompt' },
      },
      finishReason: '非预期值',
      elapsedMs: TEST_CASE_REQUEST_TIMEOUT_MS + 1,
    });

    expect(call).toEqual({
      callNumber: 4,
      kind: 'draft_coverage_patch',
      elapsedMs: TEST_CASE_REQUEST_TIMEOUT_MS,
      inputTokens: 11,
      outputTokens: 0,
      totalTokens: 10_000_000,
      finishReason: 'unknown',
    });
    expect(Object.keys(call)).toEqual([
      'callNumber', 'kind', 'elapsedMs', 'inputTokens', 'outputTokens', 'totalTokens',
      'finishReason',
    ]);
    expect(JSON.stringify(call)).not.toContain('敏感');
  });

  it('aggregates deterministic stage and generation totals without business text', () => {
    const first = qaTestCaseModelCallSuccessTelemetry(1, 'draft_initial', {
      text: '业务正文 A',
      usage: { inputTokens: 20, outputTokens: 5, totalTokens: 25 },
      finishReason: 'stop',
      elapsedMs: 30,
    });
    const second = qaTestCaseModelCallSuccessTelemetry(2, 'draft_full_repair', {
      text: '业务正文 B',
      usage: { inputTokens: 10, outputTokens: 3 },
      finishReason: 'length',
      elapsedMs: 12,
    });
    const stage = qaTestCaseStageSuccessTelemetry({
      stage: 'draft_batch',
      batchNumber: 1,
      calls: [first, second],
      wallElapsedMs: 44,
      repaired: true,
      repairReason: 'incomplete',
      finishReasonFallback: false,
      coveragePatched: false,
      repairDiagnosticCodes: [
        'context_reference', 'context_reference', 'risk_coverage', 'unsafe-value',
      ],
    });
    const generation = qaTestCaseGenerationSuccessTelemetry({
      batchCount: 1,
      requirementGroupCount: 1,
      globalPlan: false,
      totalConcurrency: 4,
      wallElapsedMs: 45,
      stages: [stage],
    });

    expect(generation).toMatchObject({
      modelCallCount: 2,
      modelElapsedMs: 42,
      wallElapsedMs: 45,
      inputTokens: 30,
      outputTokens: 8,
      totalTokens: 38,
      stages: [{
        repaired: true,
        repairReason: 'incomplete',
        repairRuleCodes: ['context_reference', 'risk_coverage'],
        repairDiagnosticCategories: [
          { category: 'context_reference', count: 2 },
          { category: 'risk_coverage', count: 1 },
        ],
        calls: [
          { kind: 'draft_initial', finishReason: 'stop' },
          { kind: 'draft_full_repair', finishReason: 'length' },
        ],
      }],
    });
    expect(JSON.stringify(generation)).not.toContain('业务正文');
    expect(JSON.stringify(generation)).not.toContain('unsafe-value');
  });
});
