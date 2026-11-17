import { describe, expect, it } from 'bun:test';
import { buildApp, hmacForTest, RUNTIME_BODY_LIMIT_BYTES } from './app.js';
import type { RuntimeConfig } from './config.js';
import {
  QA_TEST_CASE_SUCCESS_TELEMETRY,
  type Generator,
  type QATestCaseSuccessTelemetry,
} from './shared/ai/types.js';
import { qaTestCaseSkillMetadata } from './capabilities/qa-test-case/index.js';
import { changelogDraftSkillMetadata } from './capabilities/changelog-draft/index.js';
import { codeQualityReviewSkillMetadata } from './capabilities/code-quality-review/index.js';

const config: RuntimeConfig = {
  AI_API_KEY: 'test-only', AI_BASE_URL: 'https://example.test/v1', AI_MODEL: 'qwen3.6-max-preview',
  AI_INTERNAL_SECRET: 'test-internal-secret-at-least-32-characters', AI_MAX_OUTPUT_TOKENS: 16000, AI_REPORT_MAX_OUTPUT_TOKENS: 4000,
  AI_TEST_CASE_MAX_OUTPUT_TOKENS: 8000,
  AI_REQUEST_TIMEOUT: '120s', AI_HTTP_ADDR: '127.0.0.1', AI_HTTP_PORT: 8091, requestTimeoutMs: 120000,
  testCaseModel: 'qwen3.6-max-preview',
  AI_DESIGN_REVIEW_REASONING_EFFORT: 'medium', AI_DESIGN_REVIEW_IMAGE_DETAIL: 'high',
  designReviewModel: null,
  reportModel: 'qwen3.7-flash', reportMaxOutputTokens: 4000,
};

const generate: Generator = async () => ({
  output: { summary: 'ok', changes: [], items: [], risks: [], testCases: [], humanReviewRequired: true, completenessReport: null, workflowProposal: null },
  usage: { inputTokens: 10, outputTokens: 4 },
});

