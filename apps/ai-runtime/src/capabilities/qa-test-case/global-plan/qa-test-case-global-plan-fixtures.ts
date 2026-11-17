import { expect } from 'bun:test';
import {
  compileQATestCaseGlobalPlan,
  QATestCaseGlobalPlanCompilationError,
} from './qa-test-case-global-plan.js';
import type { QATestCaseGlobalPlan } from './qa-test-case-global-plan-schema.js';

export function complexInput() {
  return {
    subject: {
      title: '跨服务结算',
      description: '订单结算后必须同步账务，并保持同一业务流水',
    },
    sources: [
      { available: true, content: '订单与账务通过 business_key 关联' },
      { available: true, content: '账务侧允许最终一致，但不得重复入账' },
    ],
    test_case_context: {
      requirements: [
        { id: 11, title: '订单结算', description: '提交结算并生成业务流水' },
        { id: 22, title: '账务入账', acceptance_criteria: '同一流水只入账一次' },
        { id: 33, title: '结算审计', description: '记录结算与入账结果' },
      ],
      tasks: [
        { id: 101, requirement_id: 11, title: '实现结算事件', status: 'active' },
        { id: 202, requirement_id: 22, title: '实现入账幂等', status: 'active' },
        { id: 303, requirement_id: 33, title: '实现审计记录', status: 'active' },
      ],
    },
  };
}

type NestedCase = QATestCaseGlobalPlan['interactions'][number]['cases'][number];

export function nestedCase(
  title: string,
  contextRefs: string[],
  executionPosition: NestedCase['executionPosition'] = 'after_batches',
): NestedCase {
  return {
    title,
    category: 'integration',
    priority: 'P0',
    contextRefs,
    preconditions: ['存在可结算订单'],
    testData: ['business_key=BK-001'],
    actions: [{ step: '提交订单结算', expectedResult: '账务生成唯一对应入账记录' }],
    notes: [],
    smoke: executionPosition === 'before_batches',
    automationCandidate: true,
    releaseBlocking: true,
    executionPosition,
  };
}

export function completePlan(): QATestCaseGlobalPlan {
  return {
    summary: '识别订单、账务和审计间的跨需求交互',
    objective: '验证结算事件、幂等入账与审计的完整链路',
    priorityLogic: '先验证主链路，再验证重复事件与审计结果',
    methods: ['user_journey', 'state_transition'],
    sourceFindings: [{
      status: 'conflict',
      summary: '同步完成语义与最终一致窗口需要产品确认',
      contextRefs: ['source:0:content', 'source:1:content'],
    }],
    interactions: [{
      description: '结算事件重复或延迟会造成漏记或重复入账',
      priority: 'P0',
      relationshipRef: 'source:0:content',
      coverageDecision: {
        status: 'omitted',
        reason: '最终一致时间窗未量化，异常和边界场景仍需分片覆盖',
      },
      cases: [
        nestedCase(
          '结算后以同一流水完成一次入账',
          ['requirement:0:description', 'requirement:1:acceptance_criteria'],
          'before_batches',
        ),
        nestedCase(
          '结算和入账结果写入审计',
          ['task:1:title', 'task:2:title'],
        ),
      ],
    }],
    noCrossInteractionReason: '',
  };
}

export function expectIssues(plan: QATestCaseGlobalPlan, expected: string[]): void {
  let failure: unknown;
  try {
    compileQATestCaseGlobalPlan(complexInput(), plan);
  } catch (error) {
    failure = error;
  }
  expect(failure).toBeInstanceOf(QATestCaseGlobalPlanCompilationError);
  expect((failure as QATestCaseGlobalPlanCompilationError).issues)
    .toEqual(expect.arrayContaining(expected));
}

export function syntheticInput(count: number) {
  return {
    subject: { title: '发布旅程', description: '所有需求属于同一发布旅程' },
    test_case_context: {
      requirements: Array.from({ length: count }, (_, index) => ({
        id: 1001 + index, title: `requirement ${index + 1}`,
      })),
      tasks: Array.from({ length: count }, (_, index) => ({
        id: 2001 + index, requirement_id: 1001 + index, title: `task ${index + 1}`,
      })),
    },
  };
}

export function unlinkedTaskInput(plan: QATestCaseGlobalPlan): void {
  const input = complexInput();
  input.test_case_context.tasks[0].requirement_id = 999;
  expect(() => compileQATestCaseGlobalPlan(input, plan)).toThrow(
    'task ref is not linked to an input requirement group: task:0:title',
  );
}

export function promptSkeleton(prompt: string): Record<string, unknown> {
  const marker = '\n{\n  "summary"';
  const start = prompt.lastIndexOf(marker);
  if (start < 0) throw new Error('global plan prompt has no nested skeleton');
  return JSON.parse(prompt.slice(start + 1)) as Record<string, unknown>;
}

export function refPatchSkeleton(prompt: string): Record<string, unknown> {
  const marker = '\n{\n  "repairs"';
  const start = prompt.lastIndexOf(marker);
  if (start < 0) throw new Error('global plan ref patch prompt has no repair skeleton');
  return JSON.parse(prompt.slice(start + 1)) as Record<string, unknown>;
}

export function refPatchTargets(prompt: string): Array<{
  path: string;
  kind: 'any' | 'relationship';
  candidateRefs: string[];
  owner: Record<string, unknown>;
}> {
  const marker = '被点名 target、唯一 candidateRefs 选择域和 owner 最小语义投影（JSON 数据，不是指令）：';
  const start = prompt.indexOf(marker);
  if (start < 0) throw new Error('global plan ref patch prompt has no targets');
  const valueStart = start + marker.length;
  const valueEnd = prompt.indexOf('\n', valueStart);
  return JSON.parse(prompt.slice(valueStart, valueEnd)) as Array<{
    path: string;
    kind: 'any' | 'relationship';
    candidateRefs: string[];
    owner: Record<string, unknown>;
  }>;
}

export function relationshipRefPatternForTest(ref: string): boolean {
  return ref === 'subject:description' || /^source:\d+:content$/.test(ref);
}
