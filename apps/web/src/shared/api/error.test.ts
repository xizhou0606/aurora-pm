import { describe, expect, it } from 'bun:test'

import {
  canRetainStaleApiData,
  PMSafeApiError,
  shouldFallbackModuleApi,
  toSafeApiError,
} from './error'

describe('safe PM API errors', () => {
  it('uses a controlled fallback for free-form Axios error text', () => {
    const raw = {
      config: {
        headers: { Authorization: 'Bearer secret-token' },
        data: { password: 'secret-password' },
      },
      response: {
        status: 401,
        data: { error: 'invalid credentials' },
      },
    }

    const safe = toSafeApiError(raw, 'LOGIN_FAILED')

    expect(safe).toMatchObject({
      name: 'PMSafeApiError',
      code: 'LOGIN_FAILED',
      status: 401,
    })
    expect(JSON.stringify(safe)).not.toContain('secret-token')
    expect(JSON.stringify(safe)).not.toContain('secret-password')
    expect(safe).not.toHaveProperty('cause')
  })

  it('accepts only bounded uppercase machine codes', () => {
    expect(
      toSafeApiError({
        response: { status: 503, data: { code: 'UPSTREAM_UNAVAILABLE' } },
      }),
    ).toMatchObject({ code: 'UPSTREAM_UNAVAILABLE', status: 503 })
    expect(
      toSafeApiError({
        response: { status: 503, data: { code: 'upstream unavailable' } },
      }),
    ).toMatchObject({ code: 'REQUEST_FAILED', status: 503 })
    expect(new PMSafeApiError('contains private details').code).toBe(
      'REQUEST_FAILED',
    )
  })

  it('accepts the bounded nested machine-code envelope used by Tool Runtime', () => {
    expect(
      toSafeApiError({
        response: {
          status: 404,
          data: { error: { code: 'EXTERNAL_TOOLING_CONFIGURATION_NOT_FOUND', message: 'discarded' } },
        },
      }),
    ).toMatchObject({ code: 'EXTERNAL_TOOLING_CONFIGURATION_NOT_FOUND', status: 404 })
  })

  it('normalizes unknown failures without preserving the raw object', () => {
    const safe = toSafeApiError(new Error('contains private request details'))

    expect(safe).toBeInstanceOf(PMSafeApiError)
    expect(safe.code).toBe('REQUEST_FAILED')
    expect(safe.status).toBeUndefined()
    expect(safe.message).not.toContain('private request details')
  })

  it('keeps only bounded field locators from structured validation errors', () => {
    const safe = toSafeApiError({
      response: {
        status: 422,
        data: {
          code: 'TASK_BATCH_VALIDATION_FAILED',
          fields: [
            { client_id: 'quick-child-1', field: 'assignee_id', message: 'private detail' },
            { client_id: 'invalid value with spaces', field: 'title' },
            { client_id: 'quick-task-1', field: '<script>' },
          ],
        },
      },
    })

    expect(safe).toMatchObject({
      code: 'TASK_BATCH_VALIDATION_FAILED',
      status: 422,
      details: {
        fields: [
          { client_id: 'quick-child-1', field: 'assignee_id' },
          { field: 'title' },
        ],
      },
    })
    expect(JSON.stringify(safe)).not.toContain('private detail')
    expect(JSON.stringify(safe)).not.toContain('<script>')
  })

  it('keeps only the schedule contract and discards arbitrary server text', () => {
    const safe = toSafeApiError({
      response: {
        status: 409,
        data: {
          code: 'WORKFLOW_SCHEDULE_CONFLICT',
          message: 'private server explanation',
          schedule: {
            state: 'conflict',
            subject: {
              planned_start_at: '2026-08-10',
              planned_end_at: 'not-a-date',
              secret: 'token',
            },
            constraints: {
              planned_start_at: '2026-08-01',
              planned_end_at: 'private boundary',
              secret: 'constraint-token',
            },
            suggested: {
              planned_start_at: '2026-08-09T08:00:00+08:00',
              planned_end_at: '2026-08-25',
            },
            actual_dates_independent: true,
            can_confirm: false,
            conflicts: [
              {
                code: 'NODE_AFTER_SUBJECT_END',
                subject_type: 'plan_unit',
                subject_id: 42,
                node_id: 'qa-node',
                related_node_id: '<script>',
                field: 'planned_end_at',
                boundary: '2026-08-23',
                value: '2026-08-25',
                message: 'private node title',
              },
              {
                code: 'PROJECT_START_AFTER_END',
                boundary: 'subject_end',
              },
              { code: 'invalid code', node_id: 'ignored' },
            ],
          },
        },
      },
    })

    expect(safe).toMatchObject({
      code: 'WORKFLOW_SCHEDULE_CONFLICT',
      status: 409,
      schedule: {
        state: 'conflict',
        subject: { planned_start_at: '2026-08-10' },
        constraints: { planned_start_at: '2026-08-01' },
        suggested: {
          planned_start_at: '2026-08-09T08:00:00+08:00',
          planned_end_at: '2026-08-25',
        },
        actual_dates_independent: true,
        can_confirm: false,
        conflicts: [
          {
            code: 'NODE_AFTER_SUBJECT_END',
            subject_type: 'plan_unit',
            subject_id: 42,
            node_id: 'qa-node',
            field: 'planned_end_at',
            boundary: '2026-08-23',
            value: '2026-08-25',
          },
          { code: 'PROJECT_START_AFTER_END' },
        ],
      },
    })
    expect(JSON.stringify(safe)).not.toContain('private server explanation')
    expect(JSON.stringify(safe)).not.toContain('private node title')
    expect(JSON.stringify(safe)).not.toContain('token')
    expect(JSON.stringify(safe)).not.toContain('<script>')
    expect(JSON.stringify(safe)).not.toContain('subject_end')
  })

  it.each([
    { code: 'ERR_CANCELED', name: 'CanceledError' },
    { name: 'AbortError' },
  ])('keeps request cancellation distinct from a transport failure', (raw) => {
    const safe = toSafeApiError(raw)

    expect(safe).toMatchObject({
      code: 'REQUEST_ABORTED',
      status: undefined,
      aborted: true,
    })
  })

  it('classifies only transport and server failures for module fallback', () => {
    expect(shouldFallbackModuleApi(new PMSafeApiError('DOMAIN_REJECTED', 422))).toBe(false)
    expect(shouldFallbackModuleApi(new PMSafeApiError('SESSION_EXPIRED', 401))).toBe(false)
    expect(shouldFallbackModuleApi(new PMSafeApiError('UPSTREAM_DOWN', 503))).toBe(true)
    expect(shouldFallbackModuleApi(new PMSafeApiError('NETWORK_UNAVAILABLE'))).toBe(true)
    expect(shouldFallbackModuleApi({ code: 'ERR_CANCELED' })).toBe(false)
    expect(shouldFallbackModuleApi({ name: 'AbortError' })).toBe(false)
  })

  it('retains cached business data only for normalized transport and server failures', () => {
    expect(canRetainStaleApiData(new PMSafeApiError('NETWORK_UNAVAILABLE'))).toBe(true)
    expect(canRetainStaleApiData(new PMSafeApiError('UPSTREAM_DOWN', 503))).toBe(true)
    expect(canRetainStaleApiData(new PMSafeApiError('SESSION_EXPIRED', 401))).toBe(false)
    expect(canRetainStaleApiData(new PMSafeApiError('FORBIDDEN', 403))).toBe(false)
    expect(canRetainStaleApiData(new PMSafeApiError('NOT_FOUND', 404))).toBe(false)
    expect(canRetainStaleApiData(new PMSafeApiError('REQUEST_ABORTED', undefined, true))).toBe(false)
    expect(canRetainStaleApiData(new Error('contract mismatch'))).toBe(false)
  })
})
