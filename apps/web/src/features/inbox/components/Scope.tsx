import { INBOX_COPY } from '@/shared/copy'
import { PMScopeCascader } from '@/shared/ui'
import { scopeSummary, type InboxScope, type ScopeOptionsState } from '../model'

type ScopeProps = {
  state: ScopeOptionsState
  value: InboxScope
  onChange: (scope: InboxScope) => void
}

export function Scope({ state, value, onChange }: ScopeProps) {
  return (
    <PMScopeCascader
      idPrefix="inbox-scope"
      aria-label={INBOX_COPY.scope.label}
      state={state.status === 'ready'
        ? { status: 'ready', options: state.data }
        : state.status === 'error'
          ? { status: 'error', onRetry: state.retry }
          : { status: 'loading' }}
      value={value}
      labels={{
        label: INBOX_COPY.scope.label,
        projectSet: INBOX_COPY.scope.projectSet,
        project: INBOX_COPY.scope.project,
        requirement: INBOX_COPY.scope.requirement,
        selectProjectSetFirst: INBOX_COPY.scope.selectProjectSetFirst,
        selectProjectFirst: INBOX_COPY.scope.selectProjectFirst,
      }}
      summary={scopeSummary(value)}
      presentation="control"
      className="w-48 max-w-full"
      onChange={onChange}
    />
  )
}
