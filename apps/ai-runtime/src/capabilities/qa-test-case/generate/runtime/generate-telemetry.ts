import type {
  QATestCaseModelCallKind,
  QATestCaseModelCallTelemetry,
  QATestCaseRepairRuleCode,
  QATestCaseStageSuccessTelemetry,
  QATestCaseSuccessTelemetry,
  TestCaseRepairReason,
} from '../../../../shared/ai/types.js';
import { QA_TEST_CASE_MAX_BATCHES, QA_TEST_CASE_MAX_ITEMS } from '../../batching/qa-test-case-batching-partition.js';
import { record } from '../../batching/qa-test-case-batching-input.js';
import {
  TEST_CASE_BATCH_CONCURRENCY,
  TEST_CASE_REQUEST_TIMEOUT_MS,
  type TestCaseModelTextResult,
} from './generate-runtime.js';
import { QATestCaseDraftCompilationError } from '../../draft/fixtures/qa-test-case-draft-error.js';
import { QATestCaseGlobalPlanCompilationError } from '../../global-plan/qa-test-case-global-plan.js';

export type QATestCaseRepairDiagnosticRecorder = (
  codes: readonly QATestCaseRepairRuleCode[],
) => void;

const qaTestCaseRepairRuleCodeSet = new Set<QATestCaseRepairRuleCode>([
  'incomplete_response',
  'invalid_json',
  'schema_structure',
  'risk_coverage',
  'source_assessment',
  'context_coverage',
  'context_reference',
  'risk_linkage',
  'uniqueness',
  'smoke_selection',
  'case_breadth',
  'cross_group_relationship',
  'artifact_gate',
  'task_group',
  'relationship_reference',
  'interaction_linkage',
  'global_plan_gate',
]);

function safeQATestCaseRepairRuleCodes(
  values: readonly unknown[],
): QATestCaseRepairRuleCode[] {
  return [...new Set(values.filter((value): value is QATestCaseRepairRuleCode => (
    typeof value === 'string' && qaTestCaseRepairRuleCodeSet.has(value as QATestCaseRepairRuleCode)
  )))];
}

export function recordQATestCaseRepairRules(
  recorder: QATestCaseRepairDiagnosticRecorder | undefined,
  codes: readonly unknown[],
): void {
  const safeCodes = safeQATestCaseRepairRuleCodes(codes);
  if (safeCodes.length) recorder?.(safeCodes);
}

export function inspectionRepairRuleCode(
  failure: 'parse' | 'structure' | null,
): QATestCaseRepairRuleCode | undefined {
  if (failure === 'parse') return 'invalid_json';
  if (failure === 'structure') return 'schema_structure';
  return undefined;
}

export function draftCompilationIssueCodes(error: QATestCaseDraftCompilationError): QATestCaseRepairRuleCode[] {
  const diagnosticMessages = new Set(error.diagnostics.map((item) => item.message));
  const codes = error.issues.map((issue) => {
    if (diagnosticMessages.has(issue) || issue.includes('coverage decision')) return 'risk_coverage';
    if (issue.includes('source assessment')) return 'source_assessment';
    if (issue.includes('context item is not covered')) return 'context_coverage';
    if (issue.includes('context ref')) return 'context_reference';
    if (issue.includes('unknown risk') || issue.includes('risk is not linked')) return 'risk_linkage';
    if (issue.includes('must be unique')) return 'uniqueness';
    if (issue.includes('smoke case')) return 'smoke_selection';
    if (issue.includes('too many unrelated context items')) return 'case_breadth';
    if (issue.includes('spans multiple requirement groups')) return 'cross_group_relationship';
    return 'artifact_gate';
  });
  return [...new Set(codes)];
}

export function globalPlanIssueCodes(error: QATestCaseGlobalPlanCompilationError): QATestCaseRepairRuleCode[] {
  const codes = error.issues.map((issue) => {
    if (issue.includes('must be unique')) return 'uniqueness';
    if (issue.includes('unavailable context ref')) return 'context_reference';
    if (issue.includes('task ref is not linked')) return 'task_group';
    if (issue.includes('at least two requirement groups')) return 'cross_group_relationship';
    if (issue.includes('conflict must cite')) return 'source_assessment';
    if (issue.includes('relationship ref')) return 'relationship_reference';
    if (issue.includes('unknown interaction risk')
      || issue.includes('interaction is not linked')) return 'interaction_linkage';
    if (issue.includes('risk link')) return 'risk_linkage';
    return 'global_plan_gate';
  });
  return [...new Set(codes)];
}

