import {
  coreApi,
  pmSessionRequestConfig,
  toSafeApiError,
  unwrapApiRows,
} from '@/shared/api'
import { normalizeWorkspaceOptions } from '../hooks/selection'
import type { WorkspaceOption } from '../model/types'

export async function fetchWorkspaces(
  signal?: AbortSignal,
): Promise<WorkspaceOption[]> {
  try {
    const response = await coreApi.get(
      '/auth/me/workspaces',
      pmSessionRequestConfig(signal),
    )
    return normalizeWorkspaceOptions(unwrapApiRows<WorkspaceOption>(response))
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACES_FAILED')
  }
}
