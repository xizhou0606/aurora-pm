import type { TestCaseRepairReason } from '../../../../shared/ai/types.js';
import type { TestCaseModelTextResult } from './generate-runtime.js';
import { draftCompilationIssueCodes, globalPlanIssueCodes } from './generate-telemetry.js';
import { record } from '../../batching/qa-test-case-batching-input.js';
import { QATestCaseDraftCompilationError } from '../../draft/fixtures/qa-test-case-draft-error.js';
import { QATestCaseGlobalPlanCompilationError } from '../../global-plan/qa-test-case-global-plan.js';

type TestCaseRepairKind = 'coverage' | 'full_draft' | 'global_plan' | 'global_ref_patch';
type TestCaseFailureStep = 'invoke' | 'generation' | 'finish' | 'parse' | 'apply' | 'compile' | 'budget';
export type FullDraftRepairAttempt = 1 | 2;

export function testCaseRepairStepError(
  cause: unknown,
  kind: TestCaseRepairKind,
  step: TestCaseFailureStep,
  result?: TestCaseModelTextResult,
  coverageDiagnosticCount?: number,
  globalRefDiagnosticCount?: number,
  fullDraftRepairAttempt?: FullDraftRepairAttempt,
): Error {
  const error = cause instanceof Error
    ? new Error(cause.message, { cause })
    : new Error('test case repair failed', { cause });
  error.name = cause instanceof Error ? cause.name : 'Error';
  const row = cause && typeof cause === 'object' ? cause as Record<string, unknown> : {};
  const usage = record(result?.usage);
  const inheritedFullDraftRepairAttempt = row.qaFullDraftRepairAttempt === 1
    || row.qaFullDraftRepairAttempt === 2
    ? row.qaFullDraftRepairAttempt
    : undefined;
  const ruleFailureCodes = cause instanceof QATestCaseDraftCompilationError
    ? draftCompilationIssueCodes(cause)
    : cause instanceof QATestCaseGlobalPlanCompilationError
      ? globalPlanIssueCodes(cause)
      : [];
  const contextReferenceDiagnostics = cause instanceof QATestCaseDraftCompilationError
    ? cause.contextReferenceDiagnostics
    : [];
  Object.assign(error, {
    qaPhase: 'repair',
    qaRepairKind: kind,
    qaFailureStep: step,
    ...(coverageDiagnosticCount === undefined ? {} : { qaCoverageDiagnosticCount: coverageDiagnosticCount }),
    ...(globalRefDiagnosticCount === undefined ? {} : { qaGlobalRefDiagnosticCount: globalRefDiagnosticCount }),
    ...(fullDraftRepairAttempt === undefined && inheritedFullDraftRepairAttempt === undefined
      ? {}
      : { qaFullDraftRepairAttempt: fullDraftRepairAttempt ?? inheritedFullDraftRepairAttempt }),
    ...(ruleFailureCodes.length ? { qaRuleFailureCodes: ruleFailureCodes } : {}),
    ...(contextReferenceDiagnostics.length
      ? { qaContextReferenceDiagnostics: contextReferenceDiagnostics }
      : {}),
    ...(row.qaFinishReason === undefined ? {} : { qaFinishReason: row.qaFinishReason }),
    ...(row.qaRequestedMaxOutputTokens === undefined
      ? result?.requestedMaxOutputTokens === undefined
        ? {}
        : { qaRequestedMaxOutputTokens: result.requestedMaxOutputTokens }
      : { qaRequestedMaxOutputTokens: row.qaRequestedMaxOutputTokens }),
    ...(row.qaOutputTokens === undefined
      ? typeof usage.outputTokens === 'number' ? { qaOutputTokens: usage.outputTokens } : {}
      : { qaOutputTokens: row.qaOutputTokens }),
    ...(row.qaCallElapsedMs === undefined
      ? result?.elapsedMs === undefined ? {} : { qaCallElapsedMs: result.elapsedMs }
      : { qaCallElapsedMs: row.qaCallElapsedMs }),
    ...(row.qaProviderFailureKind === undefined
      ? {} : { qaProviderFailureKind: row.qaProviderFailureKind }),
    ...(row.qaProviderStatusCode === undefined
      ? {} : { qaProviderStatusCode: row.qaProviderStatusCode }),
    ...(row.qaProviderRetryable === undefined
      ? {} : { qaProviderRetryable: row.qaProviderRetryable }),
  });
  return error;
}

