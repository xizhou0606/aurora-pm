// Workflow JSON is compact. A smaller budget prevents the model from spending
// most of the request timeout on unnecessary reasoning or verbose output.
export const WORKFLOW_MAX_OUTPUT_TOKENS = 3000;

export function workflowGenerationOptions(maxOutputTokens: number) {
  return {
    maxOutputTokens: Math.min(maxOutputTokens, WORKFLOW_MAX_OUTPUT_TOKENS),
    providerOptions: {
      dashscope: { enable_thinking: false },
    },
  };
}

import { SpanStatusCode } from '@opentelemetry/api';
import type { CapabilityHandlerContext, GeneratorResult } from '../../shared/ai/types.js';
import { generateText } from 'ai';
import { buildWorkflowPrompt } from './prompts/prompt.js';
import { validateAndRepairWorkflowOutput } from './repair.js';

// workflow.generate 能力执行器(原 generator.ts 内 workflow.generate 分支,逐字迁移):
// 一次生成 + 一次结构校验修复循环,由 validateAndRepairWorkflowOutput 驱动。
export async function runWorkflowGeneration(ctx: CapabilityHandlerContext): Promise<GeneratorResult> {
  const result = await generateText({
    model: ctx.model('main'),
    prompt: buildWorkflowPrompt(ctx.input),
    ...workflowGenerationOptions(ctx.config.AI_MAX_OUTPUT_TOKENS),
    abortSignal: ctx.signal,
  });
  const subject = ctx.input.subject as Record<string, unknown> | undefined;
  const subjectType = typeof subject?.subject_type === 'string' ? subject.subject_type : 'requirement';
  const resolved = await validateAndRepairWorkflowOutput(result, subjectType, async (issues, firstOutput) => {
    return generateText({
      model: ctx.model('main'),
      prompt: [
        buildWorkflowPrompt(ctx.input),
        `上一版草案未通过结构校验：${issues.join('；')}。`,
        '请修复上述全部问题后重新输出完整 JSON。并行必须使用 parallel_split/parallel_join，禁止删除用户要求的并行语义。',
        `上一版输出：${firstOutput}`,
      ].join('\n'),
      ...workflowGenerationOptions(ctx.config.AI_MAX_OUTPUT_TOKENS),
      abortSignal: ctx.signal,
    });
  }, ctx.input.workflow_definition);
  ctx.span.setStatus({ code: SpanStatusCode.OK });
  return resolved;
}
