import { z } from 'zod';

export const generateRequestSchema = z.object({
  runId: z.number().int().positive(),
  workspaceId: z.number().int().positive(),
  capability: z.string().min(1).max(100),
  input: z.record(z.string(), z.unknown()),
});

export const testCaseCategorySchema = z.enum([
  'positive', 'negative', 'boundary', 'permission', 'state',
  'duplicate_submit', 'concurrency', 'timeout', 'retry', 'rollback',
  'stale_data', 'integration', 'regression',
]);

const testCasePrioritySchema = z.enum(['P0', 'P1', 'P2', 'P3']);
const testCaseIdSchema = z.string().regex(/^TC-(?:[A-Z0-9\u4e00-\u9fff]+-)+\d{3}$/);

export const testCaseSchema = z.object({
  caseId: testCaseIdSchema,
  title: z.string().min(1),
  category: testCaseCategorySchema,
  priority: testCasePrioritySchema,
  trace: z.array(z.string().min(1)).min(1),
  preconditions: z.array(z.string()).default([]),
  testData: z.array(z.string()).default([]),
  steps: z.array(z.string()).min(1),
  expectedResult: z.string().min(1),
  expectedResults: z.array(z.string().min(1)).min(1),
  notes: z.array(z.string()).default([]),
  evidence: z.array(z.string()).default([]),
});

const generatedTestCaseSchema = testCaseSchema.omit({ expectedResult: true });

const testCaseSourceSchema = z.object({
  id: z.string().min(1).max(100),
  kind: z.string().min(1).max(100),
  status: z.enum(['confirmed', 'conflict', 'assumption', 'gap']),
  summary: z.string().min(1).max(1000),
  evidence: z.array(z.string()).default([]),
});

const testCaseRiskHotspotSchema = z.object({
  id: z.string().min(1).max(100),
  description: z.string().min(1).max(1000),
  priority: testCasePrioritySchema,
  evidence: z.array(z.string()).min(1),
});

const testCaseTraceabilitySchema = z.object({
  traceId: z.string().min(1).max(100),
  caseIds: z.array(testCaseIdSchema).min(1),
  coverageTypes: z.array(testCaseCategorySchema).min(1),
  gapNote: z.string().default(''),
});

export const testCaseDesignMetadataSchema = z.object({
  objective: z.string().min(1).max(1000),
  scope: z.array(z.string().min(1)).min(1),
  exclusions: z.array(z.string()).default([]),
  sourceInventory: z.array(testCaseSourceSchema).min(1),
  assumptions: z.array(z.string()).default([]),
  informationGaps: z.array(z.string()).default([]),
  coverageStrategy: z.object({
    priorityLogic: z.string().min(1).max(2000),
    methods: z.array(z.enum([
      'equivalence_partitioning', 'boundary_value', 'decision_table',
      'state_transition', 'pairwise', 'user_journey', 'error_guessing',
      'regression_history',
    ])).min(1),
    riskHotspots: z.array(testCaseRiskHotspotSchema).default([]),
  }),
  traceability: z.array(testCaseTraceabilitySchema).min(1),
  smokeCaseIds: z.array(testCaseIdSchema).min(1),
  executionOrder: z.array(testCaseIdSchema).min(1),
  automationCandidateCaseIds: z.array(testCaseIdSchema).default([]),
  releaseBlockingCaseIds: z.array(testCaseIdSchema).default([]),
});

export const testCaseGenerationSchema = z.object({
  summary: z.string().min(1),
  design: testCaseDesignMetadataSchema.extend({
    testCases: z.array(generatedTestCaseSchema).min(1),
  }),
});

export const taskPlanChainSchema = z.object({
  lane: z.enum(['frontend', 'backend']),
  summary: z.string().min(1).max(1000),
  tasks: z.array(z.object({
    title: z.string().min(1).max(180),
    description: z.string().min(1).max(4000),
    acceptanceCriteria: z.array(z.string().min(1).max(500)).min(1).max(8),
  })).min(1).max(8),
});

