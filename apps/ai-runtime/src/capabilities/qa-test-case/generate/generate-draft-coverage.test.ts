import { describe, expect, it } from 'bun:test';
import { generateAndCompileTestCaseDraftBatch } from './draft/generate-draft.js';
import { TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS } from './runtime/generate-runtime.js';
import { incompleteHighRiskCoverageDraft, omittedCoveragePatch } from './generate-test-fixtures.js';

describe('test-case generation', () => {
  it('uses one narrow coverage patch after a full Draft repair leaves only that mismatch', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [],
        tasks: [{ title: '实现结算失败校验' }],
      },
    };
    const initial = incompleteHighRiskCoverageDraft(['subject:title']);
    const repaired = incompleteHighRiskCoverageDraft(['subject:title', 'task:0:title']);
    const reason = '任务只给出失败校验，未给出正向与边界输入的可判定结果';
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];
    const responses = [initial, repaired, omittedCoveragePatch(reason)];
    const resolved = await generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs: ['subject:title', 'task:0:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return {
          text: JSON.stringify(responses[calls.length - 1]),
          finishReason: 'stop',
          usage: { inputTokens: 2, outputTokens: 1 },
        };
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls.map((call) => call.telemetryKind)).toEqual([
      'draft_initial', 'draft_full_repair', 'draft_coverage_patch',
    ]);
    expect(calls[1].messages?.[2].content).toContain('context item is not covered: task:0');
    expect(calls[2].maxOutputTokens).toBe(TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS);
    expect(calls[2].instructions).toContain('QA COVERAGE-ONLY PATCH ADAPTER');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'validation',
      coveragePatched: true,
      usage: { inputTokens: 6, outputTokens: 3, totalTokens: 9 },
    });
  });

  it('uses one compact mixed patch for risk and target coverage left by the first full repair', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1, title: '提交结算' }],
        tasks: [{ requirement_id: 1, title: '实现结算校验' }],
      },
    };
    const allowedRefs = ['subject:title', 'requirement:0:title', 'task:0:title'];
    const initial = incompleteHighRiskCoverageDraft(['subject:title']);
    const repaired = incompleteHighRiskCoverageDraft(allowedRefs);
    repaired.cases[0].riskLinks = [{
      riskId: 'RISK-SETTLEMENT',
      contextRef: 'subject:title',
    }];
    const compactPatch = {
      riskRepairs: [{
        riskId: 'RISK-SETTLEMENT',
        resolutions: [
          { category: 'positive', kind: 'omit' },
          { category: 'boundary', kind: 'omit' },
        ],
        omissionReason: '当前输入未提供正向与边界输入的可判定结果',
      }],
      targetRepairs: [{
        targetRef: 'task:0',
        riskId: 'RISK-SETTLEMENT',
        contextRef: 'task:0:title',
        category: 'state',
        title: '校验结算任务状态',
        step: '打开实现结算校验任务',
        expectedResult: '任务列表显示实现结算校验任务及其当前状态',
      }],
    };
    const responses = [initial, repaired, compactPatch];
    const usages = [
      { inputTokens: 10, outputTokens: 5 },
      { inputTokens: 20, outputTokens: 8 },
      { inputTokens: 30, outputTokens: 12 },
    ];
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseDraftBatch>[0]['generate']>[0]> = [];
    const controller = new AbortController();

    const resolved = await generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs,
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: controller.signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return {
          text: JSON.stringify(responses[calls.length - 1]),
          finishReason: 'stop',
          usage: usages[calls.length - 1],
        };
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls.map((call) => call.telemetryKind)).toEqual([
      'draft_initial', 'draft_full_repair', 'draft_coverage_patch',
    ]);
    expect(calls[2]).toMatchObject({
      abortSignal: controller.signal,
      maxOutputTokens: TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS,
    });
    expect(calls[2].messages).toBeUndefined();
    expect(calls[2].instructions).toContain('QA MIXED COVERAGE-TARGET PATCH ADAPTER');
    expect(calls[2].prompt).toContain('"targetRef":"task:0"');
    expect(calls[2].prompt).toContain('"missingCategories":["positive","boundary"]');
    expect(calls[2].prompt).toContain('"contextRef":"task:0:title"');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'validation',
      coveragePatched: true,
      usage: { inputTokens: 60, outputTokens: 25, totalTokens: 85 },
    });
    expect(resolved.output.testCases).toEqual(expect.arrayContaining([
      expect.objectContaining({
        category: 'state',
        evidence: ['test_case_context.tasks.0.title'],
      }),
    ]));
  });
});
