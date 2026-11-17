import { ZodError } from 'zod';
import type { AIOutput } from '../../../../shared/ai/schema.js';
import type { TestCaseRepairReason } from '../../../../shared/ai/types.js';
import {
  applyQATestCaseCoveragePatch,
  compileQATestCaseDraft,
  invokeQATestCaseCoveragePatchSkill,
  parseQATestCaseCoveragePatchText,
  prepareQATestCaseCoveragePatch,
  qaTestCaseDraftSchema,
  QATestCaseDraftCompilationError,
  type QATestCaseDraft,
  type QATestCaseSkillInvocation,
} from '../../index.js';
import {
  assertCompleteTestCaseFinishReason,
  TEST_CASE_COVERAGE_PATCH_MAX_OUTPUT_TOKENS,
  TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS,
  usesMissingFinishReasonFallback,
  type GenerateTestCaseModelText,
  type TestCaseModelTextResult,
} from '../runtime/generate-runtime.js';
import { combineTokenUsages } from '../runtime/generate-telemetry.js';
import { testCaseRepairStepError } from '../runtime/generate-errors.js';
import { safeZodStructureIssue } from '../pipeline/generate-inspect.js';

export type TestCaseDraftInspection =
  | { draft: QATestCaseDraft; failure: null; issues: [] }
  | { draft: null; failure: 'parse' | 'structure'; issues: string[] };

export function inspectQATestCaseDraftText(text: string): TestCaseDraftInspection {
  const withoutThinking = text.trim()
    .replace(/<think>[\s\S]*?<\/think>/gi, '')
    .replace(/```(?:json)?/gi, '')
    .replace(/```/g, '')
    .trim();
  const fenced = [...text.matchAll(/```(?:json)?\s*([\s\S]*?)```/gi)]
    .map((match) => match[1].trim());
  const objectStart = withoutThinking.indexOf('{');
  const objectEnd = withoutThinking.lastIndexOf('}');
  const candidates = [
    withoutThinking,
    ...fenced,
    objectStart >= 0 && objectEnd > objectStart ? withoutThinking.slice(objectStart, objectEnd + 1) : '',
  ].filter((candidate, index, all) => Boolean(candidate) && all.indexOf(candidate) === index);
  let lastError: unknown = new SyntaxError('test case draft did not contain JSON');
  for (const candidate of candidates) {
    try {
      return { draft: qaTestCaseDraftSchema.parse(JSON.parse(candidate)), failure: null, issues: [] };
    } catch (error) {
      lastError = error;
    }
  }
  if (lastError instanceof ZodError) {
    return {
      draft: null,
      failure: 'structure',
      issues: [...new Set(lastError.issues.map(safeZodStructureIssue))],
    };
  }
  return {
    draft: null,
    failure: 'parse',
    issues: ['test case Draft IR must be one complete valid JSON object'],
  };
}

export function hasOnlyRiskCoverageDiagnostics(error: QATestCaseDraftCompilationError): boolean {
  if (!error.diagnostics.length) return false;
  const diagnosticMessages = new Set(error.diagnostics.map((diagnostic) => diagnostic.message));
  return error.issues.every((issue) => diagnosticMessages.has(issue));
}

export async function repairQATestCaseDraftCoverage(options: {
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
  if (!hasOnlyRiskCoverageDiagnostics(options.failure)) {
    throw testCaseRepairStepError(options.failure, 'full_draft', 'compile');
  }
  let prepared;
  try {
    prepared = prepareQATestCaseCoveragePatch(options.draft, options.failure.diagnostics);
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'coverage', 'apply', undefined, options.failure.diagnostics.length,
    );
  }
  if (!prepared.pendingDiagnostics.length) {
    let output: AIOutput;
    try {
      output = compileQATestCaseDraft(options.input, prepared.draft);
    } catch (error) {
      throw testCaseRepairStepError(
        error, 'coverage', 'compile', undefined, options.failure.diagnostics.length,
      );
    }
    return {
      output,
      usage: combineTokenUsages(...options.usage),
      repaired: true,
      repairReason: options.repairReason,
      finishReasonFallback: options.finishReasonFallback,
      coveragePatched: true,
    };
  }
  if (options.remainingMs() < TEST_CASE_COVERAGE_PATCH_MIN_REMAINING_MS) {
    throw testCaseRepairStepError(
      options.failure,
      'coverage',
      'budget',
      undefined,
      prepared.pendingDiagnostics.length,
    );
  }
  const diagnosticCount = prepared.pendingDiagnostics.length;
  let invocation: QATestCaseSkillInvocation;
  try {
    invocation = invokeQATestCaseCoveragePatchSkill({
      input: options.input,
      allowedRefs: options.allowedRefs,
      draft: prepared.draft,
      diagnostics: prepared.pendingDiagnostics,
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
    patch = parseQATestCaseCoveragePatchText(generated.text);
  } catch (error) {
    throw testCaseRepairStepError(error, 'coverage', 'parse', generated, diagnosticCount);
  }
  let repairedDraft: QATestCaseDraft;
  try {
    repairedDraft = applyQATestCaseCoveragePatch(
      prepared.draft,
      prepared.pendingDiagnostics,
      patch,
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
