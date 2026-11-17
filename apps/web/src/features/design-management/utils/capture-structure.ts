import type { DesignReviewTarget, MobileCaptureDraft } from '../model'

export type WebCaptureStructure = {
  actualUrl: string
  elements: unknown[]
  structureTruncated: boolean
  originalElements: unknown[]
  originalStructureTruncated: boolean
  warnings: Array<{ code: string; message: string }>
}

type WebStructureOptions = {
  actualUrl: string
  viewportWidth: number
  viewportHeight: number
  topbar?: Record<string, unknown>
}

export function webCaptureStructure(data: Record<string, unknown>): WebCaptureStructure | undefined {
  if (!Array.isArray(data.elements)) return undefined
  const inventory = record(data.inventory)
  return {
    actualUrl: typeof data.actual_url === 'string' ? data.actual_url : '',
    elements: data.elements,
    structureTruncated: data.structure_truncated !== false,
    originalElements: Array.isArray(inventory?.elements) ? inventory.elements : data.elements,
    originalStructureTruncated: inventory?.truncated !== false,
    warnings: normalizeWarnings(data.warnings),
  }
}

export function webStructureFiles(capture: WebCaptureStructure | undefined, options: WebStructureOptions) {
  const topbarHeight = options.topbar?.enabled === true ? 44 : 0
  const make = (elements: unknown[], structureTruncated: boolean, suffix: string) => jsonFile({
    source: 'web',
    actualUrl: capture?.actualUrl || options.actualUrl,
    elements: elements.map((element) => offsetElement(element, topbarHeight)),
    structureTruncated,
    syntheticMobileTopbar: options.topbar?.enabled === true ? options.topbar : null,
    contentViewport: { width: options.viewportWidth, height: options.viewportHeight - topbarHeight },
    finalViewport: { width: options.viewportWidth, height: options.viewportHeight },
    warnings: capture?.warnings ?? [{ code: 'structure_unavailable', message: '实现证据未由浏览器扩展采集，结构信息不可用' }],
  }, `implementation-structure${suffix}.json`)

  return {
    current: make(capture?.elements ?? [], capture?.structureTruncated ?? true, ''),
    original: capture && capture.originalElements !== capture.elements
      ? make(capture.originalElements, capture.originalStructureTruncated, '-original')
      : undefined,
  }
}

export function mobileStructureFile(capture: MobileCaptureDraft, target: DesignReviewTarget, viewportWidth: number, viewportHeight: number) {
  return jsonFile({
    source: 'mobile', target, elements: Array.isArray(capture.elements) ? capture.elements : [],
    structureTruncated: capture.structure_truncated !== false, syntheticMobileTopbar: null,
    contentViewport: { width: viewportWidth, height: viewportHeight },
    finalViewport: { width: viewportWidth, height: viewportHeight },
    warnings: normalizeWarnings(capture.warnings),
  }, 'implementation-structure.json')
}

function offsetElement(value: unknown, topbarHeight: number): unknown {
  if (!topbarHeight) return value
  const element = record(value)
  const bounds = record(element?.bounds)
  if (!element || !bounds || typeof bounds.y !== 'number') return value
  return { ...element, bounds: { ...bounds, y: bounds.y + topbarHeight } }
}

function normalizeWarnings(value: unknown): Array<{ code: string; message: string }> {
  if (!Array.isArray(value)) return []
  return value.flatMap((item) => {
    const warning = record(item)
    return typeof warning?.code === 'string' && typeof warning.message === 'string'
      ? [{ code: warning.code, message: warning.message }]
      : []
  })
}

function record(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : undefined
}

function jsonFile(value: unknown, name: string): File {
  return new File([JSON.stringify(value)], name, { type: 'application/json' })
}
