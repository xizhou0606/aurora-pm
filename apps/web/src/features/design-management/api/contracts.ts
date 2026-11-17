import type {
  CreateReviewInput,
  CreateMobileCaptureInput,
  DesignReviewDetail,
  DesignReviewSummary,
  DifferenceInput,
  HistoricalRecheckStatus,
  FigmaPreview,
  MobileCaptureDraft,
  MobileCollector,
  RequirementOption,
  ReviewCapabilities,
  ReviewListInput,
  ReviewPage,
} from '../model'

export type ReviewClient = {
  capabilities(signal?: AbortSignal): Promise<ReviewCapabilities>
  requirements(signal?: AbortSignal): Promise<RequirementOption[]>
  list(input: ReviewListInput, signal?: AbortSignal): Promise<ReviewPage>
  detail(id: string, signal?: AbortSignal): Promise<DesignReviewDetail>
  create(input: CreateReviewInput): Promise<DesignReviewDetail>
  createRecheck(id: string, input: CreateReviewInput): Promise<DesignReviewDetail>
  figmaPreview(figmaUrl: string): Promise<FigmaPreview>
  asset(id: string, assetId: string, signal?: AbortSignal): Promise<Blob>
  normalizeDOM(figmaUrl: string, inventory: Record<string, unknown>): Promise<Record<string, unknown>>
  uploadAsset(id: string, kind: string, file: File): Promise<void>
  queueAnalysis(id: string): Promise<void>
  retryAnalysis(id: string): Promise<void>
  abandonAnalysis(id: string): Promise<void>
  remove(id: string): Promise<void>
  exportFile(id: string, format: 'markdown' | 'json'): Promise<Blob>
}

export type DifferenceClient = {
  create(reviewId: string, input: DifferenceInput): Promise<unknown>
  update(reviewId: string, id: string, input: DifferenceInput): Promise<unknown>
  remove(reviewId: string, id: string): Promise<void>
  verify(reviewId: string, id: string, status: 'confirmed' | 'false_positive'): Promise<unknown>
  resolve(reviewId: string, id: string): Promise<unknown>
  reopen(reviewId: string, id: string): Promise<unknown>
  createDeepReview(input: {
    review_id: string
    design_region: { x: number; y: number; width: number; height: number }
    implementation_region: { x: number; y: number; width: number; height: number }
    focus: string
  }): Promise<unknown>
  retryDeepReview(reviewId: string, id: string): Promise<void>
  cancelDeepReview(reviewId: string, id: string): Promise<void>
  resolveRecheckMatch(reviewId: string, id: string, resolution: 'inherit' | 'new'): Promise<void>
  overrideRecheckVerification(reviewId: string, id: string, status: HistoricalRecheckStatus | null): Promise<void>
}

export type CollectorClient = {
  list(signal?: AbortSignal): Promise<MobileCollector[]>
  captures(signal?: AbortSignal): Promise<MobileCaptureDraft[]>
  createPairingCode(): Promise<{ code: string; expires_at: string }>
  createCapture(input: CreateMobileCaptureInput): Promise<MobileCaptureDraft>
  captureAsset(id: string): Promise<Blob>
  consumeCapture(id: string): Promise<MobileCaptureDraft>
  revoke(id: string): Promise<void>
  removeCapture(id: string): Promise<void>
}

export type { DesignReviewSummary }