export function combineTokenUsages(...values: unknown[]): unknown {
  const usageRows = values
    .map((value) => record(value))
    .filter((value) => Object.keys(value).length > 0);
  if (!usageRows.length) return null;

  const sum = (key: 'inputTokens' | 'outputTokens' | 'totalTokens') => usageRows.reduce(
    (total, usage) => total + (typeof usage[key] === 'number' && Number.isFinite(usage[key])
      ? usage[key]
      : 0),
    0,
  );
  const inputTokens = sum('inputTokens');
  const outputTokens = sum('outputTokens');
  const totalTokens = usageRows.reduce((total, usage) => {
    if (typeof usage.totalTokens === 'number' && Number.isFinite(usage.totalTokens)) {
      return total + usage.totalTokens;
    }
    const rowInput = typeof usage.inputTokens === 'number' && Number.isFinite(usage.inputTokens)
      ? usage.inputTokens
      : 0;
    const rowOutput = typeof usage.outputTokens === 'number' && Number.isFinite(usage.outputTokens)
      ? usage.outputTokens
      : 0;
    return total + rowInput + rowOutput;
  }, 0);
  return {
    inputTokens,
    outputTokens,
    totalTokens,
  };
}

export function combineTokenUsage(initial: unknown, repair: unknown): unknown {
  return combineTokenUsages(initial, repair);
}

const QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT = 10_000_000;
const QA_TEST_CASE_TELEMETRY_MAX_CALLS_PER_STAGE = 4;
const QA_TEST_CASE_TELEMETRY_MAX_REPAIR_DIAGNOSTICS = 100;

function boundedTelemetryInteger(value: unknown, maximum: number): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0;
  return Math.min(maximum, Math.max(0, Math.floor(value)));
}

function safeQATestCaseFinishReason(
  value: unknown,
): QATestCaseModelCallTelemetry['finishReason'] {
  switch (value) {
    case 'stop':
    case 'length':
    case 'content-filter':
    case 'tool-calls':
    case 'error':
    case 'other':
      return value;
    default:
      return 'unknown';
  }
}

export function qaTestCaseModelCallSuccessTelemetry(
  callNumber: number,
  kind: QATestCaseModelCallKind,
  result: TestCaseModelTextResult,
): QATestCaseModelCallTelemetry {
  const usage = record(result.usage);
  const inputTokens = boundedTelemetryInteger(
    usage.inputTokens,
    QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT,
  );
  const outputTokens = boundedTelemetryInteger(
    usage.outputTokens,
    QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT,
  );
  const fallbackTotal = Math.min(
    QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT,
    inputTokens + outputTokens,
  );
  const totalTokens = typeof usage.totalTokens === 'number' && Number.isFinite(usage.totalTokens)
    ? boundedTelemetryInteger(usage.totalTokens, QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT)
    : fallbackTotal;
  return {
    callNumber: boundedTelemetryInteger(callNumber, QA_TEST_CASE_TELEMETRY_MAX_CALLS_PER_STAGE),
    kind,
    elapsedMs: boundedTelemetryInteger(result.elapsedMs, TEST_CASE_REQUEST_TIMEOUT_MS),
    inputTokens,
    outputTokens,
    totalTokens,
    finishReason: safeQATestCaseFinishReason(result.finishReason),
  };
}

