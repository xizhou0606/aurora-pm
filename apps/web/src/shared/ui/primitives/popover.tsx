import { Popover as PopoverPrimitive } from '@base-ui/react/popover'

import { cn } from '@/shared/lib/utils'

import { useOverlayPortalContainer } from '../misc/overlay-portal-container'

const Popover = PopoverPrimitive.Root
const PopoverTrigger = PopoverPrimitive.Trigger
const PopoverClose = PopoverPrimitive.Close

function PopoverContent({
  anchor,
  container,
  align = 'start',
  children,
  className,
  positionerClassName,
  side = 'bottom',
  sideOffset = 4,
  ...props
}: PopoverPrimitive.Popup.Props & {
  anchor?: PopoverPrimitive.Positioner.Props['anchor']
  container?: HTMLElement
  align?: PopoverPrimitive.Positioner.Props['align']
  positionerClassName?: string
  side?: PopoverPrimitive.Positioner.Props['side']
  sideOffset?: PopoverPrimitive.Positioner.Props['sideOffset']
}) {
  const portalContainer = useOverlayPortalContainer()

  return (
    <PopoverPrimitive.Portal container={container ?? portalContainer ?? undefined}>
      <PopoverPrimitive.Positioner
        anchor={anchor}
        align={align}
        data-slot="popover-positioner"
        className={cn('z-50 outline-none', positionerClassName)}
        positionMethod="fixed"
        side={side}
        sideOffset={sideOffset}
      >
        <PopoverPrimitive.Popup
          data-slot="popover-content"
          className={cn(
            'max-h-[min(32rem,var(--available-height))] max-w-[calc(100vw-1rem)] origin-[var(--transform-origin)] overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-[var(--design-shadow-popover)] outline-none',
            'transition-control data-ending-style:scale-98 data-ending-style:opacity-0 data-starting-style:scale-98 data-starting-style:opacity-0',
            className,
          )}
          {...props}
        >
          {children}
        </PopoverPrimitive.Popup>
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  )
}

function PopoverTitle({ className, ...props }: PopoverPrimitive.Title.Props) {
  return (
    <PopoverPrimitive.Title
      className={cn('text-sm font-semibold text-foreground', className)}
      {...props}
    />
  )
}

function PopoverDescription({
  className,
  ...props
}: PopoverPrimitive.Description.Props) {
  return (
    <PopoverPrimitive.Description
      className={cn('text-xs text-muted-foreground', className)}
      {...props}
    />
  )
}

export {
  Popover,
  PopoverClose,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
}
