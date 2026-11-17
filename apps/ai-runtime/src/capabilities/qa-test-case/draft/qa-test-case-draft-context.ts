type RefKind = 'subject' | 'source' | 'requirement' | 'task';

export type ResolvedQATestCaseRef = {
  ref: string;
  targetRef: string;
  kind: RefKind;
  path: string;
  traceId: string;
  derived: boolean;
};

export type QATestCaseSourceTarget = {
  targetRef: string;
  id: string;
  kind: string;
};

export type QATestCaseSourceAssessment = {
  targetRef: string;
  status: 'confirmed' | 'conflict' | 'assumption' | 'gap';
  summary: string;
  contextRefs: string[];
};

/**
 * Requirement/task targets that the model can actually cite. Source assessment
 * ownership intentionally remains broader and still includes raw empty rows.
 */
export type QATestCaseRequiredCoverageTarget = {
  targetRef: string;
  candidateContextRefs: string[];
};

type InventoryRow = {
  id: string;
  kind: string;
  status: QATestCaseSourceAssessment['status'];
  summary: string;
  evidence: string[];
};

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function rows(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function pad(index: number): string {
  return String(index + 1).padStart(3, '0');
}

function nonEmptyString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value.trim() : null;
}

function usableLeaf(value: unknown): boolean {
  if (typeof value === 'string') return Boolean(value.trim());
  if (typeof value === 'number') return Number.isFinite(value);
  if (typeof value === 'boolean') return true;
  if (Array.isArray(value)) return value.some(usableLeaf);
  return Boolean(value && typeof value === 'object' && Object.values(record(value)).some(usableLeaf));
}