export function qaTestCaseStageSuccessTelemetry(options: {
  stage: QATestCaseStageSuccessTelemetry['stage'];
  batchNumber?: number;
  calls: readonly QATestCaseModelCallTelemetry[];
  wallElapsedMs: number;
  repaired: boolean;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  coveragePatched?: boolean;
  globalRefPatched?: boolean;
  repairDiagnosticCodes?: readonly unknown[];
}): QATestCaseStageSuccessTelemetry {
  const calls = options.calls.slice(0, QA_TEST_CASE_TELEMETRY_MAX_CALLS_PER_STAGE);
  const sum = (key: 'elapsedMs' | 'inputTokens' | 'outputTokens' | 'totalTokens') => (
    calls.reduce((total, call) => total + call[key], 0)
  );
  const repairDiagnosticCodes = (options.repairDiagnosticCodes ?? [])
    .slice(0, QA_TEST_CASE_TELEMETRY_MAX_REPAIR_DIAGNOSTICS);
  const repairRuleCodes = safeQATestCaseRepairRuleCodes(repairDiagnosticCodes);
  const repairDiagnosticCategories = repairRuleCodes.map((category) => ({
    category,
    count: repairDiagnosticCodes.filter((value) => value === category).length,
  }));
  return {
    stage: options.stage,
    ...(options.batchNumber === undefined ? {} : {
      batchNumber: boundedTelemetryInteger(options.batchNumber, QA_TEST_CASE_MAX_BATCHES),
    }),
    calls,
    modelCallCount: calls.length,
    modelElapsedMs: boundedTelemetryInteger(sum('elapsedMs'), TEST_CASE_REQUEST_TIMEOUT_MS),
    wallElapsedMs: boundedTelemetryInteger(options.wallElapsedMs, TEST_CASE_REQUEST_TIMEOUT_MS),
    inputTokens: boundedTelemetryInteger(sum('inputTokens'), QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT),
    outputTokens: boundedTelemetryInteger(sum('outputTokens'), QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT),
    totalTokens: boundedTelemetryInteger(sum('totalTokens'), QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT),
    repaired: options.repaired,
    repairReason: options.repairReason ?? 'none',
    finishReasonFallback: options.finishReasonFallback,
    coveragePatched: options.coveragePatched ?? false,
    globalRefPatched: options.globalRefPatched ?? false,
    repairRuleCodes,
    repairDiagnosticCategories,
  };
}

export function qaTestCaseGenerationSuccessTelemetry(options: {
  batchCount: number;
  requirementGroupCount: number;
  globalPlan: boolean;
  totalConcurrency: number;
  wallElapsedMs: number;
  stages: readonly QATestCaseStageSuccessTelemetry[];
}): QATestCaseSuccessTelemetry {
  const stages = options.stages.slice(0, QA_TEST_CASE_MAX_BATCHES + 1);
  const sum = (key: 'modelCallCount' | 'modelElapsedMs' | 'inputTokens' | 'outputTokens' | 'totalTokens') => (
    stages.reduce((total, stage) => total + stage[key], 0)
  );
  return {
    batchCount: boundedTelemetryInteger(options.batchCount, QA_TEST_CASE_MAX_BATCHES),
    requirementGroupCount: boundedTelemetryInteger(
      options.requirementGroupCount,
      QA_TEST_CASE_MAX_ITEMS,
    ),
    globalPlan: options.globalPlan,
    totalConcurrency: boundedTelemetryInteger(options.totalConcurrency, TEST_CASE_BATCH_CONCURRENCY),
    modelCallCount: boundedTelemetryInteger(
      sum('modelCallCount'),
      (QA_TEST_CASE_MAX_BATCHES + 1) * QA_TEST_CASE_TELEMETRY_MAX_CALLS_PER_STAGE,
    ),
    modelElapsedMs: boundedTelemetryInteger(
      sum('modelElapsedMs'),
      TEST_CASE_REQUEST_TIMEOUT_MS * TEST_CASE_BATCH_CONCURRENCY,
    ),
    wallElapsedMs: boundedTelemetryInteger(options.wallElapsedMs, TEST_CASE_REQUEST_TIMEOUT_MS),
    inputTokens: boundedTelemetryInteger(sum('inputTokens'), QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT),
    outputTokens: boundedTelemetryInteger(sum('outputTokens'), QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT),
    totalTokens: boundedTelemetryInteger(sum('totalTokens'), QA_TEST_CASE_TELEMETRY_MAX_TOKEN_COUNT),
    stages,
  };
}
