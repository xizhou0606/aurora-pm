export function safeGenerationFailureMessage(error: unknown): string {
  const name = error instanceof Error ? error.name : 'UnknownError';
  const message = error instanceof Error ? error.message : '';

  if (name === 'AbortError' || name === 'TimeoutError') {
    return 'model generation timed out or was cancelled';
  }
  if (message.startsWith('test case generation ended with')) {
    return 'test case generation did not complete';
  }
  if (message.startsWith('test case repair ended with')) {
    return 'test case repair did not complete';
  }
  if (message === 'test case model response was not a complete valid JSON artifact') {
    return message;
  }
  if (message.startsWith('test case context has ')
    || message.startsWith('test case common context exceeds ')
    || message.startsWith('test case context item ')
    || message.startsWith('test case context requires ')) {
    return 'test case input exceeds supported batching limits; split the plan unit into smaller runs';
  }
  if (message.includes('test case')) {
    return 'test case output did not satisfy the fixed QA rules';
  }
  return 'model provider request failed';
}

const testCaseStages = new Set(['draft_batch', 'global_plan']);
const testCasePhases = new Set(['generation', 'repair']);
const testCaseRepairKinds = new Set(['coverage', 'full_draft', 'global_plan', 'global_ref_patch']);
const testCaseFailureSteps = new Set(['invoke', 'generation', 'finish', 'parse', 'apply', 'compile', 'budget']);
const testCaseProviderFailureKinds = new Set([
  'retry_exhausted', 'not_retryable', 'aborted', 'api_call', 'unknown',
]);
const testCaseRuleFailureCodes = new Set([
  'risk_coverage', 'source_assessment', 'context_coverage', 'context_reference',
  'risk_linkage', 'uniqueness', 'smoke_selection', 'case_breadth',
  'cross_group_relationship', 'artifact_gate',
  'task_group', 'relationship_reference', 'interaction_linkage', 'global_plan_gate',
]);
const testCaseContextReferenceDiagnosticCodes = new Set([
  'unavailable_context_ref', 'undeclared_risk_link_context_ref', 'duplicate_context_ref',
]);
const testCaseContextReferencePathCategories = new Set([
  'source_assessment_context_refs', 'risk_context_refs', 'case_context_refs', 'case_risk_links',
]);

function safeFinishReason(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined;
  return new Set(['stop', 'length', 'content-filter', 'tool-calls', 'error', 'other', 'unknown'])
    .has(value) ? value : 'unknown';
}

function boundedInteger(value: unknown, maximum: number): number | undefined {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 && value <= maximum
    ? value
    : undefined;
}

function safeContextReferenceDiagnostics(value: unknown): Array<Record<string, unknown>> {
  if (!Array.isArray(value)) return [];
  const counts = new Map<string, { code: string; pathCategory: string; count: number }>();
  for (const item of value.slice(0, 100)) {
    if (!item || typeof item !== 'object' || Array.isArray(item)) continue;
    const row = item as Record<string, unknown>;
    if (typeof row.code !== 'string' || typeof row.pathCategory !== 'string'
      || !testCaseContextReferenceDiagnosticCodes.has(row.code)
      || !testCaseContextReferencePathCategories.has(row.pathCategory)) continue;
    const key = `${row.code}:${row.pathCategory}`;
    const current = counts.get(key);
    if (current) current.count += 1;
    else counts.set(key, { code: row.code, pathCategory: row.pathCategory, count: 1 });
  }
  return [...counts.values()].slice(0, 10);
}

