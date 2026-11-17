import { ZodError } from 'zod';
import type { TestCaseRepairReason } from '../../../../shared/ai/types.js';
import {
  applyQATestCaseGlobalPlanRefPatch,
  buildQATestCaseGlobalPlanRefPatchSkillInvocation,
  buildQATestCaseGlobalPlanRepairInstruction,
  compileQATestCaseGlobalPlan,
  parseQATestCaseGlobalPlanRefPatchText,
  qaTestCaseGlobalPlanSchema,
  QATestCaseGlobalPlanCompilationError,
  type CompiledQATestCaseGlobalPlan,
  type QATestCaseGlobalPlan,
  type QATestCaseSkillInvocation,
} from '../../index.js';
import {
  assertCompleteTestCaseFinishReason,
  incompleteTestCaseRewriteInstruction,
  TEST_CASE_GLOBAL_REF_PATCH_MAX_OUTPUT_TOKENS,
  TEST_CASE_GLOBAL_REF_PATCH_MIN_REMAINING_MS,
  TEST_CASE_REPAIR_MIN_REMAINING_MS,
  usesMissingFinishReasonFallback,
  type GenerateTestCaseModelText,
  type TestCaseModelTextResult,
} from '../runtime/generate-runtime.js';
import { combineTokenUsages } from '../runtime/generate-telemetry.js';
import { testCaseRepairStepError } from '../runtime/generate-errors.js';
import { safeZodStructureIssue } from '../pipeline/generate-inspect.js';

export type TestCaseGlobalPlanInspection =
  | { plan: QATestCaseGlobalPlan; failure: null; issues: [] }
  | { plan: null; failure: 'parse' | 'structure'; issues: string[] };

export function inspectQATestCaseGlobalPlanText(text: string): TestCaseGlobalPlanInspection {
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
  let lastError: unknown = new SyntaxError('test case global plan did not contain JSON');
  for (const candidate of candidates) {
    try {
      return { plan: qaTestCaseGlobalPlanSchema.parse(JSON.parse(candidate)), failure: null, issues: [] };
    } catch (error) {
      lastError = error;
    }
  }
  if (lastError instanceof ZodError) {
    return {
      plan: null,
      failure: 'structure',
      issues: [...new Set(lastError.issues.map(safeZodStructureIssue))],
    };
  }
  return {
    plan: null,
    failure: 'parse',
    issues: ['test case Global Plan must be one complete valid JSON object'],
  };
}

export function hasOnlyGlobalPlanRefDiagnostics(
  error: QATestCaseGlobalPlanCompilationError,
): boolean {
  if (!error.diagnostics.length) return false;
  const messages = new Set(error.diagnostics.map((diagnostic) => diagnostic.message));
  return new Set(error.diagnostics.map((diagnostic) => diagnostic.path)).size
      === error.diagnostics.length
    && error.issues.every((issue) => messages.has(issue));
}

export async function repairQATestCaseGlobalPlanRefs(options: {
  input: Record<string, unknown>;
  allowedRefs: readonly string[];
  plan: QATestCaseGlobalPlan;
  failure: QATestCaseGlobalPlanCompilationError;
  abortSignal: AbortSignal;
  remainingMs: () => number;
  generate: GenerateTestCaseModelText;
  usage: unknown[];
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  previousResult?: TestCaseModelTextResult;
}): Promise<{
  plan: CompiledQATestCaseGlobalPlan;
  usage: unknown;
  repaired: true;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  globalRefPatched: true;
}> {
  if (!hasOnlyGlobalPlanRefDiagnostics(options.failure)) {
    throw testCaseRepairStepError(
      options.failure, 'global_plan', 'compile', options.previousResult,
    );
  }
  const diagnosticCount = options.failure.diagnostics.length;
  if (options.remainingMs() < TEST_CASE_GLOBAL_REF_PATCH_MIN_REMAINING_MS) {
    throw testCaseRepairStepError(
      options.failure,
      'global_ref_patch',
      'budget',
      options.previousResult,
      undefined,
      diagnosticCount,
    );
  }
  let invocation: QATestCaseSkillInvocation;
  try {
    invocation = buildQATestCaseGlobalPlanRefPatchSkillInvocation({
      input: options.input,
      plan: options.plan,
      allowedRefs: options.allowedRefs,
      diagnostics: options.failure.diagnostics,
    });
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'global_ref_patch', 'invoke', undefined, undefined, diagnosticCount,
    );
  }
  let generated: TestCaseModelTextResult;
  try {
    generated = await options.generate({
      instructions: invocation.instructions,
      prompt: invocation.prompt,
      abortSignal: options.abortSignal,
      maxOutputTokens: TEST_CASE_GLOBAL_REF_PATCH_MAX_OUTPUT_TOKENS,
      telemetryKind: 'global_ref_patch',
    });
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'global_ref_patch', 'generation', undefined, undefined, diagnosticCount,
    );
  }
  const finishReasonFallback = usesMissingFinishReasonFallback(generated);
  if (!finishReasonFallback) {
    try {
      assertCompleteTestCaseFinishReason(generated.finishReason, 'repair', generated);
    } catch (error) {
      throw testCaseRepairStepError(
        error, 'global_ref_patch', 'finish', generated, undefined, diagnosticCount,
      );
    }
  }
  let patch;
  try {
    patch = parseQATestCaseGlobalPlanRefPatchText(generated.text);
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'global_ref_patch', 'parse', generated, undefined, diagnosticCount,
    );
  }
  let repairedPlan: QATestCaseGlobalPlan;
  try {
    repairedPlan = applyQATestCaseGlobalPlanRefPatch(
      options.plan,
      options.failure.diagnostics,
      options.allowedRefs,
      patch,
    );
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'global_ref_patch', 'apply', generated, undefined, diagnosticCount,
    );
  }
  let compiled: CompiledQATestCaseGlobalPlan;
  try {
    compiled = compileQATestCaseGlobalPlan(options.input, repairedPlan);
  } catch (error) {
    throw testCaseRepairStepError(
      error, 'global_ref_patch', 'compile', generated, undefined, diagnosticCount,
    );
  }
  return {
    plan: compiled,
    usage: combineTokenUsages(...options.usage, generated.usage),
    repaired: true,
    repairReason: options.repairReason,
    finishReasonFallback: options.finishReasonFallback || finishReasonFallback,
    globalRefPatched: true,
  };
}

