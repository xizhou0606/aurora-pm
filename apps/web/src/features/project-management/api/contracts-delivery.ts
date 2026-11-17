export type ScrumProjectDeliveryNodeType =
  | 'product_review'
  | 'design_freeze'
  | 'development_complete'
  | 'acceptance'

export type ScrumProjectDeliveryNodeLabel = {
  id?: number
  client_key?: string
  name: string
  system_key?: ScrumProjectDeliveryNodeType | string | null
  sort_order: number
  deleted?: boolean
}

export type ScrumProjectDeliveryNode = {
  label_id?: number
  label_client_key?: string
  label_name?: string
  label_deleted?: boolean
  type?: ScrumProjectDeliveryNodeType | string
  due_at?: string | null
  actual_completed_at?: string | null
  owner_user_id?: number | null
}

export type PatchScrumProjectDeliveryNodePayload = {
  label_id: number
  due_at: string | null
  actual_completed_at: string | null
  owner_user_id: number | null
}