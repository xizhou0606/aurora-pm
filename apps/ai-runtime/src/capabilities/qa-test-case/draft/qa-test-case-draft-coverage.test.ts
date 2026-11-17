import { describe, expect, it } from 'bun:test';
import { compileQATestCaseDraft } from './qa-test-case-draft-compile.js';
import { QATestCaseDraftCompilationError } from './fixtures/qa-test-case-draft-error.js';
import {
  qaTestCaseDraftSchema,
  qaTestCaseRequirementGroupLedger,
} from './fixtures/qa-test-case-draft-schema.js';
import {
  batchInput,
  completeDraft,
  multiGroupDraft,
  multiGroupInput,
} from './fixtures/qa-test-case-draft-fixtures.js';

describe('compileQATestCaseDraft', () => {
  it('compiles one explicit omission reason for mechanically calculated missing categories', () => {
    const draft = completeDraft();
    draft.cases[2].category = 'regression';

    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('coverage decision must be omitted for missing categories: boundary');

    draft.risks[0].coverageDecision = {
      status: 'omitted', reason: '当前需求未给出可判定的金额上边界',
    };
    const output = compileQATestCaseDraft(batchInput(), draft);
    expect(output.testCaseDesign?.traceability).toContainEqual(expect.objectContaining({
      traceId: 'RISK-SETTLEMENT-AMOUNT',
      gapNote: 'boundary: 当前需求未给出可判定的金额上边界',
    }));
  });

  it('orders multiple mechanically calculated missing categories deterministically', () => {
    const draft = completeDraft();
    draft.cases[1].category = 'regression';
    draft.cases[2].category = 'state';
    draft.risks[0].coverageDecision = {
      status: 'omitted', reason: '输入未提供异常与边界判定规则',
    };

    const output = compileQATestCaseDraft(batchInput(), draft);
    expect(output.testCaseDesign?.traceability).toContainEqual(expect.objectContaining({
      traceId: 'RISK-SETTLEMENT-AMOUNT',
      gapNote: 'negative, boundary: 输入未提供异常与边界判定规则',
    }));
  });

  it('rejects incompatible or reasonless high-risk coverage decisions without defaults', () => {
    const extra = completeDraft();
    extra.risks[0].coverageDecision = { status: 'omitted', reason: '不应存在的声明' };
    expect(() => compileQATestCaseDraft(batchInput(), extra))
      .toThrow('coverage decision must be complete when no categories are missing');

    const notRequired = completeDraft();
    notRequired.cases[2].category = 'regression';
    notRequired.risks[0].coverageDecision = { status: 'not_required' };
    expect(() => compileQATestCaseDraft(batchInput(), notRequired))
      .toThrow('coverage decision must be omitted for missing categories: boundary');

    const reasonless = completeDraft() as unknown as {
      risks: Array<{ coverageDecision: { status: string; reason: string } }>;
    };
    reasonless.risks[0].coverageDecision = { status: 'omitted', reason: '' };
    expect(() => qaTestCaseDraftSchema.parse(reasonless)).toThrow();
  });

  it('rejects legacy riskIds, coverageOmissions, and decision branch extras', () => {
    const legacy = completeDraft() as unknown as {
      risks: Array<Record<string, unknown>>;
      cases: Array<Record<string, unknown>>;
    };
    legacy.risks[0].coverageOmissions = [];
    delete legacy.risks[0].coverageDecision;
    legacy.cases[0].riskIds = ['RISK-SETTLEMENT-AMOUNT'];
    delete legacy.cases[0].riskLinks;
    expect(() => qaTestCaseDraftSchema.parse(legacy)).toThrow();

    const branchExtra = completeDraft() as unknown as { risks: Array<Record<string, unknown>> };
    branchExtra.risks[0].coverageDecision = { status: 'complete', reason: '不允许的字段' };
    expect(() => qaTestCaseDraftSchema.parse(branchExtra)).toThrow();
  });

  it('requires not_required for P2/P3 instead of creating low-risk exemptions', () => {
    for (const priority of ['P2', 'P3'] as const) {
      const draft = completeDraft();
      draft.risks[0].priority = priority;
      draft.cases[2].category = 'regression';
      draft.risks[0].coverageDecision = { status: 'complete' };
      expect(() => compileQATestCaseDraft(batchInput(), draft))
        .toThrow(`coverage decision must be not_required for ${priority}`);

      draft.risks[0].coverageDecision = { status: 'not_required' };
      const output = compileQATestCaseDraft(batchInput(), draft);
      expect(output.testCaseDesign?.traceability).toContainEqual(expect.objectContaining({
        traceId: 'RISK-SETTLEMENT-AMOUNT', gapNote: '',
      }));
    }
  });

  it('rejects one positive case laundering coverage across three requirement groups', () => {
    expect(() => compileQATestCaseDraft(multiGroupInput(), multiGroupDraft()))
      .toThrow('case 1 spans multiple requirement groups without integration relationship evidence');
  });

  it('reports bounded canonical effective-ref sources for every covered group', () => {
    let failure: unknown;
    try {
      compileQATestCaseDraft(multiGroupInput(), multiGroupDraft());
    } catch (error) {
      failure = error;
    }
    const issue = (failure as QATestCaseDraftCompilationError).issues.find((value) => (
      value.includes('spans multiple requirement groups')
    ));

    expect(issue).toContain(
      'requirement:0=[cases[0].contextRefs[2]->task:0:title,cases[0].riskLinks[0].contextRef->requirement:0:title]',
    );
    expect(issue).toContain(
      'requirement:1=[cases[0].contextRefs[0]->requirement:1:title,cases[0].contextRefs[3]->task:1:title]',
    );
    expect(issue).not.toContain('结算需求');

    let boundedFailure: unknown;
    try {
      compileQATestCaseDraft(multiGroupInput(7), multiGroupDraft(7));
    } catch (error) {
      boundedFailure = error;
    }
    expect(boundedFailure).toBeInstanceOf(QATestCaseDraftCompilationError);
    const bounded = (boundedFailure as QATestCaseDraftCompilationError).issues.find((value) => (
      value.includes('spans multiple requirement groups')
    ));
    expect(bounded).toContain('...(+1 groups)');
    expect(bounded!.length).toBeLessThan(1400);
  });

  it('uses the same canonical ID for numeric requirements and string task links', () => {
    const base = batchInput();
    const input = {
      ...base,
      test_case_context: {
        ...base.test_case_context,
        tasks: [{ id: 301, requirement_id: '201', title: '实现结算校验' }],
      },
    };

    expect(qaTestCaseRequirementGroupLedger(input)).toEqual([
      { targetRef: 'requirement:0', group: 'requirement:0' },
      { targetRef: 'requirement:1', group: 'requirement:1' },
      { targetRef: 'task:0', group: 'requirement:0' },
    ]);
    expect(() => compileQATestCaseDraft(input, completeDraft())).not.toThrow();
  });

  it('fails closed when number and string requirements have the same canonical ID', () => {
    const base = batchInput();
    const input = {
      ...base,
      test_case_context: {
        ...base.test_case_context,
        requirements: [
          base.test_case_context.requirements[0],
          { ...base.test_case_context.requirements[1], id: '201' },
        ],
      },
    };

    expect(() => qaTestCaseRequirementGroupLedger(input))
      .toThrow('qa test case context has duplicate canonical requirement IDs');
    expect(() => compileQATestCaseDraft(input, completeDraft()))
      .toThrow('qa test case context has duplicate canonical requirement IDs');
  });

  it('groups equal orphan links deterministically without exposing their raw ID', () => {
    const input = {
      test_case_context: {
        requirements: [],
        tasks: [
          { requirement_id: 99 },
          { requirement_id: '99' },
          { requirement_id: 'missing-two' },
          { requirement_id: null },
        ],
      },
    };

    expect(qaTestCaseRequirementGroupLedger(input)).toEqual([
      { targetRef: 'task:0', group: 'orphan-requirement:0' },
      { targetRef: 'task:1', group: 'orphan-requirement:0' },
      { targetRef: 'task:2', group: 'orphan-requirement:1' },
      { targetRef: 'task:3', group: 'task:3' },
    ]);
    expect(JSON.stringify(qaTestCaseRequirementGroupLedger(input))).not.toContain('missing-two');
  });

  it('allows cross-group coverage only for an explicit integration user journey relationship', () => {
    const draft = multiGroupDraft();
    draft.methods = ['user_journey'];
    draft.cases[0].category = 'integration';
    draft.cases[0].contextRefs.push('subject:description');

    const output = compileQATestCaseDraft(multiGroupInput(), draft);

    expect(output.testCases[0].category).toBe('integration');
    expect(output.testCases[0].evidence).toContain('subject.description');
    expect(output.testCaseDesign?.executionOrder).toEqual(['TC-QA-001']);
  });

  it('rejects relationship evidence supplied only by a risk link', () => {
    const draft = multiGroupDraft();
    draft.methods = ['user_journey'];
    draft.cases[0].category = 'integration';
    draft.risks[0].contextRefs.push('subject:description');
    draft.cases[0].riskLinks[0].contextRef = 'subject:description';

    expect(() => compileQATestCaseDraft(multiGroupInput(), draft))
      .toThrow('spans multiple requirement groups without integration relationship evidence');
  });

  it('rejects an integration case when either relationship fact or user-journey method is absent', () => {
    const missingRelationship = multiGroupDraft();
    missingRelationship.methods = ['user_journey'];
    missingRelationship.cases[0].category = 'integration';
    expect(() => compileQATestCaseDraft(multiGroupInput(), missingRelationship))
      .toThrow('spans multiple requirement groups');

    const missingMethod = multiGroupDraft();
    missingMethod.cases[0].category = 'integration';
    missingMethod.cases[0].contextRefs.push('subject:description');
    expect(() => compileQATestCaseDraft(multiGroupInput(), missingMethod))
      .toThrow('spans multiple requirement groups');
  });
});
