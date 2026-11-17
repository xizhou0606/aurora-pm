import { exactEvidencePathReference, exactEvidencePathReferencesInput } from '../../shared/evidence.js';
import type { AIOutput } from '../../shared/ai/schema.js';
import { record } from './qa-test-case-normalize.js';
import { qaTestCaseRequiredCoverageTargets } from './draft/qa-test-case-draft-context.js';

export function isObservableTestCaseExpectedResult(value: string): boolean {
  return !/(?:系统正常|符合预期)/.test(value);
}

export function isReproducibleTestCaseSetup(value: string): boolean {
  return !/^(?:系统已有数据|准备测试数据)$/.test(value.trim());
}

function missingCaseIds(ids: string[], knownIds: Set<string>): string[] {
  return ids.filter((id) => !knownIds.has(id));
}

function requiredContextCoverageIssues(input: Record<string, unknown>, output: AIOutput): string[] {
  const context = record(input.test_case_context);
  if (context.coverageRequired !== true) return [];

  const evidencePaths = output.testCases.flatMap((testCase) => testCase.evidence.flatMap((evidence) => {
    const reference = exactEvidencePathReference(input, evidence);
    return reference ? [reference.path] : [];
  }));
  const issues: string[] = [];
  for (const testCase of output.testCases) {
    const coveredItems = new Set(testCase.evidence.flatMap((evidence) => {
      const path = exactEvidencePathReference(input, evidence)?.path ?? '';
      const match = /^test_case_context\.(requirements|tasks)\.(\d+)(?:\.|$)/.exec(path);
      return match ? [`${match[1]}.${match[2]}`] : [];
    }));
    if (coveredItems.size > 8) {
      issues.push(`test case covers too many unrelated context items: ${testCase.caseId}`);
    }
  }
  for (const target of qaTestCaseRequiredCoverageTargets(input)) {
    const match = /^(requirement|task):(\d+)$/.exec(target.targetRef);
    if (!match) continue;
    const collection = match[1] === 'requirement' ? 'requirements' : 'tasks';
    const itemPath = `test_case_context.${collection}.${match[2]}`;
    if (!evidencePaths.some((path) => path === itemPath || path.startsWith(`${itemPath}.`))) {
      issues.push(`test case context item is not covered: ${itemPath}`);
    }
  }
  return issues;
}

