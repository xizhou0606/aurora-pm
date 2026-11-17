import { z } from 'zod';
import {
  qaTestCaseGlobalPlanSchema,
  type QATestCaseGlobalPlan,
  type QATestCaseGlobalPlanRefDiagnostic,
} from './qa-test-case-global-plan-schema.js';

const contextRef = z.string().regex(
  /^(?:subject:(?:title|description)|source:(?:0|[1-9]\d*):content|requirement:(?:0|[1-9]\d*):(?:title|description|acceptance_criteria)|task:(?:0|[1-9]\d*):(?:title|description|status))$/,
);
const patchPath = z.string().regex(
  /^(?:sourceFindings\[(?:0|[1-9]\d*)\]\.contextRefs\[(?:0|[1-9]\d*)\]|interactions\[(?:0|[1-9]\d*)\]\.relationshipRef|interactions\[(?:0|[1-9]\d*)\]\.cases\[(?:0|[1-9]\d*)\]\.contextRefs\[(?:0|[1-9]\d*)\])$/,
);
const relationshipRefPattern = /^(?:subject:description|source:(?:0|[1-9]\d*):content)$/;

const repairSchema = z.object({
  path: patchPath,
  replacementRef: contextRef,
}).strict();

export const qaTestCaseGlobalPlanRefPatchSchema = z.object({
  repairs: z.array(repairSchema).min(1).max(400),
}).strict();

export type QATestCaseGlobalPlanRefPatch = z.infer<typeof qaTestCaseGlobalPlanRefPatchSchema>;
export type QATestCaseGlobalPlanRefPatchTarget = {
  path: string;
  kind: 'any' | 'relationship';
  candidateRefs: string[];
};

export class QATestCaseGlobalPlanRefPatchError extends Error {
  readonly issues: string[];

  constructor(issues: string[]) {
    const distinct = [...new Set(issues)];
    super(`qa test case global plan ref patch failed: ${distinct.join('; ')}`);
    this.name = 'QATestCaseGlobalPlanRefPatchError';
    this.issues = distinct;
  }
}

export function parseQATestCaseGlobalPlanRefPatchText(
  text: string,
): QATestCaseGlobalPlanRefPatch {
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
      const parsed = qaTestCaseGlobalPlanRefPatchSchema.safeParse(value);
      if (parsed.success) return parsed.data;
    } catch {
      // Try the next complete-object candidate without exposing model output.
    }
  }
  throw new QATestCaseGlobalPlanRefPatchError([
    parsedJSON
      ? 'global_plan_ref_patch_invalid_schema'
      : 'global_plan_ref_patch_invalid_json',
  ]);
}

type RefLocation =
  | { kind: 'any'; path: string; sourceFinding: number; ref: number }
  | { kind: 'relationship'; path: string; interaction: number }
  | { kind: 'any'; path: string; interaction: number; testCase: number; ref: number };

function refLocations(plan: QATestCaseGlobalPlan): RefLocation[] {
  const locations: RefLocation[] = [];
  plan.sourceFindings.forEach((finding, sourceFinding) => {
    finding.contextRefs.forEach((_ref, ref) => locations.push({
      kind: 'any', path: `sourceFindings[${sourceFinding}].contextRefs[${ref}]`,
      sourceFinding, ref,
    }));
  });
  plan.interactions.forEach((interaction, interactionIndex) => {
    locations.push({
      kind: 'relationship', path: `interactions[${interactionIndex}].relationshipRef`,
      interaction: interactionIndex,
    });
    interaction.cases.forEach((testCase, caseIndex) => {
      testCase.contextRefs.forEach((_ref, ref) => locations.push({
        kind: 'any',
        path: `interactions[${interactionIndex}].cases[${caseIndex}].contextRefs[${ref}]`,
        interaction: interactionIndex,
        testCase: caseIndex,
        ref,
      }));
    });
  });
  return locations;
}

function locationValue(plan: QATestCaseGlobalPlan, location: RefLocation): string {
  if ('sourceFinding' in location) {
    return plan.sourceFindings[location.sourceFinding].contextRefs[location.ref];
  }
  if (location.kind === 'relationship') {
    return plan.interactions[location.interaction].relationshipRef;
  }
  return plan.interactions[location.interaction]
    .cases[location.testCase].contextRefs[location.ref];
}