function stableIdentifier(value: Record<string, unknown>): string | null {
  const raw = nonEmptyString(value.id)
    ?? (typeof value.id === 'number' && Number.isFinite(value.id) ? String(value.id) : null)
    ?? nonEmptyString(value.key);
  if (!raw) return null;
  const normalized = raw.toUpperCase()
    .replace(/[^A-Z0-9\u4e00-\u9fff]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return normalized || null;
}

function makeTraceId(
  kind: RefKind,
  index: number,
  value: Record<string, unknown>,
  used: Set<string>,
): { traceId: string; derived: boolean } {
  const prefix = { subject: 'SUBJECT', source: 'SOURCE', requirement: 'REQ', task: 'TASK' }[kind];
  const formalId = stableIdentifier(value);
  const suffix = formalId?.slice(0, 80) ?? `DERIVED-${pad(index)}`;
  let traceId = `${prefix}-${suffix}`;
  let derived = !formalId || formalId.length > 80;
  if (used.has(traceId)) {
    traceId = `${traceId.slice(0, 94)}-${pad(index)}`;
    derived = true;
  }
  used.add(traceId);
  return { traceId, derived };
}

export function resolveQATestCaseContext(
  input: Record<string, unknown>,
): Map<string, ResolvedQATestCaseRef> {
  const refs = new Map<string, ResolvedQATestCaseRef>();
  const usedTraceIds = new Set<string>();
  const addTarget = (
    targetRef: string,
    kind: RefKind,
    index: number,
    pathPrefix: string,
    value: Record<string, unknown>,
    fields: string[],
  ) => {
    const trace = makeTraceId(kind, index, value, usedTraceIds);
    fields.filter((field) => usableLeaf(value[field])).forEach((field) => {
      const ref = `${targetRef}:${field}`;
      refs.set(ref, { ref, targetRef, kind, path: `${pathPrefix}.${field}`, ...trace });
    });
  };

  addTarget('subject', 'subject', 0, 'subject', record(input.subject), ['title', 'description']);
  rows(input.sources).forEach((item, index) => {
    const source = record(item);
    if (source.available === true) {
      addTarget(`source:${index}`, 'source', index, `sources.${index}`, source, ['content']);
    }
  });
  const context = record(input.test_case_context);
  for (const kind of ['requirement', 'task'] as const) {
    const collection = kind === 'requirement' ? 'requirements' : 'tasks';
    const fields = kind === 'requirement'
      ? ['title', 'description', 'acceptance_criteria']
      : ['title', 'description', 'status'];
    rows(context[collection]).forEach((item, index) => addTarget(
      `${kind}:${index}`, kind, index, `test_case_context.${collection}.${index}`, record(item), fields,
    ));
  }
  return refs;
}

export function allowedQATestCaseContextRefs(input: Record<string, unknown>): string[] {
  return [...resolveQATestCaseContext(input).keys()];
}

export function qaTestCaseRequiredCoverageTargets(
  input: Record<string, unknown>,
): QATestCaseRequiredCoverageTarget[] {
  const targets = new Map<string, string[]>();
  for (const resolved of resolveQATestCaseContext(input).values()) {
    if (resolved.kind !== 'requirement' && resolved.kind !== 'task') continue;
    const refs = targets.get(resolved.targetRef) ?? [];
    refs.push(resolved.ref);
    targets.set(resolved.targetRef, refs);
  }
  return [...targets.entries()].map(([targetRef, candidateContextRefs]) => ({
    targetRef,
    candidateContextRefs,
  }));
}

export function qaTestCaseSourceTargets(input: Record<string, unknown>): QATestCaseSourceTarget[] {
  const targets: QATestCaseSourceTarget[] = [{ targetRef: 'subject', id: 'SRC-SUBJECT', kind: 'subject' }];
  rows(input.sources).forEach((_item, index) => targets.push({
    targetRef: `source:${index}`, id: `SRC-SOURCE-${pad(index)}`, kind: 'external_source',
  }));
  const context = record(input.test_case_context);
  for (const kind of ['requirement', 'task'] as const) {
    const collection = kind === 'requirement' ? 'requirements' : 'tasks';
    rows(context[collection]).forEach((_item, index) => targets.push({
      targetRef: `${kind}:${index}`,
      id: `SRC-${kind.toUpperCase()}-${pad(index)}`,
      kind,
    }));
  }
  return targets;
}

export function qaTestCaseSourceTargetRefs(input: Record<string, unknown>): string[] {
  return qaTestCaseSourceTargets(input).map((target) => target.targetRef);
}

export function compileQATestCaseSourceAssessments(
  input: Record<string, unknown>,
  refs: Map<string, ResolvedQATestCaseRef>,
  assessments: QATestCaseSourceAssessment[],
  issues: string[],
): InventoryRow[] {
  const targets = qaTestCaseSourceTargets(input);
  const knownTargets = new Set(targets.map((target) => target.targetRef));
  const assessmentTargets = assessments.map((assessment) => assessment.targetRef);
  if (new Set(assessmentTargets).size !== assessmentTargets.length) {
    issues.push('source assessment targets must be unique');
  }
  const compiled = new Map<string, Omit<InventoryRow, 'id' | 'kind'>>();
  for (const assessment of assessments) {
    if (!knownTargets.has(assessment.targetRef)) {
      issues.push(`source assessment has unknown target: ${assessment.targetRef}`);
      continue;
    }
    if (new Set(assessment.contextRefs).size !== assessment.contextRefs.length) {
      issues.push(`source assessment ${assessment.targetRef} context refs must be unique`);
    }
    const resolved = assessment.contextRefs.flatMap((ref) => {
      const value = refs.get(ref);
      if (value) return [value];
      issues.push(`source assessment ${assessment.targetRef} has unavailable context ref: ${ref}`);
      return [];
    });
    const resolvedTargets = new Set(resolved.map((ref) => ref.targetRef));
    if (assessment.status === 'confirmed' && !resolvedTargets.has(assessment.targetRef)) {
      issues.push(`confirmed source assessment must cite its own target: ${assessment.targetRef}`);
    }
    if (assessment.status === 'conflict') {
      if (!resolvedTargets.has(assessment.targetRef)) {
        issues.push(`conflict source assessment must cite its own target: ${assessment.targetRef}`);
      }
      if (resolvedTargets.size < 2) {
        issues.push(`conflict source assessment must cite two distinct targets: ${assessment.targetRef}`);
      }
    }
    if (!compiled.has(assessment.targetRef)) {
      compiled.set(assessment.targetRef, {
        status: assessment.status,
        summary: assessment.summary,
        evidence: resolved.map((ref) => ref.path),
      });
    }
  }
  for (const target of targets) {
    if (!compiled.has(target.targetRef)) {
      issues.push(`source assessment is missing target: ${target.targetRef}`);
    }
  }
  return targets.flatMap((target) => {
    const assessment = compiled.get(target.targetRef);
    return assessment ? [{ id: target.id, kind: target.kind, ...assessment }] : [];
  });
}

function canonicalIdentifier(value: unknown): string | null {
  if (typeof value === 'string' && value.trim()) return value.trim();
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return null;
}

export type QATestCaseRequirementGroupLedgerEntry = {
  targetRef: string;
  group: string;
};

/**
 * Build the requirement-group ownership used by both host prompts and compilers.
 * Duplicate canonical requirement IDs fail before batching can hide ambiguity.
 * Equal orphan IDs share a stable synthetic group without exposing the raw ID.
 */
export function qaTestCaseRequirementGroupLedger(
  input: Record<string, unknown>,
): QATestCaseRequirementGroupLedgerEntry[] {
  const context = record(input.test_case_context);
  const requirements = rows(context.requirements);
  const requirementIndexes = new Map<string, number[]>();
  const ledger: QATestCaseRequirementGroupLedgerEntry[] = [];
  requirements.forEach((item, index) => {
    const targetRef = `requirement:${index}`;
    ledger.push({ targetRef, group: targetRef });
    const id = canonicalIdentifier(record(item).id);
    if (!id) return;
    if (requirementIndexes.has(id)) {
      throw new Error('qa test case context has duplicate canonical requirement IDs');
    }
    requirementIndexes.set(id, [index]);
  });
  const orphanGroups = new Map<string, string>();
  rows(context.tasks).forEach((item, index) => {
    const requirementId = canonicalIdentifier(record(item).requirement_id);
    const matches = requirementId ? requirementIndexes.get(requirementId) ?? [] : [];
    let group = `task:${index}`;
    if (matches.length === 1) {
      group = `requirement:${matches[0]}`;
    } else if (matches.length === 0 && requirementId) {
      if (!orphanGroups.has(requirementId)) {
        orphanGroups.set(requirementId, `orphan-requirement:${orphanGroups.size}`);
      }
      group = orphanGroups.get(requirementId)!;
    }
    ledger.push({ targetRef: `task:${index}`, group });
  });
  return ledger;
}

export function qaTestCaseRequirementGroups(input: Record<string, unknown>): Map<string, string> {
  return new Map(qaTestCaseRequirementGroupLedger(input).map((entry) => (
    [entry.targetRef, entry.group]
  )));
}
