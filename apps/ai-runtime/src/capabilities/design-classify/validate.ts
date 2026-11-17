import type { AIOutput } from '../../shared/ai/schema.js';

// requirement.design_classify 的确定性契约校验(原 validateCapabilityOutput
// 内 design_classify 分支,逐字迁移)。
export function validateDesignClassificationOutput(
  _input: Record<string, unknown>,
  output: AIOutput,
): void {
  if (output.changes.length || output.testCases.length || output.completenessReport || output.workflowProposal || !output.humanReviewRequired) {
    throw new Error('requirement design classification violated the analysis-only contract');
  }
}
