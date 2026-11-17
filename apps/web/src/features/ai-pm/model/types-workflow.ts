import type { PMSafeSchedule } from '@/shared/api'

export type TestCase = {
  caseId?: string; title: string; category: TestCaseCategory
  priority: 'P0' | 'P1' | 'P2' | 'P3'; trace?: string[]
  preconditions: string[]; testData?: string[]; steps: string[]
  expectedResult: string; expectedResults?: string[]
  notes?: string[]; evidence: string[]
}

export type TestCaseCategory =
  | 'positive' | 'negative' | 'boundary' | 'permission' | 'state'
  | 'duplicate_submit' | 'concurrency' | 'timeout' | 'retry' | 'rollback'
  | 'stale_data' | 'integration' | 'regression'

export type TestCaseDesign = {
  objective: string; scope: string[]; exclusions: string[]
  sourceInventory: Array<{
    id: string; kind: string; status: 'confirmed' | 'conflict' | 'assumption' | 'gap'
    summary: string; evidence: string[]
  }>
  assumptions: string[]; informationGaps: string[]
  coverageStrategy: {
    priorityLogic: string
    methods: Array<'equivalence_partitioning' | 'boundary_value' | 'decision_table' | 'state_transition' | 'pairwise' | 'user_journey' | 'error_guessing' | 'regression_history'>
    riskHotspots: Array<{ id: string; description: string; priority: 'P0' | 'P1' | 'P2' | 'P3'; evidence: string[] }>
  }
  traceability: Array<{ traceId: string; caseIds: string[]; coverageTypes: TestCaseCategory[]; gapNote: string }>
  smokeCaseIds: string[]; executionOrder: string[]
  automationCandidateCaseIds: string[]; releaseBlockingCaseIds: string[]
}

export { type WorkflowNodeArtifact, type RequirementMaterial } from './types-requirement'

export type AICapability = {
  key: string; name: string; description: string; subject_types: string[]
  workflow_subject_types?: string[]; allowed_fields: string[]
  mode: 'proposal' | 'analysis' | 'artifact'
}

export type AIRun = {
  id: number; capability: string; subject_type: string; subject_id: number
  subject_version: number; status: string; model: string; error_message?: string
  test_case_delivery?: { instance_id: number; node_id: string } | null
  input?: unknown
  output?: {
    summary: string
    changes: Array<{ field: string; value: unknown; reason: string }>
    items: Array<{ title: string; description: string; evidence: string[] }>
    risks: string[]
    testCases?: TestCase[]
    testCaseDesign?: TestCaseDesign | null
    skillProvenance?: {
      name: string; repository: string; path: string; commit: string
      packageSha256: string; mode: 'standard' | 'advanced'; resources: string[]
    } | null
    humanReviewRequired: boolean
    completenessReport?: {
      verdict: 'pass' | 'needs_revision' | 'blocked'
      score: number; threshold: number; canProceed: boolean
      checkedSubjectVersion: number
      dimensions: Array<{
        key: 'background' | 'goal' | 'scope' | 'boundaries' | 'exceptions' | 'dependencies' | 'acceptance_criteria'
        label: string; status: 'pass' | 'partial' | 'missing'
        score: number; maxScore: number
        findings: string[]; evidence: string[]; remediation: string[]
      }>
      missingRequiredFields: string[]; blockingIssues: string[]
      recommendations: string[]
    } | null
    workflowProposal?: {
      mode: 'replace' | 'merge'; explanation: string; warnings: string[]
      definition: WorkflowDefinition
    } | null
    taskPlan?: {
      chains: Array<{
        lane: 'frontend' | 'backend'; summary: string
        tasks: Array<{ title: string; description: string; acceptanceCriteria: string[] }>
      }>
    } | null
    createdTasks?: Array<{
      id: number; taskKey: string; title: string; lane: 'frontend' | 'backend'
      kind: 'task' | 'subtask'; parentId?: number
    }>
  }
  created_at: string; updated_at: string; started_at?: string
  completed_at?: string; applied_at?: string
}

export type WorkflowAIExecution = {
  run_id: number; status: string; model?: string; capability?: string
  error_message?: string; subject_version?: number
  started_at?: string; completed_at?: string; updated_at?: string
  output?: AIRun['output']
}

export type WorkflowNodeData = { name: string; nodeType: string; capability?: string; config?: WorkflowNodeConfig }
export type WorkflowNodeConfig = Record<string, unknown> & {
  assignee_user_id?: number
  requirement_intake_lock?: boolean
  requirement_stage?: string
  requirements_guard?: WorkflowRequirementsGuardConfig
}

export type WorkflowRequirementsGuardConfig = {
  schema_version: number
  source: 'template_nodes'
  scope: 'current_plan_requirements'
  bindings: Array<{
    requirement_template_version_id: number
    condition: {
      all_of?: Array<{ node_id: string; success_action: 'complete' | 'approve' }>
      any_of?: Array<{ node_id: string; success_action: 'complete' | 'approve' }>
    }
  }>
  aggregate: { mode: 'all' | 'any' | 'count' | 'ratio'; min_count?: number; min_ratio?: number }
  on_satisfied: 'enable_action'
  empty_scope: 'block'
  missing_binding: 'block'
}

