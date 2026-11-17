import { compileQATestCaseDraft } from '../draft/qa-test-case-draft-compile.js';
import { QATestCaseDraftCompilationError } from '../draft/fixtures/qa-test-case-draft-error.js';
import { qaTestCaseDraftSchema } from '../draft/fixtures/qa-test-case-draft-schema.js';
import type {
  QATestCaseDraft,
  QATestCaseTargetCoverageDiagnostic,
} from '../draft/fixtures/qa-test-case-draft-schema.js';
import type { RiskCoverageDiagnostic } from '../draft/qa-test-case-draft-risk.js';
import {
  applyQATestCaseCoveragePatch,
  prepareQATestCaseCoveragePatch,
  uniqueCoverageCaseTitle,
  validateRepair,
  type PreparedQATestCaseCoveragePatch,
} from './qa-test-case-coverage-patch.js';
import {
  containsCoveragePatchSentinel,
  qaTestCaseMixedCoverageTargetPatchSchema,
  QATestCaseCoveragePatchError,
  sameValues,
  type QATestCaseEligibleTargetCoverageSpec,
  type QATestCaseMixedCoverageTargetPatch,
  type QATestCaseMixedCoverageTargetPatchEligibility,
} from './qa-test-case-coverage-patch-schema.js';

function exactTargetCoverageDiagnostics(
  failure: QATestCaseDraftCompilationError,
): QATestCaseTargetCoverageDiagnostic[] | null {
  const diagnosticMessages = new Set([
    ...failure.diagnostics.map((diagnostic) => diagnostic.message),
    ...failure.targetCoverageDiagnostics.map((diagnostic) => diagnostic.message),
  ]);
  if (!failure.diagnostics.length || !failure.targetCoverageDiagnostics.length
    || failure.issues.length !== diagnosticMessages.size
    || failure.issues.some((issue) => !diagnosticMessages.has(issue))) {
    return null;
  }
  const targets = failure.targetCoverageDiagnostics.map((diagnostic) => ({
    ...diagnostic,
    candidateContextRefs: [...diagnostic.candidateContextRefs],
  }));
  if (new Set(targets.map((diagnostic) => diagnostic.targetRef)).size !== targets.length
    || targets.some((diagnostic) => !diagnostic.candidateContextRefs.length
      || new Set(diagnostic.candidateContextRefs).size !== diagnostic.candidateContextRefs.length)) {
    return null;
  }
  return targets;
}

function sameRiskDiagnostics(
  left: readonly RiskCoverageDiagnostic[],
  right: readonly RiskCoverageDiagnostic[],
): boolean {
  return JSON.stringify(left.map((diagnostic) => ({
    code: diagnostic.code,
    riskId: diagnostic.riskId,
    priority: diagnostic.priority,
    actualCategories: diagnostic.actualCategories,
    missingCategories: diagnostic.missingCategories,
    expectedStatus: diagnostic.expectedStatus,
    message: diagnostic.message,
  }))) === JSON.stringify(right.map((diagnostic) => ({
    code: diagnostic.code,
    riskId: diagnostic.riskId,
    priority: diagnostic.priority,
    actualCategories: diagnostic.actualCategories,
    missingCategories: diagnostic.missingCategories,
    expectedStatus: diagnostic.expectedStatus,
    message: diagnostic.message,
  })));
}

function sameTargetCoverageDiagnostics(
  left: readonly QATestCaseTargetCoverageDiagnostic[],
  right: readonly QATestCaseTargetCoverageDiagnostic[],
): boolean {
  return JSON.stringify(left.map((diagnostic) => ({
    code: diagnostic.code,
    targetRef: diagnostic.targetRef,
    candidateContextRefs: diagnostic.candidateContextRefs,
    group: diagnostic.group,
    message: diagnostic.message,
  }))) === JSON.stringify(right.map((diagnostic) => ({
    code: diagnostic.code,
    targetRef: diagnostic.targetRef,
    candidateContextRefs: diagnostic.candidateContextRefs,
    group: diagnostic.group,
    message: diagnostic.message,
  })));
}

/**
 * Prepares only the narrow, two-ledger repair shape. Any structural failure,
 * stale diagnostic, or missing exact risk/leaf association is ineligible rather
 * than guessed; callers must use their full Draft repair route instead.
 */
