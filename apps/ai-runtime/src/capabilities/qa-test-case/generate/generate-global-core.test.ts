import { describe, expect, it } from 'bun:test';
import { generateAndCompileTestCaseGlobalPlan } from './global-plan/generate-global-plan-run.js';
import { TEST_CASE_GLOBAL_REF_PATCH_MAX_OUTPUT_TOKENS, TEST_CASE_REPAIR_MIN_REMAINING_MS } from './runtime/generate-runtime.js';
import {
  completeGlobalPlan,
  completeGlobalPlanInput,
  globalPlanRelationshipRefPatch,
  globalPlanWithUnavailableRelationship,
} from './generate-test-fixtures.js';

describe('test-case generation', () => {
  it('compiles and repairs the compact cross-batch Global Plan once', async () => {
    const input = completeGlobalPlanInput();
    const allowedRefs = [
      'subject:description', 'requirement:0:title', 'requirement:1:title',
    ];
    const invalid = completeGlobalPlan();
    invalid.interactions[0].relationshipRef = 'source:0:content';
    const valid = completeGlobalPlan();
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];

    const resolved = await generateAndCompileTestCaseGlobalPlan({
      input,
      allowedRefs,
      instructions: 'fixed global instructions',
      prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return {
          text: JSON.stringify(calls.length === 1 ? invalid : valid),
          finishReason: 'stop',
          usage: calls.length === 1
            ? { inputTokens: 12, outputTokens: 6 }
            : { inputTokens: 18, outputTokens: 9 },
        };
      },
    });

    expect(calls).toHaveLength(2);
    expect(calls[1].messages?.[1]).toEqual({ role: 'assistant', content: JSON.stringify(invalid) });
    expect(calls[1].messages?.[2].content).toContain('has unavailable relationship ref: source:0:content');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'validation',
      usage: { inputTokens: 30, outputTokens: 15, totalTokens: 45 },
    });
    expect(resolved.plan.cases[0]).toMatchObject({
      category: 'integration',
      evidence: [
        'test_case_context.requirements.1.title',
        'test_case_context.requirements.0.title',
        'subject.description',
      ],
      executionPosition: 'after_batches',
    });
  });

  it('reports the initial Global Plan repair budget gate with safe stage metadata', async () => {
    const invalid = completeGlobalPlan();
    invalid.interactions[0].relationshipRef = 'source:0:content';
    let calls = 0;
    const run = generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions',
      prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => TEST_CASE_REPAIR_MIN_REMAINING_MS - 1,
      generate: async () => {
        calls += 1;
        return { text: JSON.stringify(invalid), finishReason: 'stop', usage: null };
      },
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'global_plan',
      qaFailureStep: 'budget',
      qaRuleFailureCodes: ['relationship_reference'],
    });
    expect(calls).toBe(1);
  });

  it('uses the single Global Plan repair chance after an incomplete initial response', async () => {
    const input = completeGlobalPlanInput();
    const allowedRefs = [
      'subject:description', 'requirement:0:title', 'requirement:1:title',
    ];
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];
    const resolved = await generateAndCompileTestCaseGlobalPlan({
      input,
      allowedRefs,
      instructions: 'fixed global instructions',
      prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return calls.length === 1
          ? { text: '{"partial":', finishReason: 'length', usage: { inputTokens: 5, outputTokens: 3 } }
          : {
            text: JSON.stringify(completeGlobalPlan()),
            finishReason: undefined,
            usage: { inputTokens: 9, outputTokens: 7 },
          };
      },
    });

    expect(calls).toHaveLength(2);
    expect(calls[1].messages).toBeUndefined();
    expect(calls[1].prompt).toContain('上一轮 Global Plan 未正常结束');
    expect(calls[1].prompt).not.toContain('{"partial":');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'incomplete',
      finishReasonFallback: true,
      usage: { inputTokens: 14, outputTokens: 10, totalTokens: 24 },
    });
  });

  it('uses a final full Global repair when the incomplete rewrite has invalid structure', async () => {
    const controller = new AbortController();
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];
    const rewrittenText = JSON.stringify({ summary: 'still incomplete' });
    const usages = [
      { inputTokens: 5, outputTokens: 3 },
      { inputTokens: 7, outputTokens: 4 },
      { inputTokens: 11, outputTokens: 6 },
    ];
    const resolved = await generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: controller.signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return {
          text: calls.length === 1
            ? '{"partial":'
            : calls.length === 2 ? rewrittenText : JSON.stringify(completeGlobalPlan()),
          finishReason: calls.length === 1 ? 'length' : calls.length === 2 ? 'stop' : undefined,
          usage: usages[calls.length - 1],
        };
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls.every((call) => call.abortSignal === controller.signal)).toBe(true);
    expect(calls[2].messages?.[1]).toEqual({ role: 'assistant', content: rewrittenText });
    expect(calls[2].messages?.[2].content).toContain('未通过确定性门禁');
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'incomplete',
      globalRefPatched: false,
      finishReasonFallback: true,
      usage: { inputTokens: 23, outputTokens: 13, totalTokens: 36 },
    });
  });

  it('uses the narrow ref patch as call three after an incomplete rewrite', async () => {
    const responses = [
      { text: '{"partial":', finishReason: 'length', usage: { inputTokens: 2, outputTokens: 1 } },
      {
        text: JSON.stringify(globalPlanWithUnavailableRelationship()),
        finishReason: 'stop', usage: { inputTokens: 4, outputTokens: 2 },
      },
      {
        text: JSON.stringify(globalPlanRelationshipRefPatch()),
        finishReason: 'stop', usage: { inputTokens: 1, outputTokens: 1 },
      },
    ];
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];
    const resolved = await generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return responses[calls.length - 1];
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls.map((call) => call.telemetryKind)).toEqual([
      'global_initial', 'global_full_repair', 'global_ref_patch',
    ]);
    expect(calls[2].maxOutputTokens).toBe(TEST_CASE_GLOBAL_REF_PATCH_MAX_OUTPUT_TOKENS);
    expect(resolved).toMatchObject({
      repairReason: 'incomplete', globalRefPatched: true,
      usage: { inputTokens: 7, outputTokens: 4, totalTokens: 11 },
    });
  });

  it.each(['length', 'error', 'other', 'unknown'])(
    'rewrites call three from scratch when the incomplete rewrite ends with %s',
    async (finishReason) => {
      const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];
      const resolved = await generateAndCompileTestCaseGlobalPlan({
        input: completeGlobalPlanInput(),
        allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
        instructions: 'fixed global instructions', prompt: 'global prompt',
        abortSignal: new AbortController().signal,
        remainingMs: () => 120_000,
        generate: async (request) => {
          calls.push(request);
          if (calls.length === 1) {
            return { text: '{"first":', finishReason: 'length', usage: { inputTokens: 1, outputTokens: 1 } };
          }
          if (calls.length === 2) {
            return { text: '{"second":', finishReason, usage: { inputTokens: 2, outputTokens: 2 } };
          }
          return {
            text: JSON.stringify(completeGlobalPlan()), finishReason: 'stop',
            usage: { inputTokens: 3, outputTokens: 3 },
          };
        },
      });

      expect(calls).toHaveLength(3);
      expect(calls[2].messages).toBeUndefined();
      expect(calls[2].prompt).toContain('上一轮 Global Plan 未正常结束');
      expect(calls[2].prompt).not.toContain('{"second":');
      expect(resolved.usage).toEqual({ inputTokens: 6, outputTokens: 6, totalTokens: 12 });
    },
  );

  it.each(['content-filter', 'tool-calls'])(
    'does not retry a non-rewriteable incomplete Global repair finish: %s',
    async (finishReason) => {
      let calls = 0;
      const run = generateAndCompileTestCaseGlobalPlan({
        input: completeGlobalPlanInput(),
        allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
        instructions: 'fixed global instructions', prompt: 'global prompt',
        abortSignal: new AbortController().signal,
        remainingMs: () => 120_000,
        generate: async () => {
          calls += 1;
          return calls === 1
            ? { text: '{"first":', finishReason: 'length', usage: null }
            : {
                text: '{"second":', finishReason, usage: { outputTokens: 9 },
                requestedMaxOutputTokens: 6000, elapsedMs: 321,
              };
        },
      });

      await expect(run).rejects.toMatchObject({
        qaRepairKind: 'global_plan', qaFailureStep: 'finish',
        qaFinishReason: finishReason, qaRequestedMaxOutputTokens: 6000,
        qaOutputTokens: 9, qaCallElapsedMs: 321,
      });
      expect(calls).toBe(2);
    },
  );
});