export type WorkflowRequirementsGuardEvaluation = {
  state: 'satisfied' | 'unsatisfied' | 'unknown' | 'empty_scope'
  satisfied_count: number
  total_count: number
  unknown_count: number
  requirements: Array<{
    requirement_id: number
    template_version_id?: number
    bound: boolean
    state: string
    reason?: string
    targets?: Array<{ node_id: string; name?: string; state: string }>
  }>
  missing_binding_version_ids?: number[]
}

export type WorkflowAssignee = { id: number; username: string; display_name?: string; avatar_url?: string }
export type WorkflowNodeBatchUpdate = {
  node_id: string; planned_start_at?: string | null; expected_planned_start_at?: string | null
  planned_end_at?: string | null; expected_planned_end_at?: string | null; assignee_user_id?: number
}

export type WorkflowNodeAction =
  | 'complete' | 'approve' | 'reject' | 'rollback' | 'supplement_design'
  | 'retry_ai' | 'recheck_ai' | 'override_ai' | 'condition_yes' | 'condition_no'

export type WorkflowInstanceAction = WorkflowNodeAction | 'resume'

export type WorkflowInstanceNodeBase = {
  id: string; type: string; name: string; capability?: string
  config?: WorkflowNodeConfig; assignee_user_id?: number | null
  assignee_name?: string; can_act?: boolean; allowed_actions?: WorkflowNodeAction[]
  requires_requirement_lock_confirmation?: boolean
  requirement_intake_locked?: boolean
  requirements_guard?: WorkflowRequirementsGuardEvaluation
}

export type WorkflowDeliveryProgressMilestone = {
  key: string
  name: string
  weight: number
  is_acceptance?: boolean
  bindings: Array<{
    requirement_template_version_id: number
    condition: {
      all_of?: Array<{ node_id: string; success_action: 'complete' | 'approve' }>
      any_of?: Array<{ node_id: string; success_action: 'complete' | 'approve' }>
    }
  }>
}

export type WorkflowDeliveryProgressConfig = {
  schema_version: number
  milestones: WorkflowDeliveryProgressMilestone[]
}

export type WorkflowDefinition = {
  nodes: Array<{ id: string; type: string; name: string; capability?: string; config?: WorkflowNodeConfig }>
  edges: Array<{ id: string; source: string; target: string; kind?: string; condition?: string; loop?: boolean }>
  delivery_progress?: WorkflowDeliveryProgressConfig
}

export type WorkflowTemplate = {
  id: number; name: string; subject_type: 'requirement' | 'plan_unit'
  description?: string; definition?: WorkflowDefinition
  nodes?: Array<{ id: string; type: string; name: string }>
  published_version: number
  published_version_id?: number
  updated_at: string
}

export type WorkflowInstance = {
  id: number; template_id: number; template_name: string; template_version: number
  status: string; subject_schedule?: PMSafeSchedule
  subject_title?: string
  can_act?: boolean; can_manage_node_plans?: boolean
  allowed_actions?: WorkflowInstanceAction[]
  started_by_user_id?: number; subject_type: string; subject_id: number
  current_node_ids: string[]; current_nodes: WorkflowInstanceNodeBase[]
  timeline_nodes: Array<WorkflowInstanceNodeBase & {
    planned_start_at?: string | null; planned_end_at?: string | null
    actual_completed_at?: string | null
    status: 'pending' | 'active' | 'waiting' | 'completed' | 'failed' | 'rejected' | 'rolled_back' | 'skipped'
    parallel_completed?: number; parallel_total?: number
    ai_execution?: WorkflowAIExecution
  }>
  rollback_targets?: Array<{ id: string; name: string; type: string }>
  node_plans?: Record<string, { planned_start_at?: string | null; planned_end_at?: string | null; assignee_user_id?: number | null }>
  definition_edges: Array<{ id: string; source: string; target: string; kind?: string; condition?: string; loop?: boolean }>
  selected_node_ids?: string[]; selected_edge_ids?: string[]
  node_runs: Array<{ id: number; node_id: string; node_type: string; status: string; error_message?: string; started_at: string; completed_at?: string }>
  approvals: Array<{ id: number; status: string; comment?: string; approver_user_id?: number; decided_at?: string }>
  transition_history?: Array<{
    id: number; node_id: string; node_name: string; node_type: string
    action: 'started' | 'completed' | 'approved' | 'rejected' | 'failed' | 'rolled_back' | 'material_supplemented' | 'ai_started' | 'ai_completed' | 'ai_overridden' | 'condition_yes' | 'condition_no'
    status: string; operator_type: 'user' | 'system' | 'ai'
    operator_user_id?: number; operator_name: string; comment?: string
    error_message?: string; decision?: boolean
    materials?: Array<{ kind: 'design_image' | string; url: string }>
    target_node_id?: string; target_node_name?: string; occurred_at: string
  }>
  completed_steps: number; total_steps: number; progress: number
  started_at: string; completed_at?: string
}

export type WorkflowNodeOpinion = {
  id: number; workspace_id: number; instance_id: number; node_id: string
  author_user_id: number; author_display_name?: string; author_username?: string
  author_avatar_url?: string; body: string; created_at: string
  updated_at: string; edited?: boolean
}

export type WorkflowValidation = { valid: boolean; issues: string[] }