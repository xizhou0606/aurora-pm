import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

import {
  normalizeDeviceAuthorization,
  normalizeDeviceAuthorizationStatus,
  type DeviceAuthorization,
  type DeviceAuthorizationStatus,
} from '../model/contract'

export type DeviceAuthorizationDecision = 'authorize' | 'deny'
export type DeviceAuthorizationDecisionStatus = Extract<
  DeviceAuthorizationStatus,
  'approved' | 'denied'
>

function contractError(code: string): never {
  throw toSafeApiError(new Error('invalid device authorization response'), code)
}

export async function fetchDeviceAuthorization(
  userCode: string,
  signal?: AbortSignal,
): Promise<DeviceAuthorization> {
  try {
    const response = await platformApi.get('/external-tooling/device-authorizations', {
      params: { user_code: userCode },
      signal,
    })
    return normalizeDeviceAuthorization(unwrapApiData(response))
      ?? contractError('DEVICE_AUTHORIZATION_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'DEVICE_AUTHORIZATION_LOAD_FAILED')
  }
}

export async function decideDeviceAuthorization(
  userCode: string,
  decision: DeviceAuthorizationDecision,
): Promise<DeviceAuthorizationDecisionStatus> {
  try {
    const response = await platformApi.post('/external-tooling/device-authorizations', {
      user_code: userCode,
      decision,
    })
    const status = normalizeDeviceAuthorizationStatus(
      (unwrapApiData(response) as Record<string, unknown> | undefined)?.status,
    )
    return status === 'approved' || status === 'denied'
      ? status
      : contractError('DEVICE_AUTHORIZATION_DECISION_CONTRACT_INVALID')
  } catch (error) {
    throw toSafeApiError(error, 'DEVICE_AUTHORIZATION_DECISION_FAILED')
  }
}
