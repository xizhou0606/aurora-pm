import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  PMScopeCascader,
  type PMScopeCascaderLabels,
  type PMScopeCascaderState,
} from '@/shared/ui'
import type {
  ScopeOptionsState,
  WorkItemsScope,
} from '../../model/types'
import { scopeSummary } from '../../model/utils'

type ScopeProps = {
  state: ScopeOptionsState
  value: WorkItemsScope
  boardOnly?: boolean
  onChange: (scope: WorkItemsScope) => void
}

const SCOPE_LABELS = {
  label: WORK_ITEMS_COPY.scope.label,
  projectSet: WORK_ITEMS_COPY.scope.projectSet,
  project: WORK_ITEMS_COPY.scope.pmProject,
  requirement: WORK_ITEMS_COPY.scope.requirement,
  selectProjectSetFirst: WORK_ITEMS_COPY.scope.selectProjectSetFirst,
  selectProjectFirst: WORK_ITEMS_COPY.scope.selectProjectFirst,
} satisfies PMScopeCascaderLabels

function cascaderState(state: ScopeOptionsState): PMScopeCascaderState {
  if (state.status === 'loading') return { status: 'loading' }
  if (state.status === 'error') return { status: 'error', onRetry: state.retry }
  return { status: 'ready', options: state.data }
}

export function Scope({
  state,
  value,
  boardOnly = false,
  onChange,
}: ScopeProps) {
  const cascaderValue = boardOnly && state.status === 'ready'
    ? {
      projectSetIds: state.data.projectSets.map(({ id }) => id),
      projectIds: value.projectIds,
      requirementIds: value.requirementIds,
    }
    : value
  return (
    <PMScopeCascader
      presentation="control"
      className="w-48 max-w-full"
      idPrefix="work-items"
      state={cascaderState(state)}
      labels={boardOnly ? {
        ...SCOPE_LABELS,
        hierarchyHint: WORK_ITEMS_COPY.scope.boardHierarchyHint,
      } : SCOPE_LABELS}
      levels={boardOnly ? 'project-requirement' : 'all'}
      value={cascaderValue}
      summary={scopeSummary(value)}
      onChange={(selection) => onChange(boardOnly ? {
        projectSetIds: [],
        projectIds: selection.projectIds,
        requirementIds: selection.requirementIds,
      } : selection)}
    />
  )
}
