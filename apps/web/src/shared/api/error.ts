const DEFAULT_ERROR_CODE = 'REQUEST_FAILED'
const MAX_ERROR_CODE_LENGTH = 128
const MACHINE_ERROR_CODE = /^[A-Z][A-Z0-9_]*$/

type ErrorResponse = {
  status?: unknown
  data?: {
    code?: unknown
    error?: unknown
    fields?: unknown
    schedule?: unknown
  }
}

function nestedErrorCode(value: unknown): unknown {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined
  return (value as { code?: unknown }).code
}

export type PMSafeApiFieldError = {
  client_id?: string
  field: string
}

export type PMSafeApiErrorDetails = {
  fields: PMSafeApiFieldError[]
}

export type PMSafeScheduleState = 'unscheduled' | 'partial' | 'scheduled' | 'conflict'

export type PMSafeScheduleBoundary = {
  planned_start_at?: string | null
  planned_end_at?: string | null
}

export type PMSafeScheduleConflict = {
  code: string
  subject_type?: 'project_set' | 'plan_unit' | 'requirement'
  subject_id?: number
  node_id?: string
  related_node_id?: string
  field?: string
  boundary?: string
  value?: string
}

export type PMSafeSchedule = {
  state: PMSafeScheduleState
  can_confirm?: boolean
  subject?: PMSafeScheduleBoundary
  constraints?: PMSafeScheduleBoundary
  suggested?: PMSafeScheduleBoundary
  actual_dates_independent?: boolean
  conflicts: PMSafeScheduleConflict[]
}

export class PMSafeApiError extends Error {
  readonly status: number | undefined
  readonly code: string
  readonly aborted: boolean
  readonly details: PMSafeApiErrorDetails | undefined
  readonly schedule: PMSafeSchedule | undefined

  constructor(
    code = DEFAULT_ERROR_CODE,
    status?: number,
    aborted = false,
    details?: PMSafeApiErrorDetails,
    schedule?: PMSafeSchedule,
  ) {
    super('PM API request failed')
    this.name = 'PMSafeApiError'
    this.code = machineCode(code) ?? DEFAULT_ERROR_CODE
    this.status = normalizedStatus(status)
    this.aborted = aborted
    this.details = details
    this.schedule = schedule
  }
}

function safeISODate(value: unknown): string | undefined {
  const token = typeof value === 'string' ? value.trim() : ''
  if (
    !token ||
    token.length > 40 ||
    !/^\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2}))?$/.test(token)
  ) return undefined
  const day = token.slice(0, 10)
  const parsedDay = new Date(`${day}T00:00:00Z`)
  if (Number.isNaN(parsedDay.getTime()) || parsedDay.toISOString().slice(0, 10) !== day) return undefined
  if (token.length > 10 && Number.isNaN(Date.parse(token))) return undefined
  return token
}

function safeScheduleBoundary(value: unknown): PMSafeScheduleBoundary | undefined {
  if (!value || typeof value !== 'object') return undefined
  const source = value as { planned_start_at?: unknown; planned_end_at?: unknown }
  const start = source.planned_start_at === null ? null : safeISODate(source.planned_start_at)
  const end = source.planned_end_at === null ? null : safeISODate(source.planned_end_at)
  if (start === undefined && end === undefined) return undefined
  return {
    ...(start !== undefined ? { planned_start_at: start } : {}),
    ...(end !== undefined ? { planned_end_at: end } : {}),
  }
}

