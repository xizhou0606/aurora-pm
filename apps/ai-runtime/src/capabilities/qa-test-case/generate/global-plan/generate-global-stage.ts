import type { Span } from '@opentelemetry/api';
import type {
  QATestCaseModelCallKind,
  QATestCaseModelCallTelemetry,
  QATestCaseRepairRuleCode,
  QATestCaseStageSuccessTelemetry,
} from '../../../../shared/ai/types.js';
import {
  compileQATestCaseGlobalPlan,
  type CompiledQATestCaseGlobalPlan,
  type QATestCaseSkillInvocation,
} from '../../index.js';
import { generateAndCompileTestCaseGlobalPlan } from './generate-global-plan-run.js';
import { qaTestCaseStageSuccessTelemetry } from '../runtime/generate-telemetry.js';
import {
  TEST_CASE_GLOBAL_PLAN_MAX_OUTPUT_TOKENS,
  type GenerateTestCaseModelText,
} from '../runtime/generate-runtime.js';

// 全局计划阶段闭包(原 qa 分支内 generateGlobalPlan,逐字迁移):
// 成功后把占用的并发槽位交还给 Draft worker,由编排层调度。
type QATestCaseGlobalPlanCapture = (error: unknown, stage: 'draft_batch' | 'global_plan', batchNumber?: number) => Error;

export function createQATestCaseGlobalPlanStage(options: {
  input: Record<string, unknown>;
  invocation: QATestCaseSkillInvocation | null;
  allowedRefs: readonly string[];
  signal: AbortSignal;
  remainingMs: () => number;
  span: Span;
  capture: QATestCaseGlobalPlanCapture;
  modelTextGenerator: (
    maxOutputTokens: number,
    defaultKind: QATestCaseModelCallKind,
    calls: QATestCaseModelCallTelemetry[],
  ) => GenerateTestCaseModelText;
}): () => Promise<{
  plan: CompiledQATestCaseGlobalPlan;
  usage: unknown;
  telemetry: QATestCaseStageSuccessTelemetry;
} | null> {
  return async () => {
          if (!options.invocation) return null;
          const stageStartedAt = Date.now();
          const calls: QATestCaseModelCallTelemetry[] = [];
          const repairDiagnosticCodes: QATestCaseRepairRuleCode[] = [];
          try {
            const result = await generateAndCompileTestCaseGlobalPlan({
              input: options.input,
              allowedRefs: options.allowedRefs,
              instructions: options.invocation.instructions,
              prompt: options.invocation.prompt,
              abortSignal: options.signal,
              remainingMs: options.remainingMs,
              recordRepairDiagnostic: (codes) => repairDiagnosticCodes.push(...codes),
              generate: options.modelTextGenerator(
                TEST_CASE_GLOBAL_PLAN_MAX_OUTPUT_TOKENS,
                'global_initial',
                calls,
              ),
            });
            if (result.finishReasonFallback) {
              options.span.setAttribute('pm.test_case_global_plan_finish_reason_fallback', true);
            }
            if (result.repaired) {
              options.span.setAttributes({
                'pm.test_case_global_plan_repaired': true,
                'pm.test_case_global_plan_repair_reason': result.repairReason ?? 'unknown',
              });
            }
            if (result.globalRefPatched) {
              options.span.setAttributes({
                'pm.test_case_global_ref_patched': true,
              });
            }
            return {
              ...result,
              telemetry: qaTestCaseStageSuccessTelemetry({
                stage: 'global_plan',
                calls,
                wallElapsedMs: Date.now() - stageStartedAt,
                repaired: result.repaired,
                repairReason: result.repairReason,
                finishReasonFallback: result.finishReasonFallback,
                globalRefPatched: result.globalRefPatched,
                repairDiagnosticCodes,
              }),
            };
          } catch (error) {
            throw options.capture(error, 'global_plan');
          }
  };
}
