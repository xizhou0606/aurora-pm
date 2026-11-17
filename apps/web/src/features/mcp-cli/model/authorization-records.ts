import type { DataTableState } from '@/shared/ui'
export type AuthorizationRecord = {
  id: string
  user_code: string | null
  channel: 'cli' | 'mcp'
  client_name: string
  principal_id: number | null
  principal_name: string | null
  principal_username?: string | null
  grant_tools: string[]
  created_at: string
  decided_at: string | null
  expires_at: string
  status: 'pending' | 'approved' | 'denied' | 'expired' | 'revoked'
  code_valid: boolean
  credential_valid: boolean
  can_revoke: boolean
}
export type AuthorizationRecords = { items: AuthorizationRecord[]; total: number; page: number; page_size: number }

export function normalizeAuthorizationRecords(value: unknown): AuthorizationRecords | undefined {
  if (!value || typeof value !== 'object') return
  const data = value as Record<string, unknown>
  if (!Array.isArray(data.items) || !Number.isInteger(data.total) || Number(data.total) < 0
    || !Number.isInteger(data.page) || Number(data.page) < 1
    || !Number.isInteger(data.page_size) || Number(data.page_size) < 1) return
  const items: AuthorizationRecord[] = []
  for (const item of data.items) {
    if (!item || typeof item !== 'object') return
    const row = item as Record<string, unknown>
    if (typeof row.id !== 'string' || !row.id
      || !['cli', 'mcp'].includes(String(row.channel))
      || !['pending', 'approved', 'denied', 'expired', 'revoked'].includes(String(row.status))
      || typeof row.client_name !== 'string'
      || !Array.isArray(row.grant_tools) || !row.grant_tools.every((tool) => typeof tool === 'string')
      || typeof row.created_at !== 'string' || !Number.isFinite(Date.parse(row.created_at))
      || typeof row.expires_at !== 'string' || !Number.isFinite(Date.parse(row.expires_at))
      || !(row.decided_at === null || typeof row.decided_at === 'string' && Number.isFinite(Date.parse(row.decided_at)))
      || !(row.user_code === null || typeof row.user_code === 'string')
      || !(row.principal_id === null || Number.isInteger(row.principal_id) && Number(row.principal_id) > 0)
      || !(row.principal_name === null || typeof row.principal_name === 'string')
      || typeof row.credential_valid !== 'boolean' || typeof row.code_valid !== 'boolean' || typeof row.can_revoke !== 'boolean') return
    items.push(row as AuthorizationRecord)
  }
  return { items, total: Number(data.total), page: Number(data.page), page_size: Number(data.page_size) }
}

export type AuthorizationRecordsControl = {
  state: DataTableState<AuthorizationRecord>
  page: number
  pageSize: number
  total: number
  setPage: (page: number) => void
  setPageSize: (size: number) => void
  selected: AuthorizationRecord | null
  select: (record: AuthorizationRecord) => void
  close: () => void
  confirm: () => void
  pending: boolean
  error?: string
}
