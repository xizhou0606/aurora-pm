import { PMSafeApiError } from '@/shared/api'

import type { DashboardData } from '../types'
import { normalizeCanonicalDashboard } from './canonical'
import { record } from './helpers'
import { normalizeLegacyDashboard } from './legacy'

export function normalizeDashboard(payload: unknown, workspaceId: number): DashboardData {
  const source = record(payload)
  const data = normalizeCanonicalDashboard(source) ?? normalizeLegacyDashboard(source)
  if (!data) throw new PMSafeApiError('BI_SCREEN_CONTRACT_INCOMPATIBLE')
  if (data.workspace_id !== workspaceId) {
    throw new PMSafeApiError('BI_SCREEN_WORKSPACE_MISMATCH', 403)
  }
  return data
}
