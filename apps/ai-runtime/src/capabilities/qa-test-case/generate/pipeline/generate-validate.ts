import type { AIOutput } from '../../../../shared/ai/schema.js';
import { testCaseDesignIssues } from '../../index.js';

// qa.test_cases 的确定性契约校验(原 validateCapabilityOutput 内 qa 分支,逐字迁移)。
export function validateQATestCaseOutput(input: Record<string, unknown>, output: AIOutput): void {
  if (!output.testCases.length) throw new Error('test case output is empty');
  if (output.changes.length || output.items.length || output.risks.length || output.completenessReport || output.workflowProposal || !output.humanReviewRequired) {
    throw new Error('test case output violated the artifact-only contract');
  }
  const issues = testCaseDesignIssues(input, output);
  if (issues.length) throw new Error(issues.join('; '));
}
