import { describe, expect, it } from 'bun:test';
import { generateAndValidateTestCaseBatch } from './pipeline/generate-batch.js';
import { recoverTestCaseJSONModelResult } from './runtime/generate-runtime.js';
import { completeTestCaseModelText, completeTestCaseOutput, testNoObjectGeneratedError } from './generate-test-fixtures.js';

describe('test-case generation', () => {
  it('rewrites a parse failure once with the full original text and combines usage', async () => {
    const controller = new AbortController();
    const malformed = '{"summary":"sensitive source fact","design":{"testCases":[';
    const calls: Array<Parameters<Parameters<typeof generateAndValidateTestCaseBatch>[0]['generate']>[0]> = [];
    const responses = [
      { text: malformed, finishReason: 'stop', usage: { inputTokens: 10, outputTokens: 2 } },
      { text: completeTestCaseModelText(), finishReason: 'stop', usage: { inputTokens: 20, outputTokens: 4, totalTokens: 24 } },
    ];

    const resolved = await generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: controller.signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return responses[calls.length - 1];
      },
    });

    expect(calls).toHaveLength(2);
    expect(calls[1]).toMatchObject({
      instructions: 'fixed skill instructions',
      abortSignal: controller.signal,
      messages: [
        { role: 'user', content: 'batch prompt' },
        { role: 'assistant', content: malformed },
        { role: 'user', content: expect.stringContaining('完整重写') },
      ],
    });
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'parse',
      usage: { inputTokens: 30, outputTokens: 6, totalTokens: 36 },
    });
    expect(resolved.output.testCases).toHaveLength(1);
  });

  it('classifies JSON-valid schema failures and sends only safe paths and allowed enums', async () => {
    const invalidValue = JSON.parse(completeTestCaseModelText()) as {
      design: {
        coverageStrategy: { methods: string[] };
        traceability: Array<{ coverageTypes: string[] }>;
      };
    };
    invalidValue.design.coverageStrategy.methods = ['received-secret-method'];
    invalidValue.design.traceability[0].coverageTypes = ['received-secret-category'];
    const invalidText = JSON.stringify(invalidValue);
    const calls: Array<Parameters<Parameters<typeof generateAndValidateTestCaseBatch>[0]['generate']>[0]> = [];

    const resolved = await generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls.push(request);
        return calls.length === 1
          ? { text: invalidText, finishReason: 'stop', usage: null }
          : { text: completeTestCaseModelText(), finishReason: 'stop', usage: null };
      },
    });

    expect(calls).toHaveLength(2);
    expect(resolved.repairReason).toBe('structure');
    expect(calls[1].messages?.[1]).toEqual({ role: 'assistant', content: invalidText });
    const repairInstruction = calls[1].messages?.[2].content ?? '';
    expect(repairInstruction).toContain('design.coverageStrategy.methods[0] must use one of');
    expect(repairInstruction).toContain('equivalence_partitioning, boundary_value, decision_table');
    expect(repairInstruction).toContain('design.traceability[0].coverageTypes[0] must use one of');
    expect(repairInstruction).toContain('positive, negative, boundary, permission');
    expect(repairInstruction).not.toContain('received-secret-method');
    expect(repairInstruction).not.toContain('received-secret-category');
  });

  it('feeds an Output.json parse failure into the same single repair attempt', async () => {
    let calls = 0;
    const firstText = '{"summary":"provider JSON parse failure"';
    const resolved = await generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: () => recoverTestCaseJSONModelResult(async () => {
        calls += 1;
        if (calls === 1) throw testNoObjectGeneratedError(firstText, { inputTokens: 7, outputTokens: 3 });
        return {
          text: completeTestCaseModelText(),
          finishReason: 'stop',
          usage: { inputTokens: 11, outputTokens: 5, totalTokens: 16 },
        };
      }),
    });

    expect(calls).toBe(2);
    expect(resolved).toMatchObject({
      repaired: true,
      repairReason: 'parse',
      usage: { inputTokens: 18, outputTokens: 8, totalTokens: 26 },
    });
  });

  it('extracts only safe generation fields from NoObjectGeneratedError and rethrows other errors', async () => {
    const failure = testNoObjectGeneratedError('{"summary":', { inputTokens: 4, outputTokens: 2 });
    await expect(recoverTestCaseJSONModelResult(async () => { throw failure; })).resolves.toMatchObject({
      text: '{"summary":',
      finishReason: 'stop',
      usage: { inputTokens: 4, outputTokens: 2, totalTokens: 6 },
    });

    const compatibleFailure = testNoObjectGeneratedError('{"summary":', { inputTokens: 4, outputTokens: 2 });
    Object.defineProperty(compatibleFailure, 'finishReason', { value: undefined });
    await expect(recoverTestCaseJSONModelResult(async () => { throw compatibleFailure; }))
      .resolves.toMatchObject({ finishReason: 'stop' });

    const providerFailure = new Error('provider unavailable');
    await expect(recoverTestCaseJSONModelResult(async () => { throw providerFailure; }))
      .rejects.toBe(providerFailure);
  });

  it.each([
    ['invalid JSON', '{"summary":', 'stop', /complete valid JSON artifact/],
    ['truncated finish reason', completeTestCaseModelText(), 'length', /repair ended with length/],
  ])('fails closed when parse repair returns %s', async (_name, repairText, finishReason, expected) => {
    let calls = 0;
    const secret = 'source fact must not enter the error';
    const run = generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        calls += 1;
        return calls === 1
          ? { text: `{"summary":"${secret}`, finishReason: 'stop', usage: null }
          : { text: repairText, finishReason, usage: null };
      },
    });

    await expect(run).rejects.toThrow(expected);
    await expect(run).rejects.not.toThrow(secret);
    expect(calls).toBe(2);
  });

  it('does not start parse repair without the reserved time budget', async () => {
    let calls = 0;
    await expect(generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 89_999,
      generate: async () => {
        calls += 1;
        return { text: '{"summary":', finishReason: 'stop', usage: null };
      },
    })).rejects.toThrow('test case model response was not a complete valid JSON artifact');
    expect(calls).toBe(1);
  });

  it('does not repair an initial response with a non-stop finish reason', async () => {
    let calls = 0;
    await expect(generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        calls += 1;
        return { text: '{"summary":', finishReason: 'length', usage: null };
      },
    })).rejects.toThrow('test case generation ended with length');
    expect(calls).toBe(1);
  });

  it('shares one repair attempt when rewritten JSON still fails validation', async () => {
    let calls = 0;
    const invalid = completeTestCaseOutput('subject.title');
    invalid.testCases[0].expectedResults = ['符合预期'];
    const invalidText = JSON.stringify({
      summary: invalid.summary,
      design: {
        ...invalid.testCaseDesign,
        testCases: invalid.testCases.map(({ expectedResult: _expectedResult, ...testCase }) => testCase),
      },
    });

    await expect(generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: new AbortController().signal,
      remainingMs: () => 120_000,
      generate: async () => {
        calls += 1;
        return calls === 1
          ? { text: '{"summary":', finishReason: 'stop', usage: null }
          : { text: invalidText, finishReason: 'stop', usage: null };
      },
    })).rejects.toThrow('test case expected result is not observable');
    expect(calls).toBe(2);
  });

  it('uses the same abort signal for parse repair and propagates cancellation', async () => {
    const controller = new AbortController();
    let calls = 0;
    await expect(generateAndValidateTestCaseBatch({
      input: { subject: { title: '结算页' } },
      instructions: 'fixed skill instructions',
      prompt: 'batch prompt',
      abortSignal: controller.signal,
      remainingMs: () => 120_000,
      generate: async (request) => {
        calls += 1;
        expect(request.abortSignal).toBe(controller.signal);
        if (calls === 1) {
          controller.abort();
          return { text: '{"summary":', finishReason: 'stop', usage: null };
        }
        const error = new Error('cancelled');
        error.name = 'AbortError';
        throw error;
      },
    })).rejects.toMatchObject({ name: 'AbortError' });
    expect(calls).toBe(2);
  });
});
