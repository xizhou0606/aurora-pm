import type { PlatformUser } from '@/shared/api'
export type { PlatformUser }
import type { ScrumStatus, ScrumHealthStatus, ScrumProgressHealthFields, ScrumProjectNew, CreateScrumProjectNewPayload, CreateScrumProjectNewResult, PatchScrumProjectNewPayload, PatchScrumProjectNewResult, DeleteScrumProjectNewPayload, DeleteScrumProjectNewResult, ScrumVersionTaskNew, CreateScrumVersionTaskNewPayload, PublishedWorkflowTemplate, PublishedPlanUnitWorkflowTemplate, CreateScrumVersionTaskNewResult, PatchScrumVersionTaskNewPayload, PatchScrumVersionTaskNewResult, DeleteScrumVersionTaskNewPayload, DeleteScrumVersionTaskNewResult, MemberTask, ProjectMember, ProjectMembersPage, PlanUnitMembersSnapshot, PutMembersPayload, RemovalImpact, RemovalImpactItem } from './contracts-project'
export type { ScrumStatus, ScrumHealthStatus, ScrumProgressHealthFields, ScrumProjectNew, CreateScrumProjectNewPayload, CreateScrumProjectNewResult, PatchScrumProjectNewPayload, PatchScrumProjectNewResult, DeleteScrumProjectNewPayload, DeleteScrumProjectNewResult, ScrumVersionTaskNew, CreateScrumVersionTaskNewPayload, PublishedWorkflowTemplate, PublishedPlanUnitWorkflowTemplate, CreateScrumVersionTaskNewResult, PatchScrumVersionTaskNewPayload, PatchScrumVersionTaskNewResult, DeleteScrumVersionTaskNewPayload, DeleteScrumVersionTaskNewResult, MemberTask, ProjectMember, ProjectMembersPage, PlanUnitMembersSnapshot, PutMembersPayload, RemovalImpact, RemovalImpactItem } from './contracts-project'
export type { ScrumProjectDeliveryNodeType, ScrumProjectDeliveryNodeLabel, ScrumProjectDeliveryNode, PatchScrumProjectDeliveryNodePayload } from './contracts-delivery'

export type ScrumProductRequirementNew = {
  id: number
  version: number
  workspace_id?: number
  team_id: number
  project_new_id: number
  version_task_new_id: number
  title: string
  description?: string
  product_doc_url?: string
  product_owner_user_id?: number | null
  product_owner_user_ids?: number[]
  design_url?: string
  design_owner_user_id?: number | null
  design_owner_user_ids?: number[]
  frontend_owner_user_id?: number | null
  frontend_owner_user_ids?: number[]
  backend_owner_user_id?: number | null
  backend_owner_user_ids?: number[]
  tester_user_ids?: number[]
  developer_user_ids?: number[]
  acceptance_criteria?: string
  priority?: string
  status: ScrumStatus | string
  task_count?: number
  open_task_count?: number
  progress?: number
  completed_task_count?: number
  in_progress_task_count?: number
  todo_task_count?: number
  total_task_count?: number
  workflow_instance_id?: number | null
  workflow_overview?: {
    instance_id: number
    status: string
    current_node_names: string[]
    current_nodes: Array<{
      id?: string
      name: string
      planned_end_at?: string | null
      overdue: boolean
    }>
    completed_steps: number
    total_steps: number
  }
  deliverable_overview?: {
    requirement_document: boolean
    requirement_document_url?: string
    design: boolean
    design_url?: string
    design_urls?: string[]
    design_artifacts?: Array<{
      id?: number
      name: string
      content_type?: string
      size?: number
      url?: string
      created_at?: string
    }>
    test_cases: boolean
    test_case_artifact?: {
      id: number
      instance_id: number
      node_id: string
      kind: string
      name: string
      content_type: string
      size: number
      ai_run_id?: number | null
      created_at: string
    } | null
    test_case_artifacts?: Array<{
      id: number
      instance_id: number
      node_id: string
      kind: string
      name: string
      content_type: string
      size: number
      ai_run_id?: number | null
      created_at: string
    }>
  }
}

export type RequirementStatus =
  | 'backlog'
  | 'grooming'
  | 'development'
  | 'acceptance'
  | 'done'

export type PMContentPriority = 'low' | 'normal' | 'high' | 'urgent'
export type PMRequirementPriority = 'p0' | 'p1' | 'p2' | 'p3'

