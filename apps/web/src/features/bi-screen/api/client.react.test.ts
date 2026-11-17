import { describe, expect, it } from 'bun:test'

import { PMSafeApiError } from '@/shared/api'

import { shouldEscalateRuntimeFailure } from './client'

describe('BI screen runtime failure isolation', () => {
  it('escalates initial transport and 5xx failures to the single-module boundary', () => {
    for (const error of [
      new Error('transport'),
      new PMSafeApiError('SERVER', 503),
      new PMSafeApiError('BI_SCREEN_CONTRACT_INCOMPATIBLE'),
    ]) {
      expect(shouldEscalateRuntimeFailure(error, false)).toBe(true)
    }
  })

  it('keeps refetch, controlled 4xx and aborted failures inside the native state machine', () => {
    expect(shouldEscalateRuntimeFailure(new Error('refetch transport'), true)).toBe(false)
    expect(shouldEscalateRuntimeFailure(new PMSafeApiError('REFETCH_SERVER', 503), true)).toBe(false)
    for (const error of [
      new PMSafeApiError('DENIED', 403),
      new PMSafeApiError('REQUEST_ABORTED', undefined, true),
    ]) {
      expect(shouldEscalateRuntimeFailure(error, false)).toBe(false)
    }
  })
})
