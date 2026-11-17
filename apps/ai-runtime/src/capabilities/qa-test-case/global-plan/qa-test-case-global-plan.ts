import {
  qaTestCaseRequirementGroups,
  resolveQATestCaseContext,
  type ResolvedQATestCaseRef,
} from '../draft/qa-test-case-draft-context.js';
import {
  qaTestCaseGlobalPlanSchema,
  type CompiledQATestCaseGlobalPlan,
  type QATestCaseGlobalPlan,
  type QATestCaseGlobalPlanRefDiagnostic,
} from './qa-test-case-global-plan-schema.js';

function unique(values: string[]): string[] {
  return [...new Set(values)];
}

function uniqueResolved(values: ResolvedQATestCaseRef[]): ResolvedQATestCaseRef[] {
  const seen = new Set<string>();
  return values.filter((value) => {
    if (seen.has(value.ref)) return false;
    seen.add(value.ref);
    return true;
  });
}

export class QATestCaseGlobalPlanCompilationError extends Error {
  readonly issues: string[];
  readonly diagnostics: QATestCaseGlobalPlanRefDiagnostic[];

  constructor(issues: string[], diagnostics: QATestCaseGlobalPlanRefDiagnostic[] = []) {
    const distinct = unique(issues);
    super(`qa test case global plan compilation failed: ${distinct.join('; ')}`);
    this.name = 'QATestCaseGlobalPlanCompilationError';
    this.issues = distinct;
    this.diagnostics = diagnostics.map((diagnostic) => ({ ...diagnostic }));
  }
}

function inspectUnique(label: string, values: string[], issues: string[]): void {
  if (new Set(values).size !== values.length) issues.push(`${label} must be unique`);
}

function resolveRefs(
  catalog: Map<string, ResolvedQATestCaseRef>,
  values: string[],
  owner: string,
  issues: string[],
  diagnostics?: QATestCaseGlobalPlanRefDiagnostic[],
  diagnosticPath?: string,
): ResolvedQATestCaseRef[] {
  inspectUnique(`${owner} context refs`, values, issues);
  return values.flatMap((ref, index) => {
    const resolved = catalog.get(ref);
    if (resolved) return [resolved];
    const message = `${owner} has unavailable context ref: ${ref}`;
    issues.push(message);
    if (diagnostics && diagnosticPath) {
      diagnostics.push({
        code: 'unavailable_context_ref',
        path: `${diagnosticPath}[${index}]`,
        kind: 'any',
        message,
      });
    }
    return [];
  });
}

function validateCrossRequirementRefs(
  resolved: ResolvedQATestCaseRef[],
  taskGroups: Map<string, string>,
  owner: string,
  issues: string[],
): void {
  const groups = new Set<string>();
  for (const ref of resolved) {
    if (ref.kind === 'requirement') groups.add(ref.targetRef);
    if (ref.kind === 'task') {
      const group = taskGroups.get(ref.targetRef);
      if (group?.startsWith('requirement:')) groups.add(group);
      else issues.push(`${owner} task ref is not linked to an input requirement group: ${ref.ref}`);
    }
  }
  if (groups.size < 2) issues.push(`${owner} must reference at least two requirement groups`);
}

export function compileQATestCaseGlobalPlan(
  input: Record<string, unknown>,
  value: unknown,
): CompiledQATestCaseGlobalPlan {
  const plan = qaTestCaseGlobalPlanSchema.parse(value);
  const issues: string[] = [];
  const diagnostics: QATestCaseGlobalPlanRefDiagnostic[] = [];
  inspectUnique('methods', plan.methods, issues);
  inspectUnique('global case titles', plan.interactions.flatMap((interaction) => (
    interaction.cases.map((testCase) => testCase.title)
  )), issues);
  const catalog = resolveQATestCaseContext(input);
  const taskGroups = qaTestCaseRequirementGroups(input);

  const sourceFindings = plan.sourceFindings.map((finding, index) => {
    const owner = `source finding ${index + 1}`;
    const resolved = resolveRefs(
      catalog,
      finding.contextRefs,
      owner,
      issues,
      diagnostics,
      `sourceFindings[${index}].contextRefs`,
    );
    if (finding.status === 'conflict'
      && new Set(resolved.map((ref) => ref.targetRef)).size < 2) {
      issues.push(`${owner} conflict must cite at least two distinct context targets`);
    }
    return { ...finding, evidence: resolved.map((ref) => ref.path) };
  });

  const cases: CompiledQATestCaseGlobalPlan['cases'] = [];
  let globalCaseNumber = 0;
  const interactions = plan.interactions.map((interaction, interactionIndex) => {
    const riskId = `RISK-GLOBAL-${String(interactionIndex + 1).padStart(3, '0')}`;
    const owner = `interaction ${riskId}`;
    const relationship = catalog.get(interaction.relationshipRef);
    if (!relationship) {
      const message = `${owner} has unavailable relationship ref: ${interaction.relationshipRef}`;
      issues.push(message);
      diagnostics.push({
        code: 'unavailable_relationship_ref',
        path: `interactions[${interactionIndex}].relationshipRef`,
        kind: 'relationship',
        message,
      });
    }
    const interactionCaseRefs: ResolvedQATestCaseRef[] = [];
    for (const [caseIndex, testCase] of interaction.cases.entries()) {
      globalCaseNumber += 1;
      const caseOwner = `global case ${globalCaseNumber}`;
      const explicit = resolveRefs(
        catalog,
        testCase.contextRefs,
        caseOwner,
        issues,
        diagnostics,
        `interactions[${interactionIndex}].cases[${caseIndex}].contextRefs`,
      );
      validateCrossRequirementRefs(explicit, taskGroups, caseOwner, issues);
      interactionCaseRefs.push(...explicit);
      const caseEvidenceRefs = uniqueResolved([
        ...explicit,
        ...(relationship ? [relationship] : []),
      ]);
      const { actions, contextRefs: _contextRefs, ...caseSemantic } = testCase;
      cases.push({
        ...caseSemantic,
        contextRefs: caseEvidenceRefs.map((ref) => ref.ref),
        evidence: caseEvidenceRefs.map((ref) => ref.path),
        traceIds: unique([riskId, ...caseEvidenceRefs.map((ref) => ref.traceId)]),
        steps: actions.map((action) => action.step),
        expectedResults: actions.map((action) => action.expectedResult),
      });
    }
    const explicitInteractionRefs = uniqueResolved(interactionCaseRefs);
    const evidenceRefs = uniqueResolved([
      ...(relationship ? [relationship] : []),
      ...explicitInteractionRefs,
    ]);
    const { coverageDecision, cases: _cases, ...semantic } = interaction;
    return {
      ...semantic,
      riskId,
      contextRefs: explicitInteractionRefs.map((ref) => ref.ref),
      gapNote: coverageDecision.status === 'omitted'
        ? `positive, negative, boundary: ${coverageDecision.reason}`
        : '',
      evidence: evidenceRefs.map((ref) => ref.path),
      traceIds: unique([riskId, ...evidenceRefs.map((ref) => ref.traceId)]),
    };
  });
  if (issues.length) throw new QATestCaseGlobalPlanCompilationError(issues, diagnostics);
  return {
    summary: plan.summary,
    objective: plan.objective,
    priorityLogic: plan.priorityLogic,
    methods: plan.methods,
    sourceFindings,
    interactions,
    cases,
    noCrossInteractionReason: plan.noCrossInteractionReason,
  };
}