export async function repairQATestCaseGlobalPlanFinal(options: {
  input: Record<string, unknown>;
  allowedRefs: readonly string[];
  instructions: string;
  prompt: string;
  abortSignal: AbortSignal;
  remainingMs: () => number;
  generate: GenerateTestCaseModelText;
  first: TestCaseModelTextResult;
  second: TestCaseModelTextResult;
  plan?: QATestCaseGlobalPlan;
  issues: string[];
  cause: unknown;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  fromScratch: boolean;
}): Promise<{
  plan: CompiledQATestCaseGlobalPlan;
  usage: unknown;
  repaired: true;
  repairReason: TestCaseRepairReason;
  finishReasonFallback: boolean;
  globalRefPatched: false;
}> {
  if (options.remainingMs() < TEST_CASE_REPAIR_MIN_REMAINING_MS) {
    throw testCaseRepairStepError(
      options.cause, 'global_plan', 'budget', options.second,
    );
  }
  let generated: TestCaseModelTextResult;
  try {
    generated = await options.generate({
      instructions: options.instructions,
      ...(options.fromScratch
        ? {
            prompt: [
              options.prompt,
              incompleteTestCaseRewriteInstruction('Global Plan'),
            ].join('\n'),
          }
        : {
            messages: [
              { role: 'user' as const, content: options.prompt },
              {
                role: 'assistant' as const,
                content: options.plan ? JSON.stringify(options.plan) : options.second.text,
              },
              {
                role: 'user' as const,
                content: buildQATestCaseGlobalPlanRepairInstruction(
                  options.issues, options.allowedRefs,
                ),
              },
            ],
          }),
      abortSignal: options.abortSignal,
      telemetryKind: 'global_full_repair',
    });
  } catch (error) {
    throw testCaseRepairStepError(error, 'global_plan', 'generation');
  }
  const finishReasonFallback = usesMissingFinishReasonFallback(generated);
  if (!finishReasonFallback) {
    try {
      assertCompleteTestCaseFinishReason(generated.finishReason, 'repair', generated);
    } catch (error) {
      throw testCaseRepairStepError(error, 'global_plan', 'finish', generated);
    }
  }
  const inspected = inspectQATestCaseGlobalPlanText(generated.text);
  if (!inspected.plan) {
    throw testCaseRepairStepError(
      new Error(inspected.issues.join('; ')), 'global_plan', 'parse', generated,
    );
  }
  let compiled: CompiledQATestCaseGlobalPlan;
  try {
    compiled = compileQATestCaseGlobalPlan(options.input, inspected.plan);
  } catch (error) {
    throw testCaseRepairStepError(error, 'global_plan', 'compile', generated);
  }
  return {
    plan: compiled,
    usage: combineTokenUsages(options.first.usage, options.second.usage, generated.usage),
    repaired: true,
    repairReason: options.repairReason,
    finishReasonFallback: options.finishReasonFallback || finishReasonFallback,
    globalRefPatched: false,
  };
}
