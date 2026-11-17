import { describe, expect, it } from 'bun:test';
import { generateAndCompileTestCaseDraftBatch } from './draft/generate-draft.js';
import { TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS, TEST_CASE_REPAIR_MIN_REMAINING_MS } from './runtime/generate-runtime.js';
import { completeTestCaseDraft, incompleteHighRiskCoverageDraft, omittedCoveragePatch } from './generate-test-fixtures.js';
import type { QATestCaseDraft } from '../index.js';

describe('test-case generation', () => {
  it('compiles compact Draft IR without asking the model for artifact bookkeeping', async () => {
    const input = { subject: { title: '结算页' } };
    let calls = 0;
    const resolved = await generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        calls += 1;
        return {
          text: JSON.stringify(completeTestCaseDraft(['subject:title'])),
          finishReason: 'stop',
          usage: { inputTokens: 10, outputTokens: 20, totalTokens: 30 },
        };
      },
    });

    expect(calls).toBe(1);
    expect(resolved).toMatchObject({ repaired: false, repairReason: null });
    expect(resolved.output.testCases[0]).toMatchObject({
      caseId: 'TC-QA-001',
      evidence: ['subject.title'],
      steps: ['提交结算'],
      expectedResults: ['页面显示结算成功'],
    });
    expect(resolved.output.testCaseDesign?.executionOrder).toEqual(['TC-QA-001']);
  });

  it('reports the initial Draft repair budget gate with safe stage metadata', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: { coverageRequired: true, requirements: [], tasks: [{ title: '结算任务' }] },
    };
    let calls = 0;
    const run = generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs: ['subject:title', 'task:0:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => TEST_CASE_REPAIR_MIN_REMAINING_MS - 1,
      generate: async () => {
        calls += 1;
        return {
          text: JSON.stringify(completeTestCaseDraft(['subject:title'])),
          finishReason: 'stop',
          usage: null,
        };
      },
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'full_draft',
      qaFailureStep: 'budget',
      qaFullDraftRepairAttempt: 1,
      qaRuleFailureCodes: expect.arrayContaining(['context_coverage']),
    });
    expect(calls).toBe(1);
  });

  it('uses the single Draft repair chance to rewrite an incomplete initial response from scratch', async () => {
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];
    const partial = '{"partial":"source text must not be replayed"';
    const resolved = await generateAndCompileTestCaseDraftBatch({
      input: { subject: { title: '结算页' } },
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return calls.length === 1
          ? { text: partial, finishReason: undefined, usage: { inputTokens: 4, outputTokens: 2 } }
          : {
            text: JSON.stringify(completeTestCaseDraft(['subject:title'])),
            finishReason: 'stop',
            usage: { inputTokens: 8, outputTokens: 6 },
          };
      },
    });

    expect(calls).toHaveLength(2);
    expect(calls.map((call) => call.telemetryKind)).toEqual([
      'draft_initial', 'draft_full_repair',
    ]);
    expect(calls[1].messages).toBeUndefined();
    expect(calls[1].prompt).toContain('上一轮 Draft IR 未正常结束');
    expect(calls[1].prompt).not.toContain(partial);
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'incomplete',
      usage: { inputTokens: 12, outputTokens: 8, totalTokens: 20 },
    });
  });

  it('uses the final Draft repair after an incomplete rewrite contains an invalid context reference', async () => {
    const invalid = completeTestCaseDraft(['subject:title']) as QATestCaseDraft;
    invalid.risks[0].contextRefs = ['source:0:content'];
    const valid = completeTestCaseDraft(['subject:title']);
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];
    const resolved = await generateAndCompileTestCaseDraftBatch({
      input: { subject: { title: '结算页' } },
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        if (calls.length === 1) {
          return {
            text: '{"partial":', finishReason: 'length', usage: { inputTokens: 2, outputTokens: 1 },
          };
        }
        return {
          text: JSON.stringify(calls.length === 2 ? invalid : valid),
          finishReason: 'stop',
          usage: calls.length === 2
            ? { inputTokens: 4, outputTokens: 2 }
            : { inputTokens: 6, outputTokens: 3 },
        };
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls.map((call) => call.telemetryKind)).toEqual([
      'draft_initial', 'draft_full_repair', 'draft_full_repair',
    ]);
    expect(calls[1].messages).toBeUndefined();
    expect(calls[1].prompt).toContain('上一轮 Draft IR 未正常结束');
    expect(calls[2].messages?.[1]).toEqual({ role: 'assistant', content: JSON.stringify(invalid) });
    expect(calls[2].messages?.[2].content).toContain('最终一次完整 Draft IR 修复');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'incomplete',
      coveragePatched: false,
      usage: { inputTokens: 12, outputTokens: 6, totalTokens: 18 },
    });
  });

  it('keeps pure risk coverage on the narrow patch after an incomplete rewrite', async () => {
    const incomplete = incompleteHighRiskCoverageDraft(['subject:title']);
    const reason = '当前输入只明确了结算失败反馈，未给出正向与边界判定规则';
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];
    const resolved = await generateAndCompileTestCaseDraftBatch({
      input: { subject: { title: '结算页' } },
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        if (calls.length === 1) {
          return {
            text: '{"partial":', finishReason: 'length', usage: { inputTokens: 2, outputTokens: 1 },
          };
        }
        return {
          text: JSON.stringify(calls.length === 2 ? incomplete : omittedCoveragePatch(reason)),
          finishReason: 'stop',
          usage: calls.length === 2
            ? { inputTokens: 4, outputTokens: 2 }
            : { inputTokens: 3, outputTokens: 1 },
        };
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls.map((call) => call.telemetryKind)).toEqual([
      'draft_initial', 'draft_full_repair', 'draft_coverage_patch',
    ]);
    expect(calls[2].messages).toBeUndefined();
    expect(calls[2].maxOutputTokens).toBe(TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS);
    expect(calls[2].instructions).toContain('QA COVERAGE-ONLY PATCH ADAPTER');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'incomplete',
      coveragePatched: true,
      usage: { inputTokens: 9, outputTokens: 4, totalTokens: 13 },
    });
  });

  it('accepts missing finish metadata only for a strict complete Draft JSON object', async () => {
    let calls = 0;
    const resolved = await generateAndCompileTestCaseDraftBatch({
      input: { subject: { title: '结算页' } },
      allowedRefs: ['subject:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        calls += 1;
        return {
          text: JSON.stringify(completeTestCaseDraft(['subject:title'])),
          finishReason: undefined,
          usage: null,
        };
      },
    });

    expect(calls).toBe(1);
    expect(resolved).toMatchObject({
      repaired: false,
      repairReason: null,
      finishReasonFallback: true,
    });
  });

  it('does not retry a content-filtered Draft response', async () => {
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
        return { text: '', finishReason: 'content-filter', usage: null };
      },
    });

    await expect(run).rejects.toThrow('generation ended with content-filter');
    expect(calls).toBe(1);
  });
});
