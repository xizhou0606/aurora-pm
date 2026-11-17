import { describe, expect, it } from 'bun:test';
import { combineTokenUsages } from '../generate/runtime/generate-telemetry.js';
import { validateCapabilityOutput } from '../../executor.js';
import { mergeQATestCaseBatchOutputs } from './qa-test-case-batching-merge.js';
import { partitionQATestCaseInput } from './qa-test-case-batching-partition.js';
import { complexInput, outputForBatch } from './qa-test-case-batching-fixtures.js';
import { compileQATestCaseGlobalPlan } from '../global-plan/qa-test-case-global-plan.js';

describe('mergeQATestCaseBatchOutputs', () => {
  it('merges a single requirement across task-only continuation batches without a Global Plan', () => {
    const input = {
      subject: { title: '批量修改时间和负责人' },
      sources: [{
        url: 'https://example.feishu.cn/docx/requirement-141',
        content: '批量修改规则和验收说明',
      }],
      test_case_context: {
        coverageRequired: true,
        requirements: [{
          id: 141,
          title: '批量修改时间和负责人',
          product_doc_url: 'https://example.feishu.cn/docx/requirement-141',
        }],
        tasks: Array.from({ length: 15 }, (_, index) => ({
          id: index + 1,
          requirement_id: 141,
          title: `任务 ${index + 1}`,
        })),
      },
    };
    const batches = partitionQATestCaseInput(input);
    const rows = batches.map((batch, index) => {
      const context = batch.input.test_case_context as {
        requirements: unknown[];
        tasks: unknown[];
      };
      const output = outputForBatch(
        index + 1,
        context.requirements.length,
        context.tasks.length,
      );
      expect(() => validateCapabilityOutput('qa.test_cases', batch.input, output)).not.toThrow();
      return { batch, output };
    });

    expect(batches).toHaveLength(2);
    expect(batches[1].globalIndices.requirements).toEqual([]);
    const merged = mergeQATestCaseBatchOutputs(rows);

    expect(merged.testCases).toHaveLength(2);
    expect(merged.testCases.every((testCase) => !testCase.caseId.startsWith('TC-GLOBAL-')))
      .toBe(true);
    expect(() => validateCapabilityOutput('qa.test_cases', input, merged)).not.toThrow();
  });

  it('rewrites local evidence indexes, namespaces colliding IDs, and passes global coverage', () => {
    const input = complexInput(13);
    const batches = partitionQATestCaseInput(input);
    expect(batches).toHaveLength(3);
    const rows = batches.map((batch, index) => {
      const context = batch.input.test_case_context as { requirements: unknown[]; tasks: unknown[] };
      const output = outputForBatch(index + 1, context.requirements.length, context.tasks.length);
      expect(() => validateCapabilityOutput('qa.test_cases', batch.input, output)).not.toThrow();
      return { batch, output };
    });
    rows[1].output.testCases[0].title = rows[0].output.testCases[0].title;

    const merged = mergeQATestCaseBatchOutputs(rows);

    expect(merged.testCases.map((testCase) => testCase.caseId)).toEqual([
      'TC-B01-FLOW-001', 'TC-B01-FLOW-002', 'TC-B02-FLOW-001',
      'TC-B03-FLOW-001',
    ]);
    expect(merged.testCases[3].evidence).toEqual([
      'test_case_context.requirements.9.title',
      'test_case_context.requirements.10.title',
      'test_case_context.requirements.11.title',
      'test_case_context.requirements.12.title',
      'test_case_context.tasks.9.title',
      'test_case_context.tasks.10.title',
      'test_case_context.tasks.11.title',
      'test_case_context.tasks.12.title',
    ]);
    expect(merged.testCaseDesign?.sourceInventory.map((source) => source.id)).toEqual([
      'B01-SRC-SAME', 'B02-SRC-SAME', 'B03-SRC-SAME',
    ]);
    expect(merged.testCaseDesign?.sourceInventory[2].evidence).toEqual(['sources.9.content']);
    expect(merged.testCaseDesign?.coverageStrategy.riskHotspots.map((risk) => risk.id)).toEqual([
      'B01-RISK-SAME', 'B02-RISK-SAME', 'B03-RISK-SAME',
    ]);
    expect(merged.testCases[3].trace).toEqual(['B03-RISK-SAME']);
    expect(merged.testCases[0].title).toMatch(/^\[B01\]/);
    expect(merged.testCases[2].title).toMatch(/^\[B02\]/);
    expect(merged.testCases[3].title).toBe('执行第 3 批业务流程 1');
    expect(merged.testCaseDesign?.traceability[3]).toMatchObject({
      traceId: 'B03-RISK-SAME', caseIds: ['TC-B03-FLOW-001'],
    });
    expect(() => validateCapabilityOutput('qa.test_cases', input, merged)).not.toThrow();
  });

  it('does not alter an already valid single-batch output', () => {
    const input = complexInput(1);
    const batch = partitionQATestCaseInput(input)[0];
    const output = outputForBatch(1, 1, 1);

    expect(mergeQATestCaseBatchOutputs([{ batch, output }])).toBe(output);
  });

  it('preserves every bounded batch narrative instead of silently retaining only the first', () => {
    const input = complexInput(13);
    const batches = partitionQATestCaseInput(input);
    const rows = batches.map((batch, index) => {
      const context = batch.input.test_case_context as { requirements: unknown[]; tasks: unknown[] };
      const output = outputForBatch(index + 1, context.requirements.length, context.tasks.length);
      output.testCaseDesign!.objective = `验证第 ${index + 1} 批业务目标`;
      output.testCaseDesign!.coverageStrategy.priorityLogic = `第 ${index + 1} 批风险优先`;
      return { batch, output };
    });

    const merged = mergeQATestCaseBatchOutputs(rows);

    expect(merged.testCaseDesign?.objective).toBe(
      batches.map((_batch, index) => `验证第 ${index + 1} 批业务目标`).join('\n'),
    );
    expect(merged.testCaseDesign?.coverageStrategy.priorityLogic).toBe(
      batches.map((_batch, index) => `第 ${index + 1} 批风险优先`).join('\n'),
    );
  });

  it('fails closed instead of truncating oversized cross-batch narrative semantics', () => {
    const input = complexInput(13);
    const batches = partitionQATestCaseInput(input);
    const rows = batches.map((batch, index) => {
      const context = batch.input.test_case_context as { requirements: unknown[]; tasks: unknown[] };
      const output = outputForBatch(index + 1, context.requirements.length, context.tasks.length);
      output.testCaseDesign!.objective = String(index + 1).repeat(1000);
      return { batch, output };
    });

    expect(() => mergeQATestCaseBatchOutputs(rows))
      .toThrow('merged test case objective exceeds 1000 characters');
  });

  it('merges an explicit global interaction plan around namespaced batch cases', () => {
    const input = complexInput(4);
    const batches = partitionQATestCaseInput(input);
    const rows = batches.map((batch, index) => {
      const context = batch.input.test_case_context as { requirements: unknown[]; tasks: unknown[] };
      return { batch, output: outputForBatch(index + 1, context.requirements.length, context.tasks.length) };
    });
    const globalPlan = compileQATestCaseGlobalPlan(input, {
      summary: '已完成跨需求交互分析',
      objective: '验证共用订单状态的跨需求一致性',
      priorityLogic: '先执行跨需求状态阻断用例',
      methods: ['user_journey'],
      sourceFindings: [],
      interactions: [{
        description: '一个需求修改共用状态后另一需求读到旧值',
        priority: 'P2',
        relationshipRef: 'subject:description',
        coverageDecision: { status: 'not_required' },
        cases: [{
          title: '跨需求更新并读取共用状态',
          category: 'integration',
          priority: 'P2',
          contextRefs: ['requirement:1:title', 'requirement:0:title'],
          preconditions: ['两个需求关联同一订单'],
          testData: ['共用订单 A'],
          actions: [{ step: '从需求 1 更新状态后在需求 2 读取', expectedResult: '需求 2 显示更新后的状态' }],
          notes: [],
          smoke: true,
          automationCandidate: true,
          releaseBlocking: false,
          executionPosition: 'before_batches',
        }],
      }],
      noCrossInteractionReason: '',
    });

    const merged = mergeQATestCaseBatchOutputs(rows, globalPlan);

    expect(merged.testCases[0]).toMatchObject({
      caseId: 'TC-GLOBAL-001', category: 'integration',
      evidence: [
        'test_case_context.requirements.1.title',
        'test_case_context.requirements.0.title',
        'subject.description',
      ],
    });
    expect(merged.testCaseDesign?.executionOrder[0]).toBe('TC-GLOBAL-001');
    expect(merged.testCaseDesign?.coverageStrategy.riskHotspots).toContainEqual(expect.objectContaining({
      id: 'RISK-GLOBAL-001', priority: 'P2',
    }));
    expect(merged.testCaseDesign?.traceability).toContainEqual(expect.objectContaining({
      traceId: 'RISK-GLOBAL-001', caseIds: ['TC-GLOBAL-001'], coverageTypes: ['integration'],
    }));
    expect(() => validateCapabilityOutput('qa.test_cases', input, merged)).not.toThrow();
  });
});

describe('multi-batch usage', () => {
  it('flattens generation and repair usage from every batch', () => {
    expect(combineTokenUsages(
      { inputTokens: 100, outputTokens: 50, totalTokens: 150 },
      { inputTokens: 80, outputTokens: 40 },
      null,
      { inputTokens: 20, outputTokens: 10, totalTokens: 30 },
    )).toEqual({ inputTokens: 200, outputTokens: 100, totalTokens: 300 });
  });
});
