import type { AIOutput } from '../../../../shared/ai/schema.js';
import type { TestCaseRepairReason } from '../../../../shared/ai/types.js';
import {
  buildQATestCaseDraftRepairInstruction,
  compileQATestCaseDraft,
  QATestCaseDraftCompilationError,
  type QATestCaseDraft,
} from '../../index.js';
import {
  assertCompleteTestCaseFinishReason,
  canRewriteIncompleteTestCase,
  incompleteTestCaseRewriteInstruction,
  TEST_CASE_REPAIR_MIN_REMAINING_MS,
  usesMissingFinishReasonFallback,
  type GenerateTestCaseModelText,
  type TestCaseModelTextResult,
} from '../runtime/generate-runtime.js';
import {
  combineTokenUsage,
  combineTokenUsages,
  draftCompilationIssueCodes,
  inspectionRepairRuleCode,
  recordQATestCaseRepairRules,
  type QATestCaseRepairDiagnosticRecorder,
} from '../runtime/generate-telemetry.js';
import { testCaseRepairStepError } from '../runtime/generate-errors.js';
import { hasOnlyRiskCoverageDiagnostics, inspectQATestCaseDraftText, repairQATestCaseDraftCoverage } from './generate-draft-repair.js';
import { hasEligibleMixedCoverageTargetPatch, repairQATestCaseDraftFinal, repairQATestCaseDraftMixedCoverageTarget } from './generate-draft-final.js';
import { validateQATestCaseOutput } from '../pipeline/generate-validate.js';