describe('internal generation API', () => {
  it('reports the build-synced QA skill version in readiness metadata', async () => {
    const app = buildApp(config, generate);
    const response = await app.inject({ method: 'GET', url: '/health/ready' });
    const skill = qaTestCaseSkillMetadata();
    const codeQualitySkill = codeQualityReviewSkillMetadata();
    const changelogSkill = changelogDraftSkillMetadata();
    expect(response.statusCode).toBe(200);
    expect(response.json().skills).toEqual([
      {
        name: skill.name,
        commit: skill.commit,
        packageSha256: skill.packageSha256,
      },
      {
        name: codeQualitySkill.name,
        capability: codeQualitySkill.capability,
        version: codeQualitySkill.version,
        packageSha256: codeQualitySkill.packageSha256,
        sources: codeQualitySkill.sources,
      },
      {
        name: changelogSkill.name,
        capability: changelogSkill.capability,
        version: changelogSkill.version,
        packageSha256: changelogSkill.packageSha256,
        sources: changelogSkill.sources,
      },
    ]);
    await app.close();
  });

  it('rejects unsigned requests', async () => {
    const app = buildApp(config, generate);
    const response = await app.inject({ method: 'POST', url: '/internal/v1/generate', payload: { runId: 1, workspaceId: 1, capability: 'requirement.draft', input: {} } });
    expect(response.statusCode).toBe(401);
    await app.close();
  });

  it('verifies HMAC and returns structured output', async () => {
    const app = buildApp(config, generate);
    const body = JSON.stringify({ runId: 1, workspaceId: 2, capability: 'requirement.draft', input: { title: 'A' } });
    const timestamp = String(Date.now());
    const response = await app.inject({
      method: 'POST', url: '/internal/v1/generate', payload: body,
      headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(body, timestamp, config.AI_INTERNAL_SECRET) },
    });
    expect(response.statusCode).toBe(200);
    expect(response.json().data.output.humanReviewRequired).toBe(true);
    await app.close();
  });

  it('logs allowlisted QA success telemetry without changing the API response', async () => {
    const logLines: string[] = [];
    const telemetry: QATestCaseSuccessTelemetry = {
      batchCount: 1,
      requirementGroupCount: 1,
      globalPlan: false,
      totalConcurrency: 4,
      modelCallCount: 2,
      modelElapsedMs: 45,
      wallElapsedMs: 46,
      inputTokens: 120,
      outputTokens: 35,
      totalTokens: 155,
      stages: [{
        stage: 'draft_batch',
        batchNumber: 1,
        calls: [
          { callNumber: 1, kind: 'draft_initial', elapsedMs: 30, inputTokens: 70, outputTokens: 20, totalTokens: 90, finishReason: 'stop' },
          { callNumber: 2, kind: 'draft_full_repair', elapsedMs: 15, inputTokens: 50, outputTokens: 15, totalTokens: 65, finishReason: 'stop' },
        ],
        modelCallCount: 2,
        modelElapsedMs: 45,
        wallElapsedMs: 46,
        inputTokens: 120,
        outputTokens: 35,
        totalTokens: 155,
        repaired: true,
        repairReason: 'validation',
        finishReasonFallback: false,
        coveragePatched: false,
        globalRefPatched: false,
        repairRuleCodes: ['context_reference'],
        repairDiagnosticCategories: [{ category: 'context_reference', count: 1 }],
      }],
    };
    const app = buildApp(config, async (request) => {
      const result = await generate(request);
      result[QA_TEST_CASE_SUCCESS_TELEMETRY] = telemetry;
      return result;
    }, { write: (message) => { logLines.push(message); } });
    const body = JSON.stringify({
      runId: 110,
      workspaceId: 2,
      capability: 'qa.test_cases',
      input: { secretBusinessText: '绝不得写入日志' },
    });
    const timestamp = String(Date.now());
    const response = await app.inject({
      method: 'POST', url: '/internal/v1/generate', payload: body,
      headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(body, timestamp, config.AI_INTERNAL_SECRET) },
    });

    const successLog = logLines.map((line) => JSON.parse(line)).find((line) => (
      line.msg === 'QA test case generation completed'
    ));
    expect(successLog).toMatchObject({
      runId: 110,
      qaTestCaseGeneration: telemetry,
    });
    expect(logLines.join('\n')).not.toContain('绝不得写入日志');
    expect(response.json().data).not.toHaveProperty('qaTestCaseGeneration');
    expect(response.json().data).toEqual({
      runId: 110,
      model: config.AI_MODEL,
      output: expect.any(Object),
      usage: { inputTokens: 10, outputTokens: 4 },
    });
    await app.close();
  });

  it('accepts a valid signed generation request larger than Fastify default limit', async () => {
    let receivedBytes = 0;
    const app = buildApp(config, async (request) => {
      receivedBytes = Buffer.byteLength(String(request.input.largeFact), 'utf8');
      return generate(request);
    });
    const largeFact = 'x'.repeat(1024 * 1024 + 64 * 1024);
    const body = JSON.stringify({
      runId: 1,
      workspaceId: 2,
      capability: 'qa.test_cases',
      input: { largeFact },
    });
    const timestamp = String(Date.now());
    const response = await app.inject({
      method: 'POST', url: '/internal/v1/generate', payload: body,
      headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(body, timestamp, config.AI_INTERNAL_SECRET) },
    });

    expect(RUNTIME_BODY_LIMIT_BYTES).toBeGreaterThanOrEqual(8 * 1024 * 1024);
    expect(response.statusCode).toBe(200);
    expect(receivedBytes).toBe(largeFact.length);
    await app.close();
  });

  it('rejects stale signed requests', async () => {
    const app = buildApp(config, generate);
    const body = JSON.stringify({ runId: 1, workspaceId: 2, capability: 'requirement.draft', input: {} });
    const timestamp = String(Date.now() - 600_000);
    const response = await app.inject({ method: 'POST', url: '/internal/v1/generate', payload: body, headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(body, timestamp, config.AI_INTERNAL_SECRET) } });
    expect(response.statusCode).toBe(401);
    await app.close();
  });

  it('rejects a body changed after it was signed', async () => {
    const app = buildApp(config, generate);
    const signedBody = JSON.stringify({ runId: 1, workspaceId: 2, capability: 'requirement.draft', input: { title: 'A' } });
    const tamperedBody = JSON.stringify({ runId: 1, workspaceId: 2, capability: 'requirement.draft', input: { title: 'B' } });
    const timestamp = String(Date.now());
    const response = await app.inject({
      method: 'POST', url: '/internal/v1/generate', payload: tamperedBody,
      headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(signedBody, timestamp, config.AI_INTERNAL_SECRET) },
    });
    expect(response.statusCode).toBe(401);
    await app.close();
  });

  it('rejects signed requests with a timestamp too far in the future', async () => {
    const app = buildApp(config, generate);
    const body = JSON.stringify({ runId: 1, workspaceId: 2, capability: 'requirement.draft', input: {} });
    const timestamp = String(Date.now() + 600_000);
    const response = await app.inject({ method: 'POST', url: '/internal/v1/generate', payload: body, headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(body, timestamp, config.AI_INTERNAL_SECRET) } });
    expect(response.statusCode).toBe(401);
    await app.close();
  });

  it('returns a safe failure classification without persisting model text', async () => {
    const app = buildApp(config, async () => {
      throw new Error('test case expected result is not observable: 机密需求正文');
    });
    const body = JSON.stringify({ runId: 1, workspaceId: 2, capability: 'qa.test_cases', input: { subject: { title: 'A' } } });
    const timestamp = String(Date.now());
    const response = await app.inject({
      method: 'POST', url: '/internal/v1/generate', payload: body,
      headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(body, timestamp, config.AI_INTERNAL_SECRET) },
    });
    expect(response.statusCode).toBe(502);
    expect(response.json()).toMatchObject({
      error: 'model generation failed',
      message: 'test case output did not satisfy the fixed QA rules',
    });
    expect(response.body).not.toContain('机密需求正文');
    await app.close();
  });

  it('returns an actionable safe classification for batching capacity', async () => {
    const app = buildApp(config, async () => {
      throw new Error('test case context requires 21 batches; maximum supported is 20');
    });
    const body = JSON.stringify({ runId: 1, workspaceId: 2, capability: 'qa.test_cases', input: {} });
    const timestamp = String(Date.now());
    const response = await app.inject({
      method: 'POST', url: '/internal/v1/generate', payload: body,
      headers: { 'content-type': 'application/json', 'x-pm-timestamp': timestamp, 'x-pm-signature': hmacForTest(body, timestamp, config.AI_INTERNAL_SECRET) },
    });

    expect(response.statusCode).toBe(502);
    expect(response.json().message).toBe(
      'test case input exceeds supported batching limits; split the plan unit into smaller runs',
    );
    await app.close();
  });
});
