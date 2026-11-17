import type { QualityCapabilitySet } from './types-capability'

export type QualityMaterial = { url: string }

export type QualityDefectDomain = 'functional' | 'ui'

export type QualityBug = {
  id: number
  workspaceId: number
  bugKey?: string
  title: string
  status?: string
  priority?: string
  bugType?: string
  defectDomain?: QualityDefectDomain
  bugTypeLabel?: string
  dueDate?: string
  severity?: string
  foundEnvironment?: string
  operatingSystem?: string
  browser?: string
  projectSetId?: number
  projectId?: number
  requirementId?: number
  projectSetName?: string
  projectName?: string
  requirementTitle?: string
  reproductionSteps?: string
  expectedResult?: string
  actualResult?: string
  reproductionMaterials: QualityMaterial[]
  resolutionReason?: string
  resolutionReasonLabel?: string
  resolutionNote?: string
  resolutionMaterials: QualityMaterial[]
  assigneeUserId?: number
  assigneeName?: string
  reporterUserId?: number
  reporterName?: string
  reviewerUserId?: number
  reviewerName?: string
  currentOwnerName?: string
  createdAt?: string
  updatedAt?: string
  statusChangedAt?: string
  statusWaitSeconds?: number
  capabilities: QualityCapabilitySet
}

export type QualitySummary = {
  total: number
  open: number
  handling: number
  review: number
  closed: number
  highSeverity: number
}

export type QualityStatusFacets = {
  todo: number
  triage: number
  handling: number
  reopened: number
  review: number
  done: number
  canceled: number
  closed: number
}

export type QualityUser = {
  id: number
  name: string
  username?: string
  avatarUrl?: string
}

// The workspace user directory loads once and filters locally inside the
// selects; no server-side search pagination on this state.
export type QualityUsersState =
  | { status: 'loading'; rows: readonly QualityUser[] }
  | { status: 'error'; rows: readonly QualityUser[]; retry: () => Promise<void> }
  | { status: 'ready'; rows: readonly QualityUser[] }
