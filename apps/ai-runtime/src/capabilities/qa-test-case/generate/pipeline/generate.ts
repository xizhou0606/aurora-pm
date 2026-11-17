import { SpanStatusCode } from '@opentelemetry/api';
import {
  QA_TEST_CASE_SUCCESS_TELEMETRY,
  type CapabilityHandlerContext,
  type GeneratorResult,
  type QATestCaseModelCallKind,
  type QATestCaseModelCallTelemetry,
  type QATestCaseRepairRuleCode,
} from '../../../../shared/ai/types.js';
import {
  allowedQATestCaseContextRefs,
  buildQATestCaseDraftSkillInvocation,
  buildQATestCaseGlobalPlanSkillInvocation,
  mergeQATestCaseBatchOutputs,
  partitionQATestCaseInput,
  type QATestCaseBatch,
} from '../../index.js';
import { generateAndCompileTestCaseDraftBatch } from '../draft/generate-draft.js';
import { generateAndCompileTestCaseGlobalPlan } from '../global-plan/generate-global-plan-run.js';
import { createQATestCaseGlobalPlanStage } from '../global-plan/generate-global-stage.js';
import { createQATestCaseModelTextGenerator } from '../runtime/generate-model-call.js';
import {
  attachTestCaseSkillProvenance,
  qaTestCaseFormalRequirementGroupCount,
  shouldGenerateQATestCaseGlobalPlan,
} from './generate-plan.js';
import {
  combineTokenUsages,
  qaTestCaseGenerationSuccessTelemetry,
  qaTestCaseStageSuccessTelemetry,
} from '../runtime/generate-telemetry.js';
import { createTestCaseFailureLatch, raceQATestCaseAbort } from '../runtime/generate-errors.js';
import { mapWithConcurrency } from '../runtime/generate-concurrency.js';
import {
  testCaseBatchConcurrency,
  testCaseMaxOutputTokensForInput,
  TEST_CASE_GLOBAL_PLAN_MAX_OUTPUT_TOKENS,
  type GenerateTestCaseModelText,
} from '../runtime/generate-runtime.js';
import { validateQATestCaseOutput } from './generate-validate.js';

