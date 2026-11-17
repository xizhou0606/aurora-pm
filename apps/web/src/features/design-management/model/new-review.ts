import type { WebCaptureStructure } from '../utils/capture-structure'
import type { CreateReviewInput } from './inputs'
import type { DesignReviewTarget, FigmaPreview, MobileCaptureDraft } from './types'

export const MAX_EVIDENCE_BYTES = 20 * 1024 * 1024
export const MIN_REVIEW_VIEWPORT_SIZE = 240
export const MAX_REVIEW_VIEWPORT_SIZE = 7680

export type NewReviewDraft = {
  mode: 'web' | 'mobile'
  requirementId?: number
  figmaUrl: string
  implementationUrl: string
  platform: 'ios' | 'android'
  appId: string
  appName: string
  appVersion: string
  orientation: 'portrait' | 'landscape'
  viewportWidth: number
  viewportHeight: number
  devicePixelRatio: number
  domNormalization: boolean
  topbar: boolean
  scoped: boolean
  region: { x: number; y: number; width: number; height: number }
}

export type NewReviewEvidence = {
  implementation?: File
  implementationOriginal?: File
  normalizationManifest?: File
  extension: 'checking' | 'connected' | 'disconnected' | 'capturing' | 'normalizing' | 'captured'
  iframeFallbackAvailable?: boolean
  inventory?: Record<string, unknown>
  normalizationFigmaUrl?: string
  mobileCaptureId?: string
  mobileTarget?: DesignReviewTarget
  mobileCapture?: MobileCaptureDraft
  webCapture?: WebCaptureStructure
  captureViewport?: {
    width: number
    height: number
    devicePixelRatio: number
  }
  mobileLoading?: boolean
}

export type NewReviewSubmission = CreateReviewInput & {
  implementationFile: File
  implementationOriginalFile?: File
  implementationStructureFile: File
  implementationStructureOriginalFile?: File
  normalizationManifest?: File
  mobileCaptureId?: string
}

export type NewReviewValidationKey =
  | 'invalidRequirement'
  | 'invalidFigma'
  | 'invalidTarget'
  | 'mobileCaptureRequired'
  | 'figmaEvidenceRequired'
  | 'implementationEvidenceRequired'
  | 'evidenceViewportMismatch'
  | 'normalizationFailed'
  | 'invalidRegion'

export type FigmaPreviewErrorCopyKey =
  | 'invalidFigma'
  | 'figmaPreviewNotFound'
  | 'figmaPreviewForbidden'
  | 'figmaPreviewUnavailable'
  | 'figmaPreviewFailed'

export type NewReviewSubmitErrorCopyKey =
  | 'submitInvalid'
  | 'submitForbidden'
  | 'submitConflict'
  | 'submitUnavailable'
  | 'submitFailed'

export const initialNewReviewDraft: NewReviewDraft = {
  mode: 'web', figmaUrl: '', implementationUrl: '', platform: 'ios', appId: '', appName: '', appVersion: '',
  orientation: 'portrait', viewportWidth: 1440, viewportHeight: 900, devicePixelRatio: 1,
  domNormalization: false, topbar: false, scoped: false, region: { x: 0, y: 0, width: 1, height: 1 },
}

export function validHTTPURL(value: string): boolean {
  try { return ['http:', 'https:'].includes(new URL(value).protocol) } catch { return false }
}

export function validFigmaURL(value: string): boolean {
  try {
    const url = new URL(value)
    return /(^|\.)figma\.com$/.test(url.hostname) && url.pathname.includes('/design/') && Boolean(url.searchParams.get('node-id'))
  } catch { return false }
}

export function figmaPreviewErrorCopyKey(status?: number): FigmaPreviewErrorCopyKey {
  if (status === 400) return 'invalidFigma'
  if (status === 403) return 'figmaPreviewForbidden'
  if (status === 404) return 'figmaPreviewNotFound'
  if (status === undefined || status === 408 || status === 429 || status >= 500) {
    return 'figmaPreviewUnavailable'
  }
  return 'figmaPreviewFailed'
}

export function newReviewSubmitErrorCopyKey(status?: number): NewReviewSubmitErrorCopyKey {
  if (status === 400) return 'submitInvalid'
  if (status === 403) return 'submitForbidden'
  if (status === 409) return 'submitConflict'
  if (status === undefined || status === 408 || status === 429 || status >= 500) return 'submitUnavailable'
  return 'submitFailed'
}

export function previewMatchesFigma(preview: FigmaPreview, value: string): boolean {
  try {
    const url = new URL(value)
    return url.pathname.split('/').includes(preview.file_key) && url.searchParams.get('node-id')?.replaceAll('-', ':') === preview.node_id.replaceAll('-', ':')
  } catch { return false }
}

