import { SpanStatusCode } from '@opentelemetry/api';
import type { CapabilityHandlerContext, GeneratorResult } from '../../shared/ai/types.js';
import { generateText, Output } from 'ai';
import { z } from 'zod';
import { buildReportPrompt } from './prompt.js';
import { parseReportOutput } from './report.js';

// 结构化输出契约：schema 约束解码，从根上杜绝"模型输出非法 JSON（字符串内
// 未转义引号）导致整段 JSON 掉进 summary"的解析失败——评语与评分随之丢失。
// 维度短评个人汇报为空串、团队汇报非空，与 Go 侧指令一致。
const reportOutputSchema = z.object({
  summary: z.string().min(1),
  overall_comment: z.string().default(''),
  overall_score: z.number().int().min(0).max(100).nullable().default(null),
  dimension_comments: z.object({
    tasks: z.string().default(''),
    quality: z.string().default(''),
    code: z.string().default(''),
  }).default({ tasks: '', quality: '', code: '' }),
});

// report.* 能力执行器:轻量正文生成,走快速模型(reportModel)与更短超时。
export async function runReportGeneration(ctx: CapabilityHandlerContext): Promise<GeneratorResult> {
  // 日报/周报/月报：轻量正文生成，走快速模型与更短超时。结构化解码失败时
  // 退回容错解析（能兜出 summary 就不浪费这次生成），并打点便于观察。
  let summary = '';
  let overallComment = '';
  let overallScore: number | null = null;
  let dimensionComments = { tasks: '', quality: '', code: '' };
  let usage: unknown = null;
  let usedFallback = false;
  try {
    const result = await generateText({
      model: ctx.model('report'),
      output: Output.object({ schema: reportOutputSchema }),
      prompt: buildReportPrompt(ctx.input),
      maxOutputTokens: ctx.config.reportMaxOutputTokens,
      providerOptions: { dashscope: { enable_thinking: false } },
      abortSignal: ctx.signal,
    });
    summary = result.output?.summary ?? '';
    overallComment = result.output?.overall_comment ?? '';
    overallScore = result.output?.overall_score ?? null;
    dimensionComments = result.output?.dimension_comments ?? dimensionComments;
    usage = result.usage;
  } catch (error) {
    if ((error as { name?: string }).name !== 'AI_NoObjectGeneratedError') throw error;
    const raw = (error as { text?: unknown }).text;
    const parsed = parseReportOutput(typeof raw === 'string' ? raw : '');
    summary = parsed.summary;
    overallComment = parsed.overallComment;
    overallScore = parsed.overallScore ?? null;
    dimensionComments = parsed.dimensionComments;
    usedFallback = true;
  }
  if (!summary) throw new Error('report skill returned an empty summary');
  ctx.span.setAttribute('ai.model', ctx.config.reportModel);
  if (usedFallback) ctx.span.setAttribute('pm.report_parse_fallback', true);
  ctx.span.setStatus({ code: SpanStatusCode.OK });
  // 报告产出 = summary 正文 + overall_comment 综合评语 + overall_score 综合
  // 评分 + dimension_comments 三个排行的维度短评；其余 envelope 字段留空以
  // 匹配共享输出类型，Go worker 对 report.* 不校验 changes。
  const output = {
    summary,
    overall_comment: overallComment,
    overall_score: overallScore,
    dimension_comments: dimensionComments,
    changes: [],
    items: [],
    risks: [],
    testCases: [],
    humanReviewRequired: false,
    completenessReport: null,
    workflowProposal: null,
  };
  return { output, usage };
}
