import { useCallback, useMemo } from 'react'

import { qualityLifecycleModel } from '../model/selectors'
import type {
  QualityDetailState,
  QualityLifecycleAction,
  QualityMutationState,
} from '../model/types'

export function useQualityLifecycleWorkflow({
  detail,
  transition,
  canAssign,
  canResolve,
  canReturn,
  canTransition,
  openAssign,
  openResolve,
  openReturn,
}: {
  detail: QualityDetailState
  transition: { state: QualityMutationState; run: (status: string) => Promise<unknown> }
  canAssign: boolean
  canResolve: boolean
  canReturn: boolean
  canTransition: boolean
  openAssign: () => void
  openResolve: () => void
  openReturn: () => void
}) {
  const model = useMemo(
    () => {
      if (detail.status !== 'ready') return null
      const current = qualityLifecycleModel(detail.bug, transition.state)
      return {
        ...current,
        actions: current.actions.filter((action) => {
          if (action.kind === 'assign') return canAssign
          if (action.kind === 'resolve') return canResolve
          if (action.kind === 'return') return canReturn
          return canTransition
        }),
      }
    },
    [canAssign, canResolve, canReturn, canTransition, detail, transition.state],
  )
  const run = useCallback((action: QualityLifecycleAction) => {
    if (action.kind === 'assign') return openAssign()
    if (action.kind === 'resolve') return openResolve()
    if (action.kind === 'return') return openReturn()
    void transition.run(action.status).catch(() => undefined)
  }, [openAssign, openResolve, openReturn, transition.run])

  return { model, run }
}
