import type { WorkItemsClient } from './client'
import { optionalNumber, optionalString, record } from './contracts'

// --- Comment operations ---

export async function createWorkItemComment(
  client: WorkItemsClient,
  id: number,
  body: string,
): Promise<void> {
  await client.request<unknown>({
    method: 'POST', path: `/tasks/${id}/comments`, body: { body },
  })
}

export async function deleteWorkItemComment(
  client: WorkItemsClient,
  taskId: number,
  commentId: number,
): Promise<void> {
  await client.request<void>({
    method: 'DELETE',
    path: `/tasks/${taskId}/comments/${commentId}`,
  })
}

// --- Attachment operations ---

export async function deleteWorkItemAttachment(
  client: WorkItemsClient,
  attachmentId: number,
  stepId?: string,
): Promise<void> {
  await client.request<void>({
    method: 'DELETE',
    path: `/tasks/attachments/${attachmentId}`,
    query: stepId ? { step_id: stepId } : undefined,
  })
}

export async function uploadWorkItemAttachment(
  client: WorkItemsClient,
  taskId: number,
  file: File,
  stepId?: string,
): Promise<number> {
  if (!client.upload) throw new Error('upload_unavailable')
  const formData = new FormData()
  formData.append('file', file)
  if (stepId) formData.append('step_id', stepId)
  const payload = await client.upload<unknown>({
    path: `/tasks/${taskId}/attachments`,
    query: stepId ? { step_id: stepId } : undefined,
    formData,
  })
  const attachmentId = optionalNumber(record(payload).id)
  if (!attachmentId) throw new Error('invalid_attachment_upload')
  return attachmentId
}

export async function uploadWorkItemRichTextImage(
  client: WorkItemsClient,
  requirementId: number | undefined,
  file: File,
): Promise<string> {
  if (!client.upload) throw new Error('rich_text_image_upload_unavailable')
  const formData = new FormData()
  if (Number.isInteger(requirementId) && Number(requirementId) > 0) {
    formData.append('requirement_id', String(requirementId))
  }
  formData.append('file', file)
  const payload = record(await client.upload<unknown>({
    path: '/tasks/materials/upload',
    formData,
  }))
  const rawURL = optionalString(payload.url)
  if (!rawURL) throw new Error('invalid_rich_text_image_upload')
  const url = new URL(rawURL, window.location.origin)
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error('invalid_rich_text_image_upload')
  }
  return url.toString()
}

export async function downloadWorkItemAttachment(
  client: WorkItemsClient,
  attachmentId: number,
  stepId?: string,
) {
  if (!client.download) throw new Error('download_unavailable')
  return client.download({
    path: `/tasks/attachments/${attachmentId}/download`,
    query: stepId ? { step_id: stepId } : undefined,
  })
}