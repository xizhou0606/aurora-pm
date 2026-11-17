import { qaTestCaseDraftSchema } from '../draft/fixtures/qa-test-case-draft-schema.js';
import type { QATestCaseDraft } from '../draft/fixtures/qa-test-case-draft-schema.js';
import type { RiskCoverageDiagnostic } from '../draft/qa-test-case-draft-risk.js';
import {
  containsCoveragePatchSentinel,
  expectedStatus,
  qaTestCaseCoveragePatchSchema,
  qaTestCaseMixedCoverageTargetPatchSchema,
  QATestCaseCoveragePatchError,
  sameValues,
  type QATestCaseCoveragePatch,
  type QATestCaseMixedCoverageTargetPatch,
} from './qa-test-case-coverage-patch-schema.js';

const coreCategories = ['positive', 'negative', 'boundary'] as const;

function validateDiagnostics(
  draft: QATestCaseDraft,
  diagnostics: readonly RiskCoverageDiagnostic[],
  issues: string[],
): void {
  const riskIds = diagnostics.map((diagnostic) => diagnostic.riskId);
  if (new Set(riskIds).size !== riskIds.length) {
    issues.push('coverage diagnostics risk IDs must be unique');
  }
  for (const diagnostic of diagnostics) {
    const matchingRisks = draft.risks.filter((risk) => risk.riskId === diagnostic.riskId);
    if (matchingRisks.length !== 1) {
      issues.push(`coverage diagnostic risk must exist exactly once: ${diagnostic.riskId}`);
      continue;
    }
    const risk = matchingRisks[0];
    const linked = new Set(draft.cases
      .filter((testCase) => testCase.riskLinks.some((link) => (
        link.riskId === risk.riskId && risk.contextRefs.includes(link.contextRef)
      )))
      .map((testCase) => testCase.category));
    const actual = coreCategories.filter((category) => linked.has(category));
    const missing = coreCategories.filter((category) => !linked.has(category));
    const status = expectedStatus(risk.priority, missing);
    if (diagnostic.code !== 'risk_coverage_decision_mismatch'
      || diagnostic.priority !== risk.priority
      || diagnostic.expectedStatus !== status
      || !sameValues(diagnostic.actualCategories, actual)
      || !sameValues(diagnostic.missingCategories, missing)
      || risk.coverageDecision.status === status) {
      issues.push(`coverage diagnostic does not match current draft: ${diagnostic.riskId}`);
    }
  }
}

export type PreparedQATestCaseCoveragePatch = {
  draft: QATestCaseDraft;
  pendingDiagnostics: RiskCoverageDiagnostic[];
};

export function prepareQATestCaseCoveragePatch(
  draftValue: QATestCaseDraft,
  diagnostics: readonly RiskCoverageDiagnostic[],
): PreparedQATestCaseCoveragePatch {
  const draftResult = qaTestCaseDraftSchema.safeParse(draftValue);
  if (!draftResult.success) {
    throw new QATestCaseCoveragePatchError(['coverage draft must match the strict Draft IR schema']);
  }
  const issues: string[] = [];
  if (!diagnostics.length) issues.push('coverage diagnostics must not be empty');
  validateDiagnostics(draftResult.data, diagnostics, issues);
  if (issues.length) throw new QATestCaseCoveragePatchError(issues);

  const draft = structuredClone(draftResult.data);
  const pendingDiagnostics: RiskCoverageDiagnostic[] = [];
  for (const diagnostic of diagnostics) {
    const risk = draft.risks.find((candidate) => candidate.riskId === diagnostic.riskId)!;
    if (diagnostic.expectedStatus === 'complete') {
      risk.coverageDecision = { status: 'complete' };
    } else if (diagnostic.expectedStatus === 'not_required') {
      risk.coverageDecision = { status: 'not_required' };
    } else {
      pendingDiagnostics.push(structuredClone(diagnostic));
    }
  }
  return { draft: qaTestCaseDraftSchema.parse(draft), pendingDiagnostics };
}

export function validateRepair(
  draft: QATestCaseDraft,
  diagnostic: RiskCoverageDiagnostic,
  repair: QATestCaseCoveragePatch['repairs'][number],
  issues: string[],
): void {
  const owner = `coverage repair ${repair.riskId}`;
  const risk = draft.risks.find((candidate) => candidate.riskId === repair.riskId);
  if (!risk) return;
  const categories = repair.resolutions.map((resolution) => resolution.category);
  if (!sameValues(categories, diagnostic.missingCategories)) {
    issues.push(`${owner} resolutions must exactly match missing categories in order`);
  }
  for (const resolution of repair.resolutions) {
    if (resolution.kind === 'case' && !risk.contextRefs.includes(resolution.contextRef)) {
      issues.push(`${owner} case context ref must be declared by the repaired risk`);
    }
  }
  const hasOmission = repair.resolutions.some((resolution) => resolution.kind === 'omit');
  if (hasOmission && repair.omissionReason === null) {
    issues.push(`${owner} must provide an omission reason when a category is omitted`);
  }
  if (!hasOmission && repair.omissionReason !== null) {
    issues.push(`${owner} must not provide an omission reason when all categories are added`);
  }
}

export function uniqueCoverageCaseTitle(base: string, used: Set<string>): string {
  if (!used.has(base)) {
    used.add(base);
    return base;
  }
  let sequence = 2;
  while (used.has(`${base} [coverage-${sequence}]`)) sequence += 1;
  const title = `${base} [coverage-${sequence}]`;
  used.add(title);
  return title;
}