export const taskPlanSchema = z.object({
  chains: z.array(taskPlanChainSchema).length(2),
}).superRefine((plan, context) => {
  const lanes = new Set(plan.chains.map((chain) => chain.lane));
  if (lanes.size !== 2 || !lanes.has('frontend') || !lanes.has('backend')) {
    context.addIssue({ code: 'custom', path: ['chains'], message: 'task plan must contain one frontend and one backend chain' });
  }
});

export const workflowAuthoringNodeTypes = [
  'start', 'end', 'human_task', 'approval', 'condition', 'parallel_split',
  'parallel_join', 'ai_check', 'ai_generate',
] as const;

export const historicalWorkflowNodeTypes = [
  ...workflowAuthoringNodeTypes,
  'form', 'timer', 'ai_extract', 'ai_classify', 'ai_plan', 'ai_summarize',
  'ai_compare', 'outbox_action', 'create_drafts',
] as const;

export const workflowAuthoringNodeTypeSchema = z.enum(workflowAuthoringNodeTypes);
const historicalWorkflowNodeTypeSchema = z.enum(historicalWorkflowNodeTypes);

const workflowDefinitionSchema = z.object({
  nodes: z.array(z.object({
    id: z.string().min(1).max(100),
    type: historicalWorkflowNodeTypeSchema,
    name: z.string().min(1),
    capability: z.string().optional(),
    config: z.object({
      critical_gate: z.boolean().optional(),
      delay_seconds: z.number().nonnegative().optional(),
      require_design_image: z.boolean().optional(),
      test_case_artifact: z.boolean().optional(),
      test_report_artifact: z.boolean().optional(),
    }).catchall(z.unknown()).optional(),
  })).min(2),
  edges: z.array(z.object({
    id: z.string().min(1),
    source: z.string().min(1),
    target: z.string().min(1),
    kind: z.string().optional(),
    condition: z.string().optional(),
    loop: z.boolean().optional(),
  })).min(1),
});

const completenessWeights = {
  background: 15,
  goal: 15,
  scope: 15,
  boundaries: 10,
  exceptions: 15,
  dependencies: 10,
  acceptance_criteria: 20,
} as const;

const completenessReportSchema = z.object({
  verdict: z.enum(['pass', 'needs_revision', 'blocked']),
  score: z.number().int().min(0).max(100),
  threshold: z.number().int().min(0).max(100),
  canProceed: z.boolean(),
  checkedSubjectVersion: z.number().int().positive(),
  dimensions: z.array(z.object({
    key: z.enum(['background', 'goal', 'scope', 'boundaries', 'exceptions', 'dependencies', 'acceptance_criteria']),
    label: z.string().min(1),
    status: z.enum(['pass', 'partial', 'missing']),
    score: z.number().int().min(0),
    maxScore: z.number().int().positive(),
    findings: z.array(z.string()),
    evidence: z.array(z.string()),
    remediation: z.array(z.string()),
  })).length(7),
  missingRequiredFields: z.array(z.string()),
  blockingIssues: z.array(z.string()),
  recommendations: z.array(z.string()),
}).superRefine((report, context) => {
  const dimensions = new Map(report.dimensions.map((dimension) => [dimension.key, dimension]));
  if (dimensions.size !== Object.keys(completenessWeights).length) {
    context.addIssue({ code: 'custom', path: ['dimensions'], message: 'completeness dimensions must be unique and complete' });
  }
  for (const [key, weight] of Object.entries(completenessWeights)) {
    const dimension = dimensions.get(key as keyof typeof completenessWeights);
    if (!dimension || dimension.maxScore !== weight || dimension.score > weight) {
      context.addIssue({ code: 'custom', path: ['dimensions'], message: `invalid score contract for ${key}` });
      continue;
    }
    if ((dimension.status === 'pass' && dimension.score !== weight) || (dimension.status === 'missing' && dimension.score !== 0) || (dimension.status === 'partial' && dimension.score > Math.floor(weight / 2))) {
      context.addIssue({ code: 'custom', path: ['dimensions'], message: `score does not match status for ${key}` });
    }
  }
  const total = report.dimensions.reduce((sum, dimension) => sum + dimension.score, 0);
  if (report.score !== total) context.addIssue({ code: 'custom', path: ['score'], message: 'score must equal the dimension total' });
  if (report.threshold !== 80) context.addIssue({ code: 'custom', path: ['threshold'], message: 'completeness threshold must be 80' });
  const criticalMissing = ['goal', 'scope', 'acceptance_criteria'].some((key) => dimensions.get(key as keyof typeof completenessWeights)?.status === 'missing');
  const canProceed = report.score >= report.threshold && report.blockingIssues.length === 0 && !criticalMissing;
  const verdict = canProceed ? 'pass' : criticalMissing || report.blockingIssues.length > 0 ? 'blocked' : 'needs_revision';
  if (report.canProceed !== canProceed) context.addIssue({ code: 'custom', path: ['canProceed'], message: 'canProceed does not match the gate rules' });
  if (report.verdict !== verdict) context.addIssue({ code: 'custom', path: ['verdict'], message: 'verdict does not match the gate rules' });
});

