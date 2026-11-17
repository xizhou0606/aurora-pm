import { Combobox as ComboboxPrimitive } from '@base-ui/react/combobox'
import { PhCheck, PhCaretDown, PhX } from '@/shared/ui/icons'

import { cn } from '@/shared/lib/utils'

import { useOverlayPortalContainer } from '../misc/overlay-portal-container'

const Combobox = ComboboxPrimitive.Root

function ComboboxClear({
  className,
  children,
  ...props
}: ComboboxPrimitive.Clear.Props) {
  return (
    <ComboboxPrimitive.Clear
      data-slot="combobox-clear"
      className={cn(
        'absolute right-7 top-1/2 z-10 grid size-6 -translate-y-1/2 cursor-pointer place-items-center rounded-sm border-0 bg-transparent p-0 text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    >
      {children ?? <PhX aria-hidden="true" className="size-3.5" />}
    </ComboboxPrimitive.Clear>
  )
}

function ComboboxInputGroup({
  className,
  ...props
}: ComboboxPrimitive.InputGroup.Props) {
  return (
    <ComboboxPrimitive.InputGroup
      data-slot="combobox-input-group"
      className={cn(
        'relative flex h-8 min-w-0 items-center rounded-lg border border-input bg-background shadow-[var(--design-shadow-card)] transition-control',
        'focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/25 focus-within:shadow-none',
        'has-[[aria-invalid=true]]:border-destructive has-[[aria-invalid=true]]:ring-3 has-[[aria-invalid=true]]:ring-destructive/20',
        'has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-50',
        className,
      )}
      {...props}
    />
  )
}

function ComboboxInput({
  className,
  ...props
}: ComboboxPrimitive.Input.Props) {
  return (
    <ComboboxPrimitive.Input
      data-slot="combobox-input"
      className={cn(
        'h-full min-w-0 flex-1 border-0 bg-transparent px-2.5 py-1 text-sm text-foreground outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed',
        className,
      )}
      {...props}
    />
  )
}

function ComboboxTrigger({
  className,
  children,
  ...props
}: ComboboxPrimitive.Trigger.Props) {
  return (
    <ComboboxPrimitive.Trigger
      data-slot="combobox-trigger"
      className={cn(
        'flex h-full w-8 shrink-0 cursor-pointer items-center justify-center border-0 bg-transparent p-0 text-muted-foreground outline-none',
        'hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed',
        className,
      )}
      {...props}
    >
      {children ?? <PhCaretDown aria-hidden="true" className="size-4" />}
    </ComboboxPrimitive.Trigger>
  )
}

type ComboboxContentProps = ComboboxPrimitive.Popup.Props & {
  align?: ComboboxPrimitive.Positioner.Props['align']
  alignOffset?: ComboboxPrimitive.Positioner.Props['alignOffset']
  positionerClassName?: string
  side?: ComboboxPrimitive.Positioner.Props['side']
  sideOffset?: ComboboxPrimitive.Positioner.Props['sideOffset']
}

function ComboboxContent({
  align = 'start',
  alignOffset,
  children,
  className,
  positionerClassName,
  side = 'bottom',
  sideOffset = 4,
  ...props
}: ComboboxContentProps) {
  const portalContainer = useOverlayPortalContainer()

  return (
    <ComboboxPrimitive.Portal container={portalContainer ?? undefined}>
      <ComboboxPrimitive.Positioner
        align={align}
        alignOffset={alignOffset}
        data-slot="combobox-positioner"
        className={cn('z-70 outline-none', positionerClassName)}
        positionMethod="fixed"
        side={side}
        sideOffset={sideOffset}
      >
        <ComboboxPrimitive.Popup
          data-slot="combobox-content"
          className={cn(
            'w-[var(--anchor-width)] max-w-[calc(100vw-1rem)] origin-[var(--transform-origin)] overflow-hidden rounded-lg border border-border bg-popover text-popover-foreground shadow-[var(--design-shadow-popover)] outline-none',
            'transition-control data-ending-style:scale-98 data-ending-style:opacity-0 data-starting-style:scale-98 data-starting-style:opacity-0',
            className,
          )}
          {...props}
        >
          {children}
        </ComboboxPrimitive.Popup>
      </ComboboxPrimitive.Positioner>
    </ComboboxPrimitive.Portal>
  )
}

function ComboboxEmpty({
  className,
  ...props
}: ComboboxPrimitive.Empty.Props) {
  return (
    <ComboboxPrimitive.Empty
      data-slot="combobox-empty"
      className={cn(
        'px-3 py-5 text-center text-sm text-muted-foreground empty:hidden',
        className,
      )}
      {...props}
    />
  )
}

function ComboboxList({
  className,
  ...props
}: ComboboxPrimitive.List.Props) {
  return (
    <ComboboxPrimitive.List
      data-slot="combobox-list"
      className={cn(
        'relative max-h-[min(20rem,var(--available-height))] overflow-y-auto overscroll-contain p-1 scroll-py-1 outline-none data-empty:p-0',
        className,
      )}
      {...props}
    />
  )
}

function ComboboxItem({
  className,
  children,
  ...props
}: ComboboxPrimitive.Item.Props) {
  return (
    <ComboboxPrimitive.Item
      data-slot="combobox-item"
      className={cn(
        'grid min-h-8 cursor-pointer grid-cols-[1rem_minmax(0,1fr)] items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none select-none',
        'data-highlighted:bg-accent data-highlighted:text-accent-foreground data-disabled:cursor-not-allowed data-disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <ComboboxPrimitive.ItemIndicator
        data-slot="combobox-item-indicator"
        className="col-start-1"
      >
        <PhCheck aria-hidden="true" className="size-3.5" />
      </ComboboxPrimitive.ItemIndicator>
      <span className="col-start-2 truncate">{children}</span>
    </ComboboxPrimitive.Item>
  )
}

export {
  Combobox,
  ComboboxClear,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxInputGroup,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
}
export type { ComboboxContentProps }