export function applyQATestCaseCoveragePatch(
  draftValue: QATestCaseDraft,
  diagnostics: readonly RiskCoverageDiagnostic[],
  patchValue: unknown,
): QATestCaseDraft {
  const prepared = prepareQATestCaseCoveragePatch(draftValue, diagnostics);
  const parsedPatch = qaTestCaseCoveragePatchSchema.safeParse(patchValue);
  if (!parsedPatch.success) {
    throw new QATestCaseCoveragePatchError(['coverage patch must match the strict repair schema']);
  }
  const issues: string[] = [];
  if (!prepared.pendingDiagnostics.length) {
    issues.push('coverage patch has no semantic diagnostics to repair');
  }
  if (containsCoveragePatchSentinel(parsedPatch.data)) {
    issues.push('coverage_patch_contains_fixed_sentinel');
  }

  const repairIds = parsedPatch.data.repairs.map((repair) => repair.riskId);
  const expectedIds = prepared.pendingDiagnostics.map((diagnostic) => diagnostic.riskId);
  if (new Set(repairIds).size !== repairIds.length) {
    issues.push('coverage patch repair risk IDs must be unique');
  }
  if (!sameValues(repairIds, expectedIds)) {
    issues.push('coverage patch repairs must exactly match semantic diagnostics in order');
  }
  for (const diagnostic of prepared.pendingDiagnostics) {
    const repair = parsedPatch.data.repairs.find((candidate) => (
      candidate.riskId === diagnostic.riskId
    ));
    if (repair) validateRepair(prepared.draft, diagnostic, repair, issues);
  }
  if (issues.length) throw new QATestCaseCoveragePatchError(issues);

  const result = structuredClone(prepared.draft);
  const usedTitles = new Set(result.cases.map((testCase) => testCase.title));
  for (const repair of parsedPatch.data.repairs) {
    const risk = result.risks.find((candidate) => candidate.riskId === repair.riskId)!;
    for (const resolution of repair.resolutions) {
      if (resolution.kind !== 'case') continue;
      result.cases.push({
        title: uniqueCoverageCaseTitle(
          `[${repair.riskId}/${resolution.category}] ${resolution.title}`,
          usedTitles,
        ),
        category: resolution.category,
        priority: risk.priority,
        contextRefs: [],
        riskLinks: [{ riskId: repair.riskId, contextRef: resolution.contextRef }],
        preconditions: [],
        testData: [],
        actions: [{ step: resolution.step, expectedResult: resolution.expectedResult }],
        notes: [],
        smoke: false,
        automationCandidate: false,
        releaseBlocking: risk.priority === 'P0' || risk.priority === 'P1',
      });
    }
    const hasOmission = repair.resolutions.some((resolution) => resolution.kind === 'omit');
    risk.coverageDecision = hasOmission
      ? { status: 'omitted', reason: repair.omissionReason! }
      : { status: 'complete' };
  }
  return qaTestCaseDraftSchema.parse(result);
}

export function parseQATestCaseMixedCoverageTargetPatchText(
  text: string,
): QATestCaseMixedCoverageTargetPatch {
  const withoutThinking = text.trim()
    .replace(/<think>[\s\S]*?<\/think>/gi, '')
    .replace(/```(?:json)?/gi, '')
    .replace(/```/g, '')
    .trim();
  const fenced = [...text.matchAll(/```(?:json)?\s*([\s\S]*?)```/gi)]
    .map((match) => match[1].trim());
  const objectStart = withoutThinking.indexOf('{');
  const objectEnd = withoutThinking.lastIndexOf('}');
  const candidates = [
    withoutThinking,
    ...fenced,
    objectStart >= 0 && objectEnd > objectStart
      ? withoutThinking.slice(objectStart, objectEnd + 1) : '',
  ].filter((candidate, index, all) => Boolean(candidate) && all.indexOf(candidate) === index);
  let parsedJSON = false;
  for (const candidate of candidates) {
    try {
      const value = JSON.parse(candidate);
      parsedJSON = true;
      const parsed = qaTestCaseMixedCoverageTargetPatchSchema.safeParse(value);
      if (parsed.success) return parsed.data;
    } catch {
      // Try the next complete-object candidate without exposing model output.
    }
  }
  throw new QATestCaseCoveragePatchError([
    parsedJSON
      ? 'mixed_coverage_target_patch_invalid_schema'
      : 'mixed_coverage_target_patch_invalid_json',
  ]);
}

export function parseQATestCaseCoveragePatchText(text: string): QATestCaseCoveragePatch {
  const withoutThinking = text.trim()
    .replace(/<think>[\s\S]*?<\/think>/gi, '')
    .replace(/```(?:json)?/gi, '')
    .replace(/```/g, '')
    .trim();
  const fenced = [...text.matchAll(/```(?:json)?\s*([\s\S]*?)```/gi)]
    .map((match) => match[1].trim());
  const objectStart = withoutThinking.indexOf('{');
  const objectEnd = withoutThinking.lastIndexOf('}');
  const candidates = [
    withoutThinking,
    ...fenced,
    objectStart >= 0 && objectEnd > objectStart
      ? withoutThinking.slice(objectStart, objectEnd + 1) : '',
  ].filter((candidate, index, all) => Boolean(candidate) && all.indexOf(candidate) === index);
  let parsedJSON = false;
  for (const candidate of candidates) {
    try {
      const value = JSON.parse(candidate);
      parsedJSON = true;
      const parsed = qaTestCaseCoveragePatchSchema.safeParse(value);
      if (parsed.success) return parsed.data;
    } catch {
      // Try the next complete-object candidate without exposing model output.
    }
  }
  throw new QATestCaseCoveragePatchError([
    parsedJSON ? 'coverage_patch_invalid_schema' : 'coverage_patch_invalid_json',
  ]);
}