function setLocationValue(
  plan: QATestCaseGlobalPlan,
  location: RefLocation,
  replacementRef: string,
): void {
  if ('sourceFinding' in location) {
    plan.sourceFindings[location.sourceFinding].contextRefs[location.ref] = replacementRef;
    return;
  }
  if (location.kind === 'relationship') {
    plan.interactions[location.interaction].relationshipRef = replacementRef;
    return;
  }
  plan.interactions[location.interaction]
    .cases[location.testCase].contextRefs[location.ref] = replacementRef;
}

function validateDiagnostics(
  plan: QATestCaseGlobalPlan,
  allowedRefs: Set<string>,
  diagnostics: readonly QATestCaseGlobalPlanRefDiagnostic[],
  issues: string[],
): Map<string, RefLocation> {
  const locations = new Map(refLocations(plan).map((location) => [location.path, location]));
  const unavailable = [...locations.values()].filter((location) => (
    !allowedRefs.has(locationValue(plan, location))
  ));
  const diagnosticPaths = diagnostics.map((diagnostic) => diagnostic.path);
  if (new Set(diagnosticPaths).size !== diagnosticPaths.length) {
    issues.push('global plan ref diagnostics paths must be unique');
  }
  const unavailablePaths = new Set(unavailable.map((location) => location.path));
  if (unavailablePaths.size !== new Set(diagnosticPaths).size
    || diagnosticPaths.some((path) => !unavailablePaths.has(path))) {
    issues.push('global plan ref diagnostics must exactly match unavailable refs');
  }
  for (const diagnostic of diagnostics) {
    const location = locations.get(diagnostic.path);
    const expectedCode = location?.kind === 'relationship'
      ? 'unavailable_relationship_ref' : 'unavailable_context_ref';
    if (!location || diagnostic.kind !== location.kind || diagnostic.code !== expectedCode) {
      issues.push(`global plan ref diagnostic does not match path kind: ${diagnostic.path}`);
    }
  }
  return locations;
}

function siblingRefs(plan: QATestCaseGlobalPlan, location: RefLocation): string[] {
  if ('sourceFinding' in location) {
    return plan.sourceFindings[location.sourceFinding].contextRefs
      .filter((_ref, index) => index !== location.ref);
  }
  if (location.kind === 'relationship') return [];
  return plan.interactions[location.interaction].cases[location.testCase].contextRefs
    .filter((_ref, index) => index !== location.ref);
}

export function resolveQATestCaseGlobalPlanRefPatchTargets(
  planValue: QATestCaseGlobalPlan,
  diagnostics: readonly QATestCaseGlobalPlanRefDiagnostic[],
  allowedRefValues: readonly string[],
): QATestCaseGlobalPlanRefPatchTarget[] {
  const planResult = qaTestCaseGlobalPlanSchema.safeParse(planValue);
  if (!planResult.success) {
    throw new QATestCaseGlobalPlanRefPatchError(['global plan must match the strict nested IR schema']);
  }
  const allowedResult = z.array(contextRef).min(1).safeParse(allowedRefValues);
  if (!allowedResult.success || new Set(allowedResult.data).size !== allowedResult.data.length) {
    throw new QATestCaseGlobalPlanRefPatchError(['allowed global plan refs must be canonical and unique']);
  }

  const issues: string[] = [];
  if (!diagnostics.length) issues.push('global plan ref diagnostics must not be empty');
  const locations = validateDiagnostics(
    planResult.data, new Set(allowedResult.data), diagnostics, issues,
  );
  const targets = diagnostics.flatMap((diagnostic) => {
    const location = locations.get(diagnostic.path);
    if (!location) return [];
    const occupied = new Set(siblingRefs(planResult.data, location));
    const candidateRefs = allowedResult.data.filter((ref) => (
      location.kind === 'relationship'
        ? relationshipRefPattern.test(ref)
        : !occupied.has(ref)
    ));
    if (!candidateRefs.length) {
      issues.push(`global plan ref patch has no replacement candidates: ${diagnostic.path}`);
    }
    return [{ path: diagnostic.path, kind: location.kind, candidateRefs }];
  });
  if (issues.length) throw new QATestCaseGlobalPlanRefPatchError(issues);
  return targets;
}

