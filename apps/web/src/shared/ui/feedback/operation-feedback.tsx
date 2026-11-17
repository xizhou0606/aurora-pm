import { PhCheckCircle, PhXCircle, PhX } from '@/shared/ui/icons'
import { useSyncExternalStore, type PropsWithChildren } from 'react'

import { isSafeApiError, PM_API_ERROR_CODES, PMSafeApiError } from '@/shared/api'
import { copy, OPERATION_FEEDBACK_COPY } from '@/shared/copy'

export type OperationFeedback = {
  success: string
  failure: string
}

export type OperationFeedbackMeta = {
  operationFeedback: OperationFeedback
}

export function operationFeedback(
  success: string | OperationFeedback,
  failure = '操作失败',
): OperationFeedbackMeta {
  return {
    operationFeedback: typeof success === 'string'
      ? { success, failure }
      : success,
  }
}

export function operationValidationError(code = 'FORM_VALIDATION_FAILED'): Error {
  return new PMSafeApiError(code, 422)
}

function feedbackFromMeta(meta: unknown): OperationFeedback | undefined {
  if (!meta || typeof meta !== 'object') return undefined
  const feedback = (meta as Partial<OperationFeedbackMeta>).operationFeedback
  return feedback && typeof feedback.success === 'string' && typeof feedback.failure === 'string'
    ? feedback
    : undefined
}

export function operationErrorDescription(error: unknown): string {
  const errorCopy = OPERATION_FEEDBACK_COPY.error
  if (!isSafeApiError(error)) return errorCopy.default
  if (error.status === 401) return errorCopy.unauthorized
  if (error.status === 403) return errorCopy.forbidden
  if (error.status === 404) return errorCopy.notFound
  if (error.status === 409 && error.code === 'WORKFLOW_AI_STALE') return errorCopy.aiStale
  if (error.status === 409 && error.code === 'WORKFLOW_NEXT_ASSIGNEE_REQUIRED') return errorCopy.nextAssignee
  if (error.status === 409 && error.code === PM_API_ERROR_CODES.REQUIREMENT_SCOPE_EMPTY) return errorCopy.requirementScopeEmpty
  if (error.status === 409) return errorCopy.conflict
  if (error.code === PM_API_ERROR_CODES.QUALITY_ASSIGNMENT_NOTE_TOO_LONG) {
    return errorCopy.qualityAssignmentNoteTooLong
  }
  if (error.code === PM_API_ERROR_CODES.QUALITY_RESOLUTION_NOTE_TOO_LONG) {
    return errorCopy.qualityResolutionNoteTooLong
  }
  if (error.code === PM_API_ERROR_CODES.QUALITY_REGRESSION_NOTE_TOO_LONG) {
    return errorCopy.qualityRegressionNoteTooLong
  }
  if (error.status === 422) return errorCopy.validation
  if (error.status !== undefined && error.status >= 500) return errorCopy.server
  if (error.status === undefined) return errorCopy.network
  return errorCopy.request
}

type OperationToast = {
  id: number
  title: string
  description?: string
  type: 'success' | 'error'
}

let nextToastId = 0
let operationToasts: OperationToast[] = []
const toastListeners = new Set<() => void>()
const toastTimers = new Map<number, ReturnType<typeof setTimeout>>()

function publishToasts() {
  toastListeners.forEach((listener) => listener())
}

function dismissToast(id: number) {
  const timer = toastTimers.get(id)
  if (timer) clearTimeout(timer)
  toastTimers.delete(id)
  const next = operationToasts.filter((toast) => toast.id !== id)
  if (next.length === operationToasts.length) return
  operationToasts = next
  publishToasts()
}

function addToast(toast: Omit<OperationToast, 'id'>, timeout: number) {
  const id = ++nextToastId
  const next = [...operationToasts, { ...toast, id }]
  const removed = next.slice(0, -4)
  removed.forEach((item) => dismissToast(item.id))
  operationToasts = next.slice(-4)
  publishToasts()
  toastTimers.set(id, setTimeout(() => dismissToast(id), timeout))
}

function subscribeToToasts(listener: () => void) {
  toastListeners.add(listener)
  return () => toastListeners.delete(listener)
}

function getToasts() {
  return operationToasts
}

export function notifyOperationSuccess(meta: unknown): void {
  const feedback = feedbackFromMeta(meta)
  if (!feedback) return
  addToast({
    title: feedback.success,
    type: 'success',
  }, 3200)
}

export function notifyOperationError(meta: unknown, error: unknown): void {
  const feedback = feedbackFromMeta(meta)
  if (!feedback || (isSafeApiError(error) && error.aborted)) return
  addToast({
    title: feedback.failure,
    description: operationErrorDescription(error),
    type: 'error',
  }, 6000)
}

function OperationToastList() {
  const toasts = useSyncExternalStore(subscribeToToasts, getToasts, getToasts)
  return toasts.map((toast) => {
    const success = toast.type === 'success'
    const Icon = success ? PhCheckCircle : PhXCircle
    return (
      <article
        key={toast.id}
        role={success ? undefined : 'alert'}
        data-operation-feedback={toast.type}
        className="relative w-full overflow-hidden rounded-lg border border-border bg-card text-card-foreground shadow-lg"
      >
        <div className="pm-toast__content flex items-start gap-3 p-3.5">
          <Icon
            aria-hidden="true"
            className={success ? 'mt-0.5 size-5 shrink-0 text-emerald-600' : 'mt-0.5 size-5 shrink-0 text-destructive'}
          />
          <div className="min-w-0 flex-1">
            <p className="m-0 text-sm font-semibold">{toast.title}</p>
            {toast.description ? <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{toast.description}</p> : null}
          </div>
          <button
            type="button"
            aria-label={copy.ui.close}
            className="pm-toast__close inline-flex size-8 shrink-0 items-center justify-center rounded-md border-0 bg-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
            onClick={() => dismissToast(toast.id)}
          >
            <PhX aria-hidden="true" className="size-4" />
          </button>
        </div>
      </article>
    )
  })
}

export function OperationFeedbackProvider({ children }: PropsWithChildren) {
  return (
    <>
      {children}
      <div aria-live="polite" aria-relevant="additions" className="pm-toast-viewport fixed flex max-w-sm flex-col gap-2 outline-none">
        <OperationToastList />
      </div>
    </>
  )
}
