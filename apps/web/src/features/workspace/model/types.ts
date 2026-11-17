export type WorkspaceOption = {
  id: number
  name: string
  slug?: string
  description?: string
  is_default?: boolean
  enabled?: boolean
  my_role?: string
}

export type WorkspaceSelection =
  | {
      status: 'ok'
      workspaceId: number
      reason: 'current' | 'single' | 'default' | 'embedded'
    }
  | { status: 'pick' }
  | { status: 'empty' }

export type WorkspaceRouteResolution = WorkspaceSelection | { status: 'error' }
