import { platformApi, toSafeApiError, unwrapApiRows } from '@/shared/api'

export type ProjectOverviewBug = {
  requirementId: number
  severity: 'critical' | 'normal' | 'minor'
  count: number
}

function parseBug(value: unknown): ProjectOverviewBug | null {
  if (!value || typeof value !== 'object') return null
  const row = value as Record<string, unknown>
  const requirementId = Number(row.requirement_id)
  const count = Number(row.count)
  if (!Number.isInteger(requirementId) || requirementId <= 0) return null
  if (!Number.isInteger(count) || count <= 0) return null
  const severity = String(row.severity || '').toLowerCase()
  if (severity !== 'critical' && severity !== 'normal' && severity !== 'minor') return null
  return {
    requirementId,
    severity,
    count,
  }
}

export async function listProjectOverviewBugs(
  input: {
    projectSetId: number
    planUnitId?: number
  },
  signal?: AbortSignal,
): Promise<ProjectOverviewBug[]> {
  const params: Record<string, string> = {}
  if (input.planUnitId) params.project_ids = String(input.planUnitId)
  try {
    const response = await platformApi.get(
      `/project-sets/${input.projectSetId}/quality-overview`,
      { params, signal },
    )
    return unwrapApiRows<unknown>(response).flatMap((row) => {
      const bug = parseBug(row)
      return bug ? [bug] : []
    })
  } catch (error) {
    const safe = toSafeApiError(error, 'QUALITY_BUGS_FAILED')
    if (safe.aborted) throw safe
    return []
  }
}
