import type { TestCaseRepairReason } from '../../../../shared/ai/types.js';
import { buildQATestCaseGlobalPlanRepairInstruction, compileQATestCaseGlobalPlan, QATestCaseGlobalPlanCompilationError, type CompiledQATestCaseGlobalPlan, type QATestCaseGlobalPlan } from '../../index.js';
import {
  assertCompleteTestCaseFinishReason,
  canRewriteIncompleteTestCase,
  incompleteTestCaseRewriteInstruction,
  TEST_CASE_REPAIR_MIN_REMAINING_MS,
  usesMissingFinishReasonFallback,
  type GenerateTestCaseModelText,
  type TestCaseModelTextResult,
} from '../runtime/generate-runtime.js';
import { combineTokenUsage, globalPlanIssueCodes, inspectionRepairRuleCode, recordQATestCaseRepairRules, type QATestCaseRepairDiagnosticRecorder } from '../runtime/generate-telemetry.js';
import { testCaseRepairStepError } from '../runtime/generate-errors.js';
import { hasOnlyGlobalPlanRefDiagnostics, inspectQATestCaseGlobalPlanText, repairQATestCaseGlobalPlanFinal, repairQATestCaseGlobalPlanRefs } from './generate-global-plan.js';

export async function generateAndCompileTestCaseGlobalPlan(options: {
  input: Record<string, unknown>;
  allowedRefs: readonly string[];
  instructions: string;
  prompt: string;
  abortSignal: AbortSignal;
  remainingMs: () => number;
  generate: GenerateTestCaseModelText;
  recordRepairDiagnostic?: QATestCaseRepairDiagnosticRecorder;
}): Promise<{
  plan: CompiledQATestCaseGlobalPlan;
  usage: unknown;
  repaired: boolean;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  globalRefPatched: boolean;
}> {
  const initial = await options.generate({
    instructions: options.instructions,
    prompt: options.prompt,
    abortSignal: options.abortSignal,
    telemetryKind: 'global_initial',
  });
  const initialFinishReasonFallback = usesMissingFinishReasonFallback(initial);
  if (initial.finishReason !== 'stop' && !initialFinishReasonFallback) {
    if (!canRewriteIncompleteTestCase(initial.finishReason)) {
      assertCompleteTestCaseFinishReason(initial.finishReason, 'generation', initial);
    }
    let incompleteCause: unknown;
    try {
      assertCompleteTestCaseFinishReason(initial.finishReason, 'generation', initial);
    } catch (error) {
      incompleteCause = error;
    }
    if (options.remainingMs() < TEST_CASE_REPAIR_MIN_REMAINING_MS) {
      throw testCaseRepairStepError(
        incompleteCause, 'global_plan', 'budget', initial,
      );
    }
    recordQATestCaseRepairRules(options.recordRepairDiagnostic, ['incomplete_response']);
    let rewrite: TestCaseModelTextResult;
    try {
      rewrite = await options.generate({
        instructions: options.instructions,
        prompt: [
          options.prompt,
          incompleteTestCaseRewriteInstruction('Global Plan'),
        ].join('\n'),
        abortSignal: options.abortSignal,
        telemetryKind: 'global_full_repair',
      });
    } catch (error) {
      throw testCaseRepairStepError(error, 'global_plan', 'generation');
    }
    const rewriteFinishReasonFallback = usesMissingFinishReasonFallback(rewrite);
    if (!rewriteFinishReasonFallback) {
      try {
        assertCompleteTestCaseFinishReason(rewrite.finishReason, 'repair', rewrite);
      } catch (error) {
        if (!canRewriteIncompleteTestCase(rewrite.finishReason)) {
          throw testCaseRepairStepError(error, 'global_plan', 'finish', rewrite);
        }
        return repairQATestCaseGlobalPlanFinal({
          ...options,
          first: initial,
          second: rewrite,
          issues: [],
          cause: error,
          repairReason: 'incomplete',
          finishReasonFallback: initialFinishReasonFallback,
          fromScratch: true,
        });
      }
    }
    const rewritten = inspectQATestCaseGlobalPlanText(rewrite.text);
    if (!rewritten.plan) {
      recordQATestCaseRepairRules(
        options.recordRepairDiagnostic,
        [inspectionRepairRuleCode(rewritten.failure)].filter(Boolean),
      );
      const cause = new Error(rewritten.issues.join('; '));
      return repairQATestCaseGlobalPlanFinal({
        ...options,
        first: initial,
        second: rewrite,
        issues: rewritten.issues,
        cause,
        repairReason: 'incomplete',
        finishReasonFallback: initialFinishReasonFallback || rewriteFinishReasonFallback,
        fromScratch: false,
      });
    }
    try {
      return {
        plan: compileQATestCaseGlobalPlan(options.input, rewritten.plan),
        usage: combineTokenUsage(initial.usage, rewrite.usage),
        repaired: true,
        repairReason: 'incomplete',
        finishReasonFallback: initialFinishReasonFallback || rewriteFinishReasonFallback,
        globalRefPatched: false,
      };
    } catch (error) {
      if (!(error instanceof QATestCaseGlobalPlanCompilationError)) {
        throw testCaseRepairStepError(error, 'global_plan', 'compile', rewrite);
      }
      recordQATestCaseRepairRules(
        options.recordRepairDiagnostic,
        globalPlanIssueCodes(error),
      );
      if (hasOnlyGlobalPlanRefDiagnostics(error)) {
        return repairQATestCaseGlobalPlanRefs({
          ...options,
          plan: rewritten.plan,
          failure: error,
          usage: [initial.usage, rewrite.usage],
          repairReason: 'incomplete',
          finishReasonFallback: initialFinishReasonFallback || rewriteFinishReasonFallback,
          previousResult: rewrite,
        });
      }
      return repairQATestCaseGlobalPlanFinal({
        ...options,
        first: initial,
        second: rewrite,
        plan: rewritten.plan,
        issues: error.issues,
        cause: error,
        repairReason: 'incomplete',
        finishReasonFallback: initialFinishReasonFallback || rewriteFinishReasonFallback,
        fromScratch: false,
      });
    }
  }
  if (!initialFinishReasonFallback) {
    assertCompleteTestCaseFinishReason(initial.finishReason, 'generation', initial);
  }

  const inspected = inspectQATestCaseGlobalPlanText(initial.text);
  let plan = inspected.plan ?? undefined;
  let repairReason: TestCaseRepairReason = inspected.failure;
  let issues = inspected.issues;
  recordQATestCaseRepairRules(
    options.recordRepairDiagnostic,
    [inspectionRepairRuleCode(inspected.failure)].filter(Boolean),
  );
  let compilationFailure: QATestCaseGlobalPlanCompilationError | undefined;
  if (plan) {
    try {
      const compiled = compileQATestCaseGlobalPlan(options.input, plan);
      return {
        plan: compiled,
        usage: initial.usage,
        repaired: false,
        repairReason: null,
        finishReasonFallback: initialFinishReasonFallback,
        globalRefPatched: false,
      };
    } catch (error) {
      if (!(error instanceof QATestCaseGlobalPlanCompilationError)) throw error;
      recordQATestCaseRepairRules(
        options.recordRepairDiagnostic,
        globalPlanIssueCodes(error),
      );
      repairReason = 'validation';
      issues = error.issues;
      compilationFailure = error;
    }
  }

  if (options.remainingMs() < TEST_CASE_REPAIR_MIN_REMAINING_MS) {
    throw testCaseRepairStepError(
      compilationFailure ?? new Error(issues.join('; ')),
      'global_plan',
      'budget',
    );
  }
  let repair: TestCaseModelTextResult;
  try {
    repair = await options.generate({
      instructions: options.instructions,
      messages: [
        { role: 'user', content: options.prompt },
        { role: 'assistant', content: plan ? JSON.stringify(plan) : initial.text },
        {
          role: 'user',
          content: buildQATestCaseGlobalPlanRepairInstruction(issues, options.allowedRefs),
        },
      ],
      abortSignal: options.abortSignal,
      telemetryKind: 'global_full_repair',
    });
  } catch (error) {
    throw testCaseRepairStepError(error, 'global_plan', 'generation');
  }
  const repairFinishReasonFallback = usesMissingFinishReasonFallback(repair);
  if (!repairFinishReasonFallback) {
    try {
      assertCompleteTestCaseFinishReason(repair.finishReason, 'repair', repair);
    } catch (error) {
      if (!canRewriteIncompleteTestCase(repair.finishReason)) {
        throw testCaseRepairStepError(error, 'global_plan', 'finish', repair);
      }
      return repairQATestCaseGlobalPlanFinal({
        ...options,
        first: initial,
        second: repair,
        issues: [],
        cause: error,
        repairReason,
        finishReasonFallback: initialFinishReasonFallback,
        fromScratch: true,
      });
    }
  }
  const repaired = inspectQATestCaseGlobalPlanText(repair.text);
  if (!repaired.plan) {
    recordQATestCaseRepairRules(
      options.recordRepairDiagnostic,
      [inspectionRepairRuleCode(repaired.failure)].filter(Boolean),
    );
    return repairQATestCaseGlobalPlanFinal({
      ...options,
      first: initial,
      second: repair,
      issues: repaired.issues,
      cause: new Error(repaired.issues.join('; ')),
      repairReason,
      finishReasonFallback: initialFinishReasonFallback || repairFinishReasonFallback,
      fromScratch: false,
    });
  }
  plan = repaired.plan;
  let compiled: CompiledQATestCaseGlobalPlan;
  try {
    compiled = compileQATestCaseGlobalPlan(options.input, plan);
  } catch (error) {
    if (!(error instanceof QATestCaseGlobalPlanCompilationError)) {
      throw testCaseRepairStepError(error, 'global_plan', 'compile', repair);
    }
    recordQATestCaseRepairRules(
      options.recordRepairDiagnostic,
      globalPlanIssueCodes(error),
    );
    if (hasOnlyGlobalPlanRefDiagnostics(error)) {
      return repairQATestCaseGlobalPlanRefs({
        ...options,
        plan,
        failure: error,
        usage: [initial.usage, repair.usage],
        repairReason,
        finishReasonFallback: initialFinishReasonFallback || repairFinishReasonFallback,
        previousResult: repair,
      });
    }
    return repairQATestCaseGlobalPlanFinal({
      ...options,
      first: initial,
      second: repair,
      plan,
      issues: error.issues,
      cause: error,
      repairReason,
      finishReasonFallback: initialFinishReasonFallback || repairFinishReasonFallback,
      fromScratch: false,
    });
  }
  return {
    plan: compiled,
    usage: combineTokenUsage(initial.usage, repair.usage),
    repaired: true,
    repairReason,
    finishReasonFallback: initialFinishReasonFallback || repairFinishReasonFallback,
    globalRefPatched: false,
  };
}
