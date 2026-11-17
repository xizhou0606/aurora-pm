import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

import type { ReportingClient, ReportingFeishuFactsContext, ReportingRaw } from './contracts'

async function requestReporting(
  method: 'GET' | 'POST',
  path: string,
  options: { body?: unknown; query?: Readonly<Record<string, unknown>>; signal?: AbortSignal } = {},
): Promise<ReportingRaw> {
  try {
    const response = await platformApi.request({
      method,
      url: path,
      data: options.body,
      params: options.query,
      signal: options.signal,
    })
    return unwrapApiData<unknown>(response)
  } catch (error) {
    throw toSafeApiError(error, 'PM_REPORTING_REQUEST_FAILED')
  }
}

export const reportingClient: ReportingClient = Object.freeze({
  directory: (signal) => requestReporting('GET', '/reports/members', { signal }),
  memberPreview: (memberId, query, signal) => requestReporting(
    'GET',
    `/reports/members/${memberId}/preview`,
    { query, signal },
  ),
  memberGenerate: (memberId, body) => requestReporting(
    'POST',
    `/reports/members/${memberId}/generate`,
    { body },
  ),
  groupPreview: (groupId, query, signal) => requestReporting(
    'GET',
    `/reports/groups/${groupId}/preview`,
    { query, signal },
  ),
  groupGenerate: (groupId, body) => requestReporting(
    'POST',
    `/reports/groups/${groupId}/generate`,
    { body },
  ),
  departmentPreview: (departmentId, query, signal) => requestReporting(
    'GET',
    `/reports/departments/${departmentId}/preview`,
    { query, signal },
  ),
  departmentGenerate: (departmentId, body) => requestReporting(
    'POST',
    `/reports/departments/${departmentId}/generate`,
    { body },
  ),
  runStatus: (runId, signal) => requestReporting('GET', `/reports/runs/${runId}`, { signal }),
  // runId=0 publishes the facts-only report; the context body tells the
  // backend which member/group and period to re-aggregate.
  exportFeishuDoc: (runId, facts) => requestReporting(
    'POST',
    `/reports/runs/${runId}/feishu-doc`,
    {
      body: facts
        ? {
          target_kind: facts.targetKind,
          target_id: facts.targetId,
          type: facts.type,
          date: facts.anchorDate || undefined,
        }
        : undefined,
    },
  ),
})