export type ListScrumRequirementsInput = {
  projectNewId?: number
  versionTaskNewId?: number
  q?: string
  status?: RequirementStatus
  priority?: PMRequirementPriority
  limit?: number
  offset?: number
}

export type CreateScrumProductRequirementPayload = {
  version_task_new_id: number
  workflow_template_id: number
  title: string
  product_doc_url?: string
  product_owner_user_id?: number | null
  design_url?: string
  design_owner_user_id?: number | null
  developer_user_ids?: number[]
  description?: string
  acceptance_criteria?: string
  priority?: string
}

export type CreateScrumProductRequirementResult = ScrumProductRequirementNew

export type ScrumProductRequirementPatch = {
  expected_version: number
  confirm_subject_schedule?: true
} & Partial<{
    version_task_new_id: number
    title: string
    description: string
    product_doc_url: string
    product_owner_user_id: number | null
    design_url: string
    design_owner_user_id: number | null
    frontend_owner_user_id: number | null
    backend_owner_user_id: number | null
    developer_user_ids: number[]
    acceptance_criteria: string
    priority: string
  }>

export type PatchScrumProductRequirementResult = ScrumProductRequirementNew

export type DeleteScrumProductRequirementResult = { deleted: true }

export type ProjectWorkTask = {
  id: number
  workspace_id?: number
  team_id: number
  project_set_id: number
  project_id: number
  requirement_id: number
  kind: 'task' | 'subtask' | string
  task_key?: string
  title: string
  description?: string
  status: ScrumStatus | string
  priority?: string
  assignee_id?: number | null
  estimated_minutes?: number
  starts_at?: string | null
  due_at?: string | null
}

export type ProjectWorkTaskStatus =
  | 'draft'
  | 'in_dev'
  | 'pending_test'
  | 'completed'
  | 'cancelled'

export type ListProjectWorkTasksInput = {
  projectNewId: number
  versionTaskNewId: number
  requirementNewId?: number
  limit?: number
  offset?: number
}

export type CreateProjectWorkTaskPayload = {
  requirement_id: number
  kind: 'task' | 'subtask'
  title: string
  description?: string
  status?: string
  priority?: string
  assignee_id?: number | null
  estimated_minutes?: number
  starts_at?: string | null
  due_at?: string | null
}

export type PatchProjectWorkTaskPayload = Partial<{
  title: string
  description: string
  assignee_id: number | null
  estimated_minutes: number
  starts_at: string | null
  due_at: string | null
  status: string
  priority: string
}>

export type CreateProjectWorkTaskResult = ProjectWorkTask
export type PatchProjectWorkTaskResult = ProjectWorkTask
export type DeleteProjectWorkTaskResult = { deleted: true }

export type ScrumBoardNew = {
  project?: ScrumProjectNew
  version_tasks: ScrumVersionTaskNew[]
  requirements: ScrumProductRequirementNew[]
  tasks: ProjectWorkTask[]
}

export type ScrumProjectAuditLogNew = {
  id: number
  created_at?: string | null
  user_id?: number
  username?: string
  role?: string
  method?: string
  status_code?: number
  resource?: string
  resource_id?: number
  action?: string
  target?: string
  detail?: string
}

export type ScrumProjectNewsNew = {
  id: number
  workspace_id: number
  project_set_id: number
  plan_unit_id: number
  user_id: number
  title: string
  content: string
  created_at: string
  updated_at: string
}

export type ListScrumProjectsNewInput = {
  teamId?: number
  q?: string
  limit?: number
  offset?: number
}

export type ScrumDeferredListInput = {
  projectNewId?: number
  versionTaskNewId?: number
  requirementNewId?: number
  limit?: number
  offset?: number
}

export type ScrumPagedRows<T> = {
  rows: T[]
  total: number
  users?: PlatformUser[]
}

export type ProjectRequirementScopeOptions = {
  projectSets: Array<Pick<ScrumProjectNew, 'id' | 'team_id' | 'name' | 'code'>>
  projects: Array<Pick<ScrumVersionTaskNew, 'id' | 'team_id' | 'project_new_id' | 'title'>>
  requirements: Array<
    Pick<
      ScrumProductRequirementNew,
      'id' | 'team_id' | 'project_new_id' | 'version_task_new_id' | 'title'
    >
  >
}

export type WorkspaceTeamOption = {
  id: number
  name: string
  description?: string
}

export type WorkspaceTeamMember = PlatformUser & {
  role?: string
}
