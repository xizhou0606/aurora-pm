export type RequirementTableRow = {
  id: number
  title: string
  version: number
  priority?: string
  project_new_id: number
  version_task_new_id?: number | null
  product_doc_url?: string
  design_url?: string
  product_owner_user_id?: number | null
  product_owner_user_ids?: number[]
  design_owner_user_id?: number | null
  design_owner_user_ids?: number[]
  frontend_owner_user_id?: number | null
  frontend_owner_user_ids?: number[]
  backend_owner_user_id?: number | null
  backend_owner_user_ids?: number[]
  tester_user_ids?: number[]
  allowed_actions?: string[]
  task_count?: number
  total_task_count?: number
  completed_task_count?: number
  progress?: number
  workflow_instance_id?: number | null
  workflow_overview?: {
    instance_id?: number
    status?: string
    current_node_names?: string[]
    current_nodes?: Array<{ id?: string; name: string; overdue?: boolean }>
    completed_steps: number
    total_steps: number
  }
  deliverable_overview?: {
    requirement_document?: boolean
    requirement_document_url?: string
    design?: boolean
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
    test_cases?: boolean
    requirement_cover?: {
      status: 'pending' | 'ready' | 'failed'
      generation: number
      updated_at: string
    } | null
    test_case_artifact?: {
      id: number
      instance_id: number
      node_id: string
      kind: string
      name: string
      content_type: string
      size: number
      url?: string
      requirement_id?: number
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
      url?: string
      requirement_id?: number
      ai_run_id?: number | null
      created_at: string
    }>
  }
}