export const aiOutputSchema = z.object({
  summary: z.string().min(1),
  changes: z.array(z.object({
    field: z.string().min(1),
    value: z.unknown(),
    reason: z.string().min(1),
  })).default([]),
  items: z.array(z.object({
    title: z.string().min(1),
    description: z.string().default(''),
    evidence: z.array(z.string()).default([]),
  })).default([]),
  risks: z.array(z.string()).default([]),
  testCases: z.array(testCaseSchema).default([]),
  testCaseDesign: testCaseDesignMetadataSchema.nullable().optional(),
  skillProvenance: z.object({
    name: z.string().min(1),
    repository: z.url(),
    path: z.string().min(1),
    commit: z.string().regex(/^[a-f0-9]{40}$/),
    packageSha256: z.string().regex(/^[a-f0-9]{64}$/),
    mode: z.enum(['standard', 'advanced']),
    resources: z.array(z.string().min(1)),
  }).nullable().optional(),
  humanReviewRequired: z.boolean().default(true),
  completenessReport: completenessReportSchema.nullable().default(null),
  workflowProposal: z.object({
    mode: z.enum(['replace', 'merge']).default('replace'),
    explanation: z.string().default(''),
    warnings: z.array(z.string()).default([]),
    definition: workflowDefinitionSchema,
  }).nullable().default(null),
  taskPlan: taskPlanSchema.nullable().optional(),
  // 代码质量审查技能(code.commit_quality_review)的结构化结论,随报告
  // 运行输出透传给 worker;不注册会被 zod 默认剥离,worker 将拿不到逐
  // 提交结论(历史上导致评估静默缺失)。
  commitReviews: z.array(
    z.object({
      sha: z.string(),
      verdict: z.enum(['优', '良', '中', '差']),
      summary: z.string().default(''),
      issues: z.array(z.string()).default([]),
    }),
  ).nullable().optional(),
  qualityAssessment: z.object({
    score: z.number().min(0).max(100),
    conclusion: z.string().default(''),
    commonIssues: z.array(z.string()).default([]),
    risks: z.array(z.string()).default([]),
    suggestions: z.array(z.string()).default([]),
  }).nullable().optional(),
  // 版本公告草稿技能(changelog.draft)的结构化区块,随输出信封透传给调用方
  // (CLI changelog:draft);不注册会被 zod 默认剥离。
  changelogDraft: z.object({
    summary: z.array(z.string()).default([]),
    sections: z.array(z.object({
      title: z.string(),
      items: z.array(z.string()),
    })).default([]),
  }).nullable().optional(),
});

export type GenerateRequest = z.infer<typeof generateRequestSchema>;
export type AIOutput = z.infer<typeof aiOutputSchema>;