// qa.test_cases 能力编排入口(原 generator.ts 内 qa.test_cases 分支,逐字迁移):
// 分片 → 每批 Draft IR 编译/修复 → 跨批 Global Plan → 合并与全局门禁校验。
export async function runQATestCaseGeneration(ctx: CapabilityHandlerContext): Promise<GeneratorResult> {
        const batches = partitionQATestCaseInput(ctx.input);
        const requirementGroupCount = qaTestCaseFormalRequirementGroupCount(ctx.input);
        const needsGlobalPlan = shouldGenerateQATestCaseGlobalPlan(
          ctx.input,
          batches.length,
        );
        const firstAllowedRefs = allowedQATestCaseContextRefs(batches[0].input);
        const skillInvocation = buildQATestCaseDraftSkillInvocation(
          batches[0].input,
          firstAllowedRefs,
        );
        const globalAllowedRefs = needsGlobalPlan
          ? allowedQATestCaseContextRefs(ctx.input)
          : [];
        const globalInvocation = needsGlobalPlan
          ? buildQATestCaseGlobalPlanSkillInvocation(ctx.input, globalAllowedRefs)
          : null;
        const batchConcurrency = testCaseBatchConcurrency(Boolean(globalInvocation));
        const totalConcurrency = batchConcurrency + (globalInvocation ? 1 : 0);
        const testCaseModel = ctx.model('test-case');
        ctx.span.setAttributes({
          'pm.qa_skill.commit': skillInvocation.skill.commit,
          'pm.qa_skill.package_sha256': skillInvocation.skill.packageSha256,
          'pm.qa_skill.mode': skillInvocation.skill.mode,
          'pm.qa_skill.resources': skillInvocation.skill.resources,
          'pm.test_case_batch_count': batches.length,
          'pm.test_case_requirement_group_count': requirementGroupCount,
          'pm.test_case_batch_concurrency': batchConcurrency,
          'pm.test_case_global_plan': Boolean(globalInvocation),
          'pm.test_case_total_concurrency': totalConcurrency,
          'pm.test_case_model': ctx.config.testCaseModel,
          'pm.test_case_enable_thinking': true,
          'pm.test_case_thinking_budget': 512,
        });
        const startedAt = Date.now();
        const remainingMs = () => Math.max(0, ctx.requestTimeoutMs - (Date.now() - startedAt));
        const modelTextGenerator = (
          maxOutputTokens: number,
          defaultKind: QATestCaseModelCallKind,
          calls: QATestCaseModelCallTelemetry[],
        ): GenerateTestCaseModelText =>
          createQATestCaseModelTextGenerator({
            model: testCaseModel,
            configuredMaxOutputTokens: ctx.config.AI_MAX_OUTPUT_TOKENS,
            maxOutputTokens,
            defaultKind,
            calls,
          });
        const failureLatch = createTestCaseFailureLatch(
          () => ctx.abort(),
          batches.length,
        );
        const generateGlobalPlan = createQATestCaseGlobalPlanStage({
          input: ctx.input,
          invocation: globalInvocation,
          allowedRefs: globalAllowedRefs,
          signal: ctx.signal,
          remainingMs,
          span: ctx.span,
          capture: (error, stage, batchNumber) => failureLatch.capture(error, stage, batchNumber),
          modelTextGenerator,
        });
        const generateBatch = async (batch: QATestCaseBatch) => {
          const stageStartedAt = Date.now();
          const calls: QATestCaseModelCallTelemetry[] = [];
          const repairDiagnosticCodes: QATestCaseRepairRuleCode[] = [];
          const allowedRefs = batch.index === 0
            ? firstAllowedRefs
            : allowedQATestCaseContextRefs(batch.input);
          const batchInvocation = batch.index === 0
            ? skillInvocation
            : buildQATestCaseDraftSkillInvocation(batch.input, allowedRefs);
          const batchMaxOutputTokens = testCaseMaxOutputTokensForInput(
            batch.input,
            ctx.config.AI_TEST_CASE_MAX_OUTPUT_TOKENS,
          );
          const prompt = batches.length === 1
            ? batchInvocation.prompt
            : [
              `这是有界生成的第 ${batch.index + 1}/${batches.length} 个独立分片。只覆盖当前业务输入中实际给出的 requirements/tasks 数组，不得推断或引用其他分片。`,
              '若当前分片有 5-10 个不同业务事实，至少用 2 条语义独立的 cases 覆盖；不得用一条泛化用例堆叠全部 contextRefs。',
              '为保证所有分片的语义能无损合并，objective 不超过 40 个字符，priorityLogic 不超过 90 个字符。',
              batchInvocation.prompt,
            ].join('\n');
          // The model returns compact semantic IR. A deterministic compiler builds
          // the repetitive final artifact and the existing gate validates it.
          let generated;
          try {
            generated = await generateAndCompileTestCaseDraftBatch({
              input: batch.input,
              allowedRefs,
              instructions: batchInvocation.instructions,
              prompt,
              abortSignal: ctx.signal,
              remainingMs,
              recordRepairDiagnostic: (codes) => repairDiagnosticCodes.push(...codes),
              generate: modelTextGenerator(batchMaxOutputTokens, 'draft_initial', calls),
            });
          } catch (error) {
            throw failureLatch.capture(error, 'draft_batch', batch.index + 1);
          }
          ctx.span.setAttribute('pm.test_case_draft_compiler', true);
          if (generated.finishReasonFallback) {
            ctx.span.setAttribute('pm.test_case_draft_finish_reason_fallback', true);
          }
          if (generated.repaired) {
            ctx.span.setAttributes({
              'pm.test_case_repaired': true,
              'pm.test_case_repair_reason': generated.repairReason ?? 'unknown',
            });
          }
          if (generated.coveragePatched) {
            ctx.span.setAttribute('pm.test_case_coverage_patch', true);
          }
          return {
            output: generated.output,
            usage: generated.usage,
            telemetry: qaTestCaseStageSuccessTelemetry({
              stage: 'draft_batch',
              batchNumber: batch.index + 1,
              calls,
              wallElapsedMs: Date.now() - stageStartedAt,
              repaired: generated.repaired,
              repairReason: generated.repairReason,
                finishReasonFallback: generated.finishReasonFallback,
                coveragePatched: generated.coveragePatched,
                repairDiagnosticCodes,
              }),
          };
        };

        let globalResult;
        let batchResults;
        try {
          // Global starts first and initially owns one of the four provider
          // slots. Once it succeeds, its slot immediately becomes a fourth
          // Draft worker without ever raising total concurrency above four.
          const globalPromise = generateGlobalPlan();
          [globalResult, batchResults] = await raceQATestCaseAbort(Promise.all([
            globalPromise,
            mapWithConcurrency(
              batches,
              batchConcurrency,
              generateBatch,
              () => ctx.abort(),
              globalInvocation ? globalPromise : undefined,
            ),
          ]), ctx.signal);
        } catch (error) {
          throw failureLatch.primaryFailure() ?? error;
        }
        let output = mergeQATestCaseBatchOutputs(batchResults.map((result, index) => ({
          batch: batches[index],
          output: result.output,
        })), globalResult?.plan);
        validateQATestCaseOutput(ctx.input, output);
        output = attachTestCaseSkillProvenance(output, skillInvocation.skill);
        const successTelemetry = qaTestCaseGenerationSuccessTelemetry({
          batchCount: batches.length,
          requirementGroupCount,
          globalPlan: Boolean(globalResult),
          totalConcurrency,
          wallElapsedMs: Date.now() - startedAt,
          stages: [
            ...batchResults.map((result) => result.telemetry),
            ...(globalResult ? [globalResult.telemetry] : []),
          ],
        });
        ctx.span.setAttributes({
          'pm.test_case_model_call_count': successTelemetry.modelCallCount,
          'pm.test_case_model_elapsed_ms': successTelemetry.modelElapsedMs,
          'pm.test_case_wall_elapsed_ms': successTelemetry.wallElapsedMs,
          'pm.test_case_input_tokens': successTelemetry.inputTokens,
          'pm.test_case_output_tokens': successTelemetry.outputTokens,
          'pm.test_case_repaired_stage_count': successTelemetry.stages.filter(
            (stage) => stage.repaired,
          ).length,
        });
        ctx.span.setStatus({ code: SpanStatusCode.OK });
        return {
          output,
          usage: combineTokenUsages(
            ...batchResults.map((result) => result.usage),
            globalResult?.usage,
          ),
          [QA_TEST_CASE_SUCCESS_TELEMETRY]: successTelemetry,
        };
}
