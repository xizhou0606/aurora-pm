import { describe, expect, it } from 'bun:test';
import { generateAndCompileTestCaseDraftBatch } from './draft/generate-draft.js';
import {
  TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS,
  TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS,
} from './runtime/generate-runtime.js';
import { completeTestCaseDraft, incompleteHighRiskCoverageDraft, omittedCoveragePatch } from './generate-test-fixtures.js';
import type { QATestCaseDraft } from '../index.js';

describe('test-case generation', () => {
  it('repairs a semantic Draft coverage failure once and compiles the repaired draft', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1, title: '提交结算' }],
        tasks: [{ requirement_id: 1, title: '实现结算校验' }],
      },
    };
    const allowedRefs = ['subject:title', 'requirement:0:title', 'task:0:title'];
    const invalid = completeTestCaseDraft(['subject:title', 'requirement:0:title']);
    const valid = completeTestCaseDraft(allowedRefs);
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];
    const repairDiagnosticCodes: string[] = [];

    const resolved = await generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs,
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      recordRepairDiagnostic: (codes) => repairDiagnosticCodes.push(...codes),
      generate: async (request) => {
        calls.push(request);
        return {
          text: JSON.stringify(calls.length === 1 ? invalid : valid),
          finishReason: 'stop',
          usage: calls.length === 1
            ? { inputTokens: 10, outputTokens: 5 }
            : { inputTokens: 20, outputTokens: 8 },
        };
      },
    });

    expect(calls).toHaveLength(2);
    expect(calls[1].messages?.[1]).toEqual({ role: 'assistant', content: JSON.stringify(invalid) });
    expect(calls[1].messages?.[2].content).toContain('context item is not covered: task:0');
    expect(calls[1].messages?.[2].content).toContain('task:0:title');
    expect(calls[1].messages?.[2].content).toContain(JSON.stringify([
      { targetRef: 'requirement:0', group: 'requirement:0' },
      { targetRef: 'task:0', group: 'requirement:0' },
    ]));
    expect(calls[1].messages?.[2].content)
      .toContain('effectiveRefs = case.contextRefs + all valid case.riskLinks[].contextRef');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'validation',
      usage: { inputTokens: 30, outputTokens: 13, totalTokens: 43 },
    });
    expect(resolved.output.testCases[0].evidence).toEqual([
      'subject.title',
      'test_case_context.requirements.0.title',
      'test_case_context.tasks.0.title',
    ]);
    expect(repairDiagnosticCodes).toEqual(['source_assessment', 'context_coverage']);
  });

  it('uses the shared repair call as a strict coverage-only patch when possible', async () => {
    const initial = incompleteHighRiskCoverageDraft(['subject:title']);
    const reason = '当前输入只明确了结算失败反馈，未给出正向与边界判定规则';
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];
    const resolved = await generateAndCompileTestCaseDraftBatch({
      input: { subject: { title: '结算页' } },
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS,
      generate: async (request) => {
        calls.push(request);
        return {
          text: JSON.stringify(calls.length === 1 ? initial : omittedCoveragePatch(reason)),
          finishReason: 'stop',
          usage: calls.length === 1
            ? { inputTokens: 10, outputTokens: 5 }
            : { inputTokens: 6, outputTokens: 3 },
        };
      },
    });

    expect(calls).toHaveLength(2);
    expect(calls[1].messages).toBeUndefined();
    expect(calls[1].maxOutputTokens).toBe(TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS);
    expect(calls[1].instructions).toContain('QA COVERAGE-ONLY PATCH ADAPTER');
    expect(calls[1].prompt).not.toContain(initial.summary);
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'coverage',
      coveragePatched: true,
      usage: { inputTokens: 16, outputTokens: 8, totalTokens: 24 },
    });
    expect(resolved.output.testCaseDesign?.traceability.find((row) => (
      row.traceId === 'RISK-SETTLEMENT'
    ))?.gapNote).toBe(`positive, boundary: ${reason}`);
  });

  it('mechanically fixes a P2 coverage decision without another model call', async () => {
    const initial = completeTestCaseDraft(['subject:title']) as QATestCaseDraft;
    initial.risks[0].coverageDecision = { status: 'complete' };
    let calls = 0;

    const resolved = await generateAndCompileTestCaseDraftBatch({
      input: { subject: { title: '结算页' } },
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 1,
      generate: async () => {
        calls += 1;
        return {
          text: JSON.stringify(initial),
          finishReason: 'stop',
          usage: { inputTokens: 10, outputTokens: 5, totalTokens: 15 },
        };
      },
    });

    expect(calls).toBe(1);
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'coverage',
      coveragePatched: true,
      usage: { inputTokens: 10, outputTokens: 5, totalTokens: 15 },
    });
  });

  it('classifies an invalid coverage patch without exposing model content', async () => {
    let calls = 0;
    const run = generateAndCompileTestCaseDraftBatch({
      input: { subject: { title: '结算页' } },
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        calls += 1;
        return calls === 1
          ? {
            text: JSON.stringify(incompleteHighRiskCoverageDraft(['subject:title'])),
            finishReason: 'stop',
            usage: null,
          }
          : {
            text: JSON.stringify({ repairs: [] }),
            finishReason: 'stop',
            usage: { outputTokens: 42 },
            requestedMaxOutputTokens: TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS,
            elapsedMs: 1_234,
          };
      },
    });

    await expect(run).rejects.toMatchObject({
      name: 'QATestCaseCoveragePatchError',
      qaPhase: 'repair',
      qaRepairKind: 'coverage',
      qaFailureStep: 'parse',
      qaCoverageDiagnosticCount: 1,
      qaRequestedMaxOutputTokens: TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS,
      qaOutputTokens: 42,
      qaCallElapsedMs: 1_234,
    });
    expect(calls).toBe(2);
  });
});
