const EXTENSION_SOURCE = 'pm-design-review-extension'

export function captureResultMessage<T extends object>(evidence: T) {
  return {
    ...evidence,
    source: EXTENSION_SOURCE,
    type: 'PM_DESIGN_REVIEW_CAPTURE_RESULT',
  } as const
}
