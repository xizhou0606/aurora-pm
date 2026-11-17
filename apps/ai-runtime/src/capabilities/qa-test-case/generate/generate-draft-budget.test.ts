import { describe, expect, it } from 'bun:test';
import { generateAndCompileTestCaseDraftBatch } from './draft/generate-draft.js';
import { TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS, TEST_CASE_REPAIR_MIN_REMAINING_MS } from './runtime/generate-runtime.js';
import { safeGenerationDiagnostic } from '../../../shared/errors/index.js';
import {
  completeTestCaseDraft,
  incompleteHighRiskCoverageDraft,
  omittedCoveragePatch,
  omittedHighRiskCoverageDraft,
} from './generate-test-fixtures.js';
import type { QATestCaseDraft } from '../index.js';

describe('test-case generation', () => {
  it('does not make the final full Draft call below the 90 second safety budget', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1, title: '提交结算' }],
        tasks: [{ requirement_id: 1, title: '实现结算校验' }],
      },
    };
    const responses = [
      incompleteHighRiskCoverageDraft(['subject:title']),
      incompleteHighRiskCoverageDraft(['subject:title', 'requirement:0:title']),
    ];
    let calls = 0;
    let budgetChecks = 0;
    const run = generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs: ['subject:title', 'requirement:0:title', 'task:0:title'],
      instructions: 'fixed draft instructions', prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => (budgetChecks++ === 0 ? 120_000 : 89_999),
      generate: async () => ({
        text: JSON.stringify(responses[calls++]), finishReason: 'stop', usage: null,
      }),
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'full_draft',
      qaFailureStep: 'budget',
      qaRuleFailureCodes: expect.arrayContaining(['risk_coverage', 'context_coverage']),
    });
    expect(calls).toBe(2);
  });

  it('fails closed when the final full Draft leaves only coverage issues', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1, title: '提交结算' }],
        tasks: [{ requirement_id: 1, title: '实现结算校验' }],
      },
    };
    const responses = [
      incompleteHighRiskCoverageDraft(['subject:title']),
      incompleteHighRiskCoverageDraft(['subject:title', 'requirement:0:title']),
      incompleteHighRiskCoverageDraft([
        'subject:title', 'requirement:0:title', 'task:0:title',
      ]),
    ];
    let calls = 0;
    const run = generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs: ['subject:title', 'requirement:0:title', 'task:0:title'],
      instructions: 'fixed draft instructions', prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => ({
        text: JSON.stringify(responses[calls++]), finishReason: 'stop', usage: null,
      }),
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'full_draft',
      qaFailureStep: 'compile',
      qaFullDraftRepairAttempt: 2,
      qaRuleFailureCodes: ['risk_coverage'],
    });
    expect(calls).toBe(3);
  });

  it('carries only a Draft context-reference category through the terminal error', async () => {
    const invalid = completeTestCaseDraft(['subject:title']) as QATestCaseDraft;
    invalid.sourceAssessments[0].contextRefs = ['source:0:content'];
    invalid.risks[0].contextRefs = ['source:0:content'];
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
        return { text: JSON.stringify(invalid), finishReason: 'stop', usage: null };
      },
    });

    let failure: unknown;
    try {
      await run;
    } catch (error) {
      failure = error;
    }
    // bun's toMatchObject writes asymmetric matchers into the received object
    // (bun 1.4.2 bug) and its arrayContaining requires exact element equality
    // (vitest used subset matching). Keep matcher-based checks out of
    // toMatchObject and project the diagnostics to the asserted fields so the
    // error survives for safeGenerationDiagnostic below.
    const qaFailure = failure as {
      qaRuleFailureCodes: string[]
      qaContextReferenceDiagnostics: Array<{ code: string; pathCategory: string }>
    }
    expect(qaFailure).toMatchObject({
      qaRepairKind: 'full_draft',
      qaFailureStep: 'compile',
      qaFullDraftRepairAttempt: 2,
    });
    expect(qaFailure.qaRuleFailureCodes).toEqual(expect.arrayContaining(['context_reference']));
    expect(qaFailure.qaContextReferenceDiagnostics.map((row) => ({ code: row.code, pathCategory: row.pathCategory })))
      .toEqual(expect.arrayContaining([
        { code: 'unavailable_context_ref', pathCategory: 'source_assessment_context_refs' },
        { code: 'unavailable_context_ref', pathCategory: 'risk_context_refs' },
        { code: 'undeclared_risk_link_context_ref', pathCategory: 'case_risk_links' },
      ]));
    const diagnostic = safeGenerationDiagnostic(failure);
    expect(diagnostic.fullDraftRepairAttempt).toBe(2);
    expect(diagnostic.contextReferenceDiagnostics).toEqual(expect.arrayContaining([
      { code: 'unavailable_context_ref', pathCategory: 'source_assessment_context_refs', count: 1 },
      { code: 'unavailable_context_ref', pathCategory: 'risk_context_refs', count: 1 },
      { code: 'undeclared_risk_link_context_ref', pathCategory: 'case_risk_links', count: 1 },
    ]));
    expect(JSON.stringify(diagnostic)).not.toContain('source:0:content');
    expect(calls).toBe(3);
  });

  it.each([
    ['invalid schema', 'stop', JSON.stringify({ summary: 'still incomplete' }), 'parse'],
    [
      'length finish',
      'length',
      JSON.stringify(omittedHighRiskCoverageDraft(
        ['subject:title', 'requirement:0:title', 'task:0:title'],
        '当前输入未提供正向与边界输入的可判定结果',
      )),
      'finish',
    ],
  ] as const)('fails closed after a final full Draft %s', async (
    _label, finishReason, finalText, failureStep,
  ) => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ id: 1, title: '提交结算' }],
        tasks: [{ requirement_id: 1, title: '实现结算校验' }],
      },
    };
    const responses = [
      JSON.stringify(incompleteHighRiskCoverageDraft(['subject:title'])),
      JSON.stringify(incompleteHighRiskCoverageDraft([
        'subject:title', 'requirement:0:title',
      ])),
      finalText,
    ];
    let calls = 0;
    const run = generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs: ['subject:title', 'requirement:0:title', 'task:0:title'],
      instructions: 'fixed draft instructions', prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        const index = calls++;
        return {
          text: responses[index],
          finishReason: index === 2 ? finishReason : 'stop',
          usage: null,
        };
      },
    });

    await expect(run).rejects.toMatchObject({
      qaRepairKind: 'full_draft', qaFailureStep: failureStep,
    });
    expect(calls).toBe(3);
  });

  it('fails closed without a third call when the coverage-only safety budget is exhausted', async () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: { coverageRequired: true, requirements: [], tasks: [{ title: '结算任务' }] },
    };
    const responses = [
      incompleteHighRiskCoverageDraft(['subject:title']),
      incompleteHighRiskCoverageDraft(['subject:title', 'task:0:title']),
    ];
    let calls = 0;
    let budgetChecks = 0;
    const run = generateAndCompileTestCaseDraftBatch({
      input,
      allowedRefs: ['subject:title', 'task:0:title'],
      instructions: 'fixed draft instructions',
      prompt: 'draft prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => (budgetChecks++ === 0
        ? TEST_CASE_REPAIR_MIN_REMAINING_MS
        : TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS - 1),
      generate: async () => ({
        text: JSON.stringify(responses[calls++]), finishReason: 'stop', usage: null,
      }),
    });

    await expect(run).rejects.toThrow('coverage decision must be omitted');
    expect(calls).toBe(2);
  });

  it('classifies Draft schema drift safely and never reflects received enum values', async () => {
    const invalid = completeTestCaseDraft(['subject:title']) as unknown as {
      methods: string[];
      risks: Array<{ coverageDecision: { status: string; reason: unknown } }>;
      cases: Array<{ riskLinks: Array<{ contextRef: unknown }> }>;
    };
    invalid.methods = ['received-secret-method'];
    invalid.risks[0].coverageDecision = { status: 'omitted', reason: 42 };
    invalid.cases[0].riskLinks[0].contextRef = 42;
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
        return {
          text: JSON.stringify(calls.length === 1 ? invalid : completeTestCaseDraft(['subject:title'])),
          finishReason: 'stop',
          usage: null,
        };
      },
    });

    expect(resolved.repairReason).toBe('structure');
    const repairInstruction = calls[1].messages?.[2].content ?? '';
    expect(repairInstruction).toContain('methods[0] must use one of');
    expect(repairInstruction).toContain('equivalence_partitioning');
    expect(repairInstruction).toContain('risks[0].coverageDecision.reason has the wrong JSON type');
    expect(repairInstruction).toContain('cases[0].riskLinks[0].contextRef has the wrong JSON type');
    expect(repairInstruction).not.toContain('received-secret-method');
  });

  it('fails Draft generation closed after the shared single repair attempt', async () => {
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
        return { text: '{"summary":', finishReason: 'stop', usage: null };
      },
    });

    await expect(run).rejects.toThrow('Draft IR must be one complete valid JSON object');
    await expect(run).rejects.not.toThrow('结算页');
    expect(calls).toBe(2);
  });
});
