import { normalizeTestCaseGenerationOutput } from '../qa-test-case-normalize.js';

export function complexInput(pairCount: number) {
  return {
    subject: { title: '复杂结算需求', description: '多个结算需求共用同一订单状态' },
    sources: [
      ...Array.from({ length: pairCount }, (_, index) => ({
        url: `https://example.feishu.cn/docx/requirement-${index + 1}`,
        content: `需求 ${index + 1} 的飞书正文`,
      })),
      { url: 'https://example.feishu.cn/docx/global-rules', content: '项目全局测试规则' },
    ],
    test_case_context: {
      coverageRequired: true,
      requirements: Array.from({ length: pairCount }, (_, index) => ({
        id: index + 1,
        title: `需求 ${index + 1}`,
        product_doc_url: `https://example.feishu.cn/docx/requirement-${index + 1}`,
      })),
      tasks: Array.from({ length: pairCount }, (_, index) => ({
        id: index + 101,
        requirement_id: index + 1,
        title: `任务 ${index + 1}`,
      })),
      counts: { requirements: pairCount, tasks: pairCount },
    },
  };
}

export function outputForBatch(label: number, requirementCount: number, taskCount: number) {
  const evidence = [
    ...Array.from({ length: requirementCount }, (_, index) => `test_case_context.requirements[${index}].title`),
    ...Array.from({ length: taskCount }, (_, index) => `test_case_context.tasks.${index}.title`),
  ];
  const evidenceGroups = Array.from(
    { length: Math.ceil(evidence.length / 8) },
    (_, index) => evidence.slice(index * 8, (index + 1) * 8),
  );
  const testCases = evidenceGroups.map((caseEvidence, index) => ({
    caseId: `TC-FLOW-${String(index + 1).padStart(3, '0')}`,
    title: `执行第 ${label} 批业务流程 ${index + 1}`,
    category: 'positive',
    priority: 'P2',
    trace: [index === 0 ? 'RISK-SAME' : `TRACE-${String(index + 1).padStart(3, '0')}`],
    preconditions: [],
    testData: [],
    steps: ['执行业务操作'],
    expectedResults: ['界面显示该批业务处理结果'],
    notes: [],
    evidence: caseEvidence,
  }));
  const traceability = testCases.map((testCase) => ({
    traceId: testCase.trace[0],
    caseIds: [testCase.caseId],
    coverageTypes: ['positive'],
    gapNote: '',
  }));
  const caseIDs = testCases.map((testCase) => testCase.caseId);
  return normalizeTestCaseGenerationOutput({
    summary: `第 ${label} 批测试设计`,
    design: {
      objective: `验证第 ${label} 批业务事实`,
      scope: [`第 ${label} 批范围`],
      exclusions: [],
      sourceInventory: [{
        id: 'SRC-SAME', kind: '需求', status: 'confirmed', summary: '当前需求', evidence: ['sources[0].content'],
      }],
      assumptions: [],
      informationGaps: [],
      coverageStrategy: {
        priorityLogic: '按业务事实优先级执行',
        methods: ['user_journey'],
        riskHotspots: [{
          id: 'RISK-SAME', description: '业务事实未生效', priority: 'P2', evidence: [evidence[0]],
        }],
      },
      testCases,
      traceability,
      smokeCaseIds: [caseIDs[0]],
      executionOrder: caseIDs,
      automationCandidateCaseIds: caseIDs,
      releaseBlockingCaseIds: [],
    },
  });
}
