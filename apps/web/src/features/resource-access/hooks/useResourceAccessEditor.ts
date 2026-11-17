import { useEffect, useReducer } from 'react'

import { isSafeApiError } from '@/shared/api'

import {
  createResourceAccessEditorState,
  resourceAccessEditorReducer,
  unavailableSubjectCapabilities,
  type ResourceAccessEditorIntent,
  type ResourceAccessPolicy,
  type ResourceAccessPolicyReplacement,
} from '../model'

function idempotencyKey(): string {
  return globalThis.crypto?.randomUUID?.() ?? `access-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function isResourceAccessConflict(error: unknown): boolean {
  return isSafeApiError(error) && (error.code === 'POLICY_CONFLICT' || error.status === 409)
}

export function isResourceAccessUnknownResult(error: unknown): boolean {
  return isSafeApiError(error) && (
    error.code === 'UNKNOWN_OUTCOME'
    || (error.status === undefined && !error.aborted)
  )
}

export function useResourceAccessEditor(input: {
  policy: ResourceAccessPolicy | null
  save: (replacement: ResourceAccessPolicyReplacement) => Promise<ResourceAccessPolicy>
  refresh: () => Promise<ResourceAccessPolicy | undefined>
}) {
  const initialPolicy = input.policy ?? {
    resourceId: '', mode: 'private' as const, workspaceLevel: null, rules: [], policyRevision: 0,
    canManageAccess: false, effectiveLevel: null, effectiveSources: [],
    subjectCapabilities: unavailableSubjectCapabilities(),
  }
  const [state, dispatch] = useReducer(
    resourceAccessEditorReducer,
    initialPolicy,
    createResourceAccessEditorState,
  )

  useEffect(() => {
    if (input.policy) dispatch({ type: 'hydrate', policy: input.policy })
  }, [input.policy])

  const submit = async () => {
    if (state.status !== 'dirty' || state.draft.mode === 'legacy') return
    dispatch({ type: 'submit' })
    try {
      const policy = await input.save({
        mode: state.draft.mode,
        ...(state.draft.mode === 'workspace' ? { workspaceLevel: state.draft.workspaceLevel } : {}),
        rules: state.draft.mode === 'restricted' ? state.draft.rules : [],
        expectedPolicyRevision: state.base.policyRevision,
        idempotencyKey: idempotencyKey(),
      })
      dispatch({ type: 'submit_succeeded', policy })
    } catch (error) {
      if (isResourceAccessConflict(error)) {
        const latest = await input.refresh().catch(() => undefined)
        if (latest) dispatch({ type: 'submit_conflicted', latest })
        else dispatch({ type: 'submit_failed' })
      } else if (isResourceAccessUnknownResult(error)) {
        dispatch({ type: 'submit_unknown' })
      } else {
        dispatch({ type: 'submit_failed' })
      }
      throw error
    }
  }

  const reconcile = async () => {
    const latest = await input.refresh()
    if (!latest) return
    if (state.status === 'unknown_result') {
      dispatch({ type: 'resolve_unknown', latest })
    } else {
      dispatch({ type: 'hydrate', policy: latest })
    }
  }

  const edit = (action: ResourceAccessEditorIntent) => {
    dispatch(action)
  }

  const reset = () => dispatch({ type: 'hydrate', policy: initialPolicy, force: true })

  return { state, dispatch: edit, submit, reconcile, reset }
}
