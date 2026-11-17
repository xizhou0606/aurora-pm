import type { AIOutput } from '../../shared/ai/schema.js';

// requirement.task_plan 的确定性契约校验(原 validateCapabilityOutput 内
// task_plan 分支,逐字迁移)。
export function validateTaskPlanOutput(
  _input: Record<string, unknown>,
  output: AIOutput,
): void {
  if (!output.taskPlan) throw new Error('requirement task plan is missing');
  const lanes = new Set(output.taskPlan.chains.map((chain) => chain.lane));
  if (lanes.size !== 2 || !lanes.has('frontend') || !lanes.has('backend')) {
    throw new Error('requirement task plan must contain frontend and backend chains');
  }
  if (output.changes.length || output.testCases.length || output.risks.length || output.completenessReport || output.workflowProposal || !output.humanReviewRequired) {
    throw new Error('requirement task plan violated the auto-create contract');
  }
}
