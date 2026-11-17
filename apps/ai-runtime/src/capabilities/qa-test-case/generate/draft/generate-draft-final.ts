import type { AIOutput } from '../../../../shared/ai/schema.js';
import type { TestCaseRepairReason } from '../../../../shared/ai/types.js';
import {
  applyQATestCaseMixedCoverageTargetPatch,
  buildQATestCaseDraftFinalRepairInstruction,
  compileQATestCaseDraft,
  invokeQATestCaseMixedCoverageTargetPatchSkill,
  parseQATestCaseMixedCoverageTargetPatchText,
  prepareQATestCaseMixedCoverageTargetPatch,
  QATestCaseDraftCompilationError,
  type QATestCaseDraft,
  type QATestCaseSkillInvocation,
} from '../../index.js';
import {
  assertCompleteTestCaseFinishReason,
  TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS,
  TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS,
  TEST_CASE_REPAIR_MIN_REMAINING_MS,
  usesMissingFinishReasonFallback,
  type GenerateTestCaseModelText,
  type TestCaseModelTextResult,
} from '../runtime/generate-runtime.js';
import { combineTokenUsages } from '../runtime/generate-telemetry.js';
import { testCaseRepairStepError, type FullDraftRepairAttempt } from '../runtime/generate-errors.js';
import { inspectQATestCaseDraftText } from './generate-draft-repair.js';

export function hasEligibleMixedCoverageTargetPatch(
  input: Record<string, unknown>,
  draft: QATestCaseDraft,
  failure: QATestCaseDraftCompilationError,
): boolean {
  return prepareQATestCaseMixedCoverageTargetPatch(input, draft, failure).eligible;
}

export async function repairQATestCaseDraftMixedCoverageTarget(options: {
  input: Record<string, unknown>;
  allowedRefs: readonly string[];
  draft: QATestCaseDraft;
  failure: QATestCaseDraftCompilationError;
  abortSignal: AbortSignal;
  remainingMs: () => number;
  generate: GenerateTestCaseModelText;
  usage: unknown[];
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
}): Promise<{
  output: AIOutput;
  usage: unknown;
  repaired: true;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  coveragePatched: true;
}> {
  const eligibility = prepareQATestCaseMixedCoverageTargetPatch(
    options.input, options.draft, options.failure,
  );
  if (!eligibility.eligible) {
    throw testCaseRepairStepError(options.failure, 'full_draft', 'compile');
  }
  const diagnosticCount = eligibility.riskDiagnostics.length
    + eligibility.targetDiagnostics.length;
  if (options.remainingMs() < TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS) {
    throw testCaseRepairStepError(
      options.failure, 'coverage', 'budget', undefined, diagnosticCount,
    );
  }

  let invocation: QATestCaseSkillInvocation;
  try {
    invocation = invokeQATestCaseMixedCoverageTargetPatchSkill({
      input: options.input,
      allowedRefs: options.allowedRefs,
      draft: eligibility.draft,
      riskDiagnostics: eligibility.riskDiagnostics,
      targetDiagnostics: eligibility.targetDiagnostics,
      eligibleTargetCoverageSpecs: eligibility.eligibleTargetCoverageSpecs,
    });
  } catch (error) {
    throw testCaseRepairStepError(error, 'coverage', 'invoke', undefined, diagnosticCount);
  }

  let generated: TestCaseModelTextResult;
  try {
    generated = await options.generate({
      instructions: invocation.instructions,
      prompt: invocation.prompt,
      abortSignal: options.abortSignal,
      maxOutputTokens: TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS,
      telemetryKind: 'draft_coverage_patch',
    });
  } catch (error) {
    throw testCaseRepairStepError(error, 'coverage', 'generation', undefined, diagnosticCount);
  }
  const finishReasonFallback = usesMissingFinishReasonFallback(generated);
  if (!finishReasonFallback) {
    try {
      assertCompleteTestCaseFinishReason(generated.finishReason, 'repair', generated);
    } catch (error) {
      throw testCaseRepairStepError(error, 'coverage', 'finish', generated, diagnosticCount);
    }
  }

  let patch;
  try {
    patch = parseQATestCaseMixedCoverageTargetPatchText(generated.text);
  } catch (error) {
    throw testCaseRepairStepError(error, 'coverage', 'parse', generated, diagnosticCount);
  }
  let repairedDraft: QATestCaseDraft;
  try {
    repairedDraft = applyQATestCaseMixedCoverageTargetPatch(
      options.input, eligibility.draft, options.failure, patch,
    );
  } catch (error) {
    throw testCaseRepairStepError(error, 'coverage', 'apply', generated, diagnosticCount);
  }
  let output: AIOutput;
  try {
    output = compileQATestCaseDraft(options.input, repairedDraft);
  } catch (error) {
    throw testCaseRepairStepError(error, 'coverage', 'compile', generated, diagnosticCount);
  }
  return {
    output,
    usage: combineTokenUsages(...options.usage, generated.usage),
    repaired: true,
    repairReason: options.repairReason,
    finishReasonFallback: options.finishReasonFallback || finishReasonFallback,
    coveragePatched: true,
  };
}

