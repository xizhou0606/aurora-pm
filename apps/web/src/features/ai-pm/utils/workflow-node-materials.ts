import type { RequirementMaterial, WorkflowInstance, WorkflowNodeArtifact } from '../model/types'

type DesignMaterialRef = Pick<RequirementMaterial, 'url'> & Partial<Pick<RequirementMaterial, 'origin_url' | 'name' | 'content_type'>>

type TimelineNode = WorkflowInstance['timeline_nodes'][number]
type DesignImageItem = { url: string; name: string }

export function isDesignDeliverableNode(instance: WorkflowInstance, node: TimelineNode) {
  return instance.subject_type === 'requirement' && node.type === 'human_task' && (
    node.config?.require_design_image === true
    || instance.definition_edges.some((edge) => edge.target === node.id && edge.condition?.includes('needs_ui') && edge.condition.includes('true'))
  )
}

const uselessDesignName = new Set(['', 'download', 'apps', 'api', 'platform'])
const designImageExtension = /\.(?:avif|gif|jpe?g|png|svg|webp)(?:[?#].*)?$/i
const designMaterialDownloadPath = /\/materials\/\d+\/download(?:[?#].*)?$/i
const figmaDesignPath = /^\/(design|file|proto|board|make)\//i

export function isDesignLinkURL(url: string) {
  const trimmed = url.trim()
  if (!/^https?:\/\//i.test(trimmed)) return false
  if (designImageExtension.test(trimmed) || designMaterialDownloadPath.test(trimmed)) return false
  return true
}

function isDesignLinkMaterial(row: DesignMaterialRef) {
  if ((row.content_type || '').toLowerCase() === 'text/uri-list') return true
  return isDesignLinkURL(designMaterialAttachURL(row))
}

function figmaLinkRank(url: string) {
  try {
    const host = new URL(url).hostname.toLowerCase()
    if (host !== 'figma.com' && !host.endsWith('.figma.com')) return 0
    return figmaDesignPath.test(new URL(url).pathname) ? 2 : 1
  } catch {
    return 0
  }
}

export function parseDesignLinkDrafts(value: string) {
  return [...new Set(value.split(/[\s,;，；]+/).map((item) => item.trim()).filter(Boolean))]
}

export function persistedDesignLinkURLs(
  materials: readonly DesignMaterialRef[],
  historyURLs: readonly string[] = [],
  legacyURL = '',
) {
  const urls: string[] = []
  const seen = new Set<string>()
  const add = (url: string) => {
    const value = url.trim()
    if (!value || seen.has(value) || !isDesignLinkURL(value)) return
    seen.add(value)
    urls.push(value)
  }
  for (const row of materials) {
    if (!isDesignLinkMaterial(row)) continue
    add(designMaterialAttachURL(row))
  }
  for (const value of historyURLs) add(value)
  add(legacyURL)
  return urls
}

export function persistedDesignLinkURL(
  materials: readonly DesignMaterialRef[],
  historyURLs: readonly string[] = [],
  legacyURL = '',
) {
  const candidates: string[] = []
  if (isDesignLinkURL(legacyURL)) candidates.push(legacyURL.trim())
  for (const row of materials) {
    if (!isDesignLinkMaterial(row)) continue
    const url = designMaterialAttachURL(row)
    if (url) candidates.push(url)
  }
  for (const value of historyURLs) {
    if (isDesignLinkURL(value)) candidates.push(value.trim())
  }
  let best = ''
  let bestRank = -1
  for (const url of candidates) {
    const rank = figmaLinkRank(url)
    if (rank >= bestRank) {
      best = url
      bestRank = rank
    }
  }
  return best
}

export function designFileMaterialURLs(materials: readonly DesignMaterialRef[], historyURLs: readonly string[] = []) {
  const urls: string[] = []
  const seen = new Set<string>()
  const add = (url: string) => {
    const value = url.trim()
    if (!value || seen.has(value) || isDesignLinkURL(value)) return
    seen.add(value)
    urls.push(value)
  }
  for (const row of materials) {
    if (isDesignLinkMaterial(row)) continue
    add(designMaterialAttachURL(row))
  }
  for (const url of historyURLs) add(url)
  return urls
}

export function designMaterialDisplayName(url: string, fallback = '') {
  try {
    const name = decodeURIComponent(new URL(url, 'http://local.invalid').pathname.split('/').pop() || '')
    if (!name || uselessDesignName.has(name.toLowerCase())) return fallback || url
    return name
  } catch {
    return fallback || url
  }
}

export function attachableDesignURL(url: string, origin = typeof window === 'undefined' ? '' : window.location.origin) {
  const trimmed = url.trim()
  if (!trimmed) return ''
  if (/^https?:\/\//i.test(trimmed)) return trimmed
  if (!origin) return trimmed
  try {
    return new URL(trimmed, origin).toString()
  } catch {
    return trimmed
  }
}

export function nodeDesignMaterialURLs(instance: WorkflowInstance, nodeId: string) {
  const urls: string[] = []
  const seen = new Set<string>()
  for (const item of instance.transition_history ?? []) {
    if (item.node_id !== nodeId) continue
    for (const material of item.materials ?? []) {
      const url = material.kind === 'design_image' ? material.url.trim() : ''
      if (!url || seen.has(url)) continue
      seen.add(url)
      urls.push(url)
    }
  }
  return urls
}

export function resolveNodeDesignMaterialURLs(instance: WorkflowInstance, nodeId: string, legacyDesignURL?: string) {
  const attributed = nodeDesignMaterialURLs(instance, nodeId)
  if (attributed.length) return attributed
  const designNodeCount = instance.timeline_nodes.filter((node) => (
    node.status !== 'skipped' && isDesignDeliverableNode(instance, node)
  )).length
  const fallback = legacyDesignURL?.trim() || ''
  return designNodeCount === 1 && fallback ? [fallback] : []
}

export function resolveNodeDesignMaterialURL(instance: WorkflowInstance, nodeId: string, legacyDesignURL?: string) {
  const urls = resolveNodeDesignMaterialURLs(instance, nodeId, legacyDesignURL)
  return urls[urls.length - 1]
}

export function designMaterialAttachURL(row: DesignMaterialRef) {
  const origin = row.origin_url?.trim() || ''
  if (/^https?:\/\//i.test(origin)) return origin
  return attachableDesignURL(row.url)
}

export function designMaterialIsOnNode(row: DesignMaterialRef, attachedURLs: readonly string[]) {
  const attached = new Set<string>()
  for (const value of attachedURLs) {
    const url = value.trim()
    if (!url) continue
    attached.add(url)
    attached.add(attachableDesignURL(url))
  }
  return [row.url, row.origin_url, designMaterialAttachURL(row)].some((value) => {
    const url = value?.trim() || ''
    return url !== '' && (attached.has(url) || attached.has(attachableDesignURL(url)))
  })
}

export function pendingAttachDesignMaterialURLs(
  instance: WorkflowInstance,
  nodeId: string,
  materials: readonly DesignMaterialRef[],
) {
  const attached = nodeDesignMaterialURLs(instance, nodeId)
  return [...new Set(
    materials
      .filter((row) => !designMaterialIsOnNode(row, attached))
      .map((row) => designMaterialAttachURL(row))
      .filter(Boolean),
  )]
}

export function designMaterialName(url: string, rows: readonly DesignMaterialRef[], fallback = '') {
  const row = rows.find((item) => (
    item.url === url
    || item.origin_url === url
    || designMaterialAttachURL(item) === url
    || attachableDesignURL(item.url) === url
  ))
  return row?.name?.trim() || designMaterialDisplayName(url, fallback)
}

export function mergeDesignMaterialURLs(historyURLs: readonly string[], libraryURLs: readonly string[]) {
  const urls: string[] = []
  const seen = new Set<string>()
  for (const value of [...libraryURLs, ...historyURLs]) {
    const url = value.trim()
    if (!url || seen.has(url)) continue
    seen.add(url)
    urls.push(url)
  }
  return urls
}

export function designImagesFromURLs(urls: readonly string[], fallbackName: string): DesignImageItem[] {
  return urls.map((url) => ({ url, name: designMaterialDisplayName(url, fallbackName) }))
}

export function appendUniqueDesignImage(
  current: Record<string, DesignImageItem[]>,
  nodeId: string,
  fallback: readonly DesignImageItem[],
  item: DesignImageItem,
) {
  const existing = current[nodeId] ?? [...fallback]
  if (existing.some((entry) => entry.url === item.url)) {
    return { ...current, [nodeId]: existing }
  }
  return { ...current, [nodeId]: [...existing, item] }
}

function artifactsByKind(artifacts: WorkflowNodeArtifact[], kind: WorkflowNodeArtifact['kind']) {
  return artifacts.filter((artifact) => artifact.kind === kind).sort((left, right) => left.id - right.id)
}

export function selectEffectiveTestCaseArtifacts(artifacts: WorkflowNodeArtifact[]) {
  const uploaded = artifactsByKind(artifacts, 'uploaded')
  return uploaded.length ? uploaded : artifactsByKind(artifacts, 'ai_generated')
}

export function selectEffectiveTestCaseArtifact(artifacts: WorkflowNodeArtifact[]) {
  const effective = selectEffectiveTestCaseArtifacts(artifacts)
  return effective[effective.length - 1]
}
