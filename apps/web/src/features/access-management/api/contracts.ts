export type AccessRoleDTO = {
  id?: unknown
  code?: unknown
  name?: unknown
  description?: unknown
  enabled?: unknown
  system_managed?: unknown
  permission_codes?: unknown
  action_codes?: unknown
}

export type AccessGroupDTO = AccessRoleDTO & {
  is_default?: unknown
}

export type ManagedUserDTO = {
  id?: unknown
  name?: unknown
  display_name?: unknown
  username?: unknown
  email?: unknown
  avatar_url?: unknown
  status?: unknown
  group_id?: unknown
  role_ids?: unknown
  group?: unknown
  roles?: unknown
}

export type AccessCapabilitiesDTO = {
  view?: unknown
  approve?: unknown
  ban?: unknown
  manage_assignments?: unknown
  manage_permissions?: unknown
}

export type PermissionNodeDTO = {
  code?: unknown
  name?: unknown
  node_type?: unknown
  type?: unknown
  grantable?: unknown
  tools?: unknown
  object_policy?: unknown
  children?: unknown
}
