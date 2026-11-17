import { describe, expect, it } from 'bun:test';
import { validateCapabilityOutput } from '../../executor.js';
import { testCaseDesignIssues } from '../index.js';
import { completeTestCaseOutput } from './generate-test-fixtures.js';
import type { AIOutput } from '../../../shared/ai/schema.js';

describe('validateCapabilityOutput', () => {
	it('rejects an empty test-case artifact', () => {
		const output = {
			summary: '无用例', changes: [], items: [], risks: [], testCases: [], humanReviewRequired: true,
			completenessReport: null, workflowProposal: null,
		} as unknown as AIOutput;

		expect(() => validateCapabilityOutput('qa.test_cases', { subject: { title: '结算' } }, output)).toThrow('test case output is empty');
	});

  it('rejects test cases with missing, fabricated, or unavailable evidence', () => {
    const input = { subject: { title: '结算页' }, sources: [{ type: 'feishu_link', url: 'https://example.test/doc', available: false }] };
    for (const evidence of [
      [], ['subject.not_real'], ['sources[0].url'], ['subject.title proves /admin/delete exists'],
    ]) {
      const output = completeTestCaseOutput();
      output.testCases[0].evidence = evidence;
      expect(() => validateCapabilityOutput('qa.test_cases', input, output)).toThrow('evidence is missing or invalid');
    }
  });

  it('accepts test-case evidence that resolves to a supplied nested fact', () => {
    const output = completeTestCaseOutput();
    expect(() => validateCapabilityOutput('qa.test_cases', {
      subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] },
    }, output)).not.toThrow();
  });

  it('requires every requirement and task when full context coverage is enabled', () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        counts: { requirements: 2, tasks: 2 },
        requirements: [{ title: '支付' }, { title: '退款' }],
        tasks: [{ status: 'todo' }, { status: 'done' }],
      },
    };
    const output = completeTestCaseOutput('test_case_context.requirements[0].title');
    output.testCases[0].evidence.push('test_case_context.tasks[0].status');

    expect(testCaseDesignIssues(input, output)).toEqual(expect.arrayContaining([
      'test case context item is not covered: test_case_context.requirements.1',
      'test case context item is not covered: test_case_context.tasks.1',
    ]));
    expect(() => validateCapabilityOutput('qa.test_cases', input, output))
      .toThrow('test case context item is not covered');
  });

  it('does not require final-artifact coverage for empty requirement/task rows', () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        requirements: [{ title: '支付' }, { id: 2 }],
        tasks: [{ status: 'todo' }, { id: 12, requirement_id: 2 }],
      },
    };
    const output = completeTestCaseOutput('test_case_context.requirements[0].title');
    output.testCases[0].evidence.push('test_case_context.tasks[0].status');

    expect(testCaseDesignIssues(input, output)).not.toContain(
      'test case context item is not covered: test_case_context.requirements.1',
    );
    expect(testCaseDesignIssues(input, output)).not.toContain(
      'test case context item is not covered: test_case_context.tasks.1',
    );
  });

  it('lets one focused test case cover several related context facts within the deterministic limit', () => {
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        counts: { requirements: 2, tasks: 2 },
        requirements: [{ title: '支付' }, { title: '退款' }],
        tasks: [{ status: 'todo' }, { status: 'done' }],
      },
    };
    const output = completeTestCaseOutput('test_case_context.requirements[0].title');
    output.testCases[0].evidence.push(
      'test_case_context.requirements[1]',
      'test_case_context.tasks[0].status',
      'test_case_context.tasks[1].status',
    );

    expect(() => validateCapabilityOutput('qa.test_cases', input, output)).not.toThrow();
  });

  it('rejects one generic test case that claims more than eight context items', () => {
    const requirements = Array.from({ length: 9 }, (_, index) => ({ title: `需求-${index + 1}` }));
    const input = {
      subject: { title: '结算页' },
      test_case_context: {
        coverageRequired: true,
        counts: { requirements: requirements.length, tasks: 0 },
        requirements,
        tasks: [],
      },
    };
    const output = completeTestCaseOutput('test_case_context.requirements[0].title');
    output.testCases[0].evidence.push(...requirements.slice(1).map(
      (_requirement, index) => `test_case_context.requirements[${index + 1}].title`,
    ));

    expect(() => validateCapabilityOutput('qa.test_cases', input, output))
      .toThrow('test case covers too many unrelated context items');
  });

  it('rejects invalid evidence on assumption/gap sources and duplicate source or risk IDs', () => {
    const input = { subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] } };

    const invalidGap = completeTestCaseOutput();
    invalidGap.testCaseDesign!.sourceInventory[0].status = 'gap';
    invalidGap.testCaseDesign!.sourceInventory[0].evidence = ['subject.not_real'];
    expect(() => validateCapabilityOutput('qa.test_cases', input, invalidGap))
      .toThrow('source inventory evidence is missing or invalid');

    const duplicateSources = completeTestCaseOutput();
    duplicateSources.testCaseDesign!.sourceInventory.push({
      ...duplicateSources.testCaseDesign!.sourceInventory[0],
      evidence: [...duplicateSources.testCaseDesign!.sourceInventory[0].evidence],
    });
    expect(() => validateCapabilityOutput('qa.test_cases', input, duplicateSources))
      .toThrow('source inventory IDs must be unique');

    const duplicateRisks = completeTestCaseOutput();
    duplicateRisks.testCaseDesign!.coverageStrategy.riskHotspots = [
      { id: 'RISK-DUP', description: '风险一', priority: 'P2', evidence: ['subject.title'] },
      { id: 'RISK-DUP', description: '风险二', priority: 'P2', evidence: ['subject.title'] },
    ];
    expect(() => validateCapabilityOutput('qa.test_cases', input, duplicateRisks))
      .toThrow('risk hotspot IDs must be unique');
  });

  it('requires conflict sources to cite two distinct canonical fact paths', () => {
    const input = {
      subject: { title: '结算页' },
      sources: [{ available: true, content: '一份需求' }],
      test_case_context: { tasks: [{ status: 'todo' }] },
    };
    const output = completeTestCaseOutput();
    output.testCaseDesign!.sourceInventory[0].status = 'conflict';
    output.testCaseDesign!.sourceInventory[0].evidence = ['sources[0].content', 'sources.0.content'];
    expect(() => validateCapabilityOutput('qa.test_cases', input, output))
      .toThrow('source conflict must cite at least two facts');
  });

  it('rejects a high-risk trace without positive, negative, and boundary protection or an explicit gap', () => {
    const output = completeTestCaseOutput();
    output.testCaseDesign!.coverageStrategy.riskHotspots = [{
      id: 'REQ-TASK-STATUS', description: '任务状态错误会影响发布', priority: 'P0', evidence: ['subject.title'],
    }];
    expect(() => validateCapabilityOutput('qa.test_cases', {
      subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] },
    }, output)).toThrow('high-risk coverage omission is unexplained');
  });

  it('rejects duplicate execution-order entries even when every case ID is present', () => {
    const output = completeTestCaseOutput();
    output.testCaseDesign!.executionOrder = ['TC-TASK-001', 'TC-TASK-001'];
    expect(() => validateCapabilityOutput('qa.test_cases', {
      subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] },
    }, output)).toThrow('execution order must include every test case exactly once');
  });

  it('rejects traceability rows whose cases declare a different trace', () => {
    const output = completeTestCaseOutput();
    const secondCase = {
      ...output.testCases[0], caseId: 'TC-TASK-002', title: '任务状态异常', trace: ['REQ-B'],
    };
    output.testCases[0].trace = ['REQ-A'];
    output.testCases = [output.testCases[0], secondCase];
    output.testCaseDesign!.traceability = [
      { traceId: 'REQ-A', caseIds: ['TC-TASK-002'], coverageTypes: ['regression'], gapNote: '' },
      { traceId: 'REQ-B', caseIds: ['TC-TASK-001'], coverageTypes: ['regression'], gapNote: '' },
    ];
    output.testCaseDesign!.executionOrder = ['TC-TASK-001', 'TC-TASK-002'];
    output.testCaseDesign!.automationCandidateCaseIds = ['TC-TASK-001', 'TC-TASK-002'];

    expect(() => validateCapabilityOutput('qa.test_cases', {
      subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] },
    }, output)).toThrow(/traceability row is not declared by its cases|trace is absent from the matrix/);
  });

  it('rejects duplicate traceability IDs', () => {
    const output = completeTestCaseOutput();
    output.testCaseDesign!.traceability.push({
      ...output.testCaseDesign!.traceability[0],
      caseIds: [...output.testCaseDesign!.traceability[0].caseIds],
      coverageTypes: [...output.testCaseDesign!.traceability[0].coverageTypes],
    });
    expect(() => validateCapabilityOutput('qa.test_cases', {
      subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] },
    }, output)).toThrow('traceability IDs must be unique');
  });

  it('rejects a traceability row that omits an actual case category', () => {
    const output = completeTestCaseOutput();
    output.testCases.push({
      ...output.testCases[0], caseId: 'TC-TASK-002', title: '任务状态正向', category: 'positive',
    });
    output.testCaseDesign!.traceability[0].caseIds.push('TC-TASK-002');
    output.testCaseDesign!.executionOrder.push('TC-TASK-002');
    output.testCaseDesign!.automationCandidateCaseIds.push('TC-TASK-002');
    expect(() => validateCapabilityOutput('qa.test_cases', {
      subject: { title: '结算页' }, test_case_context: { tasks: [{ status: 'todo' }] },
    }, output)).toThrow('traceability coverage types do not match cases');
  });
});
