import { ContextMenu as ContextMenuPrimitive } from '@base-ui/react/context-menu'
import { forwardRef } from 'react'

import { cn } from '@/shared/lib/utils'

import { useOverlayPortalContainer } from '../misc/overlay-portal-container'

const ContextMenu = ContextMenuPrimitive.Root

const ContextMenuTrigger = forwardRef<
  HTMLDivElement,
  ContextMenuPrimitive.Trigger.Props
>(function ContextMenuTrigger({ onKeyDown, tabIndex = 0, ...props }, ref) {
  return (
    <ContextMenuPrimitive.Trigger
      ref={ref}
      tabIndex={tabIndex}
      onKeyDown={(event) => {
        onKeyDown?.(event)
        if (event.defaultPrevented) return
        if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return
        event.preventDefault()
        const bounds = event.currentTarget.getBoundingClientRect()
        event.currentTarget.dispatchEvent(new MouseEvent('contextmenu', {
          bubbles: true,
          cancelable: true,
          clientX: bounds.left,
          clientY: bounds.bottom,
        }))
      }}
      {...props}
    />
  )
})

function ContextMenuContent({
  children,
  className,
  onFocus,
  positionerClassName,
  ...props
}: ContextMenuPrimitive.Popup.Props & {
  positionerClassName?: string
}) {
  const portalContainer = useOverlayPortalContainer()

  return (
    <ContextMenuPrimitive.Portal container={portalContainer ?? undefined}>
      <ContextMenuPrimitive.Positioner
        className={cn('z-50 outline-none', positionerClassName)}
        collisionPadding={8}
        positionMethod="fixed"
      >
        <ContextMenuPrimitive.Popup
          ref={(node) => {
            if (!node) return
            window.requestAnimationFrame(() => {
              node.querySelector<HTMLElement>('[role="menuitem"]:not([data-disabled])')?.focus()
            })
          }}
          data-slot="context-menu-content"
          className={cn(
            'min-w-44 max-w-[calc(100vw-1rem)] origin-[var(--transform-origin)] overflow-hidden rounded-lg border border-border bg-popover p-1 text-popover-foreground shadow-[var(--design-shadow-popover)] outline-none',
            'transition-control data-ending-style:scale-98 data-ending-style:opacity-0 data-starting-style:scale-98 data-starting-style:opacity-0',
            className,
          )}
          onFocus={(event) => {
            onFocus?.(event)
            if (event.defaultPrevented || event.target !== event.currentTarget) return
            event.currentTarget
              .querySelector<HTMLElement>('[role="menuitem"]:not([data-disabled])')
              ?.focus()
          }}
          {...props}
        >
          {children}
        </ContextMenuPrimitive.Popup>
      </ContextMenuPrimitive.Positioner>
    </ContextMenuPrimitive.Portal>
  )
}

function ContextMenuItem({
  className,
  destructive = false,
  ...props
}: ContextMenuPrimitive.Item.Props & { destructive?: boolean }) {
  return (
    <ContextMenuPrimitive.Item
      data-slot="context-menu-item"
      data-destructive={destructive || undefined}
      className={cn(
        'flex min-h-7 cursor-default select-none items-center gap-1.5 rounded-sm px-1.5 py-1 text-xs leading-5 outline-none [&_svg]:size-4 [&_svg]:shrink-0',
        'hover:bg-muted hover:text-foreground focus-visible:bg-muted focus-visible:text-foreground data-disabled:pointer-events-none data-disabled:opacity-45 data-highlighted:bg-accent data-highlighted:text-accent-foreground',
        'data-[destructive]:text-destructive data-[destructive]:hover:bg-destructive/10 data-[destructive]:hover:text-destructive data-[destructive]:focus-visible:bg-destructive/10 data-[destructive]:focus-visible:text-destructive data-[destructive]:data-highlighted:bg-destructive/10 data-[destructive]:data-highlighted:text-destructive',
        className,
      )}
      {...props}
    />
  )
}

function ContextMenuCheckboxItem({
  className,
  ...props
}: ContextMenuPrimitive.CheckboxItem.Props) {
  return (
    <ContextMenuPrimitive.CheckboxItem
      data-slot="context-menu-checkbox-item"
      className={cn(
        'flex min-h-7 cursor-default select-none items-center gap-1.5 rounded-sm px-1.5 py-1 text-xs leading-5 outline-none [&_svg]:size-4 [&_svg]:shrink-0',
        'hover:bg-muted hover:text-foreground focus-visible:bg-muted focus-visible:text-foreground data-disabled:pointer-events-none data-disabled:opacity-45 data-highlighted:bg-accent data-highlighted:text-accent-foreground',
        className,
      )}
      {...props}
    />
  )
}

function ContextMenuCheckboxIndicator({
  className,
  ...props
}: ContextMenuPrimitive.CheckboxItemIndicator.Props) {
  return (
    <ContextMenuPrimitive.CheckboxItemIndicator
      data-slot="context-menu-checkbox-indicator"
      className={cn('ml-auto flex size-4 items-center justify-center', className)}
      {...props}
    />
  )
}

function ContextMenuSeparator({
  className,
  ...props
}: ContextMenuPrimitive.Separator.Props) {
  return (
    <ContextMenuPrimitive.Separator
      data-slot="context-menu-separator"
      className={cn('-mx-1 my-0.5 h-px bg-border', className)}
      {...props}
    />
  )
}

export {
  ContextMenu,
  ContextMenuCheckboxIndicator,
  ContextMenuCheckboxItem,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
}
