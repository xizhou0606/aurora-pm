import { copy } from '@/shared/copy'
import type { ScrumVersionTaskNew } from '../api'

const overviewCopy = copy.projectManagement.overview

// DeliveryProgressCard shows the workflow-linked delivery view (阶段 B)：
// 交付进度完全来自需求有效证据，验收条数独立于加权进度展示。
export function DeliveryProgressCard({ unit }: { unit: ScrumVersionTaskNew }) {
  if (!unit.delivery_progress_configured) return null
  const progress = Math.min(100, Math.max(0, unit.delivery_progress ?? 0))
  return (
    <section
      data-testid="pm-delivery-progress"
      aria-label={overviewCopy.deliveryTitle}
      className="grid gap-2 rounded-lg border border-border bg-card p-4 sm:p-5"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="m-0 text-sm font-semibold text-foreground">{overviewCopy.deliveryTitle}</h3>
        <span className="text-xs text-muted-foreground">
          {unit.delivery_scope_count !== undefined ? overviewCopy.deliveryScopeCount(unit.delivery_scope_count) : null}
        </span>
      </div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <strong className="text-lg font-semibold text-foreground">{overviewCopy.deliveryProgressLabel(progress)}</strong>
        <span className="text-xs text-muted-foreground">
          {unit.delivery_acceptance_configured
            ? overviewCopy.deliveryAcceptance(unit.delivery_accepted_count ?? 0, unit.delivery_scope_count ?? 0)
            : overviewCopy.deliveryAcceptanceMissing}
        </span>
        {unit.delivery_unknown_count ? (
          <span className="text-xs text-amber-700 dark:text-amber-400">{overviewCopy.deliveryUnknown(unit.delivery_unknown_count)}</span>
        ) : null}
      </div>
      <div
        role="progressbar"
        aria-valuenow={progress}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label={overviewCopy.deliveryTitle}
        className="h-2 w-full overflow-hidden rounded-full bg-muted"
      >
        <div
          className="h-full w-(--delivery-progress-width) rounded-full bg-emerald-600 dark:bg-emerald-500"
          style={{ '--delivery-progress-width': `${progress}%` }}
        />
      </div>
      {unit.delivery_milestones?.length ? (
        <ul className="m-0 grid list-none flex-wrap gap-x-4 gap-y-1 p-0 text-xs text-muted-foreground sm:flex">
          {unit.delivery_milestones.map((milestone) => (
            <li key={milestone.key}>
              {milestone.name || milestone.key} {copy.aiPM.workflowSubject.nodeMetaSeparator} {milestone.satisfied_count} {copy.aiPM.requirementPool.scopeSeparator} {unit.delivery_scope_count ?? 0}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  )
}
