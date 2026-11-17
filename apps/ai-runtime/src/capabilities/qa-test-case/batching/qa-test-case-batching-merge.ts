import { createHash } from 'node:crypto';
import { aiOutputSchema, type AIOutput } from '../../../shared/ai/schema.js';
import type { CompiledQATestCaseGlobalPlan } from '../global-plan/qa-test-case-global-plan-schema.js';
import type { ContextCollection, QATestCaseBatch, QATestCaseBatchOutput } from './qa-test-case-batching-input.js';

function batchPrefix(batchIndex: number): string {
  return `B${String(batchIndex + 1).padStart(2, '0')}`;
}

function namespaceDesignID(batchIndex: number, value: string): string {
  const namespaced = `${batchPrefix(batchIndex)}-${value}`;
  if (namespaced.length <= 100) return namespaced;
  const digest = createHash('sha256').update(namespaced).digest('hex').slice(0, 12);
  return `${namespaced.slice(0, 87)}-${digest}`;
}

function namespaceCaseID(batchIndex: number, value: string): string {
  return `TC-${batchPrefix(batchIndex)}-${value.slice(3)}`;
}

function rewriteEvidencePath(value: string, batch: QATestCaseBatch): string {
  const sourceMatch = value.match(/^sources(?:\[(\d+)\]|\.(\d+))(?=\.|$)/);
  if (sourceMatch) {
    const localIndex = Number(sourceMatch[1] ?? sourceMatch[2]);
    const globalIndex = batch.globalIndices.sources[localIndex];
    if (globalIndex === undefined) {
      throw new Error(`test case evidence references missing batch source: sources[${localIndex}]`);
    }
    return `sources.${globalIndex}${value.slice(sourceMatch[0].length)}`;
  }
  const match = value.match(/^test_case_context\.(requirements|tasks)(?:\[(\d+)\]|\.(\d+))(?=\.|$)/);
  if (!match) return value;
  const collection = match[1] as ContextCollection;
  const localIndex = Number(match[2] ?? match[3]);
  const globalIndex = batch.globalIndices[collection][localIndex];
  if (globalIndex === undefined) {
    throw new Error(`test case evidence references missing batch item: ${collection}[${localIndex}]`);
  }
  return `test_case_context.${collection}.${globalIndex}${value.slice(match[0].length)}`;
}

function namespaceBatchOutput(batch: QATestCaseBatch, output: AIOutput): AIOutput {
  const design = output.testCaseDesign;
  if (!design) throw new Error(`test case batch ${batch.index + 1} is missing design metadata`);
  const caseIDs = new Map(output.testCases.map((testCase) => [
    testCase.caseId,
    namespaceCaseID(batch.index, testCase.caseId),
  ]));
  const caseID = (value: string) => {
    const resolved = caseIDs.get(value);
    if (!resolved) throw new Error(`test case batch ${batch.index + 1} references missing case: ${value}`);
    return resolved;
  };
  const designID = (value: string) => namespaceDesignID(batch.index, value);
  const evidence = (items: string[]) => items.map((item) => rewriteEvidencePath(item, batch));

  return aiOutputSchema.parse({
    ...output,
    skillProvenance: undefined,
    testCases: output.testCases.map((testCase) => ({
      ...testCase,
      caseId: caseID(testCase.caseId),
      trace: testCase.trace.map(designID),
      evidence: evidence(testCase.evidence),
    })),
    testCaseDesign: {
      ...design,
      sourceInventory: design.sourceInventory.map((source) => ({
        ...source,
        id: designID(source.id),
        evidence: evidence(source.evidence),
      })),
      coverageStrategy: {
        ...design.coverageStrategy,
        riskHotspots: design.coverageStrategy.riskHotspots.map((risk) => ({
          ...risk,
          id: designID(risk.id),
          evidence: evidence(risk.evidence),
        })),
      },
      traceability: design.traceability.map((row) => ({
        ...row,
        traceId: designID(row.traceId),
        caseIds: row.caseIds.map(caseID),
      })),
      smokeCaseIds: design.smokeCaseIds.map(caseID),
      executionOrder: design.executionOrder.map(caseID),
      automationCandidateCaseIds: design.automationCandidateCaseIds.map(caseID),
      releaseBlockingCaseIds: design.releaseBlockingCaseIds.map(caseID),
    },
  });
}