export async function repairQATestCaseDraftFinal(options: {
  input: Record<string, unknown>;
  allowedRefs: readonly string[];
  instructions: string;
  prompt: string;
  draft: QATestCaseDraft;
  failure: QATestCaseDraftCompilationError;
  abortSignal: AbortSignal;
  remainingMs: () => number;
  generate: GenerateTestCaseModelText;
  usage: unknown[];
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  fullDraftRepairAttempt: FullDraftRepairAttempt;
}): Promise<{
  output: AIOutput;
  usage: unknown;
  repaired: true;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  coveragePatched: false;
}> {
  if (options.remainingMs() < TEST_CASE_REPAIR_MIN_REMAINING_MS) {
    throw testCaseRepairStepError(
      options.failure, 'full_draft', 'budget', undefined, undefined, undefined,
      options.fullDraftRepairAttempt,
    );
  }
  let generated: TestCaseModelTextResult;
  try {
    generated = await options.generate({
      instructions: options.instructions,
      messages: [
        { role: 'user', content: options.prompt },
        { role: 'assistant', content: JSON.stringify(options.draft) },
        {
          role: 'user',
          content: buildQATestCaseDraftFinalRepairInstruction(
            options.failure.issues, options.allowedRefs, options.input,
          ),
        },
      ],
      abortSignal: options.abortSignal,
      telemetryKind: 'draft_full_repair',
    });
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'full_draft', 'generation', undefined, undefined, undefined,
      options.fullDraftRepairAttempt,
    );
  }
  const finishReasonFallback = usesMissingFinishReasonFallback(generated);
  if (!finishReasonFallback) {
    try {
      assertCompleteTestCaseFinishReason(generated.finishReason, 'repair', generated);
    } catch (error) {
      throw testCaseRepairStepError(
        error, 'full_draft', 'finish', generated, undefined, undefined,
        options.fullDraftRepairAttempt,
      );
    }
  }
  const inspected = inspectQATestCaseDraftText(generated.text);
  if (!inspected.draft) {
    throw testCaseRepairStepError(
      new Error(inspected.issues.join('; ')), 'full_draft', 'parse', generated,
      undefined, undefined, options.fullDraftRepairAttempt,
    );
  }
  let output: AIOutput;
  try {
    output = compileQATestCaseDraft(options.input, inspected.draft);
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'full_draft', 'compile', generated, undefined, undefined,
      options.fullDraftRepairAttempt,
    );
  }
  return {
    output,
    usage: combineTokenUsages(...options.usage, generated.usage),
    repaired: true,
    repairReason: options.repairReason,
    finishReasonFallback: options.finishReasonFallback || finishReasonFallback,
    coveragePatched: false,
  };
}
