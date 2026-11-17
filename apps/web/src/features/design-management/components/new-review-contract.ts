import type {
  CreateMobileCaptureInput,
  DesignReviewDetail,
  FigmaPreview,
  FigmaPreviewErrorCopyKey,
  MobileCaptureDraft,
  MobileCollector,
  NewReviewSubmission,
  RequirementOption,
} from '../model'

export type NewReviewWorkbenchProps = {
  requirements: readonly RequirementOption[]
  requirementsState: 'loading' | 'error' | 'ready'
  onRetryRequirements: () => void
  sourceReview?: DesignReviewDetail
  initialRequirementId?: number
  preview?: FigmaPreview
  previewState: 'idle' | 'pending' | 'error' | 'ready'
  previewErrorKey?: FigmaPreviewErrorCopyKey
  submitState: 'idle' | 'pending' | 'error'
  message?: string
  collectors: readonly MobileCollector[]
  captures: readonly MobileCaptureDraft[]
  mobileCaptureState: 'idle' | 'pending' | 'error'
  onPreview: (figmaUrl: string) => void
  onResetPreview: () => void
  onNormalizeDOM: (figmaUrl: string, inventory: Record<string, unknown>) => Promise<Record<string, unknown>>
  onStartMobileCapture: (input: CreateMobileCaptureInput) => void
  onLoadMobileCapture: (id: string) => Promise<Blob>
  onSubmit: (input: NewReviewSubmission) => void
}
