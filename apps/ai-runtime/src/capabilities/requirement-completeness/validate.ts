import type { AIOutput } from '../../shared/ai/schema.js';

// requirement.completeness 的确定性契约校验(原 validateCapabilityOutput 内
// completeness 分支,逐字迁移;不含前置的 capability 早退判断)。
export function validateCompletenessOutput(input: Record<string, unknown>, output: AIOutput): void {
  const subject = input.subject as Record<string, unknown> | undefined;
  const version = typeof subject?.version === 'number' ? subject.version : 0;
  const report = output.completenessReport;
  if (!report) throw new Error('requirement completeness output is missing the strict report');
  if (version <= 0 || report.checkedSubjectVersion !== version) throw new Error('requirement completeness report used a stale subject version');
  if (output.changes.length || output.workflowProposal || !output.humanReviewRequired) throw new Error('requirement completeness output violated the analysis-only contract');
  if (JSON.stringify(output.risks) !== JSON.stringify(report.blockingIssues)) throw new Error('requirement completeness risks must match blocking issues');
}
