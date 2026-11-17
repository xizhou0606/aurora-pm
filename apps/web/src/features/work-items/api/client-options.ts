import type { WorkItemsClient } from './client'
import { array, optionalNumber, optionalString, record } from './contracts'
import type { WorkItemClientOption } from '../model/types'

export function normalizeClientOption(value: unknown): WorkItemClientOption | null {
  const source = record(value)
  const id = optionalNumber(source.id)
  const name = optionalString(source.name)
  if (!id || !name) return null
  return { id, name, usageCount: Math.max(0, Number(source.usage_count) || 0) }
}

export async function requestClientOptions(client: WorkItemsClient, signal?: AbortSignal) {
  const payload = await client.request<unknown>({ path: '/tasks/client-options', signal })
  const source = record(payload)
  return array(source.items).flatMap((value) => {
    const option = normalizeClientOption(value)
    return option ? [option] : []
  })
}

export async function createClientOption(client: WorkItemsClient, name: string) {
  const option = normalizeClientOption(await client.request<unknown>({
    method: 'POST', path: '/tasks/client-options', body: { name },
  }))
  if (!option) throw new Error('invalid_task_client_option')
  return option
}

export async function updateClientOption(client: WorkItemsClient, id: number, name: string) {
  const option = normalizeClientOption(await client.request<unknown>({
    method: 'PATCH', path: `/tasks/client-options/${id}`, body: { name },
  }))
  if (!option) throw new Error('invalid_task_client_option')
  return option
}

export async function deleteClientOption(client: WorkItemsClient, id: number) {
  await client.request<void>({ method: 'DELETE', path: `/tasks/client-options/${id}` })
}
