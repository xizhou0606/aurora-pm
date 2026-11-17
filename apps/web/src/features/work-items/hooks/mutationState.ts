import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { MutationState } from '../model/types'

export function toMutationState(input: {
  status: 'idle' | 'pending' | 'error' | 'success'
  action: string
  failures?: readonly { id: string | number; code: string; message: string }[]
  succeeded?: number
}): MutationState {
  if (input.status === 'pending') return { status: 'pending', action: input.action }
  if (input.status === 'error') {
    return { status: 'error', action: input.action, message: WORK_ITEMS_COPY.mutation.failed }
  }
  if (input.status === 'success' && input.failures?.length) {
    return {
      status: 'partial', action: input.action,
      succeeded: input.succeeded || 0, failures: input.failures,
    }
  }
  if (input.status === 'success') return { status: 'success', action: input.action }
  return { status: 'idle' }
}
