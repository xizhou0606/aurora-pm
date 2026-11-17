import { copy } from '@/shared/copy'
import { UserDirectorySelect, type UserDirectoryState } from '@/shared/ui'
import type { WorkflowNextAssigneeNode } from '../utils/workflow-next-assignees'

const subjectCopy = copy.aiPM.workflowSubject

export function WorkflowNextAssignees({
  nodes,
  values,
  users,
  disabled = false,
  onChange,
}: {
  nodes: WorkflowNextAssigneeNode[]
  values: Record<string, number | null>
  users: UserDirectoryState
  disabled?: boolean
  onChange: (nodeId: string, value: number | null) => void
}) {
  if (!nodes.length) return null
  const missing = nodes.some((node) => Number(values[node.id] || 0) <= 0)
  return (
    <section data-testid="workflow-next-assignees" className="grid gap-3 rounded-lg border border-amber-100 bg-amber-50/50 dark:bg-amber-950/40 p-3">
      <div>
        <strong className="block text-sm text-foreground">{subjectCopy.nextAssigneeTitle}</strong>
        <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.nextAssigneeHint}</p>
      </div>
      {missing && !disabled ? <p role="status" className="m-0 text-xs text-amber-800">{subjectCopy.nextAssigneeRequired}</p> : null}
      <div className={nodes.length > 1 ? 'grid gap-3 sm:grid-cols-2' : 'grid gap-3'}>
        {nodes.map((node) => (
          <UserDirectorySelect
            key={node.id}
            id={`workflow-next-assignee-${node.id}`}
            label={subjectCopy.nextAssigneeLabel(node.name)}
            value={values[node.id] ?? null}
            state={users}
            disabled={disabled}
            onValueChange={(value) => onChange(node.id, value ?? null)}
          />
        ))}
      </div>
    </section>
  )
}

export function workflowNextAssigneesPayload(values: Record<string, number | null>, nodeIds: string[]) {
  return nodeIds.flatMap((nodeId) => {
    const assignee = Number(values[nodeId] || 0)
    return Number.isInteger(assignee) && assignee > 0
      ? [{ node_id: nodeId, assignee_user_id: assignee }]
      : []
  })
}

export function workflowNextAssigneesReady(values: Record<string, number | null>, nodeIds: string[]) {
  return nodeIds.every((nodeId) => Number(values[nodeId] || 0) > 0)
}
