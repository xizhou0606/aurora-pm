import { describe, expect, it, vi } from 'bun:test';
import { APICallError, RetryError } from 'ai';
import {
  assertCompleteTestCaseFinishReason,
  attachTestCaseModelTelemetry,
  attachTestCaseProviderFailureTelemetry,
  testCaseBatchConcurrency,
  testCaseGenerationOptions,
  testCaseMaxOutputTokensForInput,
  TEST_CASE_BATCH_CONCURRENCY,
  TEST_CASE_CONTEXT_BATCH_MAX_OUTPUT_TOKENS,
  TEST_CASE_GLOBAL_PLAN_MAX_OUTPUT_TOKENS,
  TEST_CASE_MAX_OUTPUT_TOKENS,
  TEST_CASE_REQUEST_TIMEOUT_MS,
} from './runtime/generate-runtime.js';
import { combineTokenUsage } from './runtime/generate-telemetry.js';
import { createTestCaseFailureLatch, raceQATestCaseAbort } from './runtime/generate-errors.js';
import { requestTimeoutForCapability } from '../../executor.js';

describe('test-case generation', () => {
  it.each(['length', 'content-filter', 'tool-calls', 'error', 'other', 'unknown'])('rejects non-stop finish reason %s', (finishReason) => {
    expect(() => assertCompleteTestCaseFinishReason(finishReason, 'generation')).toThrow(/partial artifact/);
  });

  it('accepts only an explicit stop finish reason', () => {
    expect(() => assertCompleteTestCaseFinishReason('stop', 'generation')).not.toThrow();
  });

  it('keeps the first QA failure when shared cancellation reaches sibling calls', () => {
    let aborts = 0;
    const latch = createTestCaseFailureLatch(() => { aborts += 1; }, 4);
    const length = new Error('test case generation ended with length; refusing a partial artifact');
    Object.assign(length, {
      qaPhase: 'generation',
      qaFinishReason: 'length',
      qaRequestedMaxOutputTokens: 8000,
      qaOutputTokens: 8000,
    });

    const primary = latch.capture(length, 'draft_batch', 2);
    const sibling = latch.capture(Object.assign(new Error('cancelled'), { name: 'AbortError' }), 'global_plan');

    expect(sibling).toBe(primary);
    expect(latch.primaryFailure()).toBe(primary);
    expect(primary).toMatchObject({
      qaStage: 'draft_batch',
      qaBatchNumber: 2,
      qaBatchCount: 4,
      qaFinishReason: 'length',
      qaRequestedMaxOutputTokens: 8000,
      qaOutputTokens: 8000,
    });
    expect(aborts).toBe(2);
  });

  it('rejects promptly when an abort reaches a generation promise that never settles', async () => {
    const controller = new AbortController();
    const neverSettles = new Promise<never>(() => {});
    const main = raceQATestCaseAbort(neverSettles, controller.signal);
    const timeout = new Error('QA test case request timed out');
    timeout.name = 'TimeoutError';

    controller.abort(timeout);

    await expect(main).rejects.toBe(timeout);
  });

  it('preserves the first batch failure when abort wins the Promise.all race', async () => {
    const controller = new AbortController();
    const latch = createTestCaseFailureLatch(() => controller.abort(), 2);
    const primary = new Error('draft batch failed before sibling cancellation');
    const failingBatch = Promise.resolve().then(() => {
      throw latch.capture(primary, 'draft_batch', 1);
    });
    const stalledGlobal = new Promise<never>(() => {});
    const main = raceQATestCaseAbort(
      Promise.all([failingBatch, stalledGlobal]), controller.signal,
    ).catch((error) => Promise.reject(latch.primaryFailure() ?? error));

    await expect(main).rejects.toMatchObject({
      message: primary.message,
      qaStage: 'draft_batch',
      qaBatchNumber: 1,
      qaBatchCount: 2,
    });
  });

  it('cleans up its abort listener and handles a late loser rejection', async () => {
    const controller = new AbortController();
    const removeListener = vi.spyOn(controller.signal, 'removeEventListener');
    let rejectLate!: (error: Error) => void;
    const operation = new Promise<never>((_resolve, reject) => { rejectLate = reject; });
    const unhandled: unknown[] = [];
    const onUnhandled = (reason: unknown) => { unhandled.push(reason); };
    process.on('unhandledRejection', onUnhandled);
    try {
      const main = raceQATestCaseAbort(operation, controller.signal);
      controller.abort();
      await expect(main).rejects.toMatchObject({ name: 'AbortError' });

      rejectLate(new Error('late provider rejection'));
      await new Promise<void>((resolve) => { setImmediate(resolve); });

      expect(removeListener).toHaveBeenCalledWith('abort', expect.any(Function));
      expect(unhandled).toEqual([]);
    } finally {
      process.off('unhandledRejection', onUnhandled);
      removeListener.mockRestore();
    }
  });

  it('removes its abort listener after normal operation settlement', async () => {
    const controller = new AbortController();
    const removeListener = vi.spyOn(controller.signal, 'removeEventListener');

    await expect(raceQATestCaseAbort(
      Promise.resolve('complete'), controller.signal,
    )).resolves.toBe('complete');

    expect(removeListener).toHaveBeenCalledWith('abort', expect.any(Function));
    removeListener.mockRestore();
  });

  it('preserves AI SDK prototype getters when attaching safe model telemetry', () => {
    const result = Object.create({
      get text() { return '{"summary":"complete"}'; },
      get usage() { return { inputTokens: 12, outputTokens: 8, totalTokens: 20 }; },
      get finishReason() { return 'stop'; },
    });

    expect({ ...result }).toEqual({});
    expect(attachTestCaseModelTelemetry(result, 3500, 9123)).toEqual({
      text: '{"summary":"complete"}',
      usage: { inputTokens: 12, outputTokens: 8, totalTokens: 20 },
      finishReason: 'stop',
      requestedMaxOutputTokens: 3500,
      elapsedMs: 9123,
    });
  });

  it('classifies retry-exhausted provider failures without copying response content', () => {
    const apiError = new APICallError({
      message: 'provider response contains sensitive text',
      url: 'https://provider.example/v1/chat/completions',
      requestBodyValues: { prompt: 'sensitive prompt' },
      statusCode: 503,
      responseBody: 'sensitive response body',
      isRetryable: true,
    });
    const retryError = new RetryError({
      message: 'retry attempts exhausted',
      reason: 'maxRetriesExceeded',
      errors: [apiError],
    });

    expect(attachTestCaseProviderFailureTelemetry(retryError, 3500, 190_000))
      .toMatchObject({
        name: retryError.name,
        qaPhase: 'generation',
        qaFailureStep: 'generation',
        qaRequestedMaxOutputTokens: 3500,
        qaCallElapsedMs: 190_000,
        qaProviderFailureKind: 'retry_exhausted',
        qaProviderStatusCode: 503,
        qaProviderRetryable: true,
      });
  });

  it('flattens initial and repair token usage for backend accounting', () => {
    expect(combineTokenUsage(
      { inputTokens: 120, outputTokens: 80, totalTokens: 200 },
      { inputTokens: 60, outputTokens: 40, totalTokens: 100 },
    )).toEqual({ inputTokens: 180, outputTokens: 120, totalTokens: 300 });
    expect(combineTokenUsage(null, null)).toBeNull();
  });

  it('uses a bounded thinking-on protocol-level JSON request for flash', () => {
    expect(testCaseGenerationOptions(16000, 6000)).toMatchObject({
      maxOutputTokens: 6000,
      temperature: 0,
      seed: 20260811,
      output: { name: 'json' },
      providerOptions: { dashscope: { enable_thinking: true, thinking_budget: 512 } },
    });
    expect(testCaseGenerationOptions(16000)).toMatchObject({
      maxOutputTokens: TEST_CASE_MAX_OUTPUT_TOKENS,
      temperature: 0,
      seed: 20260811,
      output: { name: 'json' },
      providerOptions: { dashscope: { enable_thinking: true, thinking_budget: 512 } },
    });
    expect(TEST_CASE_MAX_OUTPUT_TOKENS).toBe(8000);
  });

  it('gives compact context batches the complete QA output budget', () => {
    expect(testCaseMaxOutputTokensForInput({ test_case_context: {} }, 12000))
      .toBe(TEST_CASE_CONTEXT_BATCH_MAX_OUTPUT_TOKENS);
    expect(testCaseMaxOutputTokensForInput({ test_case_context: {} }, 5000)).toBe(5000);
    expect(testCaseMaxOutputTokensForInput({}, 12000)).toBe(TEST_CASE_MAX_OUTPUT_TOKENS);
    expect(TEST_CASE_CONTEXT_BATCH_MAX_OUTPUT_TOKENS).toBe(8000);
  });

  it('caps qa.test_cases timeout so slow model calls fail closed sooner', () => {
    expect(requestTimeoutForCapability('qa.test_cases', 360_000)).toBe(TEST_CASE_REQUEST_TIMEOUT_MS);
    expect(requestTimeoutForCapability('qa.test_cases', 60_000)).toBe(60_000);
    expect(requestTimeoutForCapability('workflow.generate', 360_000)).toBe(360_000);
    expect(TEST_CASE_REQUEST_TIMEOUT_MS).toBe(330_000);
    expect(TEST_CASE_BATCH_CONCURRENCY).toBe(4);
    expect(testCaseBatchConcurrency(false)).toBe(4);
    expect(testCaseBatchConcurrency(true)).toBe(3);
    expect(TEST_CASE_GLOBAL_PLAN_MAX_OUTPUT_TOKENS).toBe(6000);
  });
});