export function applyQATestCaseGlobalPlanRefPatch(
  planValue: QATestCaseGlobalPlan,
  diagnostics: readonly QATestCaseGlobalPlanRefDiagnostic[],
  allowedRefValues: readonly string[],
  patchValue: unknown,
): QATestCaseGlobalPlan {
  const planResult = qaTestCaseGlobalPlanSchema.safeParse(planValue);
  if (!planResult.success) {
    throw new QATestCaseGlobalPlanRefPatchError(['global plan must match the strict nested IR schema']);
  }
  const patchResult = qaTestCaseGlobalPlanRefPatchSchema.safeParse(patchValue);
  if (!patchResult.success) {
    throw new QATestCaseGlobalPlanRefPatchError(['global plan ref patch must match the strict schema']);
  }
  const allowedResult = z.array(contextRef).min(1).safeParse(allowedRefValues);
  if (!allowedResult.success || new Set(allowedResult.data).size !== allowedResult.data.length) {
    throw new QATestCaseGlobalPlanRefPatchError(['allowed global plan refs must be canonical and unique']);
  }

  const issues: string[] = [];
  const allowedRefs = new Set(allowedResult.data);
  const targets = resolveQATestCaseGlobalPlanRefPatchTargets(
    planResult.data, diagnostics, allowedResult.data,
  );
  const targetByPath = new Map(targets.map((target) => [target.path, target]));
  const locations = new Map(refLocations(planResult.data).map((location) => [location.path, location]));
  const repairPaths = patchResult.data.repairs.map((repair) => repair.path);
  if (new Set(repairPaths).size !== repairPaths.length) {
    issues.push('global plan ref patch paths must be unique');
  }
  const diagnosticPaths = new Set(diagnostics.map((diagnostic) => diagnostic.path));
  if (diagnosticPaths.size !== new Set(repairPaths).size
    || repairPaths.some((path) => !diagnosticPaths.has(path))) {
    issues.push('global plan ref patch repairs must exactly match diagnostics');
  }
  for (const repair of patchResult.data.repairs) {
    const diagnostic = diagnostics.find((candidate) => candidate.path === repair.path);
    const target = targetByPath.get(repair.path);
    if (!target?.candidateRefs.includes(repair.replacementRef)) {
      issues.push(`replacement ref is not a candidate for path: ${repair.path}`);
    }
    if (!allowedRefs.has(repair.replacementRef)) {
      issues.push(`replacement ref is not in the allowed list: ${repair.path}`);
    }
    if (diagnostic?.kind === 'relationship'
      && !relationshipRefPattern.test(repair.replacementRef)) {
      issues.push(`relationship replacement ref is not allowed: ${repair.path}`);
    }
    if (!locations.has(repair.path)) {
      issues.push(`global plan ref patch path does not exist: ${repair.path}`);
    }
  }
  if (issues.length) throw new QATestCaseGlobalPlanRefPatchError(issues);

  const result = structuredClone(planResult.data);
  const resultLocations = new Map(refLocations(result).map((location) => [location.path, location]));
  for (const repair of patchResult.data.repairs) {
    setLocationValue(result, resultLocations.get(repair.path)!, repair.replacementRef);
  }
  if (refLocations(result).some((location) => !allowedRefs.has(locationValue(result, location)))) {
    throw new QATestCaseGlobalPlanRefPatchError(['global plan ref patch left unavailable refs']);
  }
  const hasDuplicateRefs = [
    ...result.sourceFindings.map((finding) => finding.contextRefs),
    ...result.interactions.flatMap((interaction) => (
      interaction.cases.map((testCase) => testCase.contextRefs)
    )),
  ].some((refs) => new Set(refs).size !== refs.length);
  if (hasDuplicateRefs) {
    throw new QATestCaseGlobalPlanRefPatchError(['global plan ref patch created duplicate refs']);
  }
  return qaTestCaseGlobalPlanSchema.parse(result);
}
