import { describe, expect, it } from 'bun:test';
import { compileQATestCaseGlobalPlan } from './qa-test-case-global-plan.js';
import {
  QA_TEST_CASE_GLOBAL_PLAN_REPLACEMENT_SENTINEL,
  qaTestCaseGlobalPlanSchema,
} from './qa-test-case-global-plan-schema.js';
import {
  completePlan,
  complexInput,
  expectIssues,
  nestedCase,
  syntheticInput,
  unlinkedTaskInput,
} from './qa-test-case-global-plan-fixtures.js';

describe('compileQATestCaseGlobalPlan nested IR', () => {
  it('rejects any unreplaced skeleton sentinel', () => {
    const plan = completePlan();
    plan.interactions[0].description = QA_TEST_CASE_GLOBAL_PLAN_REPLACEMENT_SENTINEL;
    expect(() => qaTestCaseGlobalPlanSchema.parse(plan)).toThrow(
      'global plan text must replace the fixed skeleton sentinel',
    );
  });

  it('generates deterministic risks and flattens nested cases without changing public shape', () => {
    const plan = completePlan();
    plan.interactions.push({
      description: '账务结果与审计记录可能不一致',
      priority: 'P2',
      relationshipRef: 'subject:description',
      coverageDecision: { status: 'not_required' },
      cases: [nestedCase(
        '订单与审计记录可关联',
        ['requirement:0:title', 'requirement:2:description'],
      )],
    });

    const compiled = compileQATestCaseGlobalPlan(complexInput(), plan);
    expect(compiled.interactions.map((item) => item.riskId)).toEqual([
      'RISK-GLOBAL-001', 'RISK-GLOBAL-002',
    ]);
    expect(compiled.cases.map((item) => [item.title, item.executionPosition])).toEqual([
      ['结算后以同一流水完成一次入账', 'before_batches'],
      ['结算和入账结果写入审计', 'after_batches'],
      ['订单与审计记录可关联', 'after_batches'],
    ]);
    expect(compiled.cases[0]).toMatchObject({
      traceIds: ['RISK-GLOBAL-001', 'REQ-11', 'REQ-22', 'SOURCE-DERIVED-001'],
      steps: ['提交订单结算'],
      expectedResults: ['账务生成唯一对应入账记录'],
    });
    expect(compiled.interactions[0]).not.toHaveProperty('coverageDecision');
    expect(compiled.interactions[0]).not.toHaveProperty('cases');
  });

  it('aggregates interaction evidence but never leaks sibling refs into a case', () => {
    const compiled = compileQATestCaseGlobalPlan(complexInput(), completePlan());

    expect(compiled.interactions[0].evidence).toEqual([
      'sources.0.content',
      'test_case_context.requirements.0.description',
      'test_case_context.requirements.1.acceptance_criteria',
      'test_case_context.tasks.1.title',
      'test_case_context.tasks.2.title',
    ]);
    expect(compiled.cases[0].evidence).toEqual([
      'test_case_context.requirements.0.description',
      'test_case_context.requirements.1.acceptance_criteria',
      'sources.0.content',
    ]);
    expect(compiled.cases[0].evidence).not.toContain('test_case_context.tasks.2.title');
    expect(compiled.cases[1].evidence).not.toContain(
      'test_case_context.requirements.0.description',
    );
  });

  it('requires every nested case to cite at least two requirement groups', () => {
    const oneGroup = completePlan();
    oneGroup.interactions[0].cases[0].contextRefs = [
      'requirement:0:title', 'task:0:title',
    ];
    expectIssues(oneGroup, ['global case 1 must reference at least two requirement groups']);

    const unlinkedTask = completePlan();
    unlinkedTask.interactions[0].cases[0].contextRefs = [
      'requirement:1:title', 'task:0:title',
    ];
    unlinkedTaskInput(unlinkedTask);
  });

  it('rejects unavailable leaves and relationships without nearby substitution', () => {
    const leaf = completePlan();
    leaf.interactions[0].cases[0].contextRefs = [
      'requirement:0:title', 'requirement:99:title',
    ];
    expectIssues(leaf, ['global case 1 has unavailable context ref: requirement:99:title']);

    const relationship = completePlan();
    relationship.interactions[0].relationshipRef = 'source:9:content';
    expectIssues(relationship, [
      'interaction RISK-GLOBAL-001 has unavailable relationship ref: source:9:content',
    ]);
  });

  it('rejects duplicate explicit refs and duplicate titles across interactions', () => {
    const duplicateRefs = completePlan();
    duplicateRefs.interactions[0].cases[0].contextRefs.push('requirement:0:description');
    expectIssues(duplicateRefs, ['global case 1 context refs must be unique']);

    const duplicateTitles = completePlan();
    duplicateTitles.interactions.push({
      description: '另一个跨组风险', priority: 'P2', relationshipRef: 'subject:description',
      coverageDecision: { status: 'not_required' },
      cases: [nestedCase(
        duplicateTitles.interactions[0].cases[0].title,
        ['requirement:0:title', 'requirement:2:title'],
      )],
    });
    expectIssues(duplicateTitles, ['global case titles must be unique']);
  });

  it('accepts an explicit no-interaction conclusion and requires its reason', () => {
    const plan = completePlan();
    plan.interactions = [];
    plan.noCrossInteractionReason = '现有事实无法证明跨需求交互';
    expect(compileQATestCaseGlobalPlan(complexInput(), plan).cases).toEqual([]);

    plan.noCrossInteractionReason = '';
    expect(() => qaTestCaseGlobalPlanSchema.parse(plan)).toThrow();
  });

  it.each(['P0', 'P1'] as const)('requires %s coverage omission reason', (priority) => {
    const valid = completePlan();
    valid.interactions[0].priority = priority;
    expect(() => qaTestCaseGlobalPlanSchema.parse(valid)).not.toThrow();

    valid.interactions[0].coverageDecision = { status: 'not_required' };
    expect(() => qaTestCaseGlobalPlanSchema.parse(valid)).toThrow(
      'P0/P1 interaction must explicitly explain omitted category coverage',
    );
  });

  it.each(['P2', 'P3'] as const)('requires %s not_required decision', (priority) => {
    const valid = completePlan();
    valid.interactions[0].priority = priority;
    valid.interactions[0].coverageDecision = { status: 'not_required' };
    expect(compileQATestCaseGlobalPlan(complexInput(), valid).interactions[0].gapNote).toBe('');
  });

  it('rejects old top-level arrays, interaction IDs/refs, and nested risk links', () => {
    const legacy = completePlan() as unknown as Record<string, unknown>;
    legacy.cases = [];
    const interaction = (legacy.interactions as Array<Record<string, unknown>>)[0];
    interaction.riskId = 'RISK-LEGACY';
    interaction.contextRefs = ['requirement:0:title', 'requirement:1:title'];
    const nested = (interaction.cases as Array<Record<string, unknown>>)[0];
    nested.riskLinks = [{ riskId: 'RISK-LEGACY', contextRef: 'requirement:0:title' }];
    expect(() => qaTestCaseGlobalPlanSchema.parse(legacy)).toThrow();
  });

  it('rejects more than twelve nested cases across interactions', () => {
    const plan = completePlan();
    plan.interactions[0].cases = Array.from({ length: 12 }, (_, index) => nestedCase(
      `integration ${index + 1}`,
      ['requirement:0:title', 'requirement:1:title'],
    ));
    plan.interactions.push({
      description: '额外交互', priority: 'P2', relationshipRef: 'subject:description',
      coverageDecision: { status: 'not_required' },
      cases: [nestedCase('thirteenth', ['requirement:1:title', 'requirement:2:title'])],
    });
    expect(() => qaTestCaseGlobalPlanSchema.parse(plan)).toThrow(
      'global plan must contain at most 12 nested cases',
    );
  });

  it('compiles a bounded 13 requirement plus 13 task input without cross-array keys', () => {
    const input = syntheticInput(13);
    const plan = completePlan();
    plan.sourceFindings = [];
    plan.interactions[0].relationshipRef = 'subject:description';
    plan.interactions[0].cases = Array.from({ length: 12 }, (_, index) => nestedCase(
      `journey ${index + 1}`,
      [`requirement:${index}:title`, `requirement:${index + 1}:title`],
    ));
    const compiled = compileQATestCaseGlobalPlan(input, plan);
    expect(compiled.cases).toHaveLength(12);
    expect(compiled.interactions[0].contextRefs).toHaveLength(13);
    expect(compiled.cases.every((item) => item.traceIds[0] === 'RISK-GLOBAL-001')).toBe(true);
  });

  it('requires conflicts to cite two distinct source targets', () => {
    const plan = completePlan();
    plan.sourceFindings[0].contextRefs = ['source:0:content'];
    expectIssues(plan, ['source finding 1 conflict must cite at least two distinct context targets']);
  });
});
