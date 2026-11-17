import { describe, expect, it } from 'bun:test';
import {
  applyQATestCaseCoveragePatch,
  parseQATestCaseCoveragePatchText,
  prepareQATestCaseCoveragePatch,
} from './qa-test-case-coverage-patch.js';
import {
  QA_TEST_CASE_COVERAGE_PATCH_REPLACEMENT_SENTINEL,
  QA_TEST_CASE_COVERAGE_PATCH_REASON_SENTINEL,
  QATestCaseCoveragePatchError,
  type QATestCaseCoveragePatch,
} from './qa-test-case-coverage-patch-schema.js';
import { compileQATestCaseDraft } from '../draft/qa-test-case-draft-compile.js';
import { QATestCaseDraftCompilationError } from '../draft/fixtures/qa-test-case-draft-error.js';
import { compileRiskCoverageDecisions, type RiskCoverageDiagnostic } from '../draft/qa-test-case-draft-risk.js';
import {
  caseResolution,
  completePatch,
  contextRef,
  diagnostic,
  draft,
  riskId,
  testCase,
} from './qa-test-case-coverage-patch-fixtures.js';

describe('QA test case risk coverage diagnostics', () => {
  it('reports negative plus rollback as missing positive and boundary with stable text', () => {
    const issues: string[] = [];
    const diagnostics: RiskCoverageDiagnostic[] = [];
    compileRiskCoverageDecisions(draft().risks, draft().cases, issues, diagnostics);

    expect(issues).toEqual([
      'risk RISK-PAYMENT coverage decision must be omitted for missing categories: positive, boundary',
    ]);
    expect(diagnostics).toEqual([diagnostic()]);
  });

  it('carries coverage diagnostics on compilation errors', () => {
    let failure: unknown;
    try {
      compileQATestCaseDraft({ subject: { title: 'payment' } }, draft());
    } catch (error) {
      failure = error;
    }
    expect(failure).toBeInstanceOf(QATestCaseDraftCompilationError);
    expect((failure as QATestCaseDraftCompilationError).diagnostics).toEqual([diagnostic()]);
  });

  it('does not label unrelated final-gate failures as coverage diagnostics', () => {
    const failure = new QATestCaseDraftCompilationError(['final gate failure']);
    expect(failure.diagnostics).toEqual([]);
  });
});

describe('prepareQATestCaseCoveragePatch', () => {
  it('mechanically applies complete without changing model-authored cases', () => {
    const original = draft();
    original.cases.push(testCase('positive'), testCase('boundary'));
    original.risks[0].coverageDecision = {
      status: 'omitted', reason: 'the previous decision was stale',
    };
    const before = structuredClone(original);
    const completeDiagnostic: RiskCoverageDiagnostic = {
      ...diagnostic(), actualCategories: ['positive', 'negative', 'boundary'],
      missingCategories: [], expectedStatus: 'complete',
      message: 'risk RISK-PAYMENT coverage decision must be complete when no categories are missing',
    };

    const prepared = prepareQATestCaseCoveragePatch(original, [completeDiagnostic]);

    expect(original).toEqual(before);
    expect(prepared.pendingDiagnostics).toEqual([]);
    expect(prepared.draft.risks[0].coverageDecision).toEqual({ status: 'complete' });
    expect(prepared.draft.cases).toEqual(before.cases);
    expect(() => compileQATestCaseDraft(
      { subject: { title: 'payment' } }, prepared.draft,
    )).not.toThrow();
  });

  it('mechanically applies not_required and leaves omitted diagnostics pending', () => {
    const lowRisk = draft();
    lowRisk.risks[0].priority = 'P2';
    const notRequired: RiskCoverageDiagnostic = {
      ...diagnostic(), priority: 'P2', expectedStatus: 'not_required',
      message: 'risk RISK-PAYMENT coverage decision must be not_required for P2',
    };
    const preparedLowRisk = prepareQATestCaseCoveragePatch(lowRisk, [notRequired]);
    expect(preparedLowRisk.pendingDiagnostics).toEqual([]);
    expect(preparedLowRisk.draft.risks[0].coverageDecision).toEqual({ status: 'not_required' });

    const preparedOmitted = prepareQATestCaseCoveragePatch(draft(), [diagnostic()]);
    expect(preparedOmitted.pendingDiagnostics).toEqual([diagnostic()]);
    expect(preparedOmitted.draft.risks[0].coverageDecision).toEqual({ status: 'complete' });
  });

  it('rejects stale, duplicate, empty, and unknown diagnostics', () => {
    const stale = draft();
    stale.risks[0].coverageDecision = { status: 'omitted', reason: 'boundary rules are absent' };
    expect(() => prepareQATestCaseCoveragePatch(stale, [diagnostic()]))
      .toThrow('diagnostic does not match current draft');
    expect(() => prepareQATestCaseCoveragePatch(draft(), [diagnostic(), diagnostic()]))
      .toThrow('diagnostics risk IDs must be unique');
    expect(() => prepareQATestCaseCoveragePatch(draft(), []))
      .toThrow('diagnostics must not be empty');
    expect(() => prepareQATestCaseCoveragePatch(draft(), [diagnostic('RISK-UNKNOWN')]))
      .toThrow(QATestCaseCoveragePatchError);
  });
});

