import { aiOutputSchema, type AIOutput } from '../../../../shared/ai/schema.js';
import { qaTestCaseRequirementGroupLedger, type QATestCaseSkillInvocation } from '../../index.js';

export function qaTestCaseFormalRequirementGroupCount(input: Record<string, unknown>): number {
  return new Set(
    qaTestCaseRequirementGroupLedger(input)
      .map((entry) => entry.group)
      .filter((group) => group.startsWith('requirement:')),
  ).size;
}

export function shouldGenerateQATestCaseGlobalPlan(
  input: Record<string, unknown>,
  batchCount: number,
): boolean {
  if (batchCount <= 1) return false;
  // Capacity sharding can split one requirement across several Draft batches.
  // Global Plan is reserved for interactions between formal requirements.
  return qaTestCaseFormalRequirementGroupCount(input) > 1;
}

export function attachTestCaseSkillProvenance(
  output: AIOutput,
  skill: QATestCaseSkillInvocation['skill'],
): AIOutput {
  return aiOutputSchema.parse({ ...output, skillProvenance: skill });
}
