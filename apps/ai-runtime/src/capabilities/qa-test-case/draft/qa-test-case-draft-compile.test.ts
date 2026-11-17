import { describe, expect, it } from 'bun:test';
import { validateCapabilityOutput } from '../../executor.js';
import { compileQATestCaseDraft } from './qa-test-case-draft-compile.js';
import { QATestCaseDraftCompilationError } from './fixtures/qa-test-case-draft-error.js';
import { batchInput, completeDraft } from './fixtures/qa-test-case-draft-fixtures.js';

describe('compileQATestCaseDraft', () => {
  it('compiles a strict draft into a complete output accepted by the existing gate', () => {
    const input = batchInput();
    const output = compileQATestCaseDraft(input, completeDraft());

    expect(output.testCases.map((testCase) => testCase.caseId)).toEqual([
      'TC-QA-001', 'TC-QA-002', 'TC-QA-003',
    ]);
    // bun's toMatchObject writes asymmetric matchers into the received object
    // (bun 1.4.2 bug), so matcher-based checks are split into toEqual calls to
    // keep the output usable by the validator below.
    expect(output.testCases[0]).toMatchObject({
      steps: ['打开待结算订单', '提交结算'],
      expectedResults: ['页面展示订单金额 100 元', '页面展示结算成功且金额为 100 元'],
      expectedResult: '页面展示订单金额 100 元\n页面展示结算成功且金额为 100 元',
      evidence: [
        'subject.title', 'test_case_context.requirements.0.title',
        'test_case_context.tasks.0.title', 'sources.0.content',
      ],
    });
    expect(output.testCases[0].trace).toEqual(expect.arrayContaining([
      'SUBJECT-77', 'SOURCE-DERIVED-001', 'REQ-201',
      'TASK-301', 'RISK-SETTLEMENT-AMOUNT',
    ]));
    expect(output.testCases[0].notes).toEqual(expect.arrayContaining(['SOURCE-DERIVED-001 为派生标签']));
    expect(output.testCases[1]).toMatchObject({
      evidence: ['test_case_context.requirements.0.acceptance_criteria'],
    });
    expect(output.testCases[1].trace).toEqual(expect.arrayContaining(['REQ-201', 'RISK-SETTLEMENT-AMOUNT']));
    expect(output.testCaseDesign?.sourceInventory).toContainEqual({
      id: 'SRC-SOURCE-002',
      kind: 'external_source',
      status: 'gap',
      summary: '外部来源 2 不可用',
      evidence: [],
    });
    expect(output.testCaseDesign?.traceability).toContainEqual({
      traceId: 'RISK-SETTLEMENT-AMOUNT',
      caseIds: ['TC-QA-001', 'TC-QA-002', 'TC-QA-003'],
      coverageTypes: ['positive', 'negative', 'boundary'],
      gapNote: '',
    });
    expect(output.testCaseDesign).toMatchObject({
      smokeCaseIds: ['TC-QA-001'],
      executionOrder: ['TC-QA-001', 'TC-QA-002', 'TC-QA-003'],
      automationCandidateCaseIds: ['TC-QA-001', 'TC-QA-002', 'TC-QA-003'],
      releaseBlockingCaseIds: ['TC-QA-001', 'TC-QA-002'],
    });
    expect(() => validateCapabilityOutput('qa.test_cases', input, output)).not.toThrow();
  });

  it('uses explicit source assessments without inferring confirmed from non-empty input', () => {
    const draft = completeDraft();
    const assessment = draft.sourceAssessments.find((item) => item.targetRef === 'requirement:0')!;
    assessment.status = 'gap';
    assessment.summary = '验收口径仍需人工确认';
    assessment.contextRefs = [];

    const output = compileQATestCaseDraft(batchInput(), draft);

    expect(output.testCaseDesign?.sourceInventory).toContainEqual({
      id: 'SRC-REQUIREMENT-001',
      kind: 'requirement',
      status: 'gap',
      summary: '验收口径仍需人工确认',
      evidence: [],
    });
  });

  it('accepts only an explicit cross-target conflict assessment', () => {
    const draft = completeDraft();
    const assessment = draft.sourceAssessments.find((item) => item.targetRef === 'requirement:0')!;
    assessment.status = 'conflict';
    assessment.summary = '需求标题与外部规则存在冲突';
    assessment.contextRefs = ['requirement:0:title', 'source:0:content'];

    const output = compileQATestCaseDraft(batchInput(), draft);
    expect(output.testCaseDesign?.sourceInventory).toContainEqual(expect.objectContaining({
      id: 'SRC-REQUIREMENT-001',
      status: 'conflict',
      evidence: ['test_case_context.requirements.0.title', 'sources.0.content'],
    }));

    assessment.contextRefs = ['requirement:0:title', 'requirement:0:acceptance_criteria'];
    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('conflict source assessment must cite two distinct targets: requirement:0');

    assessment.contextRefs = ['subject:title', 'source:0:content'];
    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('conflict source assessment must cite its own target: requirement:0');
  });

  it('requires exactly one valid assessment for every input target', () => {
    const draft = completeDraft();
    draft.sourceAssessments = draft.sourceAssessments.filter((item) => item.targetRef !== 'source:1');
    draft.sourceAssessments.push({
      targetRef: 'requirement:0', status: 'confirmed', summary: '重复评估', contextRefs: ['requirement:0:title'],
    });
    draft.sourceAssessments.push({
      targetRef: 'source:9', status: 'gap', summary: '未知来源', contextRefs: [],
    });
    const confirmed = draft.sourceAssessments.find((item) => item.targetRef === 'task:0')!;
    confirmed.contextRefs = ['requirement:0:title'];

    let failure: unknown;
    try {
      compileQATestCaseDraft(batchInput(), draft);
    } catch (error) {
      failure = error;
    }
    expect(failure).toBeInstanceOf(QATestCaseDraftCompilationError);
    expect((failure as QATestCaseDraftCompilationError).issues).toEqual(expect.arrayContaining([
      'source assessment targets must be unique',
      'source assessment has unknown target: source:9',
      'source assessment is missing target: source:1',
      'confirmed source assessment must cite its own target: task:0',
    ]));
  });

  it('validates non-empty assumption and gap assessment refs without replacement', () => {
    const draft = completeDraft();
    const gap = draft.sourceAssessments.find((item) => item.targetRef === 'source:1')!;
    gap.contextRefs = ['source:1:content'];

    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('source assessment source:1 has unavailable context ref: source:1:content');
  });

  it('summarizes source-assessment context-reference failures without retaining references', () => {
    const draft = completeDraft();
    const gap = draft.sourceAssessments.find((item) => item.targetRef === 'source:1')!;
    gap.contextRefs = ['source:1:content', 'source:1:content'];

    let failure: unknown;
    try {
      compileQATestCaseDraft(batchInput(), draft);
    } catch (error) {
      failure = error;
    }

    expect(failure).toBeInstanceOf(QATestCaseDraftCompilationError);
    const compilation = failure as QATestCaseDraftCompilationError;
    expect(compilation.contextReferenceDiagnostics).toEqual(expect.arrayContaining([
      { code: 'duplicate_context_ref', pathCategory: 'source_assessment_context_refs' },
      { code: 'unavailable_context_ref', pathCategory: 'source_assessment_context_refs' },
    ]));
    expect(JSON.stringify(compilation.contextReferenceDiagnostics)).not.toContain('source:1:content');
  });

  it('rejects unknown, unavailable, and empty context references without substitution', () => {
    const unavailable = completeDraft();
    unavailable.cases[0].contextRefs = ['source:1:content'];
    expect(() => compileQATestCaseDraft(batchInput(), unavailable))
      .toThrow('case 1 has unavailable context ref: source:1:content');

    const unknown = completeDraft();
    unknown.risks[0].contextRefs = ['requirement:99:title'];
    expect(() => compileQATestCaseDraft(batchInput(), unknown))
      .toThrow('risk RISK-SETTLEMENT-AMOUNT has unavailable context ref: requirement:99:title');

    const emptySubject = completeDraft();
    emptySubject.cases[0].contextRefs = ['subject:title'];
    expect(() => compileQATestCaseDraft({ ...batchInput(), subject: { title: ' ' } }, emptySubject))
      .toThrow('case 1 has unavailable context ref: subject:title');
  });

  it('summarizes Draft context-reference failures without retaining references or input text', () => {
    const draft = completeDraft();
    draft.risks[0].contextRefs = ['source:1:content'];
    draft.cases[0].contextRefs = ['subject:title', 'subject:title'];
    draft.cases[1].riskLinks[0].contextRef = 'source:1:content';
    draft.cases[2].riskLinks[0].contextRef = 'requirement:99:title';

    let failure: unknown;
    try {
      compileQATestCaseDraft(batchInput(), draft);
    } catch (error) {
      failure = error;
    }

    expect(failure).toBeInstanceOf(QATestCaseDraftCompilationError);
    const compilation = failure as QATestCaseDraftCompilationError;
    expect(compilation.contextReferenceDiagnostics).toEqual(expect.arrayContaining([
      { code: 'unavailable_context_ref', pathCategory: 'risk_context_refs' },
      { code: 'duplicate_context_ref', pathCategory: 'case_context_refs' },
      { code: 'unavailable_context_ref', pathCategory: 'case_risk_links' },
      { code: 'undeclared_risk_link_context_ref', pathCategory: 'case_risk_links' },
    ]));
    expect(compilation.contextReferenceDiagnostics).toHaveLength(6);
    expect(JSON.stringify(compilation.contextReferenceDiagnostics)).not.toContain('source:1:content');
    expect(JSON.stringify(compilation.contextReferenceDiagnostics)).not.toContain('requirement:99:title');
  });

  it('requires every requirement and task to be explicitly covered by a case', () => {
    const draft = completeDraft();
    draft.cases.forEach((testCase) => {
      testCase.contextRefs = testCase.contextRefs.filter((ref) => !ref.startsWith('requirement:1:'));
    });
    draft.cases[2].riskLinks[0].contextRef = 'requirement:0:acceptance_criteria';

    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('context item is not covered: requirement:1');
  });

  it('requires coverage only for requirement/task rows with a substantive leaf', () => {
    const input = batchInput();
    (input.test_case_context.requirements as Array<Record<string, unknown>>).push({ id: 203 });
    (input.test_case_context.tasks as Array<Record<string, unknown>>).push({
      id: 302, requirement_id: 203,
    });
    const draft = completeDraft();
    draft.sourceAssessments.push(
      { targetRef: 'requirement:2', status: 'gap', summary: '需求正文为空', contextRefs: [] },
      { targetRef: 'task:1', status: 'gap', summary: '任务正文为空', contextRefs: [] },
    );

    expect(() => compileQATestCaseDraft(input, draft)).not.toThrow();
  });

  it('rejects unknown risk links and risks that are not linked by any case', () => {
    const unknown = completeDraft();
    unknown.cases[0].riskLinks = [{ riskId: 'RISK-NOT-DECLARED', contextRef: 'subject:title' }];
    expect(() => compileQATestCaseDraft(batchInput(), unknown))
      .toThrow('case 1 references unknown risk: RISK-NOT-DECLARED');

    const orphaned = completeDraft();
    orphaned.risks.push({
      riskId: 'RISK-ORPHANED',
      description: '无人覆盖的风险',
      priority: 'P2',
      contextRefs: ['subject:title'],
      coverageDecision: { status: 'not_required' },
    });
    expect(() => compileQATestCaseDraft(batchInput(), orphaned))
      .toThrow('risk is not linked by a case: RISK-ORPHANED');
  });

  it('requires each explicit risk link leaf to be available and declared by that risk', () => {
    const draft = completeDraft();
    draft.cases[0].riskLinks[0].contextRef = 'subject:title';

    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('case 1 risk link context ref is not declared by RISK-SETTLEMENT-AMOUNT: subject:title');

    draft.cases[0].riskLinks[0].contextRef = 'source:1:content';
    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('case 1 risk link has unavailable context ref: source:1:content');
  });

  it('rejects duplicate risk IDs inside one case riskLinks array', () => {
    const draft = completeDraft();
    draft.cases[0].riskLinks.push({
      riskId: 'RISK-SETTLEMENT-AMOUNT', contextRef: 'requirement:0:acceptance_criteria',
    });

    expect(() => compileQATestCaseDraft(batchInput(), draft))
      .toThrow('case 1 risk link IDs must be unique');
  });
});
