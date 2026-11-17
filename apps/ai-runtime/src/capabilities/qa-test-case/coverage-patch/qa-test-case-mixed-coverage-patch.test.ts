import { describe, expect, it } from 'bun:test';
import {
  applyQATestCaseMixedCoverageTargetPatch,
  prepareQATestCaseMixedCoverageTargetPatch,
} from './qa-test-case-mixed-coverage-patch.js';
import {
  parseQATestCaseCoveragePatchText,
  parseQATestCaseMixedCoverageTargetPatchText,
} from './qa-test-case-coverage-patch.js';
import {
  QA_TEST_CASE_COVERAGE_PATCH_REPLACEMENT_SENTINEL,
  QATestCaseCoveragePatchError,
} from './qa-test-case-coverage-patch-schema.js';
import { compileQATestCaseDraft } from '../draft/qa-test-case-draft-compile.js';
import {
  completePatch,
  mixedDraft,
  mixedFailure,
  mixedInput,
  mixedPatch,
  riskId,
} from './qa-test-case-coverage-patch-fixtures.js';

describe('mixed risk and target coverage patch', () => {
  it('exposes only exact structural diagnostics and known risk/leaf alternatives', () => {
    const original = mixedDraft();
    const failure = mixedFailure(original);

    expect(failure.targetCoverageDiagnostics).toEqual([{
      code: 'context_item_not_covered',
      targetRef: 'task:0',
      candidateContextRefs: ['task:0:status'],
      group: 'task:0',
      message: 'context item is not covered: task:0',
    }]);
    expect(JSON.stringify(failure.targetCoverageDiagnostics)).not.toContain('payment task state can be stale');
    const eligibility = prepareQATestCaseMixedCoverageTargetPatch(
      mixedInput(), original, failure,
    );
    expect(eligibility).toMatchObject({
      eligible: true,
      eligibleTargetCoverageSpecs: [{
        targetRef: 'task:0', riskId: 'RISK-PAYMENT-TASK', contextRef: 'task:0:status',
      }],
    });
  });

  it('fails closed if a target has no exact existing risk/context association', () => {
    const original = mixedDraft();
    original.risks[1].contextRefs = ['subject:title'];
    original.cases[2].riskLinks = [{ riskId: 'RISK-PAYMENT-TASK', contextRef: 'subject:title' }];
    const failure = mixedFailure(original);

    expect(prepareQATestCaseMixedCoverageTargetPatch(
      mixedInput(), original, failure,
    )).toEqual({
      eligible: false,
      reason: 'no_safe_target_coverage_association',
      eligibleTargetCoverageSpecs: [],
    });
  });

  it('fails closed for a stale or non-pure failure before any patch is accepted', () => {
    const original = mixedDraft();
    const failure = mixedFailure(original);
    const stale = structuredClone(original);
    stale.cases[2].riskLinks = [{ riskId: 'RISK-PAYMENT-TASK', contextRef: 'task:0:status' }];

    expect(prepareQATestCaseMixedCoverageTargetPatch(
      mixedInput(), stale, failure,
    )).toEqual({
      eligible: false,
      reason: 'not_pure_mixed_coverage_failure',
      eligibleTargetCoverageSpecs: [],
    });

    const nonPure = mixedFailure(original);
    nonPure.issues.push('at least one smoke case must be selected');
    expect(prepareQATestCaseMixedCoverageTargetPatch(
      mixedInput(), original, nonPure,
    )).toEqual({
      eligible: false,
      reason: 'not_pure_mixed_coverage_failure',
      eligibleTargetCoverageSpecs: [],
    });
  });

  it('requires one ordered repair per risk and uncovered target, then passes the compiler', () => {
    const original = mixedDraft();
    const before = structuredClone(original);
    const repaired = applyQATestCaseMixedCoverageTargetPatch(
      mixedInput(), original, mixedFailure(original), mixedPatch(),
    );

    expect(original).toEqual(before);
    expect(repaired.cases.slice(-3).map((testCase) => testCase.category))
      .toEqual(['positive', 'boundary', 'state']);
    const targetCase = repaired.cases.at(-1)!;
    expect(targetCase).toMatchObject({
      title: '[RISK-PAYMENT-TASK/task:0/state] show pending payment task state',
      category: 'state', priority: 'P2', contextRefs: [],
      riskLinks: [{ riskId: 'RISK-PAYMENT-TASK', contextRef: 'task:0:status' }],
      preconditions: [], testData: [], notes: [], smoke: false,
      automationCandidate: false, releaseBlocking: false,
      actions: [{
        step: 'open the payment task after it enters pending state',
        expectedResult: 'the task row displays the pending state',
      }],
    });
    expect(() => compileQATestCaseDraft(mixedInput(), repaired)).not.toThrow();

    const swapped = mixedPatch();
    swapped.targetRepairs[0].targetRef = 'requirement:0';
    expect(() => applyQATestCaseMixedCoverageTargetPatch(
      mixedInput(), mixedDraft(), mixedFailure(), swapped,
    )).toThrow('target repairs must exactly match diagnostics in order');
  });

  it.each([
    ['core category', (patch: ReturnType<typeof mixedPatch>) => {
      patch.targetRepairs[0].category = 'positive' as never;
    }],
    ['integration category', (patch: ReturnType<typeof mixedPatch>) => {
      patch.targetRepairs[0].category = 'integration' as never;
    }],
    ['unrelated risk pair', (patch: ReturnType<typeof mixedPatch>) => {
      patch.targetRepairs[0].riskId = riskId;
      patch.targetRepairs[0].contextRef = 'requirement:0:title';
    }],
    ['fixed sentinel', (patch: ReturnType<typeof mixedPatch>) => {
      patch.targetRepairs[0].title = QA_TEST_CASE_COVERAGE_PATCH_REPLACEMENT_SENTINEL;
    }],
  ] as const)('rejects target patch %s', (_label, mutate) => {
    const patch = mixedPatch();
    mutate(patch);
    expect(() => applyQATestCaseMixedCoverageTargetPatch(
      mixedInput(), mixedDraft(), mixedFailure(), patch,
    )).toThrow(QATestCaseCoveragePatchError);
  });

  it('parses the strict compact mixed schema without exposing failed model text', () => {
    expect(parseQATestCaseMixedCoverageTargetPatchText(
      `<think>repair</think>\n\`\`\`json\n${JSON.stringify(mixedPatch())}\n\`\`\``,
    )).toEqual(mixedPatch());
    let failure: unknown;
    try {
      parseQATestCaseMixedCoverageTargetPatchText('{"targetRepairs":[],"secret":"PRIVATE_VALUE"}');
    } catch (error) {
      failure = error;
    }
    expect(failure).toBeInstanceOf(QATestCaseCoveragePatchError);
    expect((failure as Error).message).not.toContain('PRIVATE_VALUE');
  });
});

describe('parseQATestCaseCoveragePatchText', () => {
  it('accepts a strict fenced complete compact object after thinking text', () => {
    const value = completePatch();
    expect(parseQATestCaseCoveragePatchText(
      `<think>repair coverage</think>\n\`\`\`json\n${JSON.stringify(value)}\n\`\`\``,
    )).toEqual(value);
  });

  it.each([
    ['not-json PRIVATE_RECEIVED_VALUE', 'coverage_patch_invalid_json'],
    [JSON.stringify({ repairs: [], received: 'PRIVATE_RECEIVED_VALUE' }), 'coverage_patch_invalid_schema'],
  ])('classifies invalid text safely without echoing received values', (text, issue) => {
    const received = 'PRIVATE_RECEIVED_VALUE';
    let failure: unknown;
    try {
      parseQATestCaseCoveragePatchText(text);
    } catch (error) {
      failure = error;
    }
    expect(failure).toBeInstanceOf(QATestCaseCoveragePatchError);
    expect((failure as QATestCaseCoveragePatchError).issues).toEqual([issue]);
    expect((failure as Error).message).not.toContain(received);
  });
});
