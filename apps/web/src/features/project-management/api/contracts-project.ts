export type { PlatformUser }

export type ScrumStatus =
  | 'active'
  | 'planning'
  | 'backlog'
  | 'grooming'
  | 'development'
  | 'review'
  | 'acceptance'
  | 'released'
  | 'archived'
  | 'todo'
  | 'in_progress'
  | 'done'

export type ScrumHealthStatus = 'not_assessed' | 'on_track' | 'at_risk' | 'off_track'

export type ScrumProgressHealthFields = {
  progress?: number
  completed_task_count?: number
  total_task_count?: number
  expected_progress?: number
  schedule_variance?: number
  health_status?: ScrumHealthStatus | string
  health_reason?: string
}

export type ScrumProjectNew = ScrumProgressHealthFields & {
 owner_name?: string
 created_by_name?: string
  id: number
  workspace_id?: number
  team_id: number
  name: string
  code?: string
  description?: string
  owner_user_id?: number | null
  starts_at?: string | null
  due_at?: string | null
  status: ScrumStatus | string
  version_task_count?: number
  requirement_count?: number
  scrum_task_count?: number
  open_scrum_task_count?: number
  current_plan_unit_title?: string
  current_plan_unit_type?: string
  risk_level?: string
  created_by_user_id?: number | null
}

export type CreateScrumProjectNewPayload = {
  team_id: number
  name: string
  owner_user_id?: number | null
  planned_start_at: string | null
  planned_end_at: string | null
  description?: string
  status: 'active' | 'archived'
}

export type CreateScrumProjectNewResult = ScrumProjectNew

export type PatchScrumProjectNewPayload = Partial<{
  name: string
  planned_start_at: string | null
  expected_planned_start_at: string | null
  planned_end_at: string | null
  expected_planned_end_at: string | null
  description: string
  status: 'active' | 'archived'
}>

export type PatchScrumProjectNewResult = ScrumProjectNew

export type DeleteScrumProjectNewPayload = {
  id: number
}

export type DeleteScrumProjectNewResult = {
  deleted: true
}

export type ScrumVersionTaskNew = ScrumProgressHealthFields & {
 owner_name?: string
  id: number
  workspace_id?: number
  team_id: number
  project_new_id: number
  title: string
  type?: 'project' | 'version' | string
  goal?: string
  owner_user_id?: number | null
  starts_at?: string | null
  due_at?: string | null
  design_freeze_due_at?: string | null
  development_complete_due_at?: string | null
  test_acceptance_due_at?: string | null
  delivery_nodes?: ScrumProjectDeliveryNode[]
  actual_start_at?: string | null
  actual_completed_at?: string | null
  risk_level?: string
  status: ScrumStatus | string
  workflow_instance_id?: number | null
  workflow_template_id?: number | null
  workflow_intake_pending?: boolean
  workflow_intake_node_id?: string
  requirement_count?: number
  requirements_locked?: boolean
  requirements_locked_at?: string | null
  delivery_progress_configured?: boolean
  delivery_progress?: number
  delivery_scope_count?: number
  delivery_accepted_count?: number
  delivery_acceptance_configured?: boolean
  delivery_unknown_count?: number
  delivery_milestones?: Array<{ key: string; name: string; weight: number; is_acceptance?: boolean; satisfied_count: number }>
  delivery_scope_revision?: number
  scrum_task_count?: number
  open_scrum_task_count?: number
  can_manage_participants?: boolean
}

export type CreateScrumVersionTaskNewPayload = {
  project_new_id: number
  title: string
  type: 'project' | 'version'
  owner_user_id: number | null
  planned_start_at: string | null
  planned_end_at: string | null
  actual_start_at: string | null
  actual_completed_at: string | null
  risk_level: 'low' | 'medium' | 'high'
  goal: string
  workflow_template_id: number
}

export type PublishedWorkflowTemplate = {
  id: number
  name: string
  published_version: number
}

export type PublishedPlanUnitWorkflowTemplate = PublishedWorkflowTemplate

export type CreateScrumVersionTaskNewResult = ScrumVersionTaskNew

export type PatchScrumVersionTaskNewPayload = Partial<{
  title: string
  type: 'project' | 'version'
  owner_user_id: number | null
  planned_start_at: string | null
  expected_planned_start_at: string | null
  planned_end_at: string | null
  expected_planned_end_at: string | null
  design_freeze_due_at: string | null
  expected_design_freeze_due_at: string | null
  development_complete_due_at: string | null
  expected_development_complete_due_at: string | null
  test_acceptance_due_at: string | null
  expected_test_acceptance_due_at: string | null
  actual_start_at: string | null
  expected_actual_start_at: string | null
  actual_completed_at: string | null
  expected_actual_completed_at: string | null
  risk_level: 'low' | 'medium' | 'high'
  goal: string
  delivery_nodes: PatchScrumProjectDeliveryNodePayload[]
}>

export type PatchScrumVersionTaskNewResult = ScrumVersionTaskNew

export type DeleteScrumVersionTaskNewPayload = {
  id: number
}

export type DeleteScrumVersionTaskNewResult = {
  deleted: true
}

import type { ScrumProjectDeliveryNode, PatchScrumProjectDeliveryNodePayload } from './contracts-delivery'
import type { PlatformUser } from '@/shared/api'

// Project member types for the participant API.
export type ProjectMember = {
  id: number
  username: string
  display_name: string
  email?: string
}

export type ProjectMembersPage = {
  items: ProjectMember[]
  page: number
  page_size: number
  total: number
  has_more: boolean
}

export type PlanUnitMembersSnapshot = {
  workspace_id: number
  plan_unit_id: number
  project_set_id: number
  owner_user_id: number | null
  participant_user_ids: number[]
  updated_at: string
  can_manage_participants: boolean
}

export type MemberTask = {
  id: number
  task_key: string
  title: string
  status: string
  assignee_id: number
  updated_at: string
}

export type RemovalImpact = {
  tasks?: MemberTask[]
  can_resolve_tasks?: boolean
  members_updated_at?: string
  open_task_count: number
  open_bug_count: number
  active_workflow_count: number
  plan_unit_owner_count: number
  responsible_items: RemovalImpactItem[]
}

export type RemovalImpactItem = {
  resource_type: string
  resource_id: number
  title: string
  status: string
  relation: string
}

export type PutMembersPayload = {
  owner_user_id: number
  user_ids: number[]
  reason?: string
  expected_updated_at?: string
  task_resolution?: { tasks: MemberTask[]; assignee_user_id: number | null }
}