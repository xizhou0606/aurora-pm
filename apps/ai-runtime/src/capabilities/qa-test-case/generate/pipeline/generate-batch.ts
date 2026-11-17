import type { AIOutput } from '../../../../shared/ai/schema.js';
import {
  assertCompleteTestCaseFinishReason,
  TEST_CASE_REPAIR_MIN_REMAINING_MS,
  type GenerateTestCaseModelText,
} from '../runtime/generate-runtime.js';
import { combineTokenUsage } from '../runtime/generate-telemetry.js';
import {
  buildQATestCaseJSONRewriteInstruction,
  inspectTestCaseGenerationText,
  invalidTestCaseJSONMessage,
  parseCompleteTestCaseOutput,
  testCaseRepairSource,
} from './generate-inspect.js';
import { applyTestCaseMechanicalRepairs, buildQATestCaseRepairInstruction } from '../../index.js';
import { validateQATestCaseOutput } from './generate-validate.js';

export async function generateAndValidateTestCaseBatch(options: {
  input: Record<string, unknown>;
  instructions: string;
  prompt: string;
  abortSignal: AbortSignal;
  remainingMs: () => number;
  generate: GenerateTestCaseModelText;
}): Promise<{
  output: AIOutput;
  usage: unknown;
  repaired: boolean;
  repairReason: 'parse' | 'structure' | 'validation' | null;
}> {
  const initial = await options.generate({
    instructions: options.instructions,
    prompt: options.prompt,
    abortSignal: options.abortSignal,
    telemetryKind: 'draft_initial',
  });
  assertCompleteTestCaseFinishReason(initial.finishReason, 'generation', initial);

  let output: AIOutput | undefined;
  let repairReason: 'parse' | 'structure' | 'validation' | null = null;
  let validationIssues: string[] = [];
  const inspected = inspectTestCaseGenerationText(initial.text);
  if (inspected.output) {
    output = inspected.output;
  } else {
    repairReason = inspected.failure;
    validationIssues = inspected.issues;
  }

  if (output) {
    output = applyTestCaseMechanicalRepairs(options.input, output);
    try {
      validateQATestCaseOutput(options.input, output);
    } catch (error) {
      repairReason = 'validation';
      validationIssues = [error instanceof Error ? error.message : String(error)];
    }
    if (!repairReason) {
      return { output, usage: initial.usage, repaired: false, repairReason: null };
    }
  }

  if (options.remainingMs() < TEST_CASE_REPAIR_MIN_REMAINING_MS) {
    if (repairReason === 'parse') throw new Error(invalidTestCaseJSONMessage);
    throw new Error(validationIssues.join('; '));
  }

  const repair = await options.generate({
    instructions: options.instructions,
    messages: [
      { role: 'user', content: options.prompt },
      {
        role: 'assistant',
        content: output
          ? JSON.stringify(testCaseRepairSource(output))
          : initial.text,
      },
      {
        role: 'user',
        content: repairReason === 'parse'
          ? buildQATestCaseJSONRewriteInstruction()
          : buildQATestCaseRepairInstruction(validationIssues),
      },
    ],
    abortSignal: options.abortSignal,
    telemetryKind: 'draft_full_repair',
  });
  assertCompleteTestCaseFinishReason(repair.finishReason, 'repair', repair);

  output = parseCompleteTestCaseOutput(repair.text);
  output = applyTestCaseMechanicalRepairs(options.input, output);
  validateQATestCaseOutput(options.input, output);
  return {
    output,
    usage: combineTokenUsage(initial.usage, repair.usage),
    repaired: true,
    repairReason,
  };
}
