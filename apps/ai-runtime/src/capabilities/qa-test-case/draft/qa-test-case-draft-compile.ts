import { aiOutputSchema, type AIOutput } from '../../../shared/ai/schema.js';
import {
  compileQATestCaseSourceAssessments,
  qaTestCaseRequiredCoverageTargets,
  qaTestCaseRequirementGroups,
  resolveQATestCaseContext,
  type ResolvedQATestCaseRef,
} from './qa-test-case-draft-context.js';
import {
  compileRiskCoverageDecisions,
  type RiskCoverageDiagnostic,
} from './qa-test-case-draft-risk.js';
import { testCaseDesignIssues } from '../qa-test-case-issues.js';
import {
  qaTestCaseDraftSchema,
  type QATestCaseDraft,
  type QATestCaseTargetCoverageDiagnostic,
} from './fixtures/qa-test-case-draft-schema.js';
import { QATestCaseDraftCompilationError, unique } from './fixtures/qa-test-case-draft-error.js';

function pad(index: number): string { return String(index + 1).padStart(3, '0'); }

type EffectiveRefSource = { ref: ResolvedQATestCaseRef; source: string };
const MAX_CROSS_GROUPS_IN_ISSUE = 6;
const MAX_CROSS_GROUP_REFS_IN_ISSUE = 6;

function crossGroupProjection(
  sources: EffectiveRefSource[],
  requirementGroups: Map<string, string>,
): string {
  const groupOrder = new Map<string, number>();
  for (const group of requirementGroups.values()) {
    if (!groupOrder.has(group)) groupOrder.set(group, groupOrder.size);
  }
  const grouped = new Map<string, string[]>();
  for (const source of sources) {
    if (source.ref.kind !== 'requirement' && source.ref.kind !== 'task') continue;
    const group = requirementGroups.get(source.ref.targetRef) ?? source.ref.targetRef;
    const value = `${source.source}->${source.ref.ref}`;
    const values = grouped.get(group) ?? [];
    if (!values.includes(value)) values.push(value);
    grouped.set(group, values);
  }
  const ordered = [...grouped.entries()].sort((left, right) => (
    (groupOrder.get(left[0]) ?? Number.MAX_SAFE_INTEGER)
      - (groupOrder.get(right[0]) ?? Number.MAX_SAFE_INTEGER)
      || left[0].localeCompare(right[0])
  ));
  const visible = ordered.slice(0, MAX_CROSS_GROUPS_IN_ISSUE).map(([group, values]) => {
    const refs = values.slice(0, MAX_CROSS_GROUP_REFS_IN_ISSUE);
    const suffix = values.length > refs.length ? `,...(+${values.length - refs.length} refs)` : '';
    return `${group}=[${refs.join(',')}${suffix}]`;
  });
  if (ordered.length > visible.length) visible.push(`...(+${ordered.length - visible.length} groups)`);
  return visible.join(' | ');
}

function inspectUnique(label: string, values: string[], issues: string[]): void {
  if (new Set(values).size !== values.length) {
    issues.push(`${label} must be unique`);
  }
}

function resolveRefs(
  refs: Map<string, ResolvedQATestCaseRef>,
  values: string[],
  owner: string,
  issues: string[],
): ResolvedQATestCaseRef[] {
  inspectUnique(`${owner} context refs`, values, issues);
  return values.flatMap((ref) => {
    const resolved = refs.get(ref);
    if (!resolved) {
      issues.push(`${owner} has unavailable context ref: ${ref}`);
      return [];
    }
    return [resolved];
  });
}

