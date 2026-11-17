export type MutationState =
  | { status: 'unavailable'; reason: 'unsupported' | 'resource-unavailable' }
  | { status: 'idle' }
  | { status: 'pending'; action: string }
  | { status: 'error'; action: string; message: string }
  | { status: 'success'; action: string }
  | {
    status: 'partial'
    action: string
    succeeded: number
    failures: readonly { id: string | number; code: string; message: string }[]
  }

export type WorkItemPatch = {
  title?: string
  status?: string
  priority?: string
  startsAt?: string
  dueAt?: string
}
export type WorkItemDetailPatch = {
  title?: string
  description?: string
  status?: string
  priority?: string
  taskClientId?: number | null
  assigneeId?: number | null
  startsAt?: string | null
  dueAt?: string | null
  estimatedMinutes?: number | null
}

export function isValidEstimatedMinutes(value: number | null | undefined): boolean {
  return value === undefined || value === null
    || (Number.isInteger(value) && value >= 0 && value <= 999_960)
}
export type BatchMutationResult = {
  succeeded: number
  failures: readonly { id: string | number; code: string; message: string }[]
}