describe('applyQATestCaseCoveragePatch', () => {
  it('builds conservative case fields while preserving every model semantic string', () => {
    const original = draft();
    const before = structuredClone(original);
    const patch = completePatch();
    const repaired = applyQATestCaseCoveragePatch(original, [diagnostic()], patch);

    expect(original).toEqual(before);
    expect(repaired.cases.slice(0, 2)).toEqual(before.cases);
    const added = repaired.cases.slice(2);
    expect(added.map((item) => item.category)).toEqual(['positive', 'boundary']);
    expect(added[0]).toEqual({
      title: `[${riskId}/positive] positive payment scenario`,
      category: 'positive',
      priority: 'P1',
      contextRefs: [],
      riskLinks: [{ riskId, contextRef }],
      preconditions: [],
      testData: [],
      actions: [{
        step: 'submit the positive payment request',
        expectedResult: 'the page displays the positive payment status field',
      }],
      notes: [],
      smoke: false,
      automationCandidate: false,
      releaseBlocking: true,
    });
    expect(repaired.risks[0].coverageDecision).toEqual({ status: 'complete' });
    expect(() => compileQATestCaseDraft(
      { subject: { title: 'payment' } }, repaired,
    )).not.toThrow();
  });

  it('preserves an explicit omission reason verbatim and never invents one', () => {
    const reason = 'the input defines a success result but no boundary value outcome';
    const repaired = applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{
        riskId,
        resolutions: [caseResolution('positive'), { category: 'boundary', kind: 'omit' }],
        omissionReason: reason,
      }],
    });

    expect(repaired.risks[0].coverageDecision).toEqual({ status: 'omitted', reason });
    expect(repaired.cases.at(-1)?.actions[0].step)
      .toBe('submit the positive payment request');
    expect(() => compileQATestCaseDraft(
      { subject: { title: 'payment' } }, repaired,
    )).not.toThrow();

    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{
        riskId,
        resolutions: [caseResolution('positive'), { category: 'boundary', kind: 'omit' }],
        omissionReason: null,
      }],
    })).toThrow('must provide an omission reason');
    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{
        ...completePatch().repairs[0], omissionReason: 'unused semantic content',
      }],
    })).toThrow('must not provide an omission reason');
  });

  it('requires repair IDs and resolution categories to match diagnostics in order', () => {
    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{ ...completePatch().repairs[0], riskId: 'RISK-EXTRA' }],
    })).toThrow('repairs must exactly match semantic diagnostics in order');
    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{
        riskId,
        resolutions: [caseResolution('boundary'), caseResolution('positive')],
        omissionReason: null,
      }],
    })).toThrow('resolutions must exactly match missing categories in order');
    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{
        riskId,
        resolutions: [caseResolution('positive'), caseResolution('positive')],
        omissionReason: null,
      }],
    })).toThrow('resolutions must exactly match missing categories in order');
  });

  it('rejects a canonical ref not declared by the repaired risk without mutation', () => {
    const original = draft();
    const before = structuredClone(original);
    const patch = completePatch();
    const positive = patch.repairs[0].resolutions[0];
    if (positive.kind !== 'case') throw new Error('expected a case resolution');
    positive.contextRef = 'subject:description';

    expect(() => applyQATestCaseCoveragePatch(original, [diagnostic()], patch))
      .toThrow('case context ref must be declared by the repaired risk');
    expect(original).toEqual(before);
  });

  it('adds a stable mechanical suffix when the prefixed model title collides', () => {
    const original = draft();
    original.cases[1].title = `[${riskId}/positive] positive payment scenario`;
    const repaired = applyQATestCaseCoveragePatch(original, [diagnostic()], completePatch());
    expect(repaired.cases[2].title)
      .toBe(`[${riskId}/positive] positive payment scenario [coverage-2]`);
    expect(repaired.cases[2].actions[0].step)
      .toBe('submit the positive payment request');
  });

  it.each(['系统正常', '页面结果符合预期'])(
    'rejects an unobservable compact expected result before apply: %s',
    (expectedResult) => {
      const patch = completePatch();
      const positive = patch.repairs[0].resolutions[0];
      if (positive.kind !== 'case') throw new Error('expected a case resolution');
      positive.expectedResult = expectedResult;
      expect(() => parseQATestCaseCoveragePatchText(JSON.stringify(patch)))
        .toThrow('coverage_patch_invalid_schema');
      expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], patch))
        .toThrow('coverage patch must match the strict repair schema');
    },
  );

  it('rejects legacy full-case fields and schema extensions', () => {
    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{
        riskId,
        coverageDecision: { status: 'complete' },
        addCases: [testCase('positive')],
      }],
    })).toThrow('strict repair schema');
    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], {
      repairs: [{
        ...completePatch().repairs[0], coverageOmissions: [],
      }],
    })).toThrow('strict repair schema');
  });

  it.each([
    ['reason', (patch: QATestCaseCoveragePatch) => {
      patch.repairs[0].resolutions[1] = { category: 'boundary', kind: 'omit' };
      patch.repairs[0].omissionReason = QA_TEST_CASE_COVERAGE_PATCH_REASON_SENTINEL;
    }],
    ['title', (patch: QATestCaseCoveragePatch) => {
      const item = patch.repairs[0].resolutions[0];
      if (item.kind === 'case') item.title = QA_TEST_CASE_COVERAGE_PATCH_REPLACEMENT_SENTINEL;
    }],
    ['step', (patch: QATestCaseCoveragePatch) => {
      const item = patch.repairs[0].resolutions[0];
      if (item.kind === 'case') item.step = QA_TEST_CASE_COVERAGE_PATCH_REASON_SENTINEL;
    }],
    ['expected result', (patch: QATestCaseCoveragePatch) => {
      const item = patch.repairs[0].resolutions[0];
      if (item.kind === 'case') item.expectedResult = QA_TEST_CASE_COVERAGE_PATCH_REASON_SENTINEL;
    }],
  ] as const)('rejects a fixed sentinel in compact %s', (_label, insertSentinel) => {
    const patch = completePatch();
    insertSentinel(patch);
    expect(() => applyQATestCaseCoveragePatch(draft(), [diagnostic()], patch))
      .toThrow('coverage_patch_contains_fixed_sentinel');
  });
});
