import { z } from 'zod';
import { qaTestCaseRiskIdSchema } from '../draft/fixtures/qa-test-case-draft-schema.js';
import type {
  QATestCaseDraft,
  QATestCaseTargetCoverageDiagnostic,
} from '../draft/fixtures/qa-test-case-draft-schema.js';
import type {
  CoreRiskCoverageCategory,
  RiskCoverageDiagnostic,
} from '../draft/qa-test-case-draft-risk.js';
import { isObservableTestCaseExpectedResult } from '../qa-test-case-issues.js';

export const QA_TEST_CASE_COVERAGE_PATCH_REASON_SENTINEL =
  '根据业务输入填写未补齐核心类别的具体事实原因';
export const QA_TEST_CASE_COVERAGE_PATCH_REPLACEMENT_SENTINEL =
  '__QA_COVERAGE_PATCH_REPLACE_FROM_INPUT__';
export const QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL =
  '__QA_MIXED_COVERAGE_TARGET_PATCH_REPLACE_FROM_INPUT__';

const coreCategorySchema = z.enum(['positive', 'negative', 'boundary']);
const nonCoreTargetCategorySchema = z.enum([
  'permission', 'state', 'duplicate_submit', 'concurrency', 'timeout', 'retry',
  'rollback', 'stale_data', 'regression',
]);
const compactText = z.string().trim().min(1);
const contextRefSchema = z.string().regex(
  /^(?:subject:(?:title|description)|source:(?:0|[1-9]\d*):content|requirement:(?:0|[1-9]\d*):(?:title|description|acceptance_criteria)|task:(?:0|[1-9]\d*):(?:title|description|status))$/,
  'context ref must select a canonical substantive input field',
);
const observableExpectedResult = compactText.max(500).refine(
  isObservableTestCaseExpectedResult,
  { message: 'expected result must be observable' },
);

const caseResolutionSchema = z.object({
  category: coreCategorySchema,
  kind: z.literal('case'),
  contextRef: contextRefSchema,
  title: compactText.max(200),
  step: compactText.max(500),
  expectedResult: observableExpectedResult,
}).strict();
const omissionResolutionSchema = z.object({
  category: coreCategorySchema,
  kind: z.literal('omit'),
}).strict();
const resolutionSchema = z.discriminatedUnion('kind', [
  caseResolutionSchema,
  omissionResolutionSchema,
]);
const repairSchema = z.object({
  riskId: qaTestCaseRiskIdSchema,
  resolutions: z.array(resolutionSchema).min(1).max(3),
  omissionReason: compactText.max(1000).nullable(),
}).strict();

const targetRefSchema = z.string().regex(
  /^(?:requirement|task):(0|[1-9]\d*)$/,
  'target ref must select a requirement or task item',
);
const targetRepairSchema = z.object({
  targetRef: targetRefSchema,
  riskId: qaTestCaseRiskIdSchema,
  contextRef: contextRefSchema,
  category: nonCoreTargetCategorySchema,
  title: compactText.max(200),
  step: compactText.max(500),
  expectedResult: observableExpectedResult,
}).strict();

export const qaTestCaseCoveragePatchSchema = z.object({
  repairs: z.array(repairSchema).min(1),
}).strict();

export const qaTestCaseMixedCoverageTargetPatchSchema = z.object({
  riskRepairs: z.array(repairSchema).min(1),
  targetRepairs: z.array(targetRepairSchema).min(1),
}).strict();

export type QATestCaseCoveragePatch = z.infer<typeof qaTestCaseCoveragePatchSchema>;
export type QATestCaseMixedCoverageTargetPatch = z.infer<
  typeof qaTestCaseMixedCoverageTargetPatchSchema
>;
export type QATestCaseEligibleTargetCoverageSpec = {
  targetRef: string;
  riskId: string;
  contextRef: string;
};

export type QATestCaseMixedCoverageTargetPatchEligibility =
  | {
    eligible: true;
    draft: QATestCaseDraft;
    riskDiagnostics: RiskCoverageDiagnostic[];
    targetDiagnostics: QATestCaseTargetCoverageDiagnostic[];
    eligibleTargetCoverageSpecs: QATestCaseEligibleTargetCoverageSpec[];
  }
  | {
    eligible: false;
    reason:
      | 'not_pure_mixed_coverage_failure'
      | 'no_pending_risk_coverage_repair'
      | 'no_safe_target_coverage_association';
    eligibleTargetCoverageSpecs: [];
  };

export class QATestCaseCoveragePatchError extends Error {
  readonly issues: string[];

  constructor(issues: string[]) {
    const distinctIssues = [...new Set(issues)];
    super(`qa test case coverage patch failed: ${distinctIssues.join('; ')}`);
    this.name = 'QATestCaseCoveragePatchError';
    this.issues = distinctIssues;
  }
}

export const coreCategories: CoreRiskCoverageCategory[] = ['positive', 'negative', 'boundary'];

export function sameValues(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

export function expectedStatus(
  priority: QATestCaseDraft['risks'][number]['priority'],
  missing: readonly CoreRiskCoverageCategory[],
): RiskCoverageDiagnostic['expectedStatus'] {
  if (priority === 'P2' || priority === 'P3') return 'not_required';
  return missing.length ? 'omitted' : 'complete';
}

export function containsCoveragePatchSentinel(value: unknown): boolean {
  if (typeof value === 'string') {
    return value.includes(QA_TEST_CASE_COVERAGE_PATCH_REASON_SENTINEL)
      || value.includes(QA_TEST_CASE_COVERAGE_PATCH_REPLACEMENT_SENTINEL)
      || value.includes(QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL);
  }
  if (Array.isArray(value)) return value.some(containsCoveragePatchSentinel);
  if (value && typeof value === 'object') {
    return Object.values(value).some(containsCoveragePatchSentinel);
  }
  return false;
}
