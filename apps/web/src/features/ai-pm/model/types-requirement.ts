import type { WorkflowAssignee } from './types-workflow'

// WorkflowNodeArtifact is defined here because Requirement's deliverable_overview
// references it. This avoids a circular import with types-workflow.ts
export type WorkflowNodeArtifact = {
  id: number; instance_id: number; node_id: string
  kind: 'uploaded' | 'ai_generated' | 'test_report'
  name: string; content_type: string; size: number; url: string
  source?: 'file' | 'link'; ai_run_id?: number
  uploader_user_id: number; created_at: string
}

export type Requirement = {
  id: number; workspace_id: number; team_id: number; project_new_id: number
  version_task_new_id: number | null; version: number
  workflow_instance_id?: number | null; title: string; description?: string
  product_doc_url?: string; product_owner_user_id?: number | null
  product_owner_user_ids?: number[]; design_url?: string
  design_owner_user_id?: number | null; design_owner_user_ids?: number[]
  frontend_owner_user_id?: number | null; frontend_owner_user_ids?: number[]
  backend_owner_user_id?: number | null; backend_owner_user_ids?: number[]
  tester_user_ids?: number[]; acceptance_criteria?: string; priority: string
  status: string; task_count: number; completed_task_count?: number
  total_task_count?: number; progress: number
  workflow_overview?: {
    instance_id: number; status: string; current_node_names: string[]
    current_nodes?: Array<{ id?: string; name: string; planned_end_at?: string; overdue?: boolean }>
    completed_steps: number; total_steps: number
  }
  deliverable_overview?: {
    requirement_document: boolean; requirement_document_url?: string; design: boolean
    design_url?: string; design_urls?: string[]
    design_artifacts?: Array<{
      id?: number; name: string; content_type?: string; size?: number; url?: string; created_at?: string
    }>
    test_cases: boolean
    test_case_artifact?: Pick<WorkflowNodeArtifact, 'id' | 'instance_id' | 'node_id' | 'kind' | 'name' | 'content_type' | 'size' | 'ai_run_id' | 'created_at'> & { url?: string; requirement_id?: number } | null
    test_case_artifacts?: Array<Pick<WorkflowNodeArtifact, 'id' | 'instance_id' | 'node_id' | 'kind' | 'name' | 'content_type' | 'size' | 'ai_run_id' | 'created_at'> & { url?: string; requirement_id?: number }>
    requirement_cover?: { status: 'pending' | 'ready' | 'failed'; generation: number; updated_at: string } | null
  }
  allowed_actions: string[]; created_at: string; updated_at: string
}

export type RequirementPatch = {
  id: number; expected_version: number
} & Partial<Pick<Requirement,
  | 'title' | 'description' | 'product_doc_url' | 'design_url'
  | 'product_owner_user_ids' | 'design_owner_user_ids'
  | 'frontend_owner_user_id' | 'frontend_owner_user_ids'
  | 'backend_owner_user_id' | 'backend_owner_user_ids'
  | 'tester_user_ids' | 'acceptance_criteria' | 'priority'
>>

export type ProjectOption = { id: number; name: string }
export type PlanUnitOption = { id: number; project_new_id: number; title: string; status: string }

export type RequirementPoolFilters = {
  q?: string; projectId?: string; planUnit?: string; status?: string; priority?: string
  currentNodeId?: string; productOwnerUserId?: string; designOwnerUserId?: string
  frontendOwnerUserId?: string; backendOwnerUserId?: string; testerUserId?: string
  limit: number | 'all'; offset: number
}

export type RequirementPoolSummary = { total: number; unscheduled: number; delivery: number }
export type RequirementPoolPage = {
  rows: Requirement[]
  users: WorkflowAssignee[]
  total: number
  summary: RequirementPoolSummary
}

export type RequirementMaterial = {
  id: number; requirement_id: number; kind: 'design' | 'test_case'; source: string
  name: string; content_type: string; size: number; url: string
  origin_url?: string; artifact_kind: 'uploaded' | 'ai_generated'
  ai_run_id?: number; uploader_user_id: number; created_at: string
}

export type TestingLibraryKind = 'test_case' | 'test_report'

export type TestingLibraryItem = {
  id: number; kind: TestingLibraryKind; name: string; content_type: string
  size: number; url: string; source?: 'file' | 'link'; created_at: string
  instance_id: number; node_id: string; subject_type: 'requirement' | 'plan_unit'
  subject_id: number; subject_title?: string; project_id?: number
  project_name?: string; plan_unit_id?: number; plan_unit_title?: string
  uploader_user_id?: number; uploader_name?: string; uploader_avatar_url?: string
}

export type TestingLibraryFilters = {
  kind: TestingLibraryKind; q?: string; projectId?: number; planUnitId?: number
}
