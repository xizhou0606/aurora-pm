import { QUALITY_COPY } from '@/shared/copy'
import type { PMScopeSelection } from '@/shared/api'
import { PMScopeCascader, type PMScopeCascaderLabels } from '@/shared/ui'
import type { QualityScope } from '../../model/types'
import type { QualityScopeState } from '../../hooks'

const SCOPE_LABELS = {
  label: QUALITY_COPY.scope.label,
  projectSet: QUALITY_COPY.scope.projectSet,
  project: QUALITY_COPY.scope.project,
  requirement: QUALITY_COPY.scope.requirement,
  selectProjectSetFirst: QUALITY_COPY.scope.selectProjectSetFirst,
  selectProjectFirst: QUALITY_COPY.scope.selectProjectFirst,
} satisfies PMScopeCascaderLabels

export function Scope({
  state,
  value,
  onChange,
}: {
  state: QualityScopeState
  value: QualityScope
  onChange: (scope: QualityScope) => void
}) {
  const setSelection = (selection: PMScopeSelection) => onChange({
    projectSetIds: [...selection.projectSetIds],
    projectIds: [...selection.projectIds],
    requirementIds: [...selection.requirementIds],
  })
  if (state.status === 'loading') {
    return (
      <PMScopeCascader
        presentation="control"
        className="w-48 max-w-full"
        idPrefix="quality"
        state={{ status: 'loading' }}
        labels={SCOPE_LABELS}
        value={value}
        onChange={setSelection}
      />
    )
  }
  if (state.status === 'error') {
    return (
      <PMScopeCascader
        presentation="control"
        className="w-48 max-w-full"
        idPrefix="quality"
        state={{ status: 'error', onRetry: () => void state.retry() }}
        labels={SCOPE_LABELS}
        value={value}
        onChange={setSelection}
      />
    )
  }
  if (state.status === 'unavailable') return null
  const count = value.projectSetIds.length + value.projectIds.length + value.requirementIds.length
  return (
    <PMScopeCascader
      presentation="control"
      className="w-48 max-w-full"
      idPrefix="quality"
      state={{ status: 'ready', options: state.data }}
      labels={SCOPE_LABELS}
      value={value}
      summary={count ? QUALITY_COPY.scope.summarySelected(count) : QUALITY_COPY.scope.summaryAll}
      onChange={setSelection}
    />
  )
}
