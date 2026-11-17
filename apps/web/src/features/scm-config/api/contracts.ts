// DTO contracts for the SCM contribution admin surface. The backend serializes
// timestamps as RFC3339 strings and never exposes connection tokens — only a
// masked suffix (token_masked).
export interface ScmConnectionDTO {
  id: number
  name: string
  provider: 'github' | 'gitlab'
  base_url?: string
  enabled: boolean
  token_masked: string
  version: number
  updated_by: number
  created_at: string
  updated_at: string
}

export interface ScmAccessibleRepoDTO {
  external_id: string
  path: string
  default_branch?: string
}

export interface ScmRepositoryListingDTO {
  repos: ScmAccessibleRepoDTO[]
  next_cursor?: string
  has_more: boolean
}

export interface ScmFilterRulesDTO {
  extra_lockfile_patterns?: string[]
  extra_generated_patterns?: string[]
  extra_vendor_patterns?: string[]
  count_as_source?: string[]
}

export interface ScmRepositoryDTO {
  id: number
  workspace_id: number
  connection_id: number
  external_id?: string
  path: string
  default_branch?: string
  filter_rules: ScmFilterRulesDTO
  project_set_id?: number | null
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
}

export interface ScmBindingDTO {
  id: number
  provider: 'github' | 'gitlab'
  key_kind: 'email' | 'login'
  author_key: string
  display_name?: string
  user_id?: number | null
  suggested_user_id?: number | null
  status: 'matched' | 'unmatched' | 'suggested' | 'bot'
  matched_via?: string
  confirmed_by?: number
  confirmed_at?: string
}

export interface ScmSettingsDTO {
  timezone: string
  coauthor_policy: string
  bot_patterns: string[]
  page_cap: number
  daily_sync_hour: number
  daily_sync_minute: number
  weekly_sync_weekday: number
  weekly_sync_hour: number
  weekly_sync_minute: number
  monthly_sync_day: number
  monthly_sync_hour: number
  monthly_sync_minute: number
  version: number
}

export interface ScmAccountDTO {
  login: string
  display_name?: string
}

// 一次手动触发的采集结果。采集失败是合法结果（fail-visible）：error 非空
// 时 commits/pull_requests 无意义，页面必须原样展示失败原因。
export interface ScmConnectionInput {
  name: string
  provider: 'github' | 'gitlab'
  base_url?: string
  token?: string
  enabled?: boolean
}

export interface ScmRepositoryInput {
  connection_id: number
  external_id?: string
  path: string
  default_branch?: string
  filter_rules?: ScmFilterRulesDTO
  project_set_id?: number | null
  enabled?: boolean
}

export interface ScmSettingsInput {
  timezone?: string
  bot_patterns?: string[]
  page_cap?: number
  // 0 是合法取值（每日 0 点触发），可选字段缺省 = 保持服务端现值。
  daily_sync_hour?: number
  daily_sync_minute?: number
  weekly_sync_weekday?: number
  weekly_sync_hour?: number
  weekly_sync_minute?: number
  monthly_sync_day?: number
  monthly_sync_hour?: number
  monthly_sync_minute?: number
}

export interface ScmConfigClient {
  listConnections: (signal?: AbortSignal) => Promise<ScmConnectionDTO[]>
  createConnection: (body: ScmConnectionInput) => Promise<ScmConnectionDTO>
  updateConnection: (id: number, body: ScmConnectionInput) => Promise<ScmConnectionDTO>
  deleteConnection: (id: number) => Promise<void>
  testConnection: (id: number) => Promise<ScmAccountDTO>
  listAccessibleRepos: (id: number, cursor?: string, signal?: AbortSignal) => Promise<ScmRepositoryListingDTO>
  listRepositories: (signal?: AbortSignal) => Promise<ScmRepositoryDTO[]>
  createRepository: (body: ScmRepositoryInput) => Promise<ScmRepositoryDTO>
  updateRepository: (id: number, body: ScmRepositoryInput) => Promise<ScmRepositoryDTO>
  deleteRepository: (id: number) => Promise<void>
  listBindings: (status?: string, signal?: AbortSignal) => Promise<ScmBindingDTO[]>
  confirmBinding: (id: number, userId: number) => Promise<ScmBindingDTO>
  rejectBinding: (id: number) => Promise<ScmBindingDTO>
  getSettings: (signal?: AbortSignal) => Promise<ScmSettingsDTO>
  saveSettings: (body: ScmSettingsInput) => Promise<ScmSettingsDTO>
}