export function testCaseStageError(
  cause: unknown,
  qaStage: 'draft_batch' | 'global_plan',
  qaBatchNumber?: number,
  qaBatchCount?: number,
): Error {
  const error = cause instanceof Error
    ? new Error(cause.message, { cause })
    : new Error('test case generation failed', { cause });
  error.name = cause instanceof Error ? cause.name : 'Error';
  const row = cause && typeof cause === 'object' ? cause as Record<string, unknown> : {};
  Object.assign(error, {
    qaStage,
    ...(qaBatchNumber === undefined ? {} : { qaBatchNumber }),
    ...(qaBatchCount === undefined ? {} : { qaBatchCount }),
    ...(row.qaPhase === undefined ? {} : { qaPhase: row.qaPhase }),
    ...(row.qaFinishReason === undefined ? {} : { qaFinishReason: row.qaFinishReason }),
    ...(row.qaRequestedMaxOutputTokens === undefined
      ? {} : { qaRequestedMaxOutputTokens: row.qaRequestedMaxOutputTokens }),
    ...(row.qaOutputTokens === undefined ? {} : { qaOutputTokens: row.qaOutputTokens }),
    ...(row.qaCallElapsedMs === undefined ? {} : { qaCallElapsedMs: row.qaCallElapsedMs }),
    ...(row.qaRepairKind === undefined ? {} : { qaRepairKind: row.qaRepairKind }),
    ...(row.qaFailureStep === undefined ? {} : { qaFailureStep: row.qaFailureStep }),
    ...(row.qaCoverageDiagnosticCount === undefined
      ? {} : { qaCoverageDiagnosticCount: row.qaCoverageDiagnosticCount }),
    ...(row.qaGlobalRefDiagnosticCount === undefined
      ? {} : { qaGlobalRefDiagnosticCount: row.qaGlobalRefDiagnosticCount }),
    ...(row.qaFullDraftRepairAttempt === undefined
      ? {} : { qaFullDraftRepairAttempt: row.qaFullDraftRepairAttempt }),
    ...(row.qaRuleFailureCodes === undefined ? {} : { qaRuleFailureCodes: row.qaRuleFailureCodes }),
    ...(row.qaContextReferenceDiagnostics === undefined
      ? {} : { qaContextReferenceDiagnostics: row.qaContextReferenceDiagnostics }),
    ...(row.qaProviderFailureKind === undefined
      ? {} : { qaProviderFailureKind: row.qaProviderFailureKind }),
    ...(row.qaProviderStatusCode === undefined
      ? {} : { qaProviderStatusCode: row.qaProviderStatusCode }),
    ...(row.qaProviderRetryable === undefined
      ? {} : { qaProviderRetryable: row.qaProviderRetryable }),
  });
  return error;
}

export function createTestCaseFailureLatch(abort: () => void, batchCount: number): {
  capture: (
    error: unknown,
    stage: 'draft_batch' | 'global_plan',
    batchNumber?: number,
  ) => Error;
  primaryFailure: () => Error | undefined;
} {
  let primaryFailure: Error | undefined;
  return {
    capture(error, stage, batchNumber) {
      primaryFailure ??= testCaseStageError(error, stage, batchNumber, batchCount);
      abort();
      return primaryFailure;
    },
    primaryFailure: () => primaryFailure,
  };
}

function testCaseAbortError(signal: AbortSignal): Error {
  if (signal.reason instanceof Error) return signal.reason;
  const error = new Error('QA test case generation was aborted');
  error.name = 'AbortError';
  return error;
}

export function raceQATestCaseAbort<T>(operation: Promise<T>, signal: AbortSignal): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    let settled = false;
    const finish = (settle: () => void) => {
      if (settled) return;
      settled = true;
      signal.removeEventListener('abort', onAbort);
      settle();
    };
    const onAbort = () => finish(() => reject(testCaseAbortError(signal)));
    operation.then(
      (value) => finish(() => resolve(value)),
      (error) => finish(() => reject(error)),
    );
    if (signal.aborted) onAbort();
    else signal.addEventListener('abort', onAbort, { once: true });
  });
}
