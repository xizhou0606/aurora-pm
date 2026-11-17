import { createHmac, timingSafeEqual } from 'node:crypto';
import Fastify, { type FastifyInstance } from 'fastify';
import type { RuntimeConfig } from './config.js';
import {
  QA_TEST_CASE_SUCCESS_TELEMETRY,
  type Generator,
} from './shared/ai/types.js';
import { safeGenerationDiagnostic, safeGenerationFailureMessage } from './shared/errors/index.js';
import { codeQualityReviewSkillMetadata } from './capabilities/code-quality-review/index.js';
import { qaTestCaseSkillMetadata } from './capabilities/qa-test-case/index.js';
import { changelogDraftSkillMetadata } from './capabilities/changelog-draft/index.js';
import { generateRequestSchema } from './shared/ai/schema.js';
import {
  createDesignReviewGenerator,
  createNormalizationGenerator,
  designReviewRequestSchema,
  NORMALIZATION_PROMPT_VERSION,
  normalizationRequestSchema,
} from './capabilities/design-review/index.js';

declare module 'fastify' { interface FastifyRequest { rawBody?: string } }

export const RUNTIME_BODY_LIMIT_BYTES = 64 * 1024 * 1024;

function validSignature(rawBody: string, timestamp: string, signature: string, secret: string): boolean {
  const unix = Number(timestamp);
  if (!Number.isFinite(unix) || Math.abs(Date.now() - unix) > 5 * 60_000) return false;
  const expected = createHmac('sha256', secret).update(`${timestamp}.${rawBody}`).digest('hex');
  const left = Buffer.from(expected);
  const right = Buffer.from(signature);
  return left.length === right.length && timingSafeEqual(left, right);
}

export function buildApp(
  config: RuntimeConfig,
  generate: Generator,
  logStream?: { write(message: string): void },
): FastifyInstance {
  const qaSkill = qaTestCaseSkillMetadata();
  const generateDesignReview = createDesignReviewGenerator(config);
  const generateNormalization = createNormalizationGenerator(config);
  const codeQualitySkill = codeQualityReviewSkillMetadata();
  const changelogSkill = changelogDraftSkillMetadata();
  const app = Fastify({
    bodyLimit: RUNTIME_BODY_LIMIT_BYTES,
    logger: {
      redact: ['req.headers.authorization', 'req.headers.x-pm-signature'],
      ...(logStream ? { stream: logStream } : {}),
    },
  });
  app.removeContentTypeParser('application/json');
  app.addContentTypeParser('application/json', { parseAs: 'string' }, (request, body, done) => {
    const rawBody = typeof body === 'string' ? body : body.toString('utf8');
    request.rawBody = rawBody;
    try { done(null, JSON.parse(rawBody)); } catch (error) { done(error as Error); }
  });

  app.get('/health/live', async () => ({ status: 'ok', service: 'pm-ai-runtime' }));
  app.get('/health/ready', async () => ({
    status: 'ok',
    service: 'pm-ai-runtime',
    model: config.AI_MODEL,
    designReviewModel: config.designReviewModel,
    skills: [
      { name: qaSkill.name, commit: qaSkill.commit, packageSha256: qaSkill.packageSha256 },
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
    ],
  }));
  app.post('/internal/v1/generate', async (request, reply) => {
    const timestamp = String(request.headers['x-pm-timestamp'] ?? '');
    const signature = String(request.headers['x-pm-signature'] ?? '');
    if (!validSignature(request.rawBody ?? '', timestamp, signature, config.AI_INTERNAL_SECRET)) {
      return reply.code(401).send({ error: 'invalid internal signature' });
    }
    const parsed = generateRequestSchema.safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: 'invalid request', details: parsed.error.flatten() });
    try {
      const result = await generate(parsed.data);
      const successTelemetry = result[QA_TEST_CASE_SUCCESS_TELEMETRY];
      if (successTelemetry) {
        request.log.info({
          runId: parsed.data.runId,
          qaTestCaseGeneration: successTelemetry,
        }, 'QA test case generation completed');
      }
      return {
        data: {
          runId: parsed.data.runId,
          model: config.AI_MODEL,
          output: result.output,
          usage: result.usage,
        },
      };
    } catch (error) {
      const message = safeGenerationFailureMessage(error);
      request.log.error({
        generationError: safeGenerationDiagnostic(error),
      }, 'model generation failed');
      return reply.code(502).send({ error: 'model generation failed', message });
    }
  });
  app.post('/internal/v1/design-review/analyze', async (request, reply) => {
    const timestamp = String(request.headers['x-pm-timestamp'] ?? '');
    const signature = String(request.headers['x-pm-signature'] ?? '');
    if (!validSignature(request.rawBody ?? '', timestamp, signature, config.AI_INTERNAL_SECRET)) {
      return reply.code(401).send({ error: 'invalid internal signature' });
    }
    const parsed = designReviewRequestSchema.safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: 'invalid design review request', details: parsed.error.flatten() });
    if (!config.designReviewModel) return reply.code(503).send({ error: 'design review model is unavailable' });
    try {
      const result = await generateDesignReview(parsed.data);
      return {
        data: {
          reviewId: parsed.data.manifest.reviewId,
          model: result.model,
          reasoningEffort: parsed.data.manifest.reasoningEffort,
          promptVersion: result.promptVersion,
          responseId: result.responseId,
          durationMs: result.durationMs,
          output: result.output,
          usage: result.usage,
        },
      };
    } catch (error) {
      request.log.error({ generationError: safeGenerationDiagnostic(error) }, 'design review generation failed');
      return reply.code(502).send({ error: 'design review generation failed', message: safeGenerationFailureMessage(error) });
    }
  });
  app.post('/internal/v1/design-review/dom-normalize', async (request, reply) => {
    const timestamp = String(request.headers['x-pm-timestamp'] ?? '');
    const signature = String(request.headers['x-pm-signature'] ?? '');
    if (!validSignature(request.rawBody ?? '', timestamp, signature, config.AI_INTERNAL_SECRET)) {
      return reply.code(401).send({ error: 'invalid internal signature' });
    }
    const parsed = normalizationRequestSchema.safeParse(request.body);
    if (!parsed.success) return reply.code(400).send({ error: 'invalid normalization request', details: parsed.error.flatten() });
    if (!config.designReviewModel) return reply.code(503).send({ error: 'design review model is unavailable' });
    try {
      const result = await generateNormalization(parsed.data);
      return { data: { inventory_token: parsed.data.inventory.token, model: config.designReviewModel, prompt_version: NORMALIZATION_PROMPT_VERSION, output: result.output, usage: result.usage } };
    } catch (error) {
      request.log.error({ generationError: safeGenerationDiagnostic(error) }, 'design review normalization failed');
      return reply.code(502).send({ error: 'design review normalization failed', message: safeGenerationFailureMessage(error) });
    }
  });
  return app;
}

export const hmacForTest = (body: string, timestamp: string, secret: string) =>
  createHmac('sha256', secret).update(`${timestamp}.${body}`).digest('hex');
