export type ResourceAccessRuleDTO = {
  subject_kind?: unknown
  subject_id?: unknown
  level?: unknown
  include_descendants?: unknown
  subject?: unknown
}

export type ResourceAccessPolicyDTO = {
  resource_id?: unknown
  mode?: unknown
  workspace_level?: unknown
  rules?: unknown
  policy_revision?: unknown
  can_manage_access?: unknown
  effective_level?: unknown
  effective_sources?: unknown
  subject_capabilities?: unknown
}

export type ResourceAccessSubjectDTO = {
  id?: unknown
  kind?: unknown
  display_name?: unknown
  secondary_text?: unknown
}

export type ResourceAccessSubjectPageDTO = {
  items?: unknown
  total?: unknown
  has_more?: unknown
  capabilities?: unknown
  subject_capabilities?: unknown
}

export type ResourceShareLinkDTO = {
  updated_at?: unknown
  id?: unknown
  level?: unknown
  expires_at?: unknown
  created_at?: unknown
  revoked_at?: unknown
  secret?: unknown
}

export type ResourceAccessGrantDTO = {
  resource_id?: unknown
  access_secret?: unknown
  level?: unknown
  expires_at?: unknown
}
