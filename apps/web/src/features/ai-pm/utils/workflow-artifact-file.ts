import { downloadPlatformFile, downloadWorkflowNodeArtifact } from '../api/api'
import type { WorkflowNodeArtifact } from '../model/types'

type ArtifactFile = Pick<WorkflowNodeArtifact, 'id' | 'name'> & Partial<Pick<WorkflowNodeArtifact, 'url' | 'content_type' | 'source'>>

export function isHttpURL(value: string) {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export function isExternalHttpURL(value: string) {
  return isHttpURL(value) && !value.includes('/api/')
}

export function isWorkflowArtifactLink(artifact: ArtifactFile) {
  return artifact.source === 'link' || artifact.content_type === 'text/uri-list' || Boolean(artifact.url && isExternalHttpURL(artifact.url))
}

async function blobForArtifact(artifact: ArtifactFile) {
  if (artifact.url) {
    const blob = await downloadPlatformFile(artifact.url)
    if (blob) return blob
  }
  return downloadWorkflowNodeArtifact(artifact.id)
}

export function openWorkflowArtifactLink(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer')
}

export function saveBlobAsFile(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

export async function saveWorkflowNodeArtifact(artifact: ArtifactFile) {
  if (isWorkflowArtifactLink(artifact) && artifact.url) {
    openWorkflowArtifactLink(artifact.url)
    return
  }
  saveBlobAsFile(await blobForArtifact(artifact), artifact.name)
}

export async function previewWorkflowNodeArtifact(artifact: ArtifactFile) {
  if (isWorkflowArtifactLink(artifact) && artifact.url) {
    openWorkflowArtifactLink(artifact.url)
    return
  }
  const url = URL.createObjectURL(await blobForArtifact(artifact))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.target = '_blank'
  anchor.rel = 'noopener noreferrer'
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
}
