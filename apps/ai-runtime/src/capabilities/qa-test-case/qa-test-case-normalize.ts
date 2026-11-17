import {
  aiOutputSchema,
  testCaseGenerationSchema,
  type AIOutput,
} from '../../shared/ai/schema.js';

export function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function stringArray(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap((item) => {
    if (typeof item === 'string' && item.trim()) return [item.trim()];
    const row = record(item);
    const nested = row.action ?? row.step ?? row.description ?? row.content ?? row.value;
    return typeof nested === 'string' && nested.trim() ? [nested.trim()] : [];
  });
  if (typeof value === 'string' && value.trim()) return [value.trim()];
  return [];
}

function summaryText(value: unknown): string {
  if (typeof value === 'string' && value.trim()) return value.trim();
  const row = record(value);
  return stringArray(row.title ?? row.summary ?? row.description ?? row.name)[0]
    ?? '';
}

function coverageMethods(value: unknown): string[] {
  const aliases: Record<string, string> = {
    '等价类': 'equivalence_partitioning',
    '等价类划分': 'equivalence_partitioning',
    '边界值': 'boundary_value',
    '决策表': 'decision_table',
    '状态迁移': 'state_transition',
    '两两组合': 'pairwise',
    '用户旅程': 'user_journey',
    '错误推测': 'error_guessing',
    '回归历史': 'regression_history',
  };
  return [...new Set(stringArray(value)
    .map((item) => aliases[item] ?? item))];
}

function normalizedDesignMetadata(value: unknown) {
  const design = record(value);
  const sources = Array.isArray(design.sourceInventory) ? design.sourceInventory : [];
  const strategy = record(design.coverageStrategy);
  const risks = Array.isArray(strategy.riskHotspots) ? strategy.riskHotspots : [];
  const traceability = Array.isArray(design.traceability) ? design.traceability : [];
  return {
    ...design,
    scope: stringArray(design.scope),
    exclusions: stringArray(design.exclusions),
    assumptions: stringArray(design.assumptions),
    informationGaps: stringArray(design.informationGaps),
    sourceInventory: sources.map((value) => {
      const source = record(value);
      const status = String(source.status ?? '');
      return {
        ...source,
        status,
        evidence: stringArray(source.evidence),
      };
    }),
    coverageStrategy: {
      ...strategy,
      methods: coverageMethods(strategy.methods),
      riskHotspots: risks.map((value) => {
        const risk = record(value);
        return { ...risk, evidence: stringArray(risk.evidence) };
      }),
    },
    traceability: traceability.map((value) => {
      const row = record(value);
      return {
        ...row,
        caseIds: stringArray(row.caseIds),
        coverageTypes: stringArray(row.coverageTypes).map(testCaseCategory),
        gapNote: typeof row.gapNote === 'string' ? row.gapNote : '',
      };
    }),
    smokeCaseIds: stringArray(design.smokeCaseIds),
    executionOrder: stringArray(design.executionOrder),
    automationCandidateCaseIds: stringArray(design.automationCandidateCaseIds),
    releaseBlockingCaseIds: stringArray(design.releaseBlockingCaseIds),
  };
}

function testCaseCategory(value: unknown) {
  const normalized = String(value ?? '').trim().toLowerCase();
  const aliases: Record<string, string> = {
    '正向': 'positive', '正常': 'positive', success: 'positive', happy_path: 'positive', '成功': 'positive',
    '异常': 'negative', '反向': 'negative', error: 'negative', failure: 'negative', invalid: 'negative', '失败': 'negative', '非法': 'negative',
    '边界': 'boundary', edge: 'boundary', limit: 'boundary', '极限': 'boundary',
    '权限': 'permission', '鉴权': 'permission', auth: 'permission', security: 'permission', access: 'permission', '安全': 'permission', '访问控制': 'permission',
    '状态': 'state', state_transition: 'state', '重复提交': 'duplicate_submit', idempotency: 'duplicate_submit', '幂等': 'duplicate_submit',
    '并发': 'concurrency', race: 'concurrency', '超时': 'timeout', '重试': 'retry', '回滚': 'rollback', '补偿': 'rollback',
    '旧数据': 'stale_data', '过期数据': 'stale_data', cache: 'stale_data', '集成': 'integration', e2e: 'integration',
    '回归': 'regression', retest: 'regression', '复测': 'regression',
  };
  const resolved = aliases[normalized] ?? normalized;
  return resolved;
}

function testCasePriority(value: unknown) {
  const normalized = String(value ?? '').trim().toUpperCase();
  if (['P0', 'CRITICAL', 'BLOCKER', '最高'].includes(normalized)) return 'P0';
  if (['P1', 'HIGH', '高'].includes(normalized)) return 'P1';
  if (['P2', 'MEDIUM', 'NORMAL', '中'].includes(normalized)) return 'P2';
  if (['P3', 'LOW', '低'].includes(normalized)) return 'P3';
  return normalized;
}

function normalizeCaseId(value: unknown): string {
  const raw = String(value ?? '').trim().toUpperCase();
  return raw;
}

function traceArray(value: unknown): string[] {
  return stringArray(value).flatMap((item) => item
    .split(/[;,，；]/)
    .map((part) => part.trim())
    .filter(Boolean));
}

function normalizedTestCase(value: unknown) {
  const row = record(value);
  const rawExpectedResults = row.expectedResults ?? row.expected_results ?? row.expectedResult
    ?? row.expected_result ?? row.expected ?? row.result;
  return {
    caseId: normalizeCaseId(row.caseId ?? row.case_id ?? row.id),
    title: row.title ?? row.name ?? row.case_name ?? row.test_name,
    category: testCaseCategory(row.category ?? row.type ?? row.test_type ?? row.scenario_type),
    priority: testCasePriority(row.priority ?? row.level ?? row.severity),
    trace: traceArray(row.trace ?? row.traceIds ?? row.trace_ids ?? row.requirement_ids ?? row.risk_ids),
    preconditions: stringArray(row.preconditions ?? row.precondition ?? row.pre_condition ?? row.conditions ?? row.prerequisites),
    testData: stringArray(row.testData ?? row.test_data ?? row.data),
    steps: stringArray(row.steps ?? row.actions ?? row.testSteps ?? row.test_steps ?? row.procedure),
    expectedResults: stringArray(rawExpectedResults),
    notes: stringArray(row.notes ?? row.note ?? row.remarks),
    evidence: stringArray(row.evidence ?? row.basis ?? row.references ?? row.source),
  };
}

export function normalizeTestCaseGenerationOutput(value: unknown): AIOutput {
  const root = record(value);
  const nested = record(root.data);
  const source = root.design || root.testCases || root.test_cases || root.cases
    ? root
    : nested;
  const design = record(source.design);
  const designRows = design.testCases ?? design.test_cases ?? design.cases;

  if (Object.keys(design).length && designRows) {
    const rows = Array.isArray(designRows) ? designRows : [designRows];
    const normalizedRows = rows.map((row) => normalizedTestCase(row));
    const normalizedMetadata = normalizedDesignMetadata(design);
    const parsed = testCaseGenerationSchema.parse({
      summary: summaryText(source.summary),
      design: { ...normalizedMetadata, testCases: normalizedRows },
    });
    const { testCases, ...metadata } = parsed.design;
    return aiOutputSchema.parse({
      summary: parsed.summary,
      changes: [],
      items: [],
      risks: [],
      testCases: testCases.map((testCase) => ({
        ...testCase,
        expectedResult: testCase.expectedResults.join('\n'),
      })),
      testCaseDesign: metadata,
      humanReviewRequired: true,
      completenessReport: null,
      workflowProposal: null,
    });
  }

  const rowsValue = source.testCases ?? source.test_cases ?? source.cases;
  const rows = Array.isArray(rowsValue)
    ? rowsValue
    : Object.keys(record(rowsValue)).length ? [rowsValue] : [];
  const legacyCases = rows.map((row) => normalizedTestCase(row)).map((testCase) => ({
    ...testCase,
    expectedResult: testCase.expectedResults.join('\n'),
  }));
  return aiOutputSchema.parse({
    summary: summaryText(source.summary),
    changes: [],
    items: [],
    risks: [],
    testCases: legacyCases,
    testCaseDesign: null,
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: null,
  });
}

export function parseTestCaseGenerationText(text: string): AIOutput {
  const withoutThinking = text.trim()
    .replace(/<think>[\s\S]*?<\/think>/gi, '')
    .replace(/```(?:json)?/gi, '')
    .replace(/```/g, '')
    .trim();
  const fenced = [...text.matchAll(/```(?:json)?\s*([\s\S]*?)```/gi)]
    .map((match) => match[1].trim());
  const objectStart = withoutThinking.indexOf('{');
  const objectEnd = withoutThinking.lastIndexOf('}');
  const candidates = [
    withoutThinking,
    ...fenced,
    objectStart >= 0 && objectEnd > objectStart ? withoutThinking.slice(objectStart, objectEnd + 1) : '',
  ].filter((candidate, index, all) => Boolean(candidate) && all.indexOf(candidate) === index);
  let lastError: unknown = new Error('test case output did not contain JSON');
  for (const candidate of candidates) {
    try {
      const parsed = JSON.parse(candidate) as unknown;
      const normalized = normalizeTestCaseGenerationOutput(Array.isArray(parsed) ? { testCases: parsed } : parsed);
      if (normalized.testCases.length) return normalized;
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError;
}

/** Keep the legacy expectedResult alias in sync without changing model semantics. */
export function applyTestCaseMechanicalRepairs(_input: Record<string, unknown>, output: AIOutput): AIOutput {
  if (!output.testCases.length) return output;
  return {
    ...output,
    testCases: output.testCases.map((testCase) => ({
      ...testCase,
      expectedResult: testCase.expectedResults.join('\n'),
      notes: [...testCase.notes],
    })),
  };
}
