import { BI_SCREEN_COPY } from '@/shared/copy'
import {
  PMScopeCascader,
  type PMScopeCascaderLabels,
} from '@/shared/ui'

import type { Scope as ScopeValue, ScopeState } from '../types'

const SCOPE_LABELS = {
  label: BI_SCREEN_COPY.scope.label,
  projectSet: BI_SCREEN_COPY.scope.projectSet,
  project: BI_SCREEN_COPY.scope.project,
  requirement: BI_SCREEN_COPY.scope.requirement,
  selectProjectSetFirst: BI_SCREEN_COPY.scope.selectProjectSetFirst,
  selectProjectFirst: BI_SCREEN_COPY.scope.selectProjectFirst,
  noOptions: BI_SCREEN_COPY.scope.noOptions,
} satisfies PMScopeCascaderLabels

export function ScopeControl({
  state,
  value,
  summary,
  onChange,
}: {
  state: ScopeState
  value: ScopeValue
  summary: string
  onChange: (scope: ScopeValue) => void
}) {
  if (state.status === 'loading') {
    return (
      <PMScopeCascader
        presentation="control"
        className="w-48 max-w-full"
        idPrefix="bi-screen"
        state={{ status: 'loading' }}
        labels={SCOPE_LABELS}
        value={value}
        onChange={onChange}
      />
    )
  }
  if (state.status === 'error') {
    return (
      <PMScopeCascader
        presentation="control"
        className="w-48 max-w-full"
        idPrefix="bi-screen"
        state={{ status: 'error', onRetry: state.retry }}
        labels={SCOPE_LABELS}
        value={value}
        onChange={onChange}
      />
    )
  }
  if (state.status === 'invalid') return null

  return (
    <PMScopeCascader
      presentation="control"
      className="w-48 max-w-full"
      idPrefix="bi-screen"
      state={{ status: 'ready', options: state.data }}
      labels={SCOPE_LABELS}
      value={value}
      summary={summary}
      onChange={onChange}
    />
  )
}