export function compileQATestCaseDraft(
  input: Record<string, unknown>,
  value: unknown,
): AIOutput {
  const draft = qaTestCaseDraftSchema.parse(value);
  const issues: string[] = [];
  const diagnostics: RiskCoverageDiagnostic[] = [];
  const targetCoverageDiagnostics: QATestCaseTargetCoverageDiagnostic[] = [];
  inspectUnique('methods', draft.methods, issues);
  inspectUnique('risk IDs', draft.risks.map((risk) => risk.riskId), issues);
  inspectUnique('case titles', draft.cases.map((testCase) => testCase.title), issues);
  if (!draft.cases.some((testCase) => testCase.smoke)) {
    issues.push('at least one smoke case must be selected');
  }

  const refs = resolveQATestCaseContext(input);
  const sourceInventory = compileQATestCaseSourceAssessments(
    input, refs, draft.sourceAssessments, issues,
  );
  const requirementGroups = qaTestCaseRequirementGroups(input);
  const risks = new Map<string, { draft: QATestCaseDraft['risks'][number]; refs: ResolvedQATestCaseRef[] }>();
  for (const risk of draft.risks) {
    const resolved = resolveRefs(refs, risk.contextRefs, `risk ${risk.riskId}`, issues);
    if (!risks.has(risk.riskId)) risks.set(risk.riskId, { draft: risk, refs: resolved });
  }
  const referencedRisks = new Set<string>();
  const coveredContext = new Set<string>();
  const traceOrder: string[] = [];
  const traceCases = new Map<string, { caseIds: string[]; categories: TestCaseCategory[] }>();
  const riskCoverageCases: Array<{ category: string; riskLinks: Array<{ riskId: string }> }> = [];
  const addTrace = (traceId: string, caseId: string, category: TestCaseCategory) => {
    if (!traceCases.has(traceId)) {
      traceOrder.push(traceId);
      traceCases.set(traceId, { caseIds: [], categories: [] });
    }
    const row = traceCases.get(traceId)!;
    row.caseIds.push(caseId);
    if (!row.categories.includes(category)) row.categories.push(category);
  };

  const testCases = draft.cases.map((testCase, index) => {
    const caseId = `TC-QA-${pad(index)}`;
    inspectUnique(`case ${index + 1} risk link IDs`, testCase.riskLinks.map((link) => link.riskId), issues);
    const directCaseRefs = resolveRefs(refs, testCase.contextRefs, `case ${index + 1}`, issues);
    const directRefSources = testCase.contextRefs.flatMap((ref, refIndex) => {
      const resolved = refs.get(ref);
      return resolved ? [{
        ref: resolved, source: `cases[${index}].contextRefs[${refIndex}]`,
      }] : [];
    });
    const validRiskLinks = testCase.riskLinks.flatMap((link, linkIndex) => {
      const risk = risks.get(link.riskId);
      const linkedRef = refs.get(link.contextRef);
      if (!risk) {
        issues.push(`case ${index + 1} references unknown risk: ${link.riskId}`);
      }
      if (!linkedRef) {
        issues.push(`case ${index + 1} risk link has unavailable context ref: ${link.contextRef}`);
      }
      if (risk && !risk.draft.contextRefs.includes(link.contextRef)) {
        issues.push(
          `case ${index + 1} risk link context ref is not declared by ${link.riskId}: ${link.contextRef}`,
        );
      }
      if (!risk || !linkedRef || !risk.draft.contextRefs.includes(link.contextRef)) return [];
      referencedRisks.add(link.riskId);
      return [{
        riskId: link.riskId,
        ref: linkedRef,
        source: `cases[${index}].riskLinks[${linkIndex}].contextRef`,
      }];
    });
    const caseRefs = [...new Map(
      [...directCaseRefs, ...validRiskLinks.map((link) => link.ref)].map((ref) => [ref.ref, ref]),
    ).values()];
    const linkedRisks = validRiskLinks.map((link) => link.riskId);
    riskCoverageCases.push({
      category: testCase.category,
      riskLinks: validRiskLinks.map((link) => ({ riskId: link.riskId })),
    });
    caseRefs.filter((ref) => ref.kind === 'requirement' || ref.kind === 'task')
      .forEach((ref) => coveredContext.add(ref.targetRef));
    const coveredTargets = new Set(caseRefs
      .filter((ref) => ref.kind === 'requirement' || ref.kind === 'task')
      .map((ref) => ref.targetRef));
    if (coveredTargets.size > 8) {
      issues.push(`test case covers too many unrelated context items: ${caseId}`);
    }
    const coveredGroups = new Set([...coveredTargets].map((target) => (
      requirementGroups.get(target) ?? target
    )));
    const hasRelationshipFact = directCaseRefs.some((ref) => (
      ref.ref === 'subject:description' || /^source:\d+:content$/.test(ref.ref)
    ));
    const allowsCrossGroup = testCase.category === 'integration'
      && draft.methods.includes('user_journey') && hasRelationshipFact;
    if (coveredGroups.size > 1 && !allowsCrossGroup) {
      const projection = crossGroupProjection([
        ...directRefSources,
        ...validRiskLinks.map(({ ref, source }) => ({ ref, source })),
      ], requirementGroups);
      issues.push(
        `case ${index + 1} spans multiple requirement groups without integration relationship evidence; effective refs by group: ${projection}`,
      );
    }
    const trace = unique([...caseRefs.map((ref) => ref.traceId), ...linkedRisks]);
    trace.forEach((traceId) => addTrace(traceId, caseId, testCase.category));
    const derivedNotes = caseRefs.filter((ref) => ref.derived)
      .map((ref) => `${ref.traceId} 为派生标签`);
    const steps = testCase.actions.map((action) => action.step);
    const expectedResults = testCase.actions.map((action) => action.expectedResult);
    return {
      caseId,
      title: testCase.title,
      category: testCase.category,
      priority: testCase.priority,
      trace,
      preconditions: testCase.preconditions,
      testData: testCase.testData,
      steps,
      expectedResults,
      expectedResult: expectedResults.join('\n'),
      notes: unique([...testCase.notes, ...derivedNotes]),
      evidence: caseRefs.map((ref) => ref.path),
    };
  });

  for (const id of risks.keys()) {
    if (!referencedRisks.has(id)) issues.push(`risk is not linked by a case: ${id}`);
  }
  const riskGapNotes = compileRiskCoverageDecisions(
    draft.risks, riskCoverageCases, issues, diagnostics,
  );
  for (const target of qaTestCaseRequiredCoverageTargets(input)) {
    if (coveredContext.has(target.targetRef)) continue;
    const message = `context item is not covered: ${target.targetRef}`;
    issues.push(message);
    targetCoverageDiagnostics.push({
      code: 'context_item_not_covered',
      targetRef: target.targetRef,
      candidateContextRefs: [...target.candidateContextRefs],
      group: requirementGroups.get(target.targetRef) ?? target.targetRef,
      message,
    });
  }
  if (issues.length) {
    throw new QATestCaseDraftCompilationError(
      issues, diagnostics, targetCoverageDiagnostics,
    );
  }

  const caseIds = testCases.map((testCase) => testCase.caseId);
  const output = aiOutputSchema.parse({
    summary: draft.summary,
    changes: [], items: [], risks: [], testCases,
    testCaseDesign: {
      objective: draft.objective,
      scope: draft.scope,
      exclusions: draft.exclusions,
      sourceInventory,
      assumptions: draft.assumptions,
      informationGaps: draft.informationGaps,
      coverageStrategy: {
        priorityLogic: draft.priorityLogic,
        methods: draft.methods,
        riskHotspots: draft.risks.map((risk) => ({
          id: risk.riskId,
          description: risk.description,
          priority: risk.priority,
          evidence: risks.get(risk.riskId)!.refs.map((ref) => ref.path),
        })),
      },
      traceability: traceOrder.map((traceId) => ({
        traceId,
        caseIds: traceCases.get(traceId)!.caseIds,
        coverageTypes: traceCases.get(traceId)!.categories,
        gapNote: riskGapNotes.get(traceId) ?? '',
      })),
      smokeCaseIds: testCases.filter((_testCase, index) => draft.cases[index].smoke).map((item) => item.caseId),
      executionOrder: caseIds,
      automationCandidateCaseIds: testCases.filter((_testCase, index) => draft.cases[index].automationCandidate).map((item) => item.caseId),
      releaseBlockingCaseIds: testCases.filter((_testCase, index) => draft.cases[index].releaseBlocking).map((item) => item.caseId),
    },
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: null,
  });
  const finalGateIssues = testCaseDesignIssues(input, output);
  if (finalGateIssues.length) throw new QATestCaseDraftCompilationError(finalGateIssues, []);
  return output;
}

type TestCaseCategory = QATestCaseDraft['cases'][number]['category'];
