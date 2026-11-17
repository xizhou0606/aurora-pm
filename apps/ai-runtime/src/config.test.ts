import { describe, expect, it } from 'bun:test';
import { loadConfig } from './config.js';

const baseEnv = {
  AI_API_KEY: 'test-only',
  AI_INTERNAL_SECRET: 'test-internal-secret-at-least-32-characters',
};

describe('AI runtime config', () => {
  it('parses a bounded positive request timeout', () => {
    expect(loadConfig(baseEnv).requestTimeoutMs).toBe(360_000);
    expect(loadConfig({ ...baseEnv, AI_REQUEST_TIMEOUT: '120s' }).requestTimeoutMs).toBe(120_000);
    expect(loadConfig({ ...baseEnv, AI_REQUEST_TIMEOUT: '1ms' }).requestTimeoutMs).toBe(1);
    expect(loadConfig({ ...baseEnv, AI_REQUEST_TIMEOUT: '10m' }).requestTimeoutMs).toBe(600_000);
  });

  it('uses an independent output budget for detailed test-case artifacts', () => {
    expect(loadConfig(baseEnv).AI_TEST_CASE_MAX_OUTPUT_TOKENS).toBe(8000);
    expect(loadConfig({ ...baseEnv, AI_TEST_CASE_MAX_OUTPUT_TOKENS: '12000' }).AI_TEST_CASE_MAX_OUTPUT_TOKENS).toBe(12000);
    expect(() => loadConfig({ ...baseEnv, AI_TEST_CASE_MAX_OUTPUT_TOKENS: '3000' })).toThrow();
    expect(() => loadConfig({ ...baseEnv, AI_TEST_CASE_MAX_OUTPUT_TOKENS: '16001' })).toThrow();
  });

  it('defaults the test-case model to qwen3.7-flash and allows an override', () => {
    expect(loadConfig(baseEnv).testCaseModel).toBe('qwen3.7-flash');
    expect(loadConfig({ ...baseEnv, AI_MODEL: 'qwen3.7-plus' }).testCaseModel).toBe('qwen3.7-flash');
    expect(loadConfig({
      ...baseEnv,
      AI_MODEL: 'qwen3.6-max-preview',
      AI_TEST_CASE_MODEL: 'qwen3.7-plus',
    }).testCaseModel).toBe('qwen3.7-plus');
  });

  it.each(['0ms', '600001ms', '11m'])('rejects unsafe request timeout %s', (AI_REQUEST_TIMEOUT) => {
    expect(() => loadConfig({ ...baseEnv, AI_REQUEST_TIMEOUT })).toThrow(
      'AI_REQUEST_TIMEOUT must be greater than 0 and no more than 10m',
    );
  });
});
