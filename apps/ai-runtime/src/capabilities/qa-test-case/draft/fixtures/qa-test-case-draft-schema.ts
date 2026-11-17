import { z } from 'zod';
import {
  isObservableTestCaseExpectedResult,
  isReproducibleTestCaseSetup,
} from '../../qa-test-case-issues.js';
import { testCaseCategorySchema } from '../../../../shared/ai/schema.js';

export {
  allowedQATestCaseContextRefs,
  qaTestCaseRequirementGroupLedger,
  type QATestCaseRequirementGroupLedgerEntry,
} from '../qa-test-case-draft-context.js';

const text = z.string().trim().min(1);
const priority = z.enum(['P0', 'P1', 'P2', 'P3']);
const method = z.enum([
  'equivalence_partitioning', 'boundary_value', 'decision_table',
  'state_transition', 'pairwise', 'user_journey', 'error_guessing',
  'regression_history',
]);
const contextRef = z.string().regex(
  /^(?:subject:(?:title|description)|source:(?:0|[1-9]\d*):content|requirement:(?:0|[1-9]\d*):(?:title|description|acceptance_criteria)|task:(?:0|[1-9]\d*):(?:title|description|status))$/,
  'context ref must select an allowed substantive input field',
);
const sourceTargetRef = z.string().regex(
  /^(?:subject|(?:source|requirement|task):(?:0|[1-9]\d*))$/,
  'source assessment target must select one input target',
);
export const qaTestCaseRiskIdSchema = z.string().regex(
  /^RISK-[A-Z0-9](?:[A-Z0-9-]{0,93}[A-Z0-9])?$/,
  'risk ID must use RISK-* uppercase format',
);
export const qaTestCaseCoverageDecisionSchema = z.discriminatedUnion('status', [
  z.object({ status: z.literal('complete') }).strict(),
  z.object({ status: z.literal('omitted'), reason: text.max(1000) }).strict(),
  z.object({ status: z.literal('not_required') }).strict(),
]);

const riskDraftSchema = z.object({
  riskId: qaTestCaseRiskIdSchema,
  description: text.max(1000),
  priority,
  contextRefs: z.array(contextRef).min(1),
  coverageDecision: qaTestCaseCoverageDecisionSchema,
}).strict();

const observableExpectedResult = text.refine(isObservableTestCaseExpectedResult, {
  message: 'expected result must be observable',
});
const reproducibleSetup = text.refine(isReproducibleTestCaseSetup, {
  message: 'test case setup must be reproducible',
});
const actionDraftSchema = z.object({
  step: text,
  expectedResult: observableExpectedResult,
}).strict();

const sourceAssessmentDraftSchema = z.object({
  targetRef: sourceTargetRef,
  status: z.enum(['confirmed', 'conflict', 'assumption', 'gap']),
  summary: text.max(1000),
  contextRefs: z.array(contextRef),
}).strict();

const riskLinkDraftSchema = z.object({ riskId: qaTestCaseRiskIdSchema, contextRef }).strict();

export const qaTestCaseCaseDraftSchema = z.object({
  title: text,
  category: testCaseCategorySchema,
  priority,
  contextRefs: z.array(contextRef),
  riskLinks: z.array(riskLinkDraftSchema).min(1),
  preconditions: z.array(reproducibleSetup),
  testData: z.array(reproducibleSetup),
  actions: z.array(actionDraftSchema).min(1),
  notes: z.array(text),
  smoke: z.boolean(),
  automationCandidate: z.boolean(),
  releaseBlocking: z.boolean(),
}).strict();

export const qaTestCaseDraftSchema = z.object({
  summary: text,
  objective: text.max(1000),
  scope: z.array(text).min(1),
  exclusions: z.array(text),
  assumptions: z.array(text),
  informationGaps: z.array(text),
  sourceAssessments: z.array(sourceAssessmentDraftSchema).min(1),
  priorityLogic: text.max(2000),
  methods: z.array(method).min(1),
  risks: z.array(riskDraftSchema).min(1),
  cases: z.array(qaTestCaseCaseDraftSchema).min(1),
}).strict();

export type QATestCaseDraft = z.infer<typeof qaTestCaseDraftSchema>;
export type QATestCaseCaseDraft = z.infer<typeof qaTestCaseCaseDraftSchema>;
export type QATestCaseCoverageDecision = z.infer<typeof qaTestCaseCoverageDecisionSchema>;

export type QATestCaseDraftContextReferenceDiagnostic = {
  code: 'unavailable_context_ref' | 'undeclared_risk_link_context_ref' | 'duplicate_context_ref';
  pathCategory:
    | 'source_assessment_context_refs'
    | 'risk_context_refs'
    | 'case_context_refs'
    | 'case_risk_links';
};

/**
 * This is a structural coverage diagnostic only. It deliberately retains
 * stable refs and the group ledger, never the underlying requirement/task text.
 */
export type QATestCaseTargetCoverageDiagnostic = {
  code: 'context_item_not_covered';
  targetRef: string;
  candidateContextRefs: string[];
  group: string;
  message: string;
};
