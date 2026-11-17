import { copy } from '@/shared/copy'
import { StatusPill, type StatusPillTone } from '@/shared/ui'
import type { WorkflowRequirementsGuardEvaluation } from '../model/types'

const subjectCopy = copy.aiPM.workflowSubject

const reasonCopy: Record<string, string> = {
  missing_binding: subjectCopy.guardReasonMissingBinding,
  no_workflow_instance: subjectCopy.guardReasonNoWorkflowInstance,
  template_version_unavailable: subjectCopy.guardReasonTemplateVersionUnavailable,
  binding_node_missing_from_template: subjectCopy.guardReasonBindingNodeMissingFromTemplate,
  binding_targets_not_completed: subjectCopy.guardReasonBindingTargetsNotCompleted,
}

function stateTone(state: string): StatusPillTone {
  if (state === 'satisfied') return 'success'
  if (state === 'unsatisfied') return 'warning'
  return 'neutral'
}

function stateLabel(state: string): string {
  if (state === 'satisfied') return subjectCopy.guardStateSatisfied
  if (state === 'unsatisfied') return subjectCopy.guardStateUnsatisfied
  if (state === 'empty_scope') return subjectCopy.guardStateEmptyScope
  return subjectCopy.guardStateUnknown
}

function targetLabel(state: string): string {
  return state === 'satisfied' ? subjectCopy.guardStateSatisfied : subjectCopy.guardStateUnsatisfied
}

// WorkflowRequirementsGuardDetails renders the per-requirement satisfaction
// evidence of a guarded plan-unit node (关联需求流程 · 需求达标明细).
export function WorkflowRequirementsGuardDetails({ evaluation }: {
  evaluation: WorkflowRequirementsGuardEvaluation
}) {
  const missingVersions = evaluation.missing_binding_version_ids ?? []
  return (
    <div data-testid="workflow-requirements-guard" className="grid gap-2 rounded-lg border border-border bg-muted/70 p-3 text-xs">
      <div className="flex flex-wrap items-center gap-2">
        <strong className="text-sm text-foreground">{subjectCopy.guardSummary(evaluation.satisfied_count, evaluation.total_count)}</strong>
        {evaluation.unknown_count > 0 ? (
          <span className="text-amber-700 dark:text-amber-400">{subjectCopy.guardUnknownSummary(evaluation.unknown_count)}</span>
        ) : null}
        <StatusPill tone={stateTone(evaluation.state)}>{stateLabel(evaluation.state)}</StatusPill>
      </div>
      {evaluation.state === 'empty_scope' ? null : (
        <ul className="m-0 grid list-none gap-1.5 p-0">
          {evaluation.requirements.map((requirement) => (
            <li key={requirement.requirement_id} className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-card px-2.5 py-1.5">
              <span className="min-w-0 flex-1 truncate font-medium text-foreground">{subjectCopy.guardRequirementLabel(requirement.requirement_id)}</span>
              {requirement.targets?.length ? (
                <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-muted-foreground">
                  {requirement.targets.map((target) => (
                    <span key={`${requirement.requirement_id}-${target.node_id}`} className="truncate">
                      {target.name || target.node_id} {subjectCopy.nodeMetaSeparator} {targetLabel(target.state)}
                    </span>
                  ))}
                </span>
              ) : null}
              {requirement.reason ? <span className="text-amber-700 dark:text-amber-400">{reasonCopy[requirement.reason] ?? requirement.reason}</span> : null}
              <StatusPill tone={stateTone(requirement.state)}>{stateLabel(requirement.state)}</StatusPill>
            </li>
          ))}
        </ul>
      )}
      {missingVersions.length > 0 ? (
        <p className="m-0 text-amber-700 dark:text-amber-400">{subjectCopy.guardMissingBindingHint}</p>
      ) : null}
    </div>
  )
}