export function prepareQATestCaseMixedCoverageTargetPatch(
  input: Record<string, unknown>,
  draftValue: QATestCaseDraft,
  failure: QATestCaseDraftCompilationError,
): QATestCaseMixedCoverageTargetPatchEligibility {
  const targetDiagnostics = exactTargetCoverageDiagnostics(failure);
  if (!targetDiagnostics) {
    return {
      eligible: false,
      reason: 'not_pure_mixed_coverage_failure',
      eligibleTargetCoverageSpecs: [],
    };
  }

  let refreshedFailure: QATestCaseDraftCompilationError | undefined;
  try {
    compileQATestCaseDraft(input, draftValue);
  } catch (error) {
    if (error instanceof QATestCaseDraftCompilationError) refreshedFailure = error;
  }
  if (!refreshedFailure
    || !sameRiskDiagnostics(failure.diagnostics, refreshedFailure.diagnostics)
    || !sameTargetCoverageDiagnostics(
      targetDiagnostics,
      refreshedFailure.targetCoverageDiagnostics,
    )) {
    return {
      eligible: false,
      reason: 'not_pure_mixed_coverage_failure',
      eligibleTargetCoverageSpecs: [],
    };
  }

  let prepared: PreparedQATestCaseCoveragePatch;
  try {
    prepared = prepareQATestCaseCoveragePatch(draftValue, failure.diagnostics);
  } catch {
    return {
      eligible: false,
      reason: 'not_pure_mixed_coverage_failure',
      eligibleTargetCoverageSpecs: [],
    };
  }
  if (!prepared.pendingDiagnostics.length) {
    return {
      eligible: false,
      reason: 'no_pending_risk_coverage_repair',
      eligibleTargetCoverageSpecs: [],
    };
  }

  const riskCounts = new Map<string, number>();
  for (const risk of prepared.draft.risks) {
    riskCounts.set(risk.riskId, (riskCounts.get(risk.riskId) ?? 0) + 1);
  }
  const specs: QATestCaseEligibleTargetCoverageSpec[] = [];
  for (const target of targetDiagnostics) {
    for (const risk of prepared.draft.risks) {
      if (riskCounts.get(risk.riskId) !== 1) continue;
      for (const contextRef of risk.contextRefs) {
        if (!target.candidateContextRefs.includes(contextRef)) continue;
        specs.push({ targetRef: target.targetRef, riskId: risk.riskId, contextRef });
      }
    }
  }
  if (!specs.length
    || new Set(specs.map((spec) => spec.targetRef)).size !== targetDiagnostics.length) {
    return {
      eligible: false,
      reason: 'no_safe_target_coverage_association',
      eligibleTargetCoverageSpecs: [],
    };
  }
  return {
    eligible: true,
    draft: prepared.draft,
    riskDiagnostics: prepared.pendingDiagnostics,
    targetDiagnostics,
    eligibleTargetCoverageSpecs: specs,
  };
}

function mixedTargetCaseTitle(
  repair: QATestCaseMixedCoverageTargetPatch['targetRepairs'][number],
  used: Set<string>,
): string {
  return uniqueCoverageCaseTitle(
    `[${repair.riskId}/${repair.targetRef}/${repair.category}] ${repair.title}`,
    used,
  );
}

function mixedTargetRepairSpecs(
  eligibility: Extract<QATestCaseMixedCoverageTargetPatchEligibility, { eligible: true }>,
): Map<string, Set<string>> {
  const candidates = new Map<string, Set<string>>();
  for (const spec of eligibility.eligibleTargetCoverageSpecs) {
    const values = candidates.get(spec.targetRef) ?? new Set<string>();
    values.add(`${spec.riskId}\u0000${spec.contextRef}`);
    candidates.set(spec.targetRef, values);
  }
  return candidates;
}

/**
 * Applies a strictly parsed mixed patch to a clone. Both ledgers must exactly
 * match the compiler diagnostics in order; target cases are always non-core.
 */
