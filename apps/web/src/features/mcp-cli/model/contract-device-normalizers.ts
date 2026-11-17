import type {
  DeviceAuthorization,
  DeviceAuthorizationStatus,
  DeviceAuthorizeSearch,
} from './contract-types'
import { normalizeAuthorizationRequest } from './contract-normalizers'

type RecordValue = Record<string, unknown>

function record(value: unknown): RecordValue | undefined {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as RecordValue
    : undefined
}

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function normalizeDeviceUserCode(value: unknown): string | undefined {
  const candidate = text(value).toUpperCase()
  return /^[A-Z2-7]{4}-[A-Z2-7]{4}$/.test(candidate)
    ? candidate
    : undefined
}

export function parseDeviceAuthorizeSearch(
  search: Record<string, unknown>,
): DeviceAuthorizeSearch {
  const userCode = normalizeDeviceUserCode(search.user_code)
  return userCode ? { user_code: userCode } : {}
}

export function normalizeDeviceAuthorizationStatus(
  value: unknown,
): DeviceAuthorizationStatus | undefined {
  const candidate = text(value) as DeviceAuthorizationStatus
  return ['pending', 'approved', 'denied', 'consumed', 'expired'].includes(candidate)
    ? candidate
    : undefined
}

export function normalizeDeviceAuthorization(
  value: unknown,
): DeviceAuthorization | undefined {
  const item = record(value)
  const userCode = normalizeDeviceUserCode(item?.user_code)
  const status = normalizeDeviceAuthorizationStatus(item?.status)
  const requestExpiresAt = text(item?.request_expires_at)
  const request = normalizeAuthorizationRequest({
    ...item,
    id: userCode,
    oauth: false,
  })
  if (
    !userCode
    || !status
    || !request
    || !requestExpiresAt
    || !Number.isFinite(Date.parse(requestExpiresAt))
  ) return undefined
  return { ...request, userCode, status, requestExpiresAt }
}
