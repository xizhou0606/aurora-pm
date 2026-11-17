import { createOpenAICompatible } from '@ai-sdk/openai-compatible';
import { context, SpanStatusCode, trace } from '@opentelemetry/api';
import { generateText, Output } from 'ai';
import type { RuntimeConfig } from '../config.js';
import { safeGenerationFailureMessage } from '../shared/errors/index.js';
import type {
  CapabilityHandlerContext,
  GenerateRequest,
  Generator,
  GeneratorResult,
} from '../shared/ai/types.js';
import type { AIOutput } from '../shared/ai/schema.js';
import { resolveCapability, type CapabilityDescriptor } from './registry.js';

// 唯一通用执行器:替代原 createGenerator 内的能力 if-链。
// - span/超时/中止由本文件统一负责;
// - 标准(单轮结构化输出)能力按描述符执行 generateText + 归一 + 校验,
//   并复刻原分支的 NoObject 兜底与一次重试 + 错误文本恢复语义;
// - 自定义编排能力(workflow.generate/report.*/qa.test_cases)通过
//   handler 注入模型工厂与运行时上下文。

export function validateCapabilityOutput(capability: string, input: Record<string, unknown>, output: AIOutput): void {
  resolveCapability(capability).validate?.(input, output);
}

export function requestTimeoutForCapability(capability: string, configuredTimeoutMs: number): number {
  const cap = resolveCapability(capability).timeoutCapMs;
  return cap === undefined ? configuredTimeoutMs : Math.min(configuredTimeoutMs, cap);
}

type StandardCallResult = {
  output: unknown;
  usage: unknown;
};

async function executeStandardDescriptor(
  descriptor: CapabilityDescriptor,
  request: GenerateRequest,
  ctx: CapabilityHandlerContext,
): Promise<GeneratorResult> {
  const standardContext = { capability: request.capability, input: request.input };
  const reviewRequest = (): Promise<StandardCallResult> => {
    const schema = descriptor.outputSchema;
    if (!schema) throw new Error(`capability ${request.capability} requires an output schema`);
    return generateText({
      model: ctx.model(descriptor.model),
      output: Output.object({ schema }),
      prompt: descriptor.prompt(request.input),
      maxOutputTokens: descriptor.maxOutputTokens
        ? descriptor.maxOutputTokens(ctx.config)
        : ctx.config.AI_MAX_OUTPUT_TOKENS,
      ...(descriptor.providerOptions ? { providerOptions: descriptor.providerOptions } : {}),
      abortSignal: ctx.signal,
    }) as Promise<StandardCallResult>;
  };
  if (descriptor.retryOnce) {
    // 提供方偶发 5xx/截断:重试一次;仍失败时从错误携带的文本里兜底解析
    // JSON。全部失败才抛出——调用方会降级续跑。
    let draftResult: StandardCallResult;
    try {
      draftResult = await reviewRequest();
    } catch (firstError) {
      try {
        draftResult = await reviewRequest();
      } catch (secondError) {
        const recovered = descriptor.retryOnce.recover(secondError) ?? descriptor.retryOnce.recover(firstError);
        if (!recovered) throw secondError;
        ctx.span.setAttribute(descriptor.retryOnce.spanAttribute, true);
        ctx.span.setStatus({ code: SpanStatusCode.OK });
        return { output: recovered, usage: null };
      }
    }
    const output = descriptor.normalize
      ? descriptor.normalize(draftResult.output, standardContext)
      : draftResult.output as AIOutput;
    ctx.span.setStatus({ code: SpanStatusCode.OK });
    return { output, usage: draftResult.usage };
  }
  return (async () => {
    try {
      const result = await reviewRequest();
      const output = descriptor.normalize
        ? descriptor.normalize(result.output, standardContext)
        : result.output as AIOutput;
      descriptor.validate?.(request.input, output);
      ctx.span.setStatus({ code: SpanStatusCode.OK });
      return { output, usage: result.usage };
    } catch (error) {
      const fallback = descriptor.noObjectFallback;
      if (!fallback || (error as { name?: string }).name !== 'AI_NoObjectGeneratedError') throw error;
      const output = fallback.recover(error, standardContext);
      descriptor.validate?.(request.input, output);
      ctx.span.setAttribute(fallback.spanAttribute, true);
      ctx.span.setStatus({ code: SpanStatusCode.OK });
      return { output, usage: null };
    }
  })();
}

export function createGenerator(config: RuntimeConfig): Generator {
  const provider = createOpenAICompatible({
    name: 'dashscope',
    apiKey: config.AI_API_KEY,
    baseURL: config.AI_BASE_URL,
    includeUsage: true,
    supportsStructuredOutputs: true,
  });
  const tracer = trace.getTracer('pm-ai-runtime');
  return async (request) => tracer.startActiveSpan('pm.ai.generate', async (span) => {
    span.setAttributes({ 'pm.run_id': request.runId, 'pm.workspace_id': request.workspaceId, 'pm.capability': request.capability, 'ai.model': config.AI_MODEL });
    const descriptor = resolveCapability(request.capability);
    const controller = new AbortController();
    const requestTimeoutMs = requestTimeoutForCapability(request.capability, config.requestTimeoutMs);
    const timer = setTimeout(() => {
      if (descriptor.timeoutAbortReason) {
        controller.abort(descriptor.timeoutAbortReason());
        return;
      }
      controller.abort();
    }, requestTimeoutMs);
    const ctx: CapabilityHandlerContext = {
      capability: request.capability,
      input: request.input,
      config,
      signal: controller.signal,
      span,
      requestTimeoutMs,
      model: (kind) => provider(
        kind === 'main' ? config.AI_MODEL : kind === 'report' ? config.reportModel : config.testCaseModel,
      ),
      abort: (reason?: unknown) => controller.abort(reason),
    };
    try {
      return descriptor.handler
        ? await descriptor.handler(ctx)
        : await executeStandardDescriptor(descriptor, request, ctx);
    } catch (error) {
      span.recordException({
        name: error instanceof Error ? error.name : 'UnknownError',
        message: safeGenerationFailureMessage(error),
      });
      span.setStatus({ code: SpanStatusCode.ERROR });
      throw error;
    } finally {
      clearTimeout(timer);
      span.end();
      context.active();
    }
  });
}
