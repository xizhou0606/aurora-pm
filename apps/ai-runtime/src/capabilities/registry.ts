// 能力注册表:每个能力一个描述符,resolveCapability 是唯一分发入口。
// 描述符字段与原 generator.ts if-链逐一对应:
// - prompt/outputSchema/maxOutputTokens/providerOptions → generateText 参数
// - model → 主模型 / reportModel / testCaseModel 三档
// - timeoutCapMs → requestTimeoutForCapability 的上限截断
// - validate → 原 validateCapabilityOutput 内各能力分支
// - normalize/noObjectFallback/retryOnce → 原分支内的归一与兜底恢复
// - handler → 自定义多轮编排(workflow.generate、report.*、qa.test_cases)
import type { z } from 'zod';
import type { RuntimeConfig } from '../config.js';
import type {
  CapabilityHandlerContext,
  CapabilityModelKind,
  GeneratorResult,
} from '../shared/ai/types.js';
import { aiOutputSchema as aiOutputEnvelopeSchema, type AIOutput } from '../shared/ai/schema.js';

import { CHANGELOG_DRAFT_CAPABILITY, buildChangelogDraftPrompt, changelogDraftOutputSchema, normalizeChangelogDraftOutput, recoverChangelogDraftOutput } from './changelog-draft/index.js';
import {
  COMMIT_QUALITY_REVIEW_CAPABILITY,
  buildCommitQualityReviewPrompt,
  commitQualityReviewOutputSchema,
  normalizeCommitQualityReviewOutput,
  recoverCommitQualityReviewOutput,
} from './code-quality-review/index.js';
import { buildGenericPrompt } from './generic-prompt.js';
import { REPORT_REQUEST_TIMEOUT_MS, buildReportPrompt, runReportGeneration } from './report/index.js';
import { buildWorkflowPrompt, runWorkflowGeneration } from './workflow/index.js';
import { createCompletenessFallback, completenessAnalysisSchema, buildCompletenessPrompt, normalizeVerifiedCompletenessOutput, validateCompletenessOutput, type CompletenessAnalysis } from './requirement-completeness/index.js';
import { designClassificationSchema, buildDesignClassifyPrompt, normalizeDesignClassificationOutput, recoverDesignClassificationOutput, validateDesignClassificationOutput } from './design-classify/index.js';
import { taskPlanGenerationSchema, buildTaskPlanPrompt, normalizeTaskPlanGenerationOutput, validateTaskPlanOutput } from './requirement-task-plan/index.js';
import { buildQATestCaseRequestPrompt } from './qa-test-case/index.js';
import { TEST_CASE_REQUEST_TIMEOUT_MS } from './qa-test-case/generate/runtime/generate-runtime.js';
import { validateQATestCaseOutput } from './qa-test-case/generate/pipeline/generate-validate.js';
import { runQATestCaseGeneration } from './qa-test-case/generate/pipeline/generate.js';

export type CapabilityStandardContext = {
  capability: string;
  input: Record<string, unknown>;
};

export type CapabilityDescriptor = {
  capability: string;
  prompt(input: Record<string, unknown>): string;
  model: CapabilityModelKind;
  /** 标准单轮路径的结构化输出契约;handler 形态的能力可不提供。 */
  outputSchema?: z.ZodType;
  maxOutputTokens?(config: RuntimeConfig): number;
  providerOptions?: { dashscope: { enable_thinking: boolean; thinking_budget?: number } };
  /** 请求超时上限(与配置取 min);未设置时直接使用配置超时。 */
  timeoutCapMs?: number;
  /** 超时中止时注入的错误(qa.test_cases 的 TimeoutError 语义)。 */
  timeoutAbortReason?(): Error;
  /** 确定性契约校验(原 validateCapabilityOutput 分支)。 */
  validate?(input: Record<string, unknown>, output: AIOutput): void;
  /** 标准路径输出归一。 */
  normalize?(output: unknown, ctx: CapabilityStandardContext): AIOutput;
  /** NoObjectGeneratedError 兜底(completeness/design_classify)。 */
  noObjectFallback?: {
    spanAttribute: string;
    recover(error: unknown, ctx: CapabilityStandardContext): AIOutput;
  };
  /** 一次重试 + 错误文本兜底(code.commit_quality_review/changelog.draft)。 */
  retryOnce?: {
    spanAttribute: string;
    recover(error: unknown): AIOutput | null;
  };
  /** 自定义编排;注入执行器运行时(模型工厂/信号/span/预算/中止)。 */
  handler?(ctx: CapabilityHandlerContext): Promise<GeneratorResult>;
};

const dashscopeNoThinking = { dashscope: { enable_thinking: false } } as const;

const workflowDescriptor: CapabilityDescriptor = {
  capability: 'workflow.generate',
  prompt: buildWorkflowPrompt,
  model: 'main',
  handler: runWorkflowGeneration,
};

const reportDescriptor: CapabilityDescriptor = {
  capability: 'report',
  prompt: buildReportPrompt,
  model: 'report',
  timeoutCapMs: REPORT_REQUEST_TIMEOUT_MS,
  handler: runReportGeneration,
};

