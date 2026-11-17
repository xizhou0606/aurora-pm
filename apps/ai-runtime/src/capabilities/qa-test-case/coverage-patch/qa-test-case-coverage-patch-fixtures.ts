import { compileQATestCaseDraft } from '../draft/qa-test-case-draft-compile.js';
import { QATestCaseDraftCompilationError } from '../draft/fixtures/qa-test-case-draft-error.js';
import type { QATestCaseCaseDraft, QATestCaseDraft } from '../draft/fixtures/qa-test-case-draft-schema.js';
import type { RiskCoverageDiagnostic } from '../draft/qa-test-case-draft-risk.js';
import type { QATestCaseCoveragePatch, QATestCaseMixedCoverageTargetPatch } from './qa-test-case-coverage-patch-schema.js';

export const riskId = 'RISK-PAYMENT';
export const contextRef = 'subject:title';

export function testCase(
  category: QATestCaseCaseDraft['category'],
  title = `${category} payment`,
  targetRiskId = riskId,
  targetRef = contextRef,
): QATestCaseCaseDraft {
  return {
    title,
    category,
    priority: 'P1',
    contextRefs: [contextRef],
    riskLinks: [{ riskId: targetRiskId, contextRef: targetRef }],
    preconditions: [],
    testData: [],
    actions: [{ step: `execute ${category}`, expectedResult: `${category} result is visible` }],
    notes: [],
    smoke: category === 'negative',
    automationCandidate: true,
    releaseBlocking: category !== 'rollback',
  };
}

export function draft(): QATestCaseDraft {
  return {
    summary: 'payment coverage draft',
    objective: 'cover payment risk',
    scope: ['payment'],
    exclusions: [], assumptions: [], informationGaps: [],
    sourceAssessments: [{
      targetRef: 'subject', status: 'confirmed', summary: 'payment subject',
      contextRefs: [contextRef],
    }],
    priorityLogic: 'P1 before lower priorities',
    methods: ['boundary_value'],
    risks: [{
      riskId,
      description: 'payment result can be wrong',
      priority: 'P1',
      contextRefs: [contextRef],
      coverageDecision: { status: 'complete' },
    }],
    cases: [testCase('negative'), testCase('rollback')],
  };
}

export function diagnostic(targetRiskId = riskId): RiskCoverageDiagnostic {
  return {
    code: 'risk_coverage_decision_mismatch',
    riskId: targetRiskId,
    priority: 'P1',
    actualCategories: ['negative'],
    missingCategories: ['positive', 'boundary'],
    expectedStatus: 'omitted',
    message: `risk ${targetRiskId} coverage decision must be omitted for missing categories: positive, boundary`,
  };
}

export function caseResolution(
  category: 'positive' | 'negative' | 'boundary',
  title = `${category} payment scenario`,
): Extract<
  QATestCaseCoveragePatch['repairs'][number]['resolutions'][number],
  { kind: 'case' }
> {
  return {
    category,
    kind: 'case',
    contextRef,
    title,
    step: `submit the ${category} payment request`,
    expectedResult: `the page displays the ${category} payment status field`,
  };
}

export function completePatch(): QATestCaseCoveragePatch {
  return {
    repairs: [{
      riskId,
      resolutions: [caseResolution('positive'), caseResolution('boundary')],
      omissionReason: null,
    }],
  };
}

export function mixedInput() {
  return {
    subject: { title: 'payment change' },
    test_case_context: {
      coverageRequired: true,
      requirements: [{ title: 'submit payment' }],
      tasks: [{ status: 'pending' }],
    },
  };
}

export function mixedDraft(): QATestCaseDraft {
  const value = draft();
  value.sourceAssessments.push(
    {
      targetRef: 'requirement:0', status: 'confirmed', summary: 'submit payment',
      contextRefs: ['requirement:0:title'],
    },
    {
      targetRef: 'task:0', status: 'confirmed', summary: 'payment task',
      contextRefs: ['task:0:status'],
    },
  );
  value.risks[0].contextRefs = ['requirement:0:title'];
  value.cases = [
    testCase('negative', 'negative payment', riskId, 'requirement:0:title'),
  ];
  value.cases[0].contextRefs = [];
  value.risks[0].coverageDecision = { status: 'complete' };
  value.risks.push({
    riskId: 'RISK-PAYMENT-TASK',
    description: 'payment task state can be stale',
    priority: 'P2',
    contextRefs: ['subject:title', 'task:0:status'],
    coverageDecision: { status: 'not_required' },
  });
  value.cases.push({
    ...testCase('rollback', 'rollback payment', 'RISK-PAYMENT-TASK', 'task:0:status'),
    priority: 'P2',
    releaseBlocking: false,
  });
  value.cases[1].contextRefs = ['subject:title'];
  value.cases[1].riskLinks = [{ riskId, contextRef: 'requirement:0:title' }];
  value.cases.push({
    ...testCase('rollback', 'task rollback payment', 'RISK-PAYMENT-TASK', 'subject:title'),
    priority: 'P2',
    contextRefs: ['subject:title'],
    releaseBlocking: false,
  });
  return value;
}

export function mixedFailure(value: QATestCaseDraft = mixedDraft()): QATestCaseDraftCompilationError {
  try {
    compileQATestCaseDraft(mixedInput(), value);
  } catch (error) {
    if (error instanceof QATestCaseDraftCompilationError) return error;
    throw error;
  }
  throw new Error('expected mixed coverage failure');
}

export function mixedPatch(): QATestCaseMixedCoverageTargetPatch {
  return {
    riskRepairs: [{
      riskId,
      resolutions: [
        {
          category: 'positive', kind: 'case', contextRef: 'requirement:0:title',
          title: 'submit valid payment',
          step: 'submit a payment with a valid amount',
          expectedResult: 'the payment row displays a completed state',
        },
        {
          category: 'boundary', kind: 'case', contextRef: 'requirement:0:title',
          title: 'submit minimum payment',
          step: 'submit the minimum allowed payment amount',
          expectedResult: 'the payment record displays the accepted minimum amount',
        },
      ],
      omissionReason: null,
    }],
    targetRepairs: [{
      targetRef: 'task:0',
      riskId: 'RISK-PAYMENT-TASK',
      contextRef: 'task:0:status',
      category: 'state',
      title: 'show pending payment task state',
      step: 'open the payment task after it enters pending state',
      expectedResult: 'the task row displays the pending state',
    }],
  };
}
