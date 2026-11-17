import type {
  AxiosRequestConfig,
  InternalAxiosRequestConfig,
} from 'axios'
import { getPMRuntimeConfig } from '@/shared/config'
import {
  getRuntimeSession,
  getRuntimeToken,
  getRuntimeWorkspaceId,
} from '@/shared/runtime'

export type PMAxiosRequestContext = {
  scope: 'session' | 'workspace'
}

declare module 'axios' {
  interface AxiosRequestConfig {
    pmContext?: PMAxiosRequestContext
  }
}

export function pmSessionRequestConfig(
  signal?: AbortSignal,
): AxiosRequestConfig {
  return {
    signal,
    pmContext: { scope: 'session' },
  }
}

export function applyRuntimeHeaders(
  config: InternalAxiosRequestConfig,
): InternalAxiosRequestConfig {
  const token = getRuntimeToken()
  if (token) config.headers.Authorization = `Bearer ${token}`

  const workspaceId = getRuntimeWorkspaceId()
  if (config.pmContext?.scope === 'session') {
    config.headers.delete('X-Workspace-ID')
  } else if (workspaceId > 0) {
    config.headers['X-Workspace-ID'] = String(workspaceId)
  }

  const timezone = getRuntimeSession().timezone || getPMRuntimeConfig().timezone
  if (timezone) config.headers['X-Timezone'] = timezone
  return config
}
