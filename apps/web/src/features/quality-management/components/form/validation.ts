import type { FormEvent } from 'react'

import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import {
  notifyOperationError,
  operationFeedback,
  operationValidationError,
  richTextVisibleCharacterCount,
} from '@/shared/ui'

import type { QualityBugDraft } from '../../model/types'
import { QUALITY_NOTE_MAX_CHARACTERS, QUALITY_RESOLUTION_NOTE_TOO_LONG_CODE } from '../../model/payloads'

export type FormValidationField = 'title' | 'requirement' | 'resolution' | 'resolutionNote' | ''

export function validateForm(
  draft: QualityBugDraft,
  setValidation: (field: FormValidationField) => void,
): { valid: boolean } {
  if (!draft.title.trim()) {
    setValidation('title')
    notifyOperationError(
      operationFeedback(OPERATION_FEEDBACK_COPY.quality.validation),
      operationValidationError('QUALITY_TITLE_REQUIRED'),
    )
    window.requestAnimationFrame(() => document.getElementById('quality-form-title')?.focus())
    return { valid: false }
  }
  if (!draft.requirementId) {
    setValidation('requirement')
    notifyOperationError(
      operationFeedback(OPERATION_FEEDBACK_COPY.quality.validation),
      operationValidationError('QUALITY_REQUIREMENT_REQUIRED'),
    )
    window.requestAnimationFrame(() => document.getElementById('quality-form-requirement-trigger')?.focus())
    return { valid: false }
  }
  if ((draft.status === 'review' || draft.status === 'done') && !draft.resolutionReason) {
    setValidation('resolution')
    notifyOperationError(
      operationFeedback(OPERATION_FEEDBACK_COPY.quality.validation),
      operationValidationError('QUALITY_RESOLUTION_REQUIRED'),
    )
    window.requestAnimationFrame(() => document.getElementById('quality-form-resolution-reason')?.focus())
    return { valid: false }
  }
  if (
    (draft.status === 'review' || draft.status === 'done') &&
    richTextVisibleCharacterCount(draft.resolutionNote) > QUALITY_NOTE_MAX_CHARACTERS
  ) {
    setValidation('resolutionNote')
    notifyOperationError(
      operationFeedback(OPERATION_FEEDBACK_COPY.quality.validation),
      operationValidationError(QUALITY_RESOLUTION_NOTE_TOO_LONG_CODE),
    )
    return { valid: false }
  }
  return { valid: true }
}