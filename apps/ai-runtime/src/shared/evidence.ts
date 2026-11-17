export type EvidenceReference = {
  root: 'subject' | 'sources' | 'test_case_context';
  path: string;
  segments: string[];
  value: unknown;
};

const evidencePathPattern = /(?:^|[^a-zA-Z0-9_])((?:subject|sources|test_case_context)(?:(?:\[\d+\])|(?:\.[a-zA-Z_][a-zA-Z0-9_]*)|(?:\.\d+))+)/g;
const exactEvidencePathPattern = /^(?:subject|sources|test_case_context)(?:(?:\[\d+\])|(?:\.[a-zA-Z_][a-zA-Z0-9_]*)|(?:\.\d+))+$/;

function usableEvidenceValue(value: unknown): boolean {
  if (typeof value === 'string') return value.trim().length > 0;
  if (typeof value === 'number') return Number.isFinite(value);
  if (typeof value === 'boolean') return true;
  if (Array.isArray(value)) return value.length > 0;
  return Boolean(value && typeof value === 'object' && Object.keys(value as Record<string, unknown>).length > 0);
}

function resolveEvidencePath(input: Record<string, unknown>, path: string): EvidenceReference | null {
  const normalized = path.replace(/\[(\d+)\]/g, '.$1');
  const [root, ...segments] = normalized.split('.') as [EvidenceReference['root'], ...string[]];
  if (!['subject', 'sources', 'test_case_context'].includes(root) || segments.length === 0) return null;

  let current: unknown = input[root];
  if (root === 'sources') {
    const sourceIndex = Number(segments[0]);
    const sources = Array.isArray(current) ? current : [];
    const source = Number.isInteger(sourceIndex) ? sources[sourceIndex] : undefined;
    if (!source || typeof source !== 'object' || Array.isArray(source)) return null;
    if ((source as Record<string, unknown>).available === false) return null;
  }
  for (const segment of segments) {
    if (!current || typeof current !== 'object' || !Object.hasOwn(current, segment)) return null;
    current = (current as Record<string, unknown>)[segment];
  }
  if (!usableEvidenceValue(current)) return null;
  return { root, path: normalized, segments, value: current };
}

export function validEvidenceReferences(input: Record<string, unknown>, evidence: string): EvidenceReference[] | null {
  const paths = [...evidence.matchAll(evidencePathPattern)].map((match) => match[1]);
  if (paths.length === 0) return null;
  const references = paths.map((path) => resolveEvidencePath(input, path));
  if (references.some((reference) => reference === null)) return null;
  return references as EvidenceReference[];
}

export function evidenceReferencesInput(input: Record<string, unknown>, evidence: string): boolean {
  return validEvidenceReferences(input, evidence) !== null;
}

export function exactEvidencePathReference(
  input: Record<string, unknown>,
  evidence: string,
): EvidenceReference | null {
  return exactEvidencePathPattern.test(evidence) ? resolveEvidencePath(input, evidence) : null;
}

export function exactEvidencePathReferencesInput(input: Record<string, unknown>, evidence: string): boolean {
  return exactEvidencePathReference(input, evidence) !== null;
}
