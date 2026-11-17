import { describe, expect, it } from 'bun:test';
import { ZodError } from 'zod';
import { compileQATestCaseDraft } from './qa-test-case-draft-compile.js';
import { QATestCaseDraftCompilationError } from './fixtures/qa-test-case-draft-error.js';
import {
  allowedQATestCaseContextRefs,
  qaTestCaseDraftSchema,
} from './fixtures/qa-test-case-draft-schema.js';
import { batchInput, completeDraft } from './fixtures/qa-test-case-draft-fixtures.js';
import type { QATestCaseDraft } from './fixtures/qa-test-case-draft-schema.js';

describe('compileQATestCaseDraft', () => {
  it('keeps task-only continuation rows with the same requirement_id in one group', () => {
    const input = {
      subject: { title: '任务续批' },
      test_case_context: {
        coverageRequired: true,
        requirements: [],
        tasks: [
          { id: 1, requirement_id: 99, title: '任务一' },
          { id: 2, requirement_id: 99, title: '任务二' },
        ],
      },
    };
    const draft: QATestCaseDraft = {
      summary: '任务续批测试', objective: '验证同需求任务', scope: ['任务'],
      exclusions: [], assumptions: [], informationGaps: [],
      sourceAssessments: [
        { targetRef: 'subject', status: 'confirmed', summary: '主体', contextRefs: ['subject:title'] },
        { targetRef: 'task:0', status: 'confirmed', summary: '任务一', contextRefs: ['task:0:title'] },
        { targetRef: 'task:1', status: 'confirmed', summary: '任务二', contextRefs: ['task:1:title'] },
      ],
      priorityLogic: '同需求任务顺序执行', methods: ['state_transition'],
      risks: [{
        riskId: 'RISK-TASK-CONTINUATION', description: '同需求任务状态不一致', priority: 'P2',
        contextRefs: ['task:0:title'],
        coverageDecision: { status: 'not_required' },
      }],
      cases: [{
        title: '执行同需求的两个任务', category: 'positive', priority: 'P2',
        contextRefs: ['task:1:title'],
        riskLinks: [{ riskId: 'RISK-TASK-CONTINUATION', contextRef: 'task:0:title' }],
        preconditions: [], testData: [],
        actions: [{ step: '依次执行两个任务', expectedResult: '分别展示两个任务的完成结果' }],
        notes: [], smoke: true, automationCandidate: false, releaseBlocking: false,
      }],
    };

    expect(() => compileQATestCaseDraft(input, draft)).not.toThrow();
  });

  it('preserves paired actions and rejects an empty side before compilation', () => {
    const output = compileQATestCaseDraft(batchInput(), completeDraft());
    expect(output.testCases[0].steps).toHaveLength(output.testCases[0].expectedResults.length);

    const invalid = completeDraft() as unknown as Record<string, unknown>;
    const cases = invalid.cases as Array<Record<string, unknown>>;
    const actions = cases[0].actions as Array<Record<string, unknown>>;
    actions[0].expectedResult = '';
    expect(() => compileQATestCaseDraft(batchInput(), invalid)).toThrow();
  });

  it.each(['系统正常', '页面结果符合预期'])(
    'rejects an unobservable expected result at the Draft schema boundary: %s',
    (expectedResult) => {
      const invalid = completeDraft();
      invalid.cases[0].actions[0].expectedResult = expectedResult;

      expect(qaTestCaseDraftSchema.safeParse(invalid).success).toBe(false);
      expect(() => compileQATestCaseDraft(batchInput(), invalid))
        .toThrow(ZodError);
    },
  );

  it.each([
    ['preconditions', ' 系统已有数据 '],
    ['testData', '准备测试数据'],
  ] as const)('rejects non-reproducible %s at the Draft schema boundary', (field, setup) => {
    const invalid = completeDraft();
    invalid.cases[0][field] = [setup];

    expect(qaTestCaseDraftSchema.safeParse(invalid).success).toBe(false);
  });

  it('routes text-quality plus coverage mismatch through structural repair before coverage diagnostics', () => {
    const invalid = completeDraft();
    invalid.cases[0].actions[0].expectedResult = '系统正常';
    invalid.cases[2].category = 'regression';

    expect(qaTestCaseDraftSchema.safeParse(invalid).success).toBe(false);
    expect(() => compileQATestCaseDraft(batchInput(), invalid))
      .toThrow(ZodError);
  });

  it('accepts concrete observable assertions and reproducible setup', () => {
    const valid = completeDraft();
    valid.cases[0].preconditions = ['存在订单号为 SETTLE-100 的待结算订单'];
    valid.cases[0].testData = ['订单 SETTLE-100 的结算金额为 100 元'];
    valid.cases[0].actions[0].expectedResult = '页面显示订单 SETTLE-100，金额字段为 100 元';

    expect(qaTestCaseDraftSchema.safeParse(valid).success).toBe(true);
    expect(() => compileQATestCaseDraft(batchInput(), valid)).not.toThrow();
  });

  it('rejects unknown enums instead of mapping aliases', () => {
    const invalid = completeDraft() as unknown as { methods: string[] };
    invalid.methods = ['边界值'];
    expect(() => qaTestCaseDraftSchema.parse(invalid)).toThrow();
  });

  it('exposes only non-empty substantive leaf refs and rejects object or identity refs', () => {
    expect(allowedQATestCaseContextRefs(batchInput())).toEqual([
      'subject:title',
      'source:0:content',
      'requirement:0:title',
      'requirement:0:acceptance_criteria',
      'requirement:1:title',
      'task:0:title',
    ]);
    const unconfirmedSource = { ...batchInput(), sources: [{ content: '有正文但没有明确可用状态' }] };
    expect(allowedQATestCaseContextRefs(unconfirmedSource)).not.toContain('source:0:content');
    for (const ref of ['subject', 'source:0', 'source:0:url', 'requirement:0', 'task:0:id']) {
      const invalid = completeDraft() as unknown as { cases: Array<{ contextRefs: string[] }> };
      invalid.cases[0].contextRefs = [ref];
      expect(() => qaTestCaseDraftSchema.parse(invalid)).toThrow();
    }
  });

  it('aggregates independently repairable semantic issues in one failure', () => {
    const draft = completeDraft();
    draft.methods.push('user_journey');
    draft.cases.forEach((testCase) => {
      testCase.smoke = false;
      testCase.contextRefs = testCase.contextRefs.filter((ref) => (
        !ref.startsWith('requirement:1:') && !ref.startsWith('task:0:')
      ));
    });
    draft.cases[0].contextRefs = ['subject:title', 'subject:title', 'source:1:content'];
    draft.cases[2].contextRefs = ['requirement:0:acceptance_criteria'];
    draft.cases[0].riskLinks = [{ riskId: 'RISK-NOT-DECLARED', contextRef: 'subject:title' }];
    draft.cases[2].riskLinks[0].contextRef = 'requirement:0:acceptance_criteria';
    draft.cases[1].title = draft.cases[0].title;
    draft.risks.push({
      riskId: 'RISK-ORPHANED',
      description: '未关联用例的风险',
      priority: 'P2',
      contextRefs: ['subject:title'],
      coverageDecision: { status: 'not_required' },
    });

    let failure: unknown;
    try {
      compileQATestCaseDraft(batchInput(), draft);
    } catch (error) {
      failure = error;
    }

    expect(failure).toBeInstanceOf(QATestCaseDraftCompilationError);
    expect((failure as QATestCaseDraftCompilationError).issues).toEqual(expect.arrayContaining([
      'methods must be unique',
      'case titles must be unique',
      'at least one smoke case must be selected',
      'case 1 context refs must be unique',
      'case 1 has unavailable context ref: source:1:content',
      'case 1 references unknown risk: RISK-NOT-DECLARED',
      'risk is not linked by a case: RISK-ORPHANED',
      'context item is not covered: requirement:1',
      'context item is not covered: task:0',
    ]));
  });
});
