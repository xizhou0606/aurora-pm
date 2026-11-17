import { WORK_ITEMS_COPY } from '@/shared/copy'
import { DetailSection } from '@/shared/ui'
import type {
  BatchMutationResult,
  MutationState,
  UserOptionsState,
  WorkItemChildCreatePayload,
  WorkItemChildResource,
  WorkItemStatusOption,
} from '../../model/types'
import { ChildCreatePanel } from './ChildCreatePanel'
import { ChildList } from './ChildList'

export function Children({
  resource,
  users,
  workflow,
  mutation,
  canCreate,
  create,
}: {
  resource: WorkItemChildResource
  users: UserOptionsState
  workflow: readonly WorkItemStatusOption[]
  mutation: MutationState
  canCreate: boolean
  create: (drafts: readonly WorkItemChildCreatePayload[]) => Promise<BatchMutationResult>
}) {
  return (
    <DetailSection id="work-items-detail-children" title={WORK_ITEMS_COPY.detail.children} actions={canCreate ? <ChildCreatePanel mutation={mutation} users={users} statuses={workflow} create={create} /> : undefined}>
      <ChildList resource={resource} users={users} workflow={workflow} />
    </DetailSection>
  )
}
