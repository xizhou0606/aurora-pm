import { NoObjectGeneratedError } from 'ai';
import { normalizeTestCaseGenerationOutput, type QATestCaseDraft } from '../index.js';

// 测试夹具:原 src/generator.test.ts 顶层 helper,逐字迁移(仅加 export)。

export function completeTestCaseOutput(evidence = 'test_case_context.tasks[0].status') {
  return normalizeTestCaseGenerationOutput({
    summary: '已生成可执行测试用例',
    design: {
      objective: '验证任务状态展示',
      scope: ['任务状态读取'],
      exclusions: [],
      sourceInventory: [{ id: 'SRC-SUBJECT', kind: '需求', status: 'confirmed', summary: '当前需求', evidence: ['subject.title'] }],
      assumptions: [],
      informationGaps: [],
      coverageStrategy: {
        priorityLogic: '常规回归路径，影响有限',
        methods: ['state_transition'],
        riskHotspots: [],
      },
      testCases: [{
        caseId: 'TC-TASK-001', title: '任务状态', category: 'regression', priority: 'P2',
        trace: ['REQ-TASK-STATUS'], preconditions: [], testData: ['状态为 todo 的任务'],
        steps: ['查看任务'], expectedResults: ['显示待处理'], notes: ['REQ-TASK-STATUS 为派生标签'], evidence: [evidence],
      }],
      traceability: [{ traceId: 'REQ-TASK-STATUS', caseIds: ['TC-TASK-001'], coverageTypes: ['regression'], gapNote: '' }],
      smokeCaseIds: ['TC-TASK-001'],
      executionOrder: ['TC-TASK-001'],
      automationCandidateCaseIds: ['TC-TASK-001'],
      releaseBlockingCaseIds: [],
    },
  });
}

export function completeTestCaseModelText(): string {
  const output = completeTestCaseOutput('subject.title');
  return JSON.stringify({
    summary: output.summary,
    design: {
      ...output.testCaseDesign,
      testCases: output.testCases.map(({ expectedResult: _expectedResult, ...testCase }) => testCase),
    },
  });
}

export function completeTestCaseDraft(contextRefs: string[]) {
  const riskContextRef = contextRefs[contextRefs.length - 1];
  const assessmentRefs = new Map<string, string>();
  for (const ref of contextRefs) {
    const target = ref === 'subject' || ref.startsWith('subject:')
      ? 'subject'
      : /^(?:source|requirement|task):\d+/.exec(ref)?.[0];
    if (target && !assessmentRefs.has(target)) assessmentRefs.set(target, ref);
  }
  return {
    summary: '已生成结算测试设计',
    objective: '验证结算提交及结果展示',
    scope: ['结算提交'],
    exclusions: [],
    assumptions: [],
    informationGaps: [],
    sourceAssessments: [...assessmentRefs].map(([targetRef, ref]) => ({
      targetRef,
      status: 'confirmed',
      summary: `${targetRef} 已确认`,
      contextRefs: [ref],
    })),
    priorityLogic: '按业务影响执行主路径',
    methods: ['user_journey'],
    risks: [{
      riskId: 'RISK-SETTLEMENT',
      description: '结算结果未正确展示',
      priority: 'P2',
      contextRefs,
      coverageDecision: { status: 'not_required' },
    }],
    cases: [{
      title: '成功提交结算',
      category: 'positive',
      priority: 'P2',
      contextRefs: contextRefs.slice(0, -1),
      riskLinks: [{ riskId: 'RISK-SETTLEMENT', contextRef: riskContextRef }],
      preconditions: ['存在待结算记录'],
      testData: ['有效结算数据'],
      actions: [{ step: '提交结算', expectedResult: '页面显示结算成功' }],
      notes: [],
      smoke: true,
      automationCandidate: true,
      releaseBlocking: false,
    }],
  };
}

export function incompleteHighRiskCoverageDraft(contextRefs: string[]): QATestCaseDraft {
  const draft = completeTestCaseDraft(contextRefs) as QATestCaseDraft;
  draft.risks[0].priority = 'P1';
  draft.risks[0].coverageDecision = { status: 'complete' };
  draft.cases[0].category = 'negative';
  draft.cases[0].priority = 'P1';
  draft.cases[0].releaseBlocking = true;
  return draft;
}

export function omittedHighRiskCoverageDraft(contextRefs: string[], reason: string): QATestCaseDraft {
  const draft = incompleteHighRiskCoverageDraft(contextRefs);
  draft.risks[0].coverageDecision = { status: 'omitted', reason };
  return draft;
}

export function omittedCoveragePatch(reason: string) {
  return {
    repairs: [{
      riskId: 'RISK-SETTLEMENT',
      resolutions: [
        { category: 'positive', kind: 'omit' },
        { category: 'boundary', kind: 'omit' },
      ],
      omissionReason: reason,
    }],
  };
}

export function completeGlobalPlanInput() {
  return {
    subject: { title: '结算版本', description: '两个结算需求共用同一订单状态' },
    test_case_context: {
      requirements: [
        { id: 1, title: '提交结算' },
        { id: 2, title: '查询结算状态' },
      ],
      tasks: [],
    },
  };
}

export function completeGlobalPlan() {
  return {
    summary: '已完成跨需求分析',
    objective: '验证共用订单状态的一致性',
    priorityLogic: '先执行跨需求状态用例',
    methods: ['user_journey'],
    sourceFindings: [],
    interactions: [{
      description: '状态更新后查询端读到旧值',
      priority: 'P2',
      relationshipRef: 'subject:description',
      coverageDecision: { status: 'not_required' },
      cases: [{
        title: '更新后跨需求查询状态',
        category: 'integration',
        priority: 'P2',
        contextRefs: ['requirement:1:title', 'requirement:0:title'],
        preconditions: ['两个需求关联同一订单'],
        testData: ['订单 A'],
        actions: [{ step: '提交结算后查询状态', expectedResult: '查询结果显示更新后的状态' }],
        notes: [],
        smoke: true,
        automationCandidate: true,
        releaseBlocking: false,
        executionPosition: 'after_batches',
      }],
    }],
    noCrossInteractionReason: '',
  };
}

export function globalPlanWithUnavailableRelationship() {
  const plan = completeGlobalPlan();
  plan.interactions[0].relationshipRef = 'source:0:content';
  return plan;
}

export function globalPlanRelationshipRefPatch() {
  return {
    repairs: [{
      path: 'interactions[0].relationshipRef',
      replacementRef: 'subject:description',
    }],
  };
}

export function testNoObjectGeneratedError(
  text: string,
  tokens: { inputTokens: number; outputTokens: number },
): NoObjectGeneratedError {
  return new NoObjectGeneratedError({
    message: 'test parse failure',
    cause: new Error('invalid JSON'),
    text,
    response: { id: 'test-response', timestamp: new Date(0), modelId: 'test-model' },
    usage: {
      ...tokens,
      inputTokenDetails: { noCacheTokens: tokens.inputTokens, cacheReadTokens: 0, cacheWriteTokens: 0 },
      outputTokenDetails: { textTokens: tokens.outputTokens, reasoningTokens: 0 },
      totalTokens: tokens.inputTokens + tokens.outputTokens,
    },
    finishReason: 'stop',
  });
}