const completenessDescriptor: CapabilityDescriptor = {
  capability: 'requirement.completeness',
  prompt: buildCompletenessPrompt,
  model: 'main',
  outputSchema: completenessAnalysisSchema,
  maxOutputTokens: (config) => Math.min(config.AI_MAX_OUTPUT_TOKENS, 5000),
  providerOptions: dashscopeNoThinking,
  normalize: (output, ctx) => normalizeVerifiedCompletenessOutput(ctx.input, output as CompletenessAnalysis),
  validate: validateCompletenessOutput,
  noObjectFallback: {
    spanAttribute: 'pm.completeness_format_fallback',
    recover: (_error, ctx) => createCompletenessFallback(ctx.input, 'format'),
  },
};

const qaDescriptor: CapabilityDescriptor = {
  capability: 'qa.test_cases',
  prompt: buildQATestCaseRequestPrompt,
  model: 'test-case',
  timeoutCapMs: TEST_CASE_REQUEST_TIMEOUT_MS,
  timeoutAbortReason: () => {
    const error = new Error('QA test case request timed out');
    error.name = 'TimeoutError';
    return error;
  },
  validate: validateQATestCaseOutput,
  handler: runQATestCaseGeneration,
};

const taskPlanDescriptor: CapabilityDescriptor = {
  capability: 'requirement.task_plan',
  prompt: buildTaskPlanPrompt,
  model: 'main',
  outputSchema: taskPlanGenerationSchema,
  maxOutputTokens: (config) => Math.min(config.AI_MAX_OUTPUT_TOKENS, 5000),
  providerOptions: dashscopeNoThinking,
  normalize: (output) => normalizeTaskPlanGenerationOutput(output),
  validate: validateTaskPlanOutput,
};

const designClassifyDescriptor: CapabilityDescriptor = {
  capability: 'requirement.design_classify',
  prompt: buildDesignClassifyPrompt,
  model: 'main',
  outputSchema: designClassificationSchema,
  maxOutputTokens: (config) => Math.min(config.AI_MAX_OUTPUT_TOKENS, 2000),
  providerOptions: dashscopeNoThinking,
  normalize: (output) => normalizeDesignClassificationOutput(output),
  validate: validateDesignClassificationOutput,
  noObjectFallback: {
    spanAttribute: 'pm.design_classification_format_fallback',
    recover: (error) => recoverDesignClassificationOutput(error),
  },
};

const codeQualityDescriptor: CapabilityDescriptor = {
  capability: COMMIT_QUALITY_REVIEW_CAPABILITY,
  prompt: buildCommitQualityReviewPrompt,
  model: 'main',
  outputSchema: commitQualityReviewOutputSchema,
  maxOutputTokens: (config) => Math.min(config.AI_MAX_OUTPUT_TOKENS, 12000),
  providerOptions: dashscopeNoThinking,
  normalize: (output) => normalizeCommitQualityReviewOutput(output),
  retryOnce: {
    spanAttribute: 'pm.commit_quality_format_fallback',
    recover: (error) => recoverCommitQualityReviewOutput(error),
  },
};

const changelogDescriptor: CapabilityDescriptor = {
  capability: CHANGELOG_DRAFT_CAPABILITY,
  prompt: buildChangelogDraftPrompt,
  model: 'main',
  outputSchema: changelogDraftOutputSchema,
  maxOutputTokens: (config) => Math.min(config.AI_MAX_OUTPUT_TOKENS, 4000),
  providerOptions: dashscopeNoThinking,
  normalize: (output) => normalizeChangelogDraftOutput(output),
  retryOnce: {
    spanAttribute: 'pm.changelog_draft_format_fallback',
    recover: (error) => recoverChangelogDraftOutput(error),
  },
};

const exactDescriptors = new Map<string, CapabilityDescriptor>([
  [workflowDescriptor.capability, workflowDescriptor],
  [completenessDescriptor.capability, completenessDescriptor],
  [qaDescriptor.capability, qaDescriptor],
  [taskPlanDescriptor.capability, taskPlanDescriptor],
  [designClassifyDescriptor.capability, designClassifyDescriptor],
  [codeQualityDescriptor.capability, codeQualityDescriptor],
  [changelogDescriptor.capability, changelogDescriptor],
]);

// 通用信封兜底:同一描述符按 capability 缓存(prompt 依赖能力名)。
const genericDescriptors = new Map<string, CapabilityDescriptor>();

function genericDescriptor(capability: string): CapabilityDescriptor {
  const cached = genericDescriptors.get(capability);
  if (cached) return cached;
  const descriptor: CapabilityDescriptor = {
    capability,
    prompt: (input) => buildGenericPrompt(capability, input),
    model: 'main',
    outputSchema: aiOutputEnvelopeSchema,
  };
  genericDescriptors.set(capability, descriptor);
  return descriptor;
}

/** 解析能力描述符:report.* 前缀族 → 精确匹配 → 通用信封兜底。 */
export function resolveCapability(capability: string): CapabilityDescriptor {
  if (capability.startsWith('report.')) return reportDescriptor;
  return exactDescriptors.get(capability) ?? genericDescriptor(capability);
}

// createGenerator 薄封装:唯一执行器由 executor.ts 提供;入口统一从本注册表导入。
export { createGenerator, requestTimeoutForCapability, validateCapabilityOutput } from './executor.js';
