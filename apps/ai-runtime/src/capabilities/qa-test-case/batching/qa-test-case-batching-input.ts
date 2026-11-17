import type { AIOutput } from '../../../shared/ai/schema.js';
import {
  qaTestCaseRequirementGroupLedger,
  type QATestCaseRequirementGroupLedgerEntry,
} from '../draft/qa-test-case-draft-context.js';

export type ContextCollection = 'requirements' | 'tasks';

export type ContextItem = {
  collection: ContextCollection;
  globalIndex: number;
  group: string;
};

export type BatchGlobalIndices = Record<ContextCollection, number[]> & { sources: number[] };

export type QATestCaseBatch = {
  index: number;
  input: Record<string, unknown>;
  globalIndices: BatchGlobalIndices;
};

export type QATestCaseBatchOutput = {
  batch: QATestCaseBatch;
  output: AIOutput;
};

export function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

export function byteLength(value: unknown): number {
  return Buffer.byteLength(JSON.stringify(value), 'utf8');
}

export function relatedContextItems(
  requirements: unknown[],
  tasks: unknown[],
  ledger: readonly QATestCaseRequirementGroupLedgerEntry[],
): ContextItem[] {
  const groupByTarget = new Map(ledger.map((entry) => (
    [entry.targetRef, entry.group]
  )));
  const claimedTasks = new Set<number>();
  const ordered: ContextItem[] = [];
  for (const requirementIndex of requirements.keys()) {
    const group = `requirement:${requirementIndex}`;
    ordered.push({ collection: 'requirements', globalIndex: requirementIndex, group });
    for (const taskIndex of tasks.keys()) {
      if (groupByTarget.get(`task:${taskIndex}`) !== group) continue;
      ordered.push({ collection: 'tasks', globalIndex: taskIndex, group });
      claimedTasks.add(taskIndex);
    }
  }
  for (const taskIndex of tasks.keys()) {
    if (!claimedTasks.has(taskIndex)) {
      const group = groupByTarget.get(`task:${taskIndex}`) ?? `task:${taskIndex}`;
      ordered.push({ collection: 'tasks', globalIndex: taskIndex, group });
    }
  }
  return ordered;
}

function normalizedURL(value: unknown): string {
  if (typeof value !== 'string') return '';
  const raw = value.trim();
  if (!raw) return '';
  try {
    const parsed = new URL(raw);
    parsed.hash = '';
    if (parsed.port === '443' && parsed.protocol === 'https:') parsed.port = '';
    parsed.hostname = parsed.hostname.toLowerCase();
    parsed.pathname = parsed.pathname.replace(/\/+$/, '');
    return parsed.toString().replace(/\/$/, '');
  } catch {
    return raw.replace(/\/+$/, '');
  }
}

function canonicalIdentifier(value: unknown): string | null {
  if (typeof value === 'string' && value.trim()) return value.trim();
  if (typeof value === 'number' && Number.isFinite(value)) return String(value);
  return null;
}

function relevantSourceIndices(
  input: Record<string, unknown>,
  requirements: unknown[],
  tasks: unknown[],
  selected: ContextItem[],
): number[] {
  const sources = Array.isArray(input.sources) ? input.sources : [];
  const requirementURLsByID = new Map<string, Set<string>>();
  const allRequirementURLs = new Set<string>();
  for (const requirement of requirements) {
    const row = record(requirement);
    const url = normalizedURL(row.product_doc_url);
    if (!url) continue;
    allRequirementURLs.add(url);
    const id = canonicalIdentifier(row.id);
    if (!id) continue;
    const urls = requirementURLsByID.get(id) ?? new Set<string>();
    urls.add(url);
    requirementURLsByID.set(id, urls);
  }

  const relevantURLs = new Set<string>();
  const relevantRequirementIDs = new Set<string>();
  for (const item of selected) {
    if (item.collection === 'requirements') {
      const id = canonicalIdentifier(record(requirements[item.globalIndex]).id);
      if (id) relevantRequirementIDs.add(id);
      const url = normalizedURL(record(requirements[item.globalIndex]).product_doc_url);
      if (url) relevantURLs.add(url);
      continue;
    }
    const requirementID = canonicalIdentifier(record(tasks[item.globalIndex]).requirement_id);
    if (!requirementID) continue;
    relevantRequirementIDs.add(requirementID);
    for (const url of requirementURLsByID.get(requirementID) ?? []) relevantURLs.add(url);
  }

  return sources.flatMap((source, index) => {
    const row = record(source);
    if (Array.isArray(row.requirement_ids)) {
      return row.requirement_ids.some((id) => {
        const canonical = canonicalIdentifier(id);
        return canonical !== null && relevantRequirementIDs.has(canonical);
      }) ? [index] : [];
    }
    if (row.type === 'internal_document') return [];
    const url = normalizedURL(row.url);
    // A source that cannot be associated with any requirement is global input
    // and must remain visible to every batch.
    return !url || !allRequirementURLs.has(url) || relevantURLs.has(url) ? [index] : [];
  });
}

export function batchInput(
  input: Record<string, unknown>,
  context: Record<string, unknown>,
  requirements: unknown[],
  tasks: unknown[],
  selected: ContextItem[],
): Pick<QATestCaseBatch, 'input' | 'globalIndices'> {
  const requirementIndices = selected
    .filter((item) => item.collection === 'requirements')
    .map((item) => item.globalIndex);
  const taskIndices = selected
    .filter((item) => item.collection === 'tasks')
    .map((item) => item.globalIndex);
  const sources = Array.isArray(input.sources) ? input.sources : [];
  const sourceIndices = relevantSourceIndices(input, requirements, tasks, selected);
  return {
    input: {
      ...input,
      ...(Array.isArray(input.sources) && {
        sources: sourceIndices.map((index) => sources[index]),
      }),
      test_case_context: {
        ...context,
        requirements: requirementIndices.map((index) => requirements[index]),
        tasks: taskIndices.map((index) => tasks[index]),
        counts: {
          requirements: requirementIndices.length,
          tasks: taskIndices.length,
        },
      },
    },
    globalIndices: { requirements: requirementIndices, tasks: taskIndices, sources: sourceIndices },
  };
}
