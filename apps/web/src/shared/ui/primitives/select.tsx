import { Select as SelectPrimitive } from '@base-ui/react/select'
import { PhCheck, PhCaretDown, PhCaretUp } from '@/shared/ui/icons'

import { cn } from '@/shared/lib/utils'

import { useOverlayPortalContainer } from '../misc/overlay-portal-container'

const Select = SelectPrimitive.Root

function SelectLabel({
  className,
  ...props
}: SelectPrimitive.Label.Props) {
  return (
    <SelectPrimitive.Label
      data-slot="select-label"
      className={cn(
        'hidden cursor-default text-xs font-medium leading-none text-foreground',
        className,
      )}
      {...props}
    />
  )
}

function SelectTrigger({
  className,
  children,
  ...props
}: SelectPrimitive.Trigger.Props) {
  return (
    <SelectPrimitive.Trigger
      data-slot="select-trigger"
      className={cn(
        'flex h-8 min-w-0 cursor-pointer items-center justify-between gap-2 rounded-lg border border-input bg-background px-2.5 py-1 text-sm text-foreground shadow-[var(--design-shadow-card)] outline-none transition-control dark:bg-input/30 select-none',
        'hover:not-data-disabled:bg-muted data-popup-open:bg-muted',
        'focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/25 focus-visible:shadow-none',
        'data-disabled:cursor-not-allowed data-disabled:opacity-50',
        'aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20',
        className,
      )}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon
        data-slot="select-icon"
        className="shrink-0 text-muted-foreground"
      >
        <PhCaretDown aria-hidden="true" className="size-4" />
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  )
}

function SelectValue({
  className,
  ...props
}: SelectPrimitive.Value.Props) {
  return (
    <SelectPrimitive.Value
      data-slot="select-value"
      className={cn(
        'min-w-0 flex-1 truncate text-left data-placeholder:text-muted-foreground',
        className,
      )}
      {...props}
    />
  )
}

type SelectContentProps = SelectPrimitive.Popup.Props & {
  align?: SelectPrimitive.Positioner.Props['align']
  alignOffset?: SelectPrimitive.Positioner.Props['alignOffset']
  positionerClassName?: string
  side?: SelectPrimitive.Positioner.Props['side']
  sideOffset?: SelectPrimitive.Positioner.Props['sideOffset']
}

function SelectContent({
  align = 'start',
  alignOffset,
  children,
  className,
  positionerClassName,
  side = 'bottom',
  sideOffset = 4,
  ...props
}: SelectContentProps) {
  const portalContainer = useOverlayPortalContainer()

  return (
    <SelectPrimitive.Portal container={portalContainer ?? undefined}>
      <SelectPrimitive.Positioner
        align={align}
        alignItemWithTrigger={false}
        alignOffset={alignOffset}
        data-slot="select-positioner"
        className={cn('z-50 outline-none select-none', positionerClassName)}
        positionMethod="fixed"
        side={side}
        sideOffset={sideOffset}
      >
        <SelectPrimitive.Popup
          data-slot="select-content"
          className={cn(
            'group relative w-[var(--anchor-width)] max-w-[calc(100vw-1rem)] origin-[var(--transform-origin)] overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-[var(--design-shadow-popover)] outline-none',
            'transition-control data-ending-style:scale-98 data-ending-style:opacity-0 data-starting-style:scale-98 data-starting-style:opacity-0',
            className,
          )}
          {...props}
        >
          <SelectPrimitive.ScrollUpArrow
            data-slot="select-scroll-up-arrow"
            className="top-0 left-0 z-10 flex h-5 w-full cursor-default items-center justify-center bg-popover text-muted-foreground"
          >
            <PhCaretUp aria-hidden="true" className="size-3.5" />
          </SelectPrimitive.ScrollUpArrow>
          <SelectPrimitive.List
            data-slot="select-list"
            className="relative max-h-[min(20rem,var(--available-height))] overflow-y-auto overscroll-contain p-1 scroll-py-1 outline-none"
          >
            {children}
          </SelectPrimitive.List>
          <SelectPrimitive.ScrollDownArrow
            data-slot="select-scroll-down-arrow"
            className="bottom-0 left-0 z-10 flex h-5 w-full cursor-default items-center justify-center bg-popover text-muted-foreground"
          >
            <PhCaretDown aria-hidden="true" className="size-3.5" />
          </SelectPrimitive.ScrollDownArrow>
        </SelectPrimitive.Popup>
      </SelectPrimitive.Positioner>
    </SelectPrimitive.Portal>
  )
}

function SelectItem({
  className,
  children,
  ...props
}: SelectPrimitive.Item.Props) {
  return (
    <SelectPrimitive.Item
      data-slot="select-item"
      className={cn(
        'grid min-h-8 cursor-pointer grid-cols-[1rem_minmax(0,1fr)] items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none select-none',
        'data-highlighted:bg-accent data-highlighted:text-accent-foreground data-disabled:cursor-not-allowed data-disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <SelectPrimitive.ItemIndicator
        data-slot="select-item-indicator"
        className="col-start-1"
      >
        <PhCheck aria-hidden="true" className="size-3.5" />
      </SelectPrimitive.ItemIndicator>
      <SelectPrimitive.ItemText
        data-slot="select-item-text"
        className="col-start-2 truncate"
      >
        {children}
      </SelectPrimitive.ItemText>
    </SelectPrimitive.Item>
  )
}

export {
  Select,
  SelectContent,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
}
export type { SelectContentProps }
