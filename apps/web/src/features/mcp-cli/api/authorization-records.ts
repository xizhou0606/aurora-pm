import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import { normalizeAuthorizationRecords } from '../model/authorization-records'

export async function fetchAuthorizationRecords(page: number, pageSize: number, signal?: AbortSignal) {
  try {
    const response = await platformApi.get('/external-tooling/authorization-records', {
      params: { page, page_size: pageSize }, signal,
    })
    const data = normalizeAuthorizationRecords(unwrapApiData(response))
    if (!data) throw new Error('AUTHORIZATION_RECORDS_CONTRACT_INVALID')
    return data
  } catch (error) {
    throw toSafeApiError(error, 'AUTHORIZATION_RECORDS_LOAD_FAILED')
  }
}

export async function revokeAuthorizationRecord(id: string) {
  try {
    await platformApi.post(`/external-tooling/authorization-records/${encodeURIComponent(id)}/revoke`, {})
  } catch (error) {
    throw toSafeApiError(error, 'AUTHORIZATION_RECORD_REVOKE_FAILED')
  }
}
