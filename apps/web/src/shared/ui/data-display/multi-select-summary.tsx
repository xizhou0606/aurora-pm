import { Tooltip } from '@base-ui/react/tooltip'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { useOverlayPortalContainer } from '../misc/overlay-portal-container'

export type MultiSelectSummaryProps = {
  label: string
  values: readonly string[]
  className?: string
}

export function MultiSelectSummary({
  label,
  values,
  className,
}: MultiSelectSummaryProps) {
  const summary = values.join('、')
  const portalContainer = useOverlayPortalContainer()

  return (
    <Tooltip.Root>
      <Tooltip.Trigger
        render={<span />}
        delay={150}
        aria-label={copy.ui.userSelect.selectedOptions(label, summary)}
        className={cn('min-w-0 flex-1 truncate text-left', className)}
      >
        {summary}
      </Tooltip.Trigger>
      <Tooltip.Portal container={portalContainer ?? undefined}>
        <Tooltip.Positioner
          side="bottom"
          sideOffset={6}
          className="z-80"
          positionMethod="fixed"
        >
          <Tooltip.Popup className="max-h-64 w-max min-w-48 max-w-72 overflow-y-auto rounded-md border border-border bg-card text-card-foreground shadow-[var(--design-shadow-popover)] outline-none">
            <p className="sticky top-0 z-1 m-0 border-b border-border bg-card px-3 py-2 text-xs font-medium text-muted-foreground">
              {copy.ui.userSelect.selected}
            </p>
            <ul className="m-0 grid list-none gap-1 p-2">
              {values.map((value) => (
                <li key={value} className="rounded-sm px-2 py-1.5 text-sm">
                  {value}
                </li>
              ))}
            </ul>
          </Tooltip.Popup>
        </Tooltip.Positioner>
      </Tooltip.Portal>
    </Tooltip.Root>
  )
}