export function applyQATestCaseMixedCoverageTargetPatch(
  input: Record<string, unknown>,
  draftValue: QATestCaseDraft,
  failure: QATestCaseDraftCompilationError,
  patchValue: unknown,
): QATestCaseDraft {
  const eligibility = prepareQATestCaseMixedCoverageTargetPatch(input, draftValue, failure);
  if (!eligibility.eligible) {
    throw new QATestCaseCoveragePatchError([
      `mixed coverage target patch is not eligible: ${eligibility.reason}`,
    ]);
  }
  const parsedPatch = qaTestCaseMixedCoverageTargetPatchSchema.safeParse(patchValue);
  if (!parsedPatch.success) {
    throw new QATestCaseCoveragePatchError(['mixed coverage target patch must match the strict repair schema']);
  }
  const patch = parsedPatch.data;
  const issues: string[] = [];
  if (containsCoveragePatchSentinel(patch)) {
    issues.push('mixed_coverage_target_patch_contains_fixed_sentinel');
  }
  const expectedRiskIds = eligibility.riskDiagnostics.map((diagnostic) => diagnostic.riskId);
  const actualRiskIds = patch.riskRepairs.map((repair) => repair.riskId);
  if (new Set(actualRiskIds).size !== actualRiskIds.length) {
    issues.push('mixed coverage target patch risk repair IDs must be unique');
  }
  if (!sameValues(actualRiskIds, expectedRiskIds)) {
    issues.push('mixed coverage target patch risk repairs must exactly match diagnostics in order');
  }
  for (const diagnostic of eligibility.riskDiagnostics) {
    const repair = patch.riskRepairs.find((candidate) => candidate.riskId === diagnostic.riskId);
    if (repair) validateRepair(eligibility.draft, diagnostic, repair, issues);
  }

  const expectedTargetRefs = eligibility.targetDiagnostics.map((diagnostic) => diagnostic.targetRef);
  const actualTargetRefs = patch.targetRepairs.map((repair) => repair.targetRef);
  if (new Set(actualTargetRefs).size !== actualTargetRefs.length) {
    issues.push('mixed coverage target patch target repair refs must be unique');
  }
  if (!sameValues(actualTargetRefs, expectedTargetRefs)) {
    issues.push('mixed coverage target patch target repairs must exactly match diagnostics in order');
  }
  const candidates = mixedTargetRepairSpecs(eligibility);
  for (const repair of patch.targetRepairs) {
    if (!candidates.get(repair.targetRef)?.has(`${repair.riskId}\u0000${repair.contextRef}`)) {
      issues.push(`mixed coverage target repair must select an eligible risk/context pair: ${repair.targetRef}`);
    }
  }
  if (issues.length) throw new QATestCaseCoveragePatchError(issues);

  let withRiskRepairs: QATestCaseDraft;
  try {
    withRiskRepairs = applyQATestCaseCoveragePatch(
      eligibility.draft, eligibility.riskDiagnostics, { repairs: patch.riskRepairs },
    );
  } catch (error) {
    if (error instanceof QATestCaseCoveragePatchError) throw error;
    throw new QATestCaseCoveragePatchError(['mixed coverage target risk repairs could not be applied']);
  }

  const result = structuredClone(withRiskRepairs);
  const usedTitles = new Set(result.cases.map((testCase) => testCase.title));
  for (const repair of patch.targetRepairs) {
    const risk = result.risks.find((candidate) => candidate.riskId === repair.riskId);
    if (!risk || !risk.contextRefs.includes(repair.contextRef)) {
      throw new QATestCaseCoveragePatchError([
        `mixed coverage target repair became stale: ${repair.targetRef}`,
      ]);
    }
    result.cases.push({
      title: mixedTargetCaseTitle(repair, usedTitles),
      category: repair.category,
      priority: risk.priority,
      contextRefs: [],
      riskLinks: [{ riskId: repair.riskId, contextRef: repair.contextRef }],
      preconditions: [],
      testData: [],
      actions: [{ step: repair.step, expectedResult: repair.expectedResult }],
      notes: [],
      smoke: false,
      automationCandidate: false,
      releaseBlocking: risk.priority === 'P0' || risk.priority === 'P1',
    });
  }
  const normalized = qaTestCaseDraftSchema.parse(result);
  try {
    compileQATestCaseDraft(input, normalized);
  } catch (error) {
    if (error instanceof QATestCaseDraftCompilationError) {
      throw new QATestCaseCoveragePatchError(['mixed coverage target patch did not clear the compiler gate']);
    }
    throw error;
  }
  return normalized;
}
