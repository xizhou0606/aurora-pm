import { z } from 'zod';

const schema = z.object({
  AI_API_KEY: z.string().min(1),
  AI_BASE_URL: z.url().default('https://dashscope.aliyuncs.com/compatible-mode/v1'),
  AI_MODEL: z.string().min(1).default('qwen3.6-max-preview'),
  AI_TEST_CASE_MODEL: z.string().min(1).optional(),
  AI_DESIGN_REVIEW_MODEL: z.string().min(1).optional(),
  AI_DESIGN_REVIEW_REASONING_EFFORT: z.enum(['low', 'medium', 'high']).default('medium'),
  AI_DESIGN_REVIEW_IMAGE_DETAIL: z.enum(['low', 'high', 'auto']).default('high'),
  AI_REPORT_MODEL: z.string().min(1).optional(),
  AI_REPORT_MAX_OUTPUT_TOKENS: z.coerce.number().int().min(500).max(16000).default(4000),
  AI_INTERNAL_SECRET: z.string().min(32),
  AI_MAX_OUTPUT_TOKENS: z.coerce.number().int().positive().max(32000).default(16000),
  AI_TEST_CASE_MAX_OUTPUT_TOKENS: z.coerce.number().int().min(4000).max(16000).default(8000),
  AI_REQUEST_TIMEOUT: z.string().default('360s'),
  AI_HTTP_ADDR: z.string().default('0.0.0.0'),
  AI_HTTP_PORT: z.coerce.number().int().positive().default(8091),
});

export type RuntimeConfig = z.infer<typeof schema> & {
  requestTimeoutMs: number;
  testCaseModel: string;
  designReviewModel: string | null;
  reportModel: string;
  reportMaxOutputTokens: number;
};

export function loadConfig(env: NodeJS.ProcessEnv = process.env): RuntimeConfig {
  const parsed = schema.parse(env);
  const match = /^(\d+)(ms|s|m)$/.exec(parsed.AI_REQUEST_TIMEOUT);
  if (!match) throw new Error('AI_REQUEST_TIMEOUT must use ms, s, or m');
  const multiplier = match[2] === 'ms' ? 1 : match[2] === 's' ? 1000 : 60000;
  const requestTimeoutMs = Number(match[1]) * multiplier;
  if (!Number.isSafeInteger(requestTimeoutMs) || requestTimeoutMs <= 0 || requestTimeoutMs > 10 * 60_000) {
    throw new Error('AI_REQUEST_TIMEOUT must be greater than 0 and no more than 10m');
  }
  return {
    ...parsed,
    requestTimeoutMs,
    testCaseModel: parsed.AI_TEST_CASE_MODEL?.trim() || 'qwen3.7-flash',
    designReviewModel: parsed.AI_DESIGN_REVIEW_MODEL?.trim() || null,
    reportModel: parsed.AI_REPORT_MODEL?.trim() || 'qwen3.7-flash',
    reportMaxOutputTokens: parsed.AI_REPORT_MAX_OUTPUT_TOKENS,
  };
}