function unique<T>(values: T[]): T[] {
  return [...new Set(values)];
}

function mergeNarrative(values: string[], maximum: number, field: string): string {
  const merged = unique(values.map((value) => value.trim()).filter(Boolean)).join('\n');
  if (merged.length > maximum) {
    throw new Error(`merged test case ${field} exceeds ${maximum} characters; split the input into smaller runs`);
  }
  return merged;
}

/** Merge validated batches plus an optional model-selected cross-batch plan. */
export function mergeQATestCaseBatchOutputs(
  rows: QATestCaseBatchOutput[],
  globalPlan?: CompiledQATestCaseGlobalPlan,
): AIOutput {
  if (!rows.length) throw new Error('test case generation returned no batches');
  if (rows.length === 1 && !globalPlan) return rows[0].output;
  const ordered = [...rows].sort((left, right) => left.batch.index - right.batch.index);
  const outputs = ordered.map(({ batch, output }) => rows.length === 1
    ? output
    : namespaceBatchOutput(batch, output));
  const designs = outputs.map((output) => output.testCaseDesign!);
  const titleCounts = new Map<string, number>();
  for (const output of outputs) {
    for (const testCase of output.testCases) {
      titleCounts.set(testCase.title, (titleCounts.get(testCase.title) ?? 0) + 1);
    }
  }
  const batchCases = outputs.flatMap((output, batchIndex) => output.testCases.map((testCase) => ({
    ...testCase,
    title: (titleCounts.get(testCase.title) ?? 0) > 1
      ? `[${batchPrefix(ordered[batchIndex].batch.index)}] ${testCase.title}`
      : testCase.title,
  })));
  const globalCases = (globalPlan?.cases ?? []).map((testCase, index) => {
    const caseId = `TC-GLOBAL-${String(index + 1).padStart(3, '0')}`;
    return {
      caseId,
      title: testCase.title,
      category: testCase.category,
      priority: testCase.priority,
      trace: testCase.traceIds,
      preconditions: testCase.preconditions,
      testData: testCase.testData,
      steps: testCase.steps,
      expectedResults: testCase.expectedResults,
      expectedResult: testCase.expectedResults.join('\n'),
      notes: unique([
        ...testCase.notes,
        ...testCase.traceIds.filter((traceId) => traceId.includes('DERIVED'))
          .map((traceId) => `${traceId} 为派生标签`),
      ]),
      evidence: testCase.evidence,
    };
  });
  const beforeGlobalCases = globalCases.filter((_testCase, index) => (
    globalPlan?.cases[index].executionPosition === 'before_batches'
  ));
  const afterGlobalCases = globalCases.filter((_testCase, index) => (
    globalPlan?.cases[index].executionPosition === 'after_batches'
  ));
  const testCases = [...beforeGlobalCases, ...batchCases, ...afterGlobalCases];

  const globalTraceRows = new Map<string, { caseIds: string[]; gapNote: string }>();
  const interactionGaps = new Map((globalPlan?.interactions ?? []).map((interaction) => (
    [interaction.riskId, interaction.gapNote]
  )));
  globalCases.forEach((testCase, index) => {
    for (const traceId of globalPlan!.cases[index].traceIds) {
      const row = globalTraceRows.get(traceId) ?? {
        caseIds: [], gapNote: interactionGaps.get(traceId) ?? '',
      };
      row.caseIds.push(testCase.caseId);
      globalTraceRows.set(traceId, row);
    }
  });

  const globalFindings = globalPlan?.sourceFindings ?? [];
  const sourceInventory = designs.flatMap((design) => design.sourceInventory).map((source) => {
    const matches = globalFindings.filter((finding) => (
      finding.evidence.some((path) => source.evidence.includes(path))
    ));
    if (!matches.length) return source;
    const conflict = matches.find((finding) => finding.status === 'conflict');
    const selected = conflict ?? matches[0];
    return {
      ...source,
      status: selected.status,
      summary: unique([source.summary, ...matches.map((finding) => finding.summary)]).join('；'),
      evidence: unique([...source.evidence, ...matches.flatMap((finding) => finding.evidence)]),
    };
  });
  sourceInventory.push(...globalFindings.map((finding, index) => ({
    id: `SRC-GLOBAL-${String(index + 1).padStart(3, '0')}`,
    kind: 'cross_requirement_finding',
    status: finding.status,
    summary: finding.summary,
    evidence: finding.evidence,
  })));

  const globalSummary = globalPlan
    ? [globalPlan.summary, globalPlan.noCrossInteractionReason].filter(Boolean).join('\n')
    : '';
  const beforeGlobalIDs = beforeGlobalCases.map((testCase) => testCase.caseId);
  const afterGlobalIDs = afterGlobalCases.map((testCase) => testCase.caseId);
  const globalIDsForFlag = (flag: 'smoke' | 'automationCandidate' | 'releaseBlocking') => (
    globalCases.filter((_testCase, index) => globalPlan?.cases[index][flag]).map((testCase) => testCase.caseId)
  );

  return aiOutputSchema.parse({
    summary: unique([...outputs.map((output) => output.summary), globalSummary].filter(Boolean)).join('\n'),
    changes: [],
    items: [],
    risks: [],
    testCases,
    testCaseDesign: {
      objective: mergeNarrative([
        ...designs.map((design) => design.objective),
        ...(globalPlan ? [globalPlan.objective] : []),
      ], 1000, 'objective'),
      scope: unique(designs.flatMap((design) => design.scope)),
      exclusions: unique(designs.flatMap((design) => design.exclusions)),
      sourceInventory,
      assumptions: unique([
        ...designs.flatMap((design) => design.assumptions),
        ...globalFindings.filter((finding) => finding.status === 'assumption').map((finding) => finding.summary),
      ]),
      informationGaps: unique([
        ...designs.flatMap((design) => design.informationGaps),
        ...globalFindings.filter((finding) => finding.status === 'gap').map((finding) => finding.summary),
      ]),
      coverageStrategy: {
        priorityLogic: mergeNarrative(
          [
            ...designs.map((design) => design.coverageStrategy.priorityLogic),
            ...(globalPlan ? [globalPlan.priorityLogic] : []),
          ],
          2000,
          'priority logic',
        ),
        methods: unique([
          ...designs.flatMap((design) => design.coverageStrategy.methods),
          ...(globalPlan?.methods ?? []),
        ]),
        riskHotspots: [
          ...designs.flatMap((design) => design.coverageStrategy.riskHotspots),
          ...(globalPlan?.interactions ?? []).map((interaction) => ({
            id: interaction.riskId,
            description: interaction.description,
            priority: interaction.priority,
            evidence: interaction.evidence,
          })),
        ],
      },
      traceability: [
        ...designs.flatMap((design) => design.traceability),
        ...[...globalTraceRows].map(([traceId, row]) => ({
          traceId,
          caseIds: row.caseIds,
          coverageTypes: ['integration' as const],
          gapNote: row.gapNote,
        })),
      ],
      smokeCaseIds: [
        ...designs.flatMap((design) => design.smokeCaseIds),
        ...globalIDsForFlag('smoke'),
      ],
      executionOrder: [
        ...beforeGlobalIDs,
        ...designs.flatMap((design) => design.executionOrder),
        ...afterGlobalIDs,
      ],
      automationCandidateCaseIds: [
        ...designs.flatMap((design) => design.automationCandidateCaseIds),
        ...globalIDsForFlag('automationCandidate'),
      ],
      releaseBlockingCaseIds: [
        ...designs.flatMap((design) => design.releaseBlockingCaseIds),
        ...globalIDsForFlag('releaseBlocking'),
      ],
    },
    humanReviewRequired: true,
    completenessReport: null,
    workflowProposal: null,
  });
}
