import { PhPlus } from '@/shared/ui/icons'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { Button, ResponsivePanel } from '@/shared/ui'
import { useChildDrafts } from '../../hooks'
import type {
  BatchMutationResult,
  MutationState,
  UserOptionsState,
  WorkItemChildCreatePayload,
  WorkItemStatusOption,
} from '../../model/types'
import { ChildDraftTable } from './ChildDraftTable'

export function ChildCreatePanel({
  mutation,
  users,
  statuses,
  create,
}: {
  mutation: MutationState
  users: UserOptionsState
  statuses: readonly WorkItemStatusOption[]
  create: (drafts: readonly WorkItemChildCreatePayload[]) => Promise<BatchMutationResult>
}) {
  const readyUsers = users.status === 'ready' ? users.data : []
  const pending = mutation.status === 'pending'
  const draft = useChildDrafts({ users: readyUsers, statuses, pending, create })
  const rows = draft.state.status === 'editing' ? draft.state.rows : []
  const errors = draft.state.status === 'editing' ? draft.state.errors : {}
  const firstError = rows.findIndex(({ clientId }) => Boolean(errors[clientId]))
  const result = draft.state.status === 'editing' ? draft.state.result : undefined
  return (
    <>
      <Button type="button" size="sm" variant="outline" onClick={draft.open}>
        <PhPlus aria-hidden="true" />
        {WORK_ITEMS_COPY.detail.createChildren}
      </Button>
      <ResponsivePanel
        open={draft.state.status === 'editing'}
        onOpenChange={(open) => open ? draft.open() : draft.close()}
        title={WORK_ITEMS_COPY.detail.createChildren}
        description={WORK_ITEMS_COPY.detail.childCreateDescription}
        closeLabel={WORK_ITEMS_COPY.mutation.cancel}
        pending={pending}
        className="max-w-[min(96vw,1180px)]"
        contentClassName="overflow-x-hidden"
        footer={(
          <Button type="button" disabled={pending || !rows.length} onClick={() => void draft.submit()}>
            {WORK_ITEMS_COPY.detail.createChildren}
          </Button>
        )}
      >
        <div className="grid gap-3">
          {result ? (
            <p role="status" className="m-0 rounded-md border border-amber-300 bg-amber-50 dark:bg-amber-950/40 p-2 text-sm text-amber-800">
              {WORK_ITEMS_COPY.detail.childPartialResult(result.succeeded, result.failures.length)}
            </p>
          ) : null}
          {firstError >= 0 ? (
            <p role="alert" className="m-0 text-sm text-destructive">
              {WORK_ITEMS_COPY.detail.childValidationError(firstError + 1)}
            </p>
          ) : null}
          <ChildDraftTable
            rows={rows}
            errors={errors}
            disabled={pending}
            users={users}
            statuses={statuses}
            onUpdate={draft.update}
            onRemove={draft.remove}
          />
          <Button type="button" size="sm" variant="outline" className="w-fit" disabled={pending} onClick={draft.add}>
            <PhPlus aria-hidden="true" />
            {WORK_ITEMS_COPY.detail.addChildRow}
          </Button>
        </div>
      </ResponsivePanel>
    </>
  )
}
