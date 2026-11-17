import { useNavigate, useParams } from '@tanstack/react-router'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton, OperationalPage } from '@/shared/ui'
import { useRequirement } from '../api/api'
import { useInjectedAIPMEnvironment } from '../components/environment'
import { requirementRoleOwners, sameOwnerIDs, type RequirementRoleOwners } from '../utils/requirement-role-owners'
import type { Requirement } from '../model/types'
import { useRequirementsOperationAccess } from '../hooks/useOperationAccess'
import { RequirementDetailReady } from './RequirementDetailReady'

export const detailCopy = copy.aiPM.requirementDetail
export const commonCopy = copy.aiPM.common
export const poolCopy = copy.aiPM.requirementPool
export const statusLabels: Record<string, string> = {
  backlog: poolCopy.backlog,
  grooming: poolCopy.grooming,
  development: poolCopy.development,
  acceptance: poolCopy.acceptance,
  done: poolCopy.done,
}
export function requirementStatusTone(status?: string) {
  switch (status) {
    case 'development': return 'info' as const
    case 'done': return 'success' as const
    case 'backlog':
    case 'grooming':
    case 'acceptance': return 'warning' as const
    default: return 'neutral' as const
  }
}
export const priorityOptions = [
  { value: 'p0', label: poolCopy.p0 },
  { value: 'p1', label: poolCopy.p1 },
  { value: 'p2', label: poolCopy.p2 },
  { value: 'p3', label: poolCopy.p3 },
]

export type RequirementEditable = Pick<Requirement, 'title' | 'priority'> & {
  description: string
  acceptance_criteria: string
} & RequirementRoleOwners

export type RequirementEditor = {
  base: { version: number; fields: RequirementEditable }
  draft: RequirementEditable
}

export function editableFields(requirement: Requirement): RequirementEditable {
  return {
    title: requirement.title,
    description: requirement.description ?? '',
    acceptance_criteria: requirement.acceptance_criteria ?? '',
    priority: requirement.priority,
    ...requirementRoleOwners(requirement),
  }
}

export function sameEditable(left: RequirementEditable, right: RequirementEditable) {
  return left.title === right.title &&
    left.description === right.description &&
    left.acceptance_criteria === right.acceptance_criteria &&
    left.priority === right.priority &&
    sameOwnerIDs(left.product_owner_user_ids, right.product_owner_user_ids) &&
    sameOwnerIDs(left.design_owner_user_ids, right.design_owner_user_ids) &&
    sameOwnerIDs(left.frontend_owner_user_ids, right.frontend_owner_user_ids) &&
    sameOwnerIDs(left.backend_owner_user_ids, right.backend_owner_user_ids) &&
    sameOwnerIDs(left.tester_user_ids, right.tester_user_ids)
}

export function initialEditor(requirement: Requirement): RequirementEditor {
  const fields = editableFields(requirement)
  return { base: { version: requirement.version, fields }, draft: fields }
}

export function reconcileRequirement(editor: RequirementEditor, requirement: Requirement): RequirementEditor {
  if (requirement.version < editor.base.version) return editor
  const incoming = editableFields(requirement)
  if (sameEditable(incoming, editor.base.fields)) {
    return { ...editor, base: { version: requirement.version, fields: incoming } }
  }
  if (sameEditable(incoming, editor.draft)) {
    return { base: { version: requirement.version, fields: incoming }, draft: incoming }
  }
  if (sameEditable(editor.draft, editor.base.fields)) {
    return { base: { version: requirement.version, fields: incoming }, draft: incoming }
  }
  return editor
}

export function RequirementDetailPage() {
  'use no memo'
  const injected = useInjectedAIPMEnvironment()
  if (injected) {
    const workflowNode = typeof injected.search.workflow_node === 'string'
      ? injected.search.workflow_node
      : undefined
    return (
      <RequirementDetailSession
        initialNodeId={workflowNode}
        onBack={() => void injected.navigate('/workspace/requirements')}
        requirementId={injected.requirementId ?? 0}
        workspaceId={injected.workspaceId}
      />
    )
  }
  return <RoutedRequirementDetailPage />
}

function RoutedRequirementDetailPage() {
  'use no memo'
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const params = useParams({ from: '/workspace/requirements/$requirementId' })
  const navigate = useNavigate()
  return (
    <RequirementDetailSession
      onBack={() => void navigate({ to: '/workspace/requirements' } as never)}
      requirementId={Number(params.requirementId)}
      workspaceId={workspaceId}
    />
  )
}

function RequirementDetailSession({
  initialNodeId,
  onBack,
  requirementId,
  workspaceId,
}: {
  initialNodeId?: string
  onBack: () => void
  requirementId: number
  workspaceId: number
}) {
  'use no memo'
  const access = useRequirementsOperationAccess()
  const query = useRequirement(workspaceId, requirementId)
  if (access.status === 'ready' && !access.canViewPool()) return null
  if (query.isPending) return <OperationalPage title={detailCopy.title}><div className="p-5"><LoadingSkeleton rows={10} /></div></OperationalPage>
  if (!query.data) return <OperationalPage title={detailCopy.title}><div className="p-5"><EmptyState role="alert" title={detailCopy.loadFailed} action={<Button variant="outline" onClick={onBack}>{detailCopy.backPool}</Button>} /></div></OperationalPage>
  return <RequirementDetailReady
    key={`${workspaceId}:${query.data.id}`}
    initialNodeId={initialNodeId}
    onBack={onBack}
    requirement={query.data}
    workspaceId={workspaceId}
    reloadRequirement={async () => (
      await query.refetch({ throwOnError: true })
    ).data}
  />
}
