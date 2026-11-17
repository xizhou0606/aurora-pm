import { Tooltip } from '@base-ui/react/tooltip'

import { cn } from '@/shared/lib/utils'

import { useOverlayPortalContainer } from './overlay-portal-container'

export type HoverPreviewTextProps = {
  text: string
  className?: string
}

export function HoverPreviewText({ text, className }: HoverPreviewTextProps) {
  const portalContainer = useOverlayPortalContainer()

  return (
    <Tooltip.Root>
      <Tooltip.Trigger
        render={<span />}
        delay={150}
        aria-label={text}
        className={cn('block min-w-0 truncate', className)}
      >
        {text}
      </Tooltip.Trigger>
      <Tooltip.Portal container={portalContainer ?? undefined}>
        <Tooltip.Positioner
          side="bottom"
          sideOffset={6}
          className="z-80"
          positionMethod="fixed"
        >
          <Tooltip.Popup className="max-w-96 break-words rounded-md border border-border bg-card px-3 py-2 text-sm text-card-foreground shadow-[var(--design-shadow-popover)] outline-none">
            {text}
          </Tooltip.Popup>
        </Tooltip.Positioner>
      </Tooltip.Portal>
    </Tooltip.Root>
  )
}
