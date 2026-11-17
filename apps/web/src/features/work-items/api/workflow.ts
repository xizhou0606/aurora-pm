import type { WorkItemsClient } from './client'
import { array, optionalString, record } from './contracts'
import type { WorkItemWorkflowCatalog, WorkItemWorkflowStatus } from '../model/types'

function payloadArray(value: unknown): unknown[] {
  if (Array.isArray(value)) return value
  const source = record(value)
  if (Array.isArray(source.data)) return source.data
  if (Array.isArray(source.items)) return source.items
  return []
}

function payloadRecord(value: unknown): Record<string, unknown> {
  const source = record(value)
  return Object.keys(record(source.data)).length ? record(source.data) : source
}

function normalizeStatuses(value: unknown): WorkItemWorkflowStatus[] {
  const seen = new Set<string>()
  return payloadArray(value).map((entry, index) => {
    const source = record(entry)
    const key = optionalString(source.status_key)?.toLowerCase()
    if (!key || seen.has(key)) throw new Error('invalid_work_item_status_catalog')
    seen.add(key)
    const order = Number(source.sort_order)
    return {
      key,
      label: optionalString(source.label) || key,
      sortOrder: Number.isFinite(order) ? order : index * 10,
    }
  }).sort((left, right) => left.sortOrder - right.sortOrder || left.key.localeCompare(right.key))
}

function normalizeWorkflow(value: unknown) {
  const source = payloadRecord(value)
  const transitions = array(source.transitions).map((entry) => {
    const transition = record(entry)
    const from = optionalString(transition.from_status)?.toLowerCase()
    const to = optionalString(transition.to_status)?.toLowerCase()
    if (!from || !to) throw new Error('invalid_work_item_workflow')
    return { from, to }
  })
  return { transitions, strict: source.enabled === true || source.enforce === true }
}

export async function requestWorkItemWorkflowCatalog(
  client: WorkItemsClient,
  scope: { teamId?: number; projectId?: number } = {},
  signal?: AbortSignal,
): Promise<WorkItemWorkflowCatalog> {
  const query = { team_id: scope.teamId || 0, project_id: scope.projectId || 0, kind: 'task' }
  const [statusesPayload, workflowPayload] = await Promise.all([
    client.request<unknown>({ path: '/issue-statuses', query, signal }),
    client.request<unknown>({ path: '/issue-workflow-transitions', query, signal }),
  ])
  const statuses = normalizeStatuses(statusesPayload)
  const workflow = normalizeWorkflow(workflowPayload)
  return { statuses, ...workflow }
}
