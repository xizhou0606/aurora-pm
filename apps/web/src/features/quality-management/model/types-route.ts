export type QualityScope = {
  projectSetIds: number[]
  projectIds: number[]
  requirementIds: number[]
}

export type QualityRelationFilter = '' | 'reported' | 'assigned' | 'responsible' | 'resolved'
export type QualityDefectDomainFilter = '' | 'functional' | 'ui'

export type QualityFilters = {
  q: string
  status: string
  severities: string[]
  environments: string[]
  assigneeUserIds: number[]
  ownerUserIds: number[]
  reporterUserIds: number[]
  relation: QualityRelationFilter
  defectDomain: QualityDefectDomainFilter
  createdFrom: string
  createdTo: string
  sortBy: 'priority' | 'severity' | 'status' | 'created_at' | 'status_changed_at'
  sortOrder: 'asc' | 'desc'
}

export type QualityRouteState = {
  kind: 'list' | 'create' | 'detail'
  view: 'list' | 'dashboard'
  bugId?: number
  mode: 'view' | 'edit'
  from?: string
  scope: QualityScope
  filters: QualityFilters
  page: number
  pageSize: 10 | 20 | 50 | 100
  sourceExecutionId?: number
  sourceTestId?: number
  actualResult?: string
  invalidScope?: boolean
  needsRouteNormalization?: boolean
}