export function figmaPreviewViewport(
  preview: FigmaPreview | undefined,
  figmaURL: string,
): Pick<NewReviewDraft, 'viewportWidth' | 'viewportHeight'> | undefined {
  if (!preview || !previewMatchesFigma(preview, figmaURL)) return undefined
  const viewportWidth = Math.round(preview.width)
  const viewportHeight = Math.round(preview.height)
  if (
    !Number.isFinite(viewportWidth)
    || !Number.isFinite(viewportHeight)
    || viewportWidth < MIN_REVIEW_VIEWPORT_SIZE
    || viewportWidth > MAX_REVIEW_VIEWPORT_SIZE
    || viewportHeight < MIN_REVIEW_VIEWPORT_SIZE
    || viewportHeight > MAX_REVIEW_VIEWPORT_SIZE
  ) return undefined
  return { viewportWidth, viewportHeight }
}

export function asMobileTarget(value?: Record<string, unknown>): DesignReviewTarget | undefined {
  if (!value || value.kind !== 'mobile' || (value.platform !== 'ios' && value.platform !== 'android') || typeof value.app_id !== 'string' || !value.app_id.trim()) return undefined
  return value as DesignReviewTarget
}

export function withoutMobileEvidence(current: NewReviewEvidence): NewReviewEvidence {
  return {
    ...current,
    implementation: undefined,
    mobileCaptureId: undefined,
    mobileTarget: undefined,
    mobileCapture: undefined,
    mobileLoading: false,
  }
}

export async function loadMobileCaptureEvidence(
  captureID: string,
  captures: readonly MobileCaptureDraft[],
  platform: NewReviewDraft['platform'],
  load: (id: string) => Promise<Blob>,
) {
  const capture = captures.find((item) => item.id === captureID)
  const target = capture ? asMobileTarget(capture.target) : undefined
  if (!capture || !target || target.platform !== platform) throw new Error('invalid mobile capture')
  const blob = await load(captureID)
  if (!blob.type.startsWith('image/') || blob.size > MAX_EVIDENCE_BYTES) throw new Error('invalid mobile capture image')
  const screen = target.screen as { width?: number; height?: number; scale?: number } | undefined
  return {
    capture,
    target,
    implementation: new File([blob], `mobile-capture-${captureID}.png`, { type: blob.type }),
    screen,
  }
}

export function newReviewValidationIssue(
  draft: NewReviewDraft,
  evidence: NewReviewEvidence,
  preview?: FigmaPreview,
): NewReviewValidationKey | undefined {
  if (!draft.requirementId) return 'invalidRequirement'
  if (!validFigmaURL(draft.figmaUrl)) return 'invalidFigma'
  if ((draft.mode === 'web' && !validHTTPURL(draft.implementationUrl)) || (draft.mode === 'mobile' && !draft.appId.trim())) return 'invalidTarget'
  if (draft.mode === 'mobile' && (!evidence.mobileCaptureId || !evidence.mobileTarget)) return 'mobileCaptureRequired'
  if (!preview || !previewMatchesFigma(preview, draft.figmaUrl)) return 'figmaEvidenceRequired'
  if (!evidence.implementation) return 'implementationEvidenceRequired'
  if (draft.mode === 'web' && evidence.captureViewport && (
    evidence.captureViewport.width !== draft.viewportWidth
    || evidence.captureViewport.height !== draft.viewportHeight - (draft.topbar ? 44 : 0)
    || evidence.captureViewport.devicePixelRatio !== draft.devicePixelRatio
  )) return 'evidenceViewportMismatch'
  if (draft.mode === 'web' && draft.domNormalization && (!evidence.implementationOriginal || !evidence.normalizationManifest || evidence.normalizationFigmaUrl !== draft.figmaUrl.trim())) return 'normalizationFailed'
  if (draft.scoped && !validRegion(draft.region, preview.width, preview.height, draft.viewportWidth, draft.viewportHeight)) return 'invalidRegion'
  return undefined
}

export function editableMobileTarget(target: DesignReviewTarget, draft: NewReviewDraft): DesignReviewTarget {
  return {
    ...target,
    platform: draft.platform,
    app_id: draft.appId.trim(),
    app_name: draft.appName.trim(),
    app_version: draft.appVersion.trim(),
    orientation: draft.orientation,
  }
}

function validRegion(
  region: NewReviewDraft['region'],
  designWidth: number,
  designHeight: number,
  implementationWidth: number,
  implementationHeight: number,
) {
  return region.x >= 0 && region.y >= 0 && region.width > 0 && region.height > 0
    && region.x + region.width <= 1 && region.y + region.height <= 1
    && region.width * designWidth >= 16 && region.height * designHeight >= 16
    && region.width * implementationWidth >= 16 && region.height * implementationHeight >= 16
}
