import { APICallError, NoObjectGeneratedError, Output, RetryError } from 'ai';
import type { QATestCaseModelCallKind, QATestCaseModelCallTelemetry } from '../../../../shared/ai/types.js';
import { record } from '../../batching/qa-test-case-batching-input.js';

// Bound first-pass latency before the provider ceiling. Responses that reach
// this limit are rejected because a partial test artifact is not publishable.
export const TEST_CASE_MAX_OUTPUT_TOKENS = 8000;
export const TEST_CASE_CONTEXT_BATCH_MAX_OUTPUT_TOKENS = 8000;
export const TEST_CASE_GLOBAL_PLAN_MAX_OUTPUT_TOKENS = 6000;
export const TEST_CASE_GLOBAL_REF_PATCH_MAX_OUTPUT_TOKENS = 1500;
export const TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS = 2500;
// Leave the worker 30 seconds to receive and persist the final response.
export const TEST_CASE_REQUEST_TIMEOUT_MS = 330_000;
export const TEST_CASE_REPAIR_MIN_REMAINING_MS = 90_000;
export const TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS = 50_000;
export const TEST_CASE_GLOBAL_REF_PATCH_MIN_REMAINING_MS = 45_000;
export const TEST_CASE_BATCH_CONCURRENCY = 4;

export function testCaseBatchConcurrency(hasGlobalPlan: boolean): number {
  return hasGlobalPlan
    ? Math.max(1, TEST_CASE_BATCH_CONCURRENCY - 1)
    : TEST_CASE_BATCH_CONCURRENCY;
}

export function testCaseGenerationOptions(
  maxOutputTokens: number,
  testCaseMaxOutputTokens = TEST_CASE_MAX_OUTPUT_TOKENS,
) {
  return {
    maxOutputTokens: Math.min(maxOutputTokens, testCaseMaxOutputTokens),
    temperature: 0,
    seed: 20260811,
    output: Output.json(),
    providerOptions: {
      // Fast tier = qwen3.7-flash; keep thinking on, but cap budget for wall-clock.
      dashscope: { enable_thinking: true, thinking_budget: 512 },
    },
  };
}

export function testCaseMaxOutputTokensForInput(
  input: Record<string, unknown>,
  configuredMaxOutputTokens: number,
): number {
  const context = input.test_case_context;
  const fixedLimit = context && typeof context === 'object' && !Array.isArray(context)
    ? TEST_CASE_CONTEXT_BATCH_MAX_OUTPUT_TOKENS
    : TEST_CASE_MAX_OUTPUT_TOKENS;
  return Math.min(configuredMaxOutputTokens, fixedLimit);
}

export function assertCompleteTestCaseFinishReason(
  finishReason: unknown,
  phase: 'generation' | 'repair',
  result?: TestCaseModelTextResult,
): void {
  if (finishReason !== 'stop') {
    const error = new Error(`test case ${phase} ended with ${String(finishReason)}; refusing a partial artifact`);
    const usage = record(result?.usage);
    Object.assign(error, {
      qaPhase: phase,
      qaFinishReason: typeof finishReason === 'string' ? finishReason : 'unknown',
      ...(result?.requestedMaxOutputTokens === undefined
        ? {} : { qaRequestedMaxOutputTokens: result.requestedMaxOutputTokens }),
      ...(typeof usage.outputTokens === 'number' ? { qaOutputTokens: usage.outputTokens } : {}),
      ...(result?.elapsedMs === undefined ? {} : { qaCallElapsedMs: result.elapsedMs }),
    });
    throw error;
  }
}

export function canRewriteIncompleteTestCase(finishReason: unknown): boolean {
  return finishReason === undefined
    || finishReason === 'length'
    || finishReason === 'error'
    || finishReason === 'other'
    || finishReason === 'unknown';
}

function hasStrictCompleteJSONObject(text: string): boolean {
  const trimmed = text.trim();
  if (!trimmed.startsWith('{') || !trimmed.endsWith('}')) return false;
  try {
    const value = JSON.parse(trimmed);
    return Boolean(value && typeof value === 'object' && !Array.isArray(value));
  } catch {
    return false;
  }
}

