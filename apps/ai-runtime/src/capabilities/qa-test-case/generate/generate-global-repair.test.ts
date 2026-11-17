import { describe, expect, it } from 'bun:test';
import { generateAndCompileTestCaseGlobalPlan } from './global-plan/generate-global-plan-run.js';
import {
  TEST_CASE_GLOBAL_REF_PATCH_MAX_OUTPUT_TOKENS,
  TEST_CASE_GLOBAL_REF_PATCH_MIN_REMAINING_MS,
} from './runtime/generate-runtime.js';
import {
  completeGlobalPlan,
  completeGlobalPlanInput,
  globalPlanRelationshipRefPatch,
  globalPlanWithUnavailableRelationship,
} from './generate-test-fixtures.js';

describe('test-case generation', () => {
  it('uses a final full third call when a normal Global repair cannot be parsed', async () => {
    const initial = globalPlanWithUnavailableRelationship();
    const malformed = '{"summary":"repair baseline"';
    const responses = [JSON.stringify(initial), malformed, JSON.stringify(completeGlobalPlan())];
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];
    const resolved = await generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return { text: responses[calls.length - 1], finishReason: 'stop', usage: null };
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls[2].messages?.[1]).toEqual({ role: 'assistant', content: malformed });
    expect(calls[2].messages?.[2].content).toContain('one complete valid JSON object');
    expect(resolved.repairReason).toBe('validation');
  });

  it('rewrites call three from scratch when a normal Global repair ends with length', async () => {
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
          return {
            text: JSON.stringify(globalPlanWithUnavailableRelationship()),
            finishReason: 'stop', usage: null,
          };
        }
        if (calls.length === 2) {
          return { text: '{"repair":', finishReason: 'length', usage: null };
        }
        return { text: JSON.stringify(completeGlobalPlan()), finishReason: 'stop', usage: null };
      },
    });

    expect(calls).toHaveLength(3);
    expect(calls[2].messages).toBeUndefined();
    expect(calls[2].prompt).toContain('上一轮 Global Plan 未正常结束');
    expect(calls[2].prompt).not.toContain('{"repair":');
    expect(resolved).toMatchObject({ repairReason: 'validation', globalRefPatched: false });
  });

  it('does not start a normal final Global repair below the 90 second budget', async () => {
    const initial = completeGlobalPlan();
    initial.methods.push('user_journey');
    const mixed = globalPlanWithUnavailableRelationship();
    mixed.methods.push('user_journey');
    let calls = 0;
    let checks = 0;
    const run = generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => (checks++ === 0 ? 120_000 : 89_999),
      generate: async () => {
        calls += 1;
        return {
          text: JSON.stringify(calls === 1 ? initial : mixed), finishReason: 'stop',
          usage: { outputTokens: 31 }, requestedMaxOutputTokens: 6000, elapsedMs: 654,
        };
      },
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'global_plan', qaFailureStep: 'budget',
      qaRuleFailureCodes: expect.arrayContaining(['uniqueness', 'relationship_reference']),
      qaRequestedMaxOutputTokens: 6000, qaOutputTokens: 31, qaCallElapsedMs: 654,
    });
    expect(calls).toBe(2);
  });

  it('does not make a third Global ref call below the 45 second safety budget', async () => {
    const initial = completeGlobalPlan();
    initial.methods.push('user_journey');
    const responses = [initial, globalPlanWithUnavailableRelationship()];
    let calls = 0;
    let budgetChecks = 0;
    const run = generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => (budgetChecks++ === 0 ? 120_000 : 44_999),
      generate: async () => ({
        text: JSON.stringify(responses[calls++]), finishReason: 'stop', usage: null,
      }),
    });

    let failure: unknown;
    try { await run; } catch (error) { failure = error; }
    expect(TEST_CASE_GLOBAL_REF_PATCH_MIN_REMAINING_MS).toBe(45_000);
    expect(calls).toBe(2);
    expect(failure).toMatchObject({
      qaRepairKind: 'global_ref_patch', qaFailureStep: 'budget', qaGlobalRefDiagnosticCount: 1,
    });
  });

  it('fails closed after an invalid Global ref patch without a fourth call', async () => {
    const initial = completeGlobalPlan();
    initial.methods.push('user_journey');
    const responses = [
      initial,
      globalPlanWithUnavailableRelationship(),
      { repairs: [] },
    ];
    let calls = 0;
    const run = generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => ({
        text: JSON.stringify(responses[calls++]), finishReason: 'stop', usage: null,
      }),
    });

    let failure: unknown;
    try { await run; } catch (error) { failure = error; }
    expect(calls).toBe(3);
    expect(failure).toMatchObject({
      qaRepairKind: 'global_ref_patch', qaFailureStep: 'parse', qaGlobalRefDiagnosticCount: 1,
    });
  });

  it('uses one narrow third call to patch Global Plan refs and combines all usage', async () => {
    const input = completeGlobalPlanInput();
    const initial = completeGlobalPlan();
    initial.methods.push('user_journey');
    const repaired = globalPlanWithUnavailableRelationship();
    const responses = [initial, repaired, globalPlanRelationshipRefPatch()];
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];
    const usages = [
      { inputTokens: 10, outputTokens: 4 },
      { inputTokens: 20, outputTokens: 5 },
      { inputTokens: 3, outputTokens: 2 },
    ];

    const resolved = await generateAndCompileTestCaseGlobalPlan({
      input,
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions',
      prompt: 'global prompt',
      abortSignal: new AbortController().signal,
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
    expect(calls[2].messages).toBeUndefined();
    expect(TEST_CASE_GLOBAL_REF_PATCH_MAX_OUTPUT_TOKENS).toBe(1500);
    expect(calls[2].maxOutputTokens).toBe(1500);
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'validation',
      globalRefPatched: true,
      usage: { inputTokens: 33, outputTokens: 11, totalTokens: 44 },
    });
    expect(resolved.plan.interactions[0].relationshipRef).toBe('subject:description');
  });

  it('uses a final full third call for mixed Global compilation failures', async () => {
    const initial = completeGlobalPlan();
    initial.methods.push('user_journey');
    const mixed = globalPlanWithUnavailableRelationship();
    mixed.methods.push('user_journey');
    const responses = [initial, mixed, completeGlobalPlan()];
    const calls: Array<Parameters<Parameters<typeof generateAndCompileTestCaseGlobalPlan>[0]['generate']>[0]> = [];
    const resolved = await generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => ({
        text: JSON.stringify(responses[calls.push(request) - 1]),
        finishReason: 'stop', usage: { inputTokens: calls.length, outputTokens: calls.length },
      }),
    });

    expect(calls).toHaveLength(3);
    expect(calls[2].messages?.[1]).toEqual({ role: 'assistant', content: JSON.stringify(mixed) });
    expect(calls[2].messages?.[2].content).toContain('methods must be unique');
    expect(resolved).toMatchObject({
      repairReason: 'validation', globalRefPatched: false,
      usage: { inputTokens: 6, outputTokens: 6, totalTokens: 12 },
    });
  });

  it('does not start the final full Global repair below the 90 second budget', async () => {
    let calls = 0;
    let checks = 0;
    const run = generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => (checks++ === 0 ? 120_000 : 89_999),
      generate: async () => {
        calls += 1;
        return calls === 1
          ? { text: '{"first":', finishReason: 'length', usage: null }
          : {
              text: JSON.stringify({ summary: 'still incomplete' }),
              finishReason: 'stop', usage: { outputTokens: 17 },
              requestedMaxOutputTokens: 6000, elapsedMs: 456,
            };
      },
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'global_plan', qaFailureStep: 'budget',
      qaRequestedMaxOutputTokens: 6000, qaOutputTokens: 17, qaCallElapsedMs: 456,
    });
    expect(calls).toBe(2);
  });

  it.each([
    ['parse', JSON.stringify({ summary: 'still incomplete' })],
    ['compile', JSON.stringify({ ...completeGlobalPlan(), methods: ['user_journey', 'user_journey'] })],
  ] as const)('fails closed at final Global %s without a fourth call', async (step, finalText) => {
    let calls = 0;
    const run = generateAndCompileTestCaseGlobalPlan({
      input: completeGlobalPlanInput(),
      allowedRefs: ['subject:description', 'requirement:0:title', 'requirement:1:title'],
      instructions: 'fixed global instructions', prompt: 'global prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        calls += 1;
        if (calls === 1) return { text: '{"first":', finishReason: 'length', usage: null };
        return {
          text: calls === 2 ? JSON.stringify({ summary: 'invalid rewrite' }) : finalText,
          finishReason: 'stop', usage: { outputTokens: 29 },
          requestedMaxOutputTokens: 6000, elapsedMs: 789,
        };
      },
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'global_plan', qaFailureStep: step,
      qaRequestedMaxOutputTokens: 6000, qaOutputTokens: 29, qaCallElapsedMs: 789,
    });
    expect(calls).toBe(3);
  });
});
