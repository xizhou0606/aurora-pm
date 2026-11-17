import type { AIOutput } from '../../shared/ai/schema.js';
import { parseWorkflowOutput } from './normalize.js';
import { workflowProposalIssues } from './issues.js';

export type WorkflowOutputInspection = {
  output: AIOutput | null;
  issues: string[];
};

export function inspectWorkflowOutput(text: string, subjectType = 'requirement', currentDefinition?: unknown): WorkflowOutputInspection {
  try {
    const output = parseWorkflowOutput(text, subjectType, currentDefinition);
    return {
      output,
      issues: output.workflowProposal
        ? workflowProposalIssues(output.workflowProposal.definition, currentDefinition, subjectType)
        : ['未生成工作流提案'],
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return {
      output: null,
      issues: [`工作流输出无法解析：${message.slice(0, 500)}`],
    };
  }
}

type WorkflowTextResult = { text: string; usage: unknown };

export async function validateAndRepairWorkflowOutput(
  initialResult: WorkflowTextResult,
  subjectType: string,
  repair: (issues: string[], previousOutput: string) => Promise<WorkflowTextResult>,
  currentDefinition?: unknown,
): Promise<{ output: AIOutput; usage: unknown }> {
  let result = initialResult;
  let inspection = inspectWorkflowOutput(result.text, subjectType, currentDefinition);
  if (inspection.issues.length) {
    result = await repair(inspection.issues, result.text);
    inspection = inspectWorkflowOutput(result.text, subjectType, currentDefinition);
  }
  if (!inspection.output || inspection.issues.length) {
    throw new Error(`workflow proposal failed structural validation: ${inspection.issues.join('；')}`);
  }
  return { output: inspection.output, usage: result.usage };
}