export function usesMissingFinishReasonFallback(result: TestCaseModelTextResult): boolean {
  return (result.finishReason === undefined || result.finishReason === 'unknown')
    && hasStrictCompleteJSONObject(result.text);
}

export function incompleteTestCaseRewriteInstruction(kind: 'Draft IR' | 'Global Plan'): string {
  return [
    `上一轮 ${kind} 未正常结束，上一轮内容已丢弃。请基于同一业务输入从头生成一个更紧凑的完整 JSON 对象。`,
    '不得续写、引用或解释上一轮内容；不得输出 Markdown、思考过程或 JSON 之外的文字。',
    '本次必须完整结束并包含契约要求的全部字段，否则整次运行失败。',
  ].join('\n');
}

export type TestCaseModelTextResult = {
  text: string;
  usage: unknown;
  finishReason: unknown;
  requestedMaxOutputTokens?: number;
  elapsedMs?: number;
};

export type TestCaseModelTextRequest = {
  instructions: string;
  prompt?: string;
  messages?: Array<{ role: 'user' | 'assistant'; content: string }>;
  abortSignal: AbortSignal;
  maxOutputTokens?: number;
  telemetryKind?: QATestCaseModelCallKind;
};

export type GenerateTestCaseModelText = (
  request: TestCaseModelTextRequest,
) => Promise<TestCaseModelTextResult>;

export function attachTestCaseModelTelemetry(
  result: TestCaseModelTextResult,
  requestedMaxOutputTokens: number,
  elapsedMs: number,
): TestCaseModelTextResult {
  // AI SDK results expose core fields through prototype getters. Read them
  // explicitly; object spread silently drops those accessors.
  return {
    text: result.text,
    usage: result.usage,
    finishReason: result.finishReason,
    requestedMaxOutputTokens,
    elapsedMs,
  };
}

function apiCallFailure(error: unknown): APICallError | undefined {
  const candidate = RetryError.isInstance(error) ? error.lastError : error;
  if (APICallError.isInstance(candidate)) return candidate;
  if (candidate instanceof Error && APICallError.isInstance(candidate.cause)) {
    return candidate.cause;
  }
  return undefined;
}

export function attachTestCaseProviderFailureTelemetry(
  cause: unknown,
  requestedMaxOutputTokens: number,
  elapsedMs: number,
): Error {
  const error = cause instanceof Error
    ? new Error(cause.message, { cause })
    : new Error('test case model provider call failed', { cause });
  error.name = cause instanceof Error ? cause.name : 'Error';
  const apiError = apiCallFailure(cause);
  const failureKind = RetryError.isInstance(cause)
    ? cause.reason === 'maxRetriesExceeded'
      ? 'retry_exhausted'
      : cause.reason === 'errorNotRetryable'
        ? 'not_retryable'
        : 'aborted'
    : apiError ? 'api_call' : 'unknown';
  Object.assign(error, {
    qaPhase: 'generation',
    qaFailureStep: 'generation',
    qaRequestedMaxOutputTokens: requestedMaxOutputTokens,
    qaCallElapsedMs: elapsedMs,
    qaProviderFailureKind: failureKind,
    ...(apiError?.statusCode === undefined ? {} : { qaProviderStatusCode: apiError.statusCode }),
    ...(apiError === undefined ? {} : { qaProviderRetryable: apiError.isRetryable }),
  });
  return error;
}

export async function recoverTestCaseJSONModelResult(
  generate: () => Promise<TestCaseModelTextResult>,
): Promise<TestCaseModelTextResult> {
  try {
    return await generate();
  } catch (error) {
    if (!NoObjectGeneratedError.isInstance(error)) throw error;
    return {
      text: error.text ?? '',
      usage: error.usage ?? null,
      // AI SDK invokes Output.json().parseCompleteOutput only after a `stop`.
      // Some compatible-provider errors omit the copied finishReason, but the
      // NoObject error itself still proves that the stop-only parse path ran.
      finishReason: error.finishReason ?? 'stop',
    };
  }
}
