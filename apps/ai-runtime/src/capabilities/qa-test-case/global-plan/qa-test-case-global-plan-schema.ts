import { z } from 'zod';

export const QA_TEST_CASE_GLOBAL_PLAN_REPLACEMENT_SENTINEL =
  '__QA_GLOBAL_PLAN_REPLACE_FROM_INPUT__';
const withoutReplacementSentinel = (value: string) => (
  !value.includes(QA_TEST_CASE_GLOBAL_PLAN_REPLACEMENT_SENTINEL)
);
const text = z.string().trim().min(1).refine(
  withoutReplacementSentinel,
  'global plan text must replace the fixed skeleton sentinel',
);
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
const relationshipRef = z.string().regex(
  /^(?:subject:description|source:(?:0|[1-9]\d*):content)$/,
  'relationship ref must select subject:description or source:N:content',
);
const refs = z.array(contextRef).min(1).max(16);

const sourceFindingSchema = z.object({
  status: z.enum(['conflict', 'assumption', 'gap']),
  summary: text.max(800),
  contextRefs: refs,
}).strict();

const coverageDecisionSchema = z.discriminatedUnion('status', [
  z.object({ status: z.literal('omitted'), reason: text.max(800) }).strict(),
  z.object({ status: z.literal('not_required') }).strict(),
]);

const actionSchema = z.object({
  step: text.max(800),
  expectedResult: text.max(800),
}).strict();

const globalCaseSchema = z.object({
  title: text.max(300),
  category: z.literal('integration'),
  priority,
  contextRefs: refs,
  preconditions: z.array(text.max(800)).max(12),
  testData: z.array(text.max(800)).max(12),
  actions: z.array(actionSchema).min(1).max(12),
  notes: z.array(text.max(800)).max(12),
  smoke: z.boolean(),
  automationCandidate: z.boolean(),
  releaseBlocking: z.boolean(),
  executionPosition: z.enum(['before_batches', 'after_batches']),
}).strict();

const interactionSchema = z.object({
  description: text.max(800),
  priority,
  relationshipRef,
  coverageDecision: coverageDecisionSchema,
  cases: z.array(globalCaseSchema).min(1).max(12),
}).strict().superRefine((interaction, context) => {
  const highRisk = interaction.priority === 'P0' || interaction.priority === 'P1';
  if (highRisk && interaction.coverageDecision.status !== 'omitted') {
    context.addIssue({
      code: 'custom', path: ['coverageDecision'],
      message: 'P0/P1 interaction must explicitly explain omitted category coverage',
    });
  }
  if (!highRisk && interaction.coverageDecision.status !== 'not_required') {
    context.addIssue({
      code: 'custom', path: ['coverageDecision'],
      message: 'P2/P3 interaction must use not_required coverage decision',
    });
  }
});

export const qaTestCaseGlobalPlanSchema = z.object({
  summary: text.max(800),
  objective: text.max(800),
  priorityLogic: text.max(1600),
  methods: z.array(method).min(1).max(8),
  sourceFindings: z.array(sourceFindingSchema).max(12),
  interactions: z.array(interactionSchema).max(12),
  noCrossInteractionReason: z.string().trim().max(800).refine(
    withoutReplacementSentinel,
    'global plan reason must replace the fixed skeleton sentinel',
  ),
}).strict().superRefine((plan, context) => {
  const hasInteractions = plan.interactions.length > 0;
  const caseCount = plan.interactions.reduce((total, interaction) => (
    total + interaction.cases.length
  ), 0);
  if (caseCount > 12) {
    context.addIssue({
      code: 'custom', path: ['interactions'],
      message: 'global plan must contain at most 12 nested cases',
    });
  }
  if (hasInteractions && plan.noCrossInteractionReason) {
    context.addIssue({
      code: 'custom', path: ['noCrossInteractionReason'],
      message: 'reason must be empty when cross-requirement interactions exist',
    });
  }
  if (!hasInteractions && !plan.noCrossInteractionReason) {
    context.addIssue({
      code: 'custom', path: ['noCrossInteractionReason'],
      message: 'reason is required when no cross-requirement interaction exists',
    });
  }
});

export type QATestCaseGlobalPlan = z.infer<typeof qaTestCaseGlobalPlanSchema>;
type GlobalInteraction = QATestCaseGlobalPlan['interactions'][number];
type GlobalCase = GlobalInteraction['cases'][number];

type CompiledRefs = { contextRefs: string[]; evidence: string[] };
type CompiledTraceRefs = CompiledRefs & { traceIds: string[] };
export type CompiledQATestCaseGlobalPlan = {
  summary: string;
  objective: string;
  priorityLogic: string;
  methods: QATestCaseGlobalPlan['methods'];
  sourceFindings: Array<QATestCaseGlobalPlan['sourceFindings'][number] & CompiledRefs>;
  interactions: Array<Omit<GlobalInteraction, 'coverageDecision' | 'cases'> & CompiledTraceRefs & {
    riskId: string;
    gapNote: string;
  }>;
  cases: Array<Omit<GlobalCase, 'actions' | 'contextRefs'> & CompiledTraceRefs & {
    steps: string[];
    expectedResults: string[];
  }>;
  noCrossInteractionReason: string;
};

export type QATestCaseGlobalPlanRefDiagnostic = {
  code: 'unavailable_context_ref' | 'unavailable_relationship_ref';
  path: string;
  kind: 'any' | 'relationship';
  message: string;
};
