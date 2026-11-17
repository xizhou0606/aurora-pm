import { PhCheckCircle, PhArrowCounterClockwise, PhPaperPlaneTilt, PhUserGear } from '@/shared/ui/icons'

import { QUALITY_COPY } from '@/shared/copy'
import { Button } from '@/shared/ui'
import type { QualityLifecycleAction, QualityLifecycleModel } from '../../model/types'

function ActionIcon({ action }: { action: QualityLifecycleAction }) {
  if (action.kind === 'assign') return <PhUserGear aria-hidden="true" />
  if (action.kind === 'return') return <PhArrowCounterClockwise aria-hidden="true" />
  if (action.kind === 'resolve') return <PhCheckCircle aria-hidden="true" />
  return action.status === 'done' || action.status === 'closed'
    ? <PhCheckCircle aria-hidden="true" />
    : <PhPaperPlaneTilt aria-hidden="true" />
}

export function LifecycleBar({
  model,
  onAction,
}: {
  model: QualityLifecycleModel
  onAction: (action: QualityLifecycleAction) => void
}) {
  if (!model.actions.length) return null

  return (
    <nav
      className="sticky bottom-3 z-20 mx-auto mt-4 flex w-fit max-w-[calc(100%-1rem)] flex-col items-center gap-1 rounded-lg border border-border bg-card p-1.5 shadow-[var(--design-shadow-popover)]"
      aria-label={QUALITY_COPY.lifecycle.title}
      title={model.currentLabel}
    >
      <div className="flex max-w-full flex-nowrap items-center gap-1 overflow-x-auto overscroll-x-contain">
        {model.actions.map((action) => (
          <Button
            key={action.id}
            type="button"
            size="sm"
            variant="ghost"
            className="shrink-0 gap-2 px-3"
            disabled={action.disabled}
            onClick={() => onAction(action)}
          >
            <span className="grid size-7 shrink-0 place-items-center rounded-full bg-muted text-muted-foreground [&>svg]:size-4">
              <ActionIcon action={action} />
            </span>
            {action.label}
          </Button>
        ))}
      </div>
    </nav>
  )
}