function safeSchedule(value: unknown): PMSafeSchedule | undefined {
  if (!value || typeof value !== 'object') return undefined
  const source = value as {
    state?: unknown
    subject?: unknown
    constraints?: unknown
    suggested?: unknown
    actual_dates_independent?: unknown
    can_confirm?: unknown
    conflicts?: unknown
  }
  const state = typeof source.state === 'string' && (
    source.state === 'unscheduled' ||
    source.state === 'partial' ||
    source.state === 'scheduled' ||
    source.state === 'conflict'
  ) ? source.state : undefined
  if (!state) return undefined
  const subject = safeScheduleBoundary(source.subject)
  const constraints = safeScheduleBoundary(source.constraints)
  const suggested = safeScheduleBoundary(source.suggested)
  const conflicts = Array.isArray(source.conflicts) ? source.conflicts.slice(0, 100).flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const conflict = entry as {
      code?: unknown
      subject_type?: unknown
      subject_id?: unknown
      node_id?: unknown
      related_node_id?: unknown
      field?: unknown
      boundary?: unknown
      value?: unknown
    }
    const code = machineCode(conflict.code)
    if (!code) return []
    const subjectType: PMSafeScheduleConflict['subject_type'] = conflict.subject_type === 'project_set' || conflict.subject_type === 'plan_unit' || conflict.subject_type === 'requirement'
      ? conflict.subject_type
      : undefined
    const subjectId = Number(conflict.subject_id)
    const safeSubjectId = Number.isSafeInteger(subjectId) && subjectId > 0 ? subjectId : undefined
    const nodeId = safeFieldToken(conflict.node_id, 128)
    const relatedNodeId = safeFieldToken(conflict.related_node_id, 128)
    const field = safeFieldToken(conflict.field, 64)
    const boundary = safeISODate(conflict.boundary)
    const dateValue = safeISODate(conflict.value)
    return [{
      code,
      ...(subjectType ? { subject_type: subjectType } : {}),
      ...(safeSubjectId ? { subject_id: safeSubjectId } : {}),
      ...(nodeId ? { node_id: nodeId } : {}),
      ...(relatedNodeId ? { related_node_id: relatedNodeId } : {}),
      ...(field ? { field } : {}),
      ...(boundary ? { boundary } : {}),
      ...(dateValue ? { value: dateValue } : {}),
    }]
  }) : []
  return {
    state,
    ...(typeof source.can_confirm === 'boolean' ? { can_confirm: source.can_confirm } : {}),
    ...(subject ? { subject } : {}),
    ...(constraints ? { constraints } : {}),
    ...(suggested ? { suggested } : {}),
    ...(typeof source.actual_dates_independent === 'boolean'
      ? { actual_dates_independent: source.actual_dates_independent }
      : {}),
    conflicts,
  }
}

function safeFieldToken(value: unknown, maxLength: number): string | undefined {
  const token = typeof value === 'string' ? value.trim() : ''
  return token && token.length <= maxLength && /^[A-Za-z0-9_.:-]+$/.test(token)
    ? token
    : undefined
}

function safeErrorDetails(value: unknown): PMSafeApiErrorDetails | undefined {
  if (!Array.isArray(value)) return undefined
  const fields = value.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const source = entry as { client_id?: unknown; field?: unknown }
    const field = safeFieldToken(source.field, 64)
    if (!field) return []
    const clientId = safeFieldToken(source.client_id, 128)
    return [{ ...(clientId ? { client_id: clientId } : {}), field }]
  })
  return fields.length ? { fields } : undefined
}

function machineCode(value: unknown): string | undefined {
  const code = typeof value === 'string' ? value.trim() : ''
  return (
    code &&
    code.length <= MAX_ERROR_CODE_LENGTH &&
    MACHINE_ERROR_CODE.test(code)
  )
    ? code
    : undefined
}

function normalizedStatus(value: unknown): number | undefined {
  const status = Number(value)
  return Number.isInteger(status) && status >= 400 && status <= 599
    ? status
    : undefined
}

export function toSafeApiError(
  error: unknown,
  fallbackCode = DEFAULT_ERROR_CODE,
): PMSafeApiError {
  if (error instanceof PMSafeApiError) return error

  const requestError = error as { code?: unknown; name?: unknown } | null
  const aborted = requestError?.code === 'ERR_CANCELED' ||
    requestError?.name === 'CanceledError' ||
    requestError?.name === 'AbortError'
  const response = (error as { response?: ErrorResponse } | null)?.response
  const code =
    machineCode(response?.data?.code) ??
    machineCode(response?.data?.error) ??
    machineCode(nestedErrorCode(response?.data?.error)) ??
    machineCode(fallbackCode) ??
    DEFAULT_ERROR_CODE
  return new PMSafeApiError(
    aborted ? 'REQUEST_ABORTED' : code,
    normalizedStatus(response?.status),
    aborted,
    safeErrorDetails(response?.data?.fields),
    safeSchedule(response?.data?.schedule),
  )
}

export function isSafeApiError(error: unknown): error is PMSafeApiError {
  return error instanceof PMSafeApiError
}

/**
 * Cached business data may only survive a background refresh when the request
 * failed at the transport layer or with a server error. Authentication,
 * authorization, contract and cancellation failures must fail closed instead
 * of leaving previously authorized data visible.
 */
export function canRetainStaleApiData(error: unknown): boolean {
  return isSafeApiError(error)
    && !error.aborted
    && (error.status === undefined || error.status >= 500)
}

/**
 * Returns true only when a module cannot safely continue on its native path.
 * Controlled 4xx responses and request cancellation stay inside the native
 * state machine; transport and 5xx failures may use the module's compatibility
 * fallback.
 */
export function shouldFallbackModuleApi(error: unknown): boolean {
  const requestError = error as { code?: unknown; name?: unknown } | null
  if (
    requestError?.code === 'ERR_CANCELED' ||
    requestError?.name === 'CanceledError' ||
    requestError?.name === 'AbortError'
  ) {
    return false
  }
  if (!isSafeApiError(error)) return true
  return canRetainStaleApiData(error)
}
