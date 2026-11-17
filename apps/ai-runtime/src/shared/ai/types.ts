import type { Span } from '@opentelemetry/api';
import type { LanguageModel } from 'ai';
import type { RuntimeConfig } from '../../config.js';
import type { AIOutput, GenerateRequest } from './schema.js';

export type { GenerateRequest };

export const QA_TEST_CASE_SUCCESS_TELEMETRY = Symbol('qa-test-case-success-telemetry');

export type QATestCaseModelCallKind =
  | 'draft_initial'
  | 'draft_full_repair'
  | 'draft_coverage_patch'
  | 'global_initial'
  | 'global_full_repair'
  | 'global_ref_patch';

export type QATestCaseModelCallTelemetry = {
  callNumber: number;
  kind: QATestCaseModelCallKind;
  elapsedMs: number;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  finishReason: 'stop' | 'length' | 'content-filter' | 'tool-calls' | 'error' | 'other' | 'unknown';
};

// This closed set deliberately describes gates rather than business content.
// It is safe to retain in successful runtime logs for latency/repair analysis.
export type QATestCaseRepairRuleCode =
  | 'incomplete_response'
  | 'invalid_json'
  | 'schema_structure'
  | 'risk_coverage'
  | 'source_assessment'
  | 'context_coverage'
  | 'context_reference'
  | 'risk_linkage'
  | 'uniqueness'
  | 'smoke_selection'
  | 'case_breadth'
  | 'cross_group_relationship'
  | 'artifact_gate'
  | 'task_group'
  | 'relationship_reference'
  | 'interaction_linkage'
  | 'global_plan_gate';

export type QATestCaseRepairDiagnosticCategory = {
  category: QATestCaseRepairRuleCode;
  count: number;
};

export type TestCaseRepairReason = 'incomplete' | 'parse' | 'structure' | 'validation' | 'coverage' | null;

export type QATestCaseStageSuccessTelemetry = {
  stage: 'draft_batch' | 'global_plan';
  batchNumber?: number;
  calls: QATestCaseModelCallTelemetry[];
  modelCallCount: number;
  modelElapsedMs: number;
  wallElapsedMs: number;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  repaired: boolean;
  repairReason: Exclude<TestCaseRepairReason, null> | 'none';
  finishReasonFallback: boolean;
  coveragePatched: boolean;
  globalRefPatched: boolean;
  repairRuleCodes: QATestCaseRepairRuleCode[];
  repairDiagnosticCategories: QATestCaseRepairDiagnosticCategory[];
};

export type QATestCaseSuccessTelemetry = {
  batchCount: number;
  requirementGroupCount: number;
  globalPlan: boolean;
  totalConcurrency: number;
  modelCallCount: number;
  modelElapsedMs: number;
  wallElapsedMs: number;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  stages: QATestCaseStageSuccessTelemetry[];
};

export type GeneratorResult = {
  output: AIOutput;
  usage: unknown;
  [QA_TEST_CASE_SUCCESS_TELEMETRY]?: QATestCaseSuccessTelemetry;
};

export type Generator = (request: GenerateRequest) => Promise<GeneratorResult>;

export type CapabilityModelKind = 'main' | 'report' | 'test-case';

// 执行器注入给能力 handler 的运行时上下文:模型工厂、取消信号、span、
// 请求预算与主动中止入口(失败闩锁需要中止兄弟调用)。
export type CapabilityHandlerContext = {
  capability: string;
  input: Record<string, unknown>;
  config: RuntimeConfig;
  signal: AbortSignal;
  span: Span;
  requestTimeoutMs: number;
  model(kind: CapabilityModelKind): LanguageModel;
  abort(reason?: unknown): void;
};