export function testCaseDesignIssues(input: Record<string, unknown>, output: AIOutput): string[] {
  const issues: string[] = [];
  const design = output.testCaseDesign;
  if (!design) return ['test case design metadata is missing'];

  const caseIds = output.testCases.map((testCase) => testCase.caseId);
  const knownIds = new Set(caseIds);
  if (knownIds.size !== caseIds.length) issues.push('test case IDs must be unique');
  const titles = output.testCases.map((testCase) => testCase.title.trim());
  if (new Set(titles).size !== titles.length) issues.push('test case titles must be unique');

  for (const testCase of output.testCases) {
    if (!testCase.evidence.length || testCase.evidence.some((item) => !exactEvidencePathReferencesInput(input, item))) {
      issues.push(`test case evidence is missing or invalid: ${testCase.caseId}`);
    }
    if (testCase.expectedResults.some((item) => !isObservableTestCaseExpectedResult(item))) {
      issues.push(`test case expected result is not observable: ${testCase.caseId}`);
    }
    if (testCase.steps.length !== testCase.expectedResults.length) {
      issues.push(`test case steps and expected results must align: ${testCase.caseId}`);
    }
    if ([...testCase.preconditions, ...testCase.testData]
      .some((item) => !isReproducibleTestCaseSetup(item))) {
      issues.push(`test case setup is not reproducible: ${testCase.caseId}`);
    }
  }

  const sourceIds = design.sourceInventory.map((source) => source.id);
  if (new Set(sourceIds).size !== sourceIds.length) issues.push('source inventory IDs must be unique');
  for (const source of design.sourceInventory) {
    if (source.evidence.some((item) => !exactEvidencePathReferencesInput(input, item))
      || (['confirmed', 'conflict'].includes(source.status) && !source.evidence.length)) {
      issues.push(`source inventory evidence is missing or invalid: ${source.id}`);
    }
    const distinctEvidencePaths = new Set(source.evidence.flatMap((item) => {
      const reference = exactEvidencePathReference(input, item);
      return reference ? [reference.path] : [];
    }));
    if (source.status === 'conflict' && distinctEvidencePaths.size < 2) {
      issues.push(`source conflict must cite at least two facts: ${source.id}`);
    }
  }
  const riskIds = design.coverageStrategy.riskHotspots.map((risk) => risk.id);
  if (new Set(riskIds).size !== riskIds.length) issues.push('risk hotspot IDs must be unique');
  for (const risk of design.coverageStrategy.riskHotspots) {
    if (!risk.evidence.length || risk.evidence.some((item) => !exactEvidencePathReferencesInput(input, item))) {
      issues.push(`risk hotspot evidence is missing or invalid: ${risk.id}`);
    }
  }

  const executionLists = [
    ['smoke subset', design.smokeCaseIds],
    ['execution order', design.executionOrder],
    ['automation candidates', design.automationCandidateCaseIds],
    ['release blockers', design.releaseBlockingCaseIds],
  ] as const;
  for (const [label, ids] of executionLists) {
    const missing = missingCaseIds(ids, knownIds);
    if (missing.length) issues.push(`${label} references missing cases: ${missing.join(', ')}`);
  }
  if (design.executionOrder.length !== caseIds.length
    || new Set(design.executionOrder).size !== caseIds.length
    || missingCaseIds(caseIds, new Set(design.executionOrder)).length) {
    issues.push('execution order must include every test case exactly once');
  }

  const traceIds = design.traceability.map((row) => row.traceId);
  if (new Set(traceIds).size !== traceIds.length) issues.push('traceability IDs must be unique');
  const traceRows = new Map(design.traceability.map((row) => [row.traceId, row]));
  for (const row of design.traceability) {
    const missing = missingCaseIds(row.caseIds, knownIds);
    if (missing.length) issues.push(`traceability references missing cases: ${row.traceId}`);
    if (new Set(row.caseIds).size !== row.caseIds.length) {
      issues.push(`traceability contains duplicate case IDs: ${row.traceId}`);
    }
    const rowCases = output.testCases.filter((testCase) => row.caseIds.includes(testCase.caseId));
    if (rowCases.some((testCase) => !testCase.trace.includes(row.traceId))) {
      issues.push(`traceability row is not declared by its cases: ${row.traceId}`);
    }
    const actualTypes = new Set(rowCases.map((testCase) => testCase.category));
    const declaredTypes = new Set(row.coverageTypes);
    if (declaredTypes.size !== row.coverageTypes.length
      || declaredTypes.size !== actualTypes.size
      || [...actualTypes].some((type) => !declaredTypes.has(type))) {
      issues.push(`traceability coverage types do not match cases: ${row.traceId}`);
    }
  }
  for (const testCase of output.testCases) {
    if (new Set(testCase.trace).size !== testCase.trace.length) {
      issues.push(`test case trace IDs must be unique: ${testCase.caseId}`);
    }
    if (testCase.trace.some((traceId) => !traceRows.get(traceId)?.caseIds.includes(testCase.caseId))) {
      issues.push(`test case trace is absent from the matrix: ${testCase.caseId}`);
    }
    if (![...traceRows.values()].some((row) => row.caseIds.includes(testCase.caseId))) {
      issues.push(`test case is absent from the traceability matrix: ${testCase.caseId}`);
    }
  }

  for (const risk of design.coverageStrategy.riskHotspots.filter((item) => ['P0', 'P1'].includes(item.priority))) {
    const row = traceRows.get(risk.id);
    if (!row) {
      issues.push(`high-risk hotspot is absent from traceability: ${risk.id}`);
      continue;
    }
    const categories = new Set(output.testCases
      .filter((testCase) => row.caseIds.includes(testCase.caseId))
      .map((testCase) => testCase.category));
    const missingProtection = ['positive', 'negative', 'boundary']
      .filter((category) => !categories.has(category as 'positive' | 'negative' | 'boundary'));
    if (missingProtection.length && !row.gapNote.trim()) {
      issues.push(`high-risk coverage omission is unexplained: ${risk.id} (${missingProtection.join(', ')})`);
    }
  }

  issues.push(...requiredContextCoverageIssues(input, output));

  return [...new Set(issues)];
}
