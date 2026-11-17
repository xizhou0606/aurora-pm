import type { QATestCaseDraft } from './qa-test-case-draft-schema.js';

export function batchInput() {
  return {
    subject: { id: 77, title: '复杂结算需求' },
    sources: [
      { available: true, content: '结算金额必须与订单明细一致' },
      { available: false, url: 'https://example.test/unavailable' },
    ],
    test_case_context: {
      coverageRequired: true,
      counts: { requirements: 2, tasks: 1 },
      requirements: [
        { id: 201, title: '提交结算', acceptance_criteria: '展示结算结果' },
        { id: 202, title: '金额边界' },
      ],
      tasks: [{ id: 301, requirement_id: 201, title: '实现结算校验' }],
    },
  };
}

export function completeDraft(): QATestCaseDraft {
  return {
    summary: '结算测试设计已生成',
    objective: '验证结算主路径、异常路径和金额边界',
    scope: ['结算提交与结果展示'],
    exclusions: [],
    assumptions: ['测试环境可创建待结算订单'],
    informationGaps: [],
    sourceAssessments: [
      { targetRef: 'subject', status: 'confirmed', summary: '当前测试主体', contextRefs: ['subject:title'] },
      { targetRef: 'source:0', status: 'confirmed', summary: '结算规则正文', contextRefs: ['source:0:content'] },
      { targetRef: 'source:1', status: 'gap', summary: '外部来源 2 不可用', contextRefs: [] },
      { targetRef: 'requirement:0', status: 'confirmed', summary: '提交结算需求', contextRefs: ['requirement:0:title'] },
      { targetRef: 'requirement:1', status: 'confirmed', summary: '金额边界需求', contextRefs: ['requirement:1:title'] },
      { targetRef: 'task:0', status: 'confirmed', summary: '结算校验任务', contextRefs: ['task:0:title'] },
    ],
    priorityLogic: '先执行发布阻断的主路径，再执行异常与边界路径',
    methods: ['user_journey', 'boundary_value'],
    risks: [{
      riskId: 'RISK-SETTLEMENT-AMOUNT',
      description: '结算金额计算错误会导致发布阻断',
      priority: 'P1',
      contextRefs: ['source:0:content', 'requirement:0:acceptance_criteria', 'requirement:1:title'],
      coverageDecision: { status: 'complete' },
    }],
    cases: [
      {
        title: '提交有效结算并展示结果',
        category: 'positive',
        priority: 'P0',
        contextRefs: ['subject:title', 'requirement:0:title', 'task:0:title'],
        riskLinks: [{ riskId: 'RISK-SETTLEMENT-AMOUNT', contextRef: 'source:0:content' }],
        preconditions: ['存在一笔金额为 100 元的待结算订单'],
        testData: ['订单金额：100 元'],
        actions: [
          { step: '打开待结算订单', expectedResult: '页面展示订单金额 100 元' },
          { step: '提交结算', expectedResult: '页面展示结算成功且金额为 100 元' },
        ],
        notes: [],
        smoke: true,
        automationCandidate: true,
        releaseBlocking: true,
      },
      {
        title: '拒绝不一致的结算金额',
        category: 'negative',
        priority: 'P1',
        contextRefs: [],
        riskLinks: [{
          riskId: 'RISK-SETTLEMENT-AMOUNT', contextRef: 'requirement:0:acceptance_criteria',
        }],
        preconditions: ['存在一笔金额为 100 元的待结算订单'],
        testData: ['提交金额：101 元'],
        actions: [{ step: '提交金额 101 元的结算请求', expectedResult: '页面拒绝提交并保留待结算状态' }],
        notes: [],
        smoke: false,
        automationCandidate: true,
        releaseBlocking: true,
      },
      {
        title: '提交结算金额下边界',
        category: 'boundary',
        priority: 'P1',
        contextRefs: [],
        riskLinks: [{ riskId: 'RISK-SETTLEMENT-AMOUNT', contextRef: 'requirement:1:title' }],
        preconditions: ['存在一笔金额为 0.01 元的待结算订单'],
        testData: ['订单金额：0.01 元'],
        actions: [{ step: '提交金额 0.01 元的结算请求', expectedResult: '页面展示结算成功且金额为 0.01 元' }],
        notes: [],
        smoke: false,
        automationCandidate: true,
        releaseBlocking: false,
      },
    ],
  };
}

export function multiGroupInput(groupCount = 3) {
  return {
    subject: { title: '跨需求结算旅程', description: '三个需求通过同一结算单号按顺序衔接' },
    test_case_context: {
      coverageRequired: true,
      counts: { requirements: groupCount, tasks: groupCount },
      requirements: Array.from({ length: groupCount }, (_, index) => ({
        id: index + 1, title: `结算需求 ${index + 1}`,
      })),
      tasks: Array.from({ length: groupCount }, (_, index) => ({
        id: index + 11, requirement_id: index + 1, title: `结算任务 ${index + 1}`,
      })),
    },
  };
}

export function multiGroupDraft(groupCount = 3): QATestCaseDraft {
  const refs = [
    ...Array.from({ length: groupCount }, (_, index) => `requirement:${index}:title`),
    ...Array.from({ length: groupCount }, (_, index) => `task:${index}:title`),
  ];
  return {
    summary: '跨需求测试设计',
    objective: '验证三个需求',
    scope: ['结算需求'],
    exclusions: [], assumptions: [], informationGaps: [],
    sourceAssessments: [
      { targetRef: 'subject', status: 'confirmed', summary: '测试主体', contextRefs: ['subject:title'] },
      ...Array.from({ length: groupCount }, (_, index) => ({
        targetRef: `requirement:${index}`,
        status: 'confirmed' as const,
        summary: `需求 ${index + 1}`,
        contextRefs: [`requirement:${index}:title`],
      })),
      ...Array.from({ length: groupCount }, (_, index) => ({
        targetRef: `task:${index}`,
        status: 'confirmed' as const,
        summary: `任务 ${index + 1}`,
        contextRefs: [`task:${index}:title`],
      })),
    ],
    priorityLogic: '按需求优先级执行',
    methods: ['boundary_value'],
    risks: [{
      riskId: 'RISK-CROSS-GROUP', description: '泛化用例掩盖独立行为', priority: 'P2',
      contextRefs: ['requirement:0:title'],
      coverageDecision: { status: 'not_required' },
    }],
    cases: [{
      title: '一次验证三个独立需求', category: 'positive', priority: 'P2',
      contextRefs: refs.filter((ref) => ref !== 'requirement:0:title'),
      riskLinks: [{ riskId: 'RISK-CROSS-GROUP', contextRef: 'requirement:0:title' }],
      preconditions: [], testData: [],
      actions: [{ step: '执行三个需求', expectedResult: '分别展示三个需求的处理结果' }],
      notes: [], smoke: true, automationCandidate: false, releaseBlocking: false,
    }],
  };
}
