import { copy } from '@/shared/copy'
import { EmptyState, StatusPill } from '@/shared/ui'
import { Link } from '@tanstack/react-router'
import { usePlanUnitIntakeRequirements } from '../api/workflow-queries'

const subjectCopy = copy.aiPM.workflowSubject
const poolCopy = copy.aiPM.requirementPool

const statusLabels: Record<string, string> = {
  backlog: poolCopy.backlog,
  grooming: poolCopy.grooming,
  development: poolCopy.development,
  acceptance: poolCopy.acceptance,
  done: poolCopy.done,
}

const priorityLabels: Record<string, string> = {
  p0: poolCopy.p0,
  p1: poolCopy.p1,
  p2: poolCopy.p2,
  p3: poolCopy.p3,
}

// WorkflowPlanUnitRequirementScope renders the requirements already entered
// into the plan unit inside the project 需求录入 node dialog — the same set
// that the intake lock will freeze against new creations.
export function WorkflowPlanUnitRequirementScope({ workspaceId, planUnitId }: {
  workspaceId: number
  planUnitId: number
}) {
  const query = usePlanUnitIntakeRequirements(workspaceId, planUnitId)
  if (query.isPending) {
    return (
      <div className="grid gap-2 rounded-lg border border-border bg-muted/70 p-3 text-xs">
        <strong className="text-sm text-foreground">{subjectCopy.intakeScopeTitle}</strong>
        <span className="text-xs text-muted-foreground">{copy.aiPM.common.loading}</span>
      </div>
    )
  }
  if (query.isError || !query.data) {
    return (
      <div className="grid gap-2 rounded-lg border border-border bg-muted/70 p-3 text-xs">
        <strong className="text-sm text-foreground">{subjectCopy.intakeScopeTitle}</strong>
        <span role="alert" className="text-xs text-muted-foreground">{subjectCopy.intakeScopeLoadFailed}</span>
      </div>
    )
  }
  const rows = query.data
  return (
    <div data-testid="workflow-intake-requirement-scope" className="grid gap-2 rounded-lg border border-border bg-muted/70 p-3 text-xs">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <strong className="text-sm text-foreground">{subjectCopy.intakeScopeTitle}</strong>
        <span className="text-muted-foreground">{subjectCopy.intakeScopeCount(rows.length)}</span>
      </div>
      {rows.length === 0 ? (
        <EmptyState title={subjectCopy.intakeScopeEmpty} />
      ) : (
        <ul className="m-0 grid list-none gap-1.5 p-0">
          {rows.map((row) => (
            <li key={row.id} className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-card px-2.5 py-1.5">
              <Link
                to="/workspace/requirements/$requirementId"
                params={{ requirementId: String(row.id) }}
                className="min-w-0 flex-1 truncate font-medium text-blue-600 hover:underline dark:text-blue-400"
              >
                {row.title}
              </Link>
              {priorityLabels[row.priority] ? <span className="text-muted-foreground">{priorityLabels[row.priority]}</span> : null}
              <StatusPill tone={row.status === 'done' ? 'success' : 'neutral'}>
                {statusLabels[row.status] ?? row.status}
              </StatusPill>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
