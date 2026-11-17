import { PM_ACTION_CODES, type PMActionCode } from '@/generated/access-catalog'
import { coreApi, toSafeApiError, unwrapApiData } from '@/shared/api'

const knownCodes = new Set<string>(PM_ACTION_CODES)

export async function listMyActionPermissions(): Promise<PMActionCode[]> {
  try {
    const response = await coreApi.get('/access/me/action-permissions')
    const payload = unwrapApiData<{ action_codes?: unknown }>(response)
    if (!Array.isArray(payload?.action_codes)) return []
    return [...new Set(payload.action_codes.filter((code): code is PMActionCode => (
      typeof code === 'string' && knownCodes.has(code)
    )))]
  } catch (error) {
    throw toSafeApiError(error, 'ACTION_PERMISSIONS_LOAD_FAILED')
  }
}
