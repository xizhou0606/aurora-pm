// QA 测试用例能力(qa.test_cases)对外入口(原 src/qa-test-case*.ts 全族)。
// 公开符号与搬迁前保持一致,外部统一从本目录入口导入;
// qa-test-case-skill 技能加载/lock 校验/元数据保持单文件内聚。
export {
  applyTestCaseMechanicalRepairs,
  normalizeTestCaseGenerationOutput,
  parseTestCaseGenerationText,
} from './qa-test-case-normalize.js';
export {
  isObservableTestCaseExpectedResult,
  isReproducibleTestCaseSetup,
  testCaseDesignIssues,
} from './qa-test-case-issues.js';
export {
  QA_TEST_CASE_SKILL_NAME,
  QA_TEST_CASE_SKILL_REPOSITORY,
  QA_TEST_CASE_SKILL_REPOSITORY_PATH,
  buildFixedQATestCaseSkillInvocation,
  buildQATestCaseDraftFinalRepairInstruction,
  buildQATestCaseDraftRepairInstruction,
  buildQATestCaseDraftRequestPrompt,
  buildQATestCaseDraftSkillInvocation,
  buildQATestCaseRepairInstruction,
  buildQATestCaseRequestPrompt,
  buildQATestCaseSkillInvocation,
  invokeQATestCaseCoveragePatchSkill,
  invokeQATestCaseMixedCoverageTargetPatchSkill,
  loadFixedQATestCaseSkill,
  qaTestCaseSkillBundle,
  qaTestCaseSkillMetadata,
  requiresAdvancedQATestCaseRules,
} from './skill/qa-test-case-skill.js';
export type {
  FixedQATestCaseSkillBundle,
  QATestCaseCoveragePatchSkillInput,
  QATestCaseMixedCoverageTargetPatchSkillInput,
  QATestCaseSkillInvocation,
  QATestCaseSkillMetadata,
} from './skill/qa-test-case-skill.js';
export {
  allowedQATestCaseContextRefs,
  qaTestCaseCaseDraftSchema,
  qaTestCaseCoverageDecisionSchema,
  qaTestCaseDraftSchema,
  qaTestCaseRequirementGroupLedger,
  qaTestCaseRiskIdSchema,
} from './draft/fixtures/qa-test-case-draft-schema.js';
export type {
  QATestCaseCaseDraft,
  QATestCaseCoverageDecision,
  QATestCaseDraft,
  QATestCaseDraftContextReferenceDiagnostic,
  QATestCaseRequirementGroupLedgerEntry,
  QATestCaseTargetCoverageDiagnostic,
} from './draft/fixtures/qa-test-case-draft-schema.js';
export { compileQATestCaseDraft } from './draft/qa-test-case-draft-compile.js';
export { QATestCaseDraftCompilationError } from './draft/fixtures/qa-test-case-draft-error.js';
export {
  compileQATestCaseSourceAssessments,
  qaTestCaseRequiredCoverageTargets,
  qaTestCaseRequirementGroups,
  qaTestCaseSourceTargetRefs,
  qaTestCaseSourceTargets,
  resolveQATestCaseContext,
} from './draft/qa-test-case-draft-context.js';
export type {
  QATestCaseRequiredCoverageTarget,
  QATestCaseSourceAssessment,
  QATestCaseSourceTarget,
  ResolvedQATestCaseRef,
} from './draft/qa-test-case-draft-context.js';
export { compileRiskCoverageDecisions } from './draft/qa-test-case-draft-risk.js';
export type {
  CoreRiskCoverageCategory,
  RiskCoverageDiagnostic,
} from './draft/qa-test-case-draft-risk.js';
export {
  QA_TEST_CASE_COVERAGE_PATCH_REASON_SENTINEL,
  QA_TEST_CASE_COVERAGE_PATCH_REPLACEMENT_SENTINEL,
  QA_TEST_CASE_MIXED_COVERAGE_TARGET_PATCH_REPLACEMENT_SENTINEL,
  qaTestCaseCoveragePatchSchema,
  qaTestCaseMixedCoverageTargetPatchSchema,
  QATestCaseCoveragePatchError,
} from './coverage-patch/qa-test-case-coverage-patch-schema.js';
export type {
  QATestCaseCoveragePatch,
  QATestCaseEligibleTargetCoverageSpec,
  QATestCaseMixedCoverageTargetPatch,
  QATestCaseMixedCoverageTargetPatchEligibility,
} from './coverage-patch/qa-test-case-coverage-patch-schema.js';
export {
  applyQATestCaseCoveragePatch,
  parseQATestCaseCoveragePatchText,
  parseQATestCaseMixedCoverageTargetPatchText,
  prepareQATestCaseCoveragePatch,
} from './coverage-patch/qa-test-case-coverage-patch.js';
export type { PreparedQATestCaseCoveragePatch } from './coverage-patch/qa-test-case-coverage-patch.js';
export {
  applyQATestCaseMixedCoverageTargetPatch,
  prepareQATestCaseMixedCoverageTargetPatch,
} from './coverage-patch/qa-test-case-mixed-coverage-patch.js';
export {
  QA_TEST_CASE_BATCH_MAX_BYTES,
  QA_TEST_CASE_BATCH_MAX_ITEMS,
  QA_TEST_CASE_MAX_BATCHES,
  QA_TEST_CASE_MAX_ITEMS,
  partitionQATestCaseInput,
} from './batching/qa-test-case-batching-partition.js';
export type {
  QATestCaseBatch,
  QATestCaseBatchOutput,
} from './batching/qa-test-case-batching-input.js';
export { mergeQATestCaseBatchOutputs } from './batching/qa-test-case-batching-merge.js';
export {
  QA_TEST_CASE_GLOBAL_PLAN_REPLACEMENT_SENTINEL,
  qaTestCaseGlobalPlanSchema,
} from './global-plan/qa-test-case-global-plan-schema.js';
export type {
  CompiledQATestCaseGlobalPlan,
  QATestCaseGlobalPlan,
  QATestCaseGlobalPlanRefDiagnostic,
} from './global-plan/qa-test-case-global-plan-schema.js';
export {
  compileQATestCaseGlobalPlan,
  QATestCaseGlobalPlanCompilationError,
} from './global-plan/qa-test-case-global-plan.js';
export {
  buildQATestCaseGlobalPlanRepairInstruction,
  buildQATestCaseGlobalPlanSkillInvocation,
} from './global-plan/prompts/qa-test-case-global-plan-prompt.js';
export {
  QA_TEST_CASE_GLOBAL_PLAN_REF_PATCH_REPLACEMENT_SENTINEL,
  buildQATestCaseGlobalPlanRefPatchSkillInvocation,
} from './global-plan/prompts/qa-test-case-global-plan-ref-patch-prompt.js';
export type { QATestCaseGlobalPlanRefPatchSkillInput } from './global-plan/prompts/qa-test-case-global-plan-ref-patch-prompt.js';
export {
  applyQATestCaseGlobalPlanRefPatch,
  parseQATestCaseGlobalPlanRefPatchText,
  qaTestCaseGlobalPlanRefPatchSchema,
  resolveQATestCaseGlobalPlanRefPatchTargets,
  QATestCaseGlobalPlanRefPatchError,
} from './global-plan/qa-test-case-global-plan-ref-patch.js';
export type {
  QATestCaseGlobalPlanRefPatch,
  QATestCaseGlobalPlanRefPatchTarget,
} from './global-plan/qa-test-case-global-plan-ref-patch.js';
