import { describe, expect, it, vi } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'
import {
  withWorkItemsRuntimeFailure,
  type WorkItemsClient,
} from './client'

function clientRejecting(error: unknown): WorkItemsClient {
  const reject = () => Promise.reject(error)
  return {
    request: reject,
    requestPage: reject,
    requestPMScopeOptions: reject,
    requestPMUsers: reject,
  }
}

describe('WorkItems runtime failure boundary', () => {
  it.each([
    new PMSafeApiError('UPSTREAM_DOWN', 503),
    new PMSafeApiError('NETWORK_UNAVAILABLE'),
  ])('reports transport and server failures', async (error) => {
    const onRuntimeFailure = vi.fn()
    const client = withWorkItemsRuntimeFailure(
      clientRejecting(error),
      onRuntimeFailure,
    )
    await expect(client.request({ path: '/tasks' })).rejects.toBe(error)
    expect(onRuntimeFailure).toHaveBeenCalledTimes(1)
    expect(onRuntimeFailure).toHaveBeenCalledWith(error)
  })

  it.each([
    new PMSafeApiError('DOMAIN_REJECTED', 422),
    new PMSafeApiError('SESSION_EXPIRED', 401),
    new PMSafeApiError('REQUEST_ABORTED', undefined, true),
  ])('keeps 4xx and abort failures in native state', async (error) => {
    const onRuntimeFailure = vi.fn()
    const client = withWorkItemsRuntimeFailure(
      clientRejecting(error),
      onRuntimeFailure,
    )
    await expect(client.request({ path: '/tasks' })).rejects.toBe(error)
    expect(onRuntimeFailure).not.toHaveBeenCalled()
  })
})