export async function generateAndCompileTestCaseDraftBatch(options: {
  input: Record<string, unknown>;
  allowedRefs: readonly string[];
  instructions: string;
  prompt: string;
  abortSignal: AbortSignal;
  remainingMs: () => number;
  generate: GenerateTestCaseModelText;
  recordRepairDiagnostic?: QATestCaseRepairDiagnosticRecorder;
}): Promise<{
  output: AIOutput;
  usage: unknown;
  repaired: boolean;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  coveragePatched: boolean;
}> {
  const initial = await options.generate({
    instructions: options.instructions,
    prompt: options.prompt,
    abortSignal: options.abortSignal,
    telemetryKind: 'draft_initial',
  });
  const initialFinishReasonFallback = usesMissingFinishReasonFallback(initial);
  if (initial.finishReason !== 'stop' && !initialFinishReasonFallback) {
    if (!canRewriteIncompleteTestCase(initial.finishReason)
      || options.remainingMs() < TEST_CASE_REPAIR_MIN_REMAINING_MS) {
      assertCompleteTestCaseFinishReason(initial.finishReason, 'generation', initial);
    }
    recordQATestCaseRepairRules(options.recordRepairDiagnostic, ['incomplete_response']);
    let rewrite: TestCaseModelTextResult;
    try {
      rewrite = await options.generate({
        instructions: options.instructions,
        prompt: [
          options.prompt,
          incompleteTestCaseRewriteInstruction('Draft IR'),
        ].join('\n'),
        abortSignal: options.abortSignal,
        telemetryKind: 'draft_full_repair',
      });
    } catch (error) {
      throw testCaseRepairStepError(
        error, 'full_draft', 'generation', undefined, undefined, undefined, 1,
      );
    }
    const rewriteFinishReasonFallback = usesMissingFinishReasonFallback(rewrite);
    if (!rewriteFinishReasonFallback) {
      try {
        assertCompleteTestCaseFinishReason(rewrite.finishReason, 'repair', rewrite);
      } catch (error) {
        throw testCaseRepairStepError(
          error, 'full_draft', 'finish', rewrite, undefined, undefined, 1,
        );
      }
    }
    const rewritten = inspectQATestCaseDraftText(rewrite.text);
    if (!rewritten.draft) {
      recordQATestCaseRepairRules(
        options.recordRepairDiagnostic,
        [inspectionRepairRuleCode(rewritten.failure)].filter(Boolean),
      );
      throw testCaseRepairStepError(
        new Error(rewritten.issues.join('; ')), 'full_draft', 'parse', rewrite,
        undefined, undefined, 1,
      );
    }
    try {
      return {
        output: compileQATestCaseDraft(options.input, rewritten.draft),
        usage: combineTokenUsage(initial.usage, rewrite.usage),
        repaired: true,
        repairReason: 'incomplete' as const,
        finishReasonFallback: rewriteFinishReasonFallback,
        coveragePatched: false,
      };
    } catch (error) {
      if (!(error instanceof QATestCaseDraftCompilationError)) throw error;
      recordQATestCaseRepairRules(
        options.recordRepairDiagnostic,
        draftCompilationIssueCodes(error),
      );
      const continuation = {
        ...options,
        draft: rewritten.draft,
        failure: error,
        usage: [initial.usage, rewrite.usage],
        repairReason: 'incomplete' as const,
        finishReasonFallback: rewriteFinishReasonFallback,
        fullDraftRepairAttempt: 2 as const,
      };
      if (hasOnlyRiskCoverageDiagnostics(error)) {
        return repairQATestCaseDraftCoverage(continuation);
      }
      return hasEligibleMixedCoverageTargetPatch(options.input, rewritten.draft, error)
        ? repairQATestCaseDraftMixedCoverageTarget(continuation)
        : repairQATestCaseDraftFinal(continuation);
    }
  }
  if (!initialFinishReasonFallback) {
    assertCompleteTestCaseFinishReason(initial.finishReason, 'generation', initial);
  }

  const inspected = inspectQATestCaseDraftText(initial.text);
  let draft = inspected.draft ?? undefined;
  let repairReason: TestCaseRepairReason = inspected.failure;
  let issues = inspected.issues;
  recordQATestCaseRepairRules(
    options.recordRepairDiagnostic,
    [inspectionRepairRuleCode(inspected.failure)].filter(Boolean),
  );
  let compilationFailure: QATestCaseDraftCompilationError | undefined;
  if (draft) {
    try {
      const output = compileQATestCaseDraft(options.input, draft);
      return {
        output,
        usage: initial.usage,
        repaired: false,
        repairReason: null,
        finishReasonFallback: initialFinishReasonFallback,
        coveragePatched: false,
      };
    } catch (error) {
      if (!(error instanceof QATestCaseDraftCompilationError)) throw error;
      recordQATestCaseRepairRules(
        options.recordRepairDiagnostic,
        draftCompilationIssueCodes(error),
      );
      repairReason = 'validation';
      issues = error.issues;
      compilationFailure = error;
    }
  }

  if (draft && compilationFailure && hasOnlyRiskCoverageDiagnostics(compilationFailure)) {
    return repairQATestCaseDraftCoverage({
      ...options,
      draft,
      failure: compilationFailure,
      usage: [initial.usage],
      repairReason: 'coverage',
      finishReasonFallback: initialFinishReasonFallback,
    });
  }
  if (draft && compilationFailure && hasEligibleMixedCoverageTargetPatch(
    options.input, draft, compilationFailure,
  )) {
    return repairQATestCaseDraftMixedCoverageTarget({
      ...options,
      draft,
      failure: compilationFailure,
      usage: [initial.usage],
      repairReason: 'coverage',
      finishReasonFallback: initialFinishReasonFallback,
    });
  }

  if (options.remainingMs() < TEST_CASE_REPAIR_MIN_REMAINING_MS) {
    throw testCaseRepairStepError(
      compilationFailure ?? new Error(issues.join('; ')),
      'full_draft',
      'budget',
      undefined,
      undefined,
      undefined,
      1,
    );
  }

  let repair: TestCaseModelTextResult;
  try {
    repair = await options.generate({
      instructions: options.instructions,
      messages: [
        { role: 'user', content: options.prompt },
        { role: 'assistant', content: draft ? JSON.stringify(draft) : initial.text },
        {
          role: 'user',
          content: buildQATestCaseDraftRepairInstruction(issues, options.allowedRefs, options.input),
        },
      ],
      abortSignal: options.abortSignal,
      telemetryKind: 'draft_full_repair',
    });
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'full_draft', 'generation', undefined, undefined, undefined, 1,
    );
  }
  const repairFinishReasonFallback = usesMissingFinishReasonFallback(repair);
  if (!repairFinishReasonFallback) {
    try {
      assertCompleteTestCaseFinishReason(repair.finishReason, 'repair', repair);
    } catch (error) {
      throw testCaseRepairStepError(
        error, 'full_draft', 'finish', repair, undefined, undefined, 1,
      );
    }
  }

  const repaired = inspectQATestCaseDraftText(repair.text);
  if (!repaired.draft) {
    recordQATestCaseRepairRules(
      options.recordRepairDiagnostic,
      [inspectionRepairRuleCode(repaired.failure)].filter(Boolean),
    );
    throw testCaseRepairStepError(
      new Error(repaired.issues.join('; ')),
      'full_draft',
      'parse',
      repair,
      undefined,
      undefined,
      1,
    );
  }
  draft = repaired.draft;
  try {
    return {
      output: compileQATestCaseDraft(options.input, draft),
      usage: combineTokenUsage(initial.usage, repair.usage),
      repaired: true,
      repairReason,
      finishReasonFallback: initialFinishReasonFallback || repairFinishReasonFallback,
      coveragePatched: false,
    };
  } catch (error) {
    if (!(error instanceof QATestCaseDraftCompilationError)) throw error;
    recordQATestCaseRepairRules(
      options.recordRepairDiagnostic,
      draftCompilationIssueCodes(error),
    );
    const continuation = {
      ...options,
      draft,
      failure: error,
      usage: [initial.usage, repair.usage],
      repairReason,
      finishReasonFallback: initialFinishReasonFallback || repairFinishReasonFallback,
      fullDraftRepairAttempt: 2 as const,
    };
    if (hasOnlyRiskCoverageDiagnostics(error)) {
      return repairQATestCaseDraftCoverage(continuation);
    }
    return hasEligibleMixedCoverageTargetPatch(options.input, draft, error)
      ? repairQATestCaseDraftMixedCoverageTarget(continuation)
      : repairQATestCaseDraftFinal(continuation);
  }
}