export function safeGenerationDiagnostic(error: unknown): Record<string, unknown> {
  const row = error && typeof error === 'object'
    ? error as Record<string, unknown>
    : {};
  const diagnostic: Record<string, unknown> = {
    name: error instanceof Error ? error.name : 'UnknownError',
    message: safeGenerationFailureMessage(error),
  };
  if (typeof row.qaStage === 'string' && testCaseStages.has(row.qaStage)) {
    diagnostic.stage = row.qaStage;
  }
  if (Number.isSafeInteger(row.qaBatchNumber) && Number(row.qaBatchNumber) > 0) {
    diagnostic.batchNumber = row.qaBatchNumber;
  }
  if (typeof row.qaPhase === 'string' && testCasePhases.has(row.qaPhase)) {
    diagnostic.phase = row.qaPhase;
  }
  const fullDraftRepair = row.qaRepairKind === 'full_draft';
  if (typeof row.qaRepairKind === 'string' && testCaseRepairKinds.has(row.qaRepairKind)) {
    diagnostic.repairKind = row.qaRepairKind;
  }
  if (typeof row.qaFailureStep === 'string' && testCaseFailureSteps.has(row.qaFailureStep)) {
    diagnostic.failureStep = row.qaFailureStep;
  }
  const finishReason = safeFinishReason(row.qaFinishReason);
  if (finishReason) diagnostic.finishReason = finishReason;
  const requestedMaxOutputTokens = boundedInteger(row.qaRequestedMaxOutputTokens, 32_000);
  if (requestedMaxOutputTokens !== undefined) diagnostic.requestedMaxOutputTokens = requestedMaxOutputTokens;
  const outputTokens = boundedInteger(row.qaOutputTokens, 100_000);
  if (outputTokens !== undefined) diagnostic.outputTokens = outputTokens;
  const callElapsedMs = boundedInteger(row.qaCallElapsedMs, 10 * 60_000);
  if (callElapsedMs !== undefined) diagnostic.callElapsedMs = callElapsedMs;
  const batchCount = boundedInteger(row.qaBatchCount, 100);
  if (batchCount !== undefined) diagnostic.batchCount = batchCount;
  const fullDraftRepairAttempt = boundedInteger(row.qaFullDraftRepairAttempt, 2);
  if (fullDraftRepair && (fullDraftRepairAttempt === 1 || fullDraftRepairAttempt === 2)) {
    diagnostic.fullDraftRepairAttempt = fullDraftRepairAttempt;
  }
  const coverageDiagnosticCount = boundedInteger(row.qaCoverageDiagnosticCount, 100);
  if (coverageDiagnosticCount !== undefined) {
    diagnostic.coverageDiagnosticCount = coverageDiagnosticCount;
  }
  const globalRefDiagnosticCount = boundedInteger(row.qaGlobalRefDiagnosticCount, 100);
  if (globalRefDiagnosticCount !== undefined) {
    diagnostic.globalRefDiagnosticCount = globalRefDiagnosticCount;
  }
  if (typeof row.qaProviderFailureKind === 'string'
    && testCaseProviderFailureKinds.has(row.qaProviderFailureKind)) {
    diagnostic.providerFailureKind = row.qaProviderFailureKind;
  }
  const providerStatusCode = boundedInteger(row.qaProviderStatusCode, 599);
  if (providerStatusCode !== undefined && providerStatusCode >= 400) {
    diagnostic.providerStatusCode = providerStatusCode;
  }
  if (typeof row.qaProviderRetryable === 'boolean') {
    diagnostic.providerRetryable = row.qaProviderRetryable;
  }
  if (Array.isArray(row.qaRuleFailureCodes)) {
    const ruleFailureCodes = [...new Set(row.qaRuleFailureCodes.filter((value): value is string => (
      typeof value === 'string' && testCaseRuleFailureCodes.has(value)
    )))].slice(0, 10);
    if (ruleFailureCodes.length) diagnostic.ruleFailureCodes = ruleFailureCodes;
  }
  const contextReferenceDiagnostics = safeContextReferenceDiagnostics(
    row.qaContextReferenceDiagnostics,
  );
  if (contextReferenceDiagnostics.length) {
    diagnostic.contextReferenceDiagnostics = contextReferenceDiagnostics;
  }
  return diagnostic;
}
