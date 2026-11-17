import { Children, Fragment, isValidElement, type CSSProperties, type ReactNode } from 'react'

import { PhX } from '@/shared/ui/icons'
import { cn } from '@/shared/lib/utils'
import { copy } from '@/shared/copy'
import { Button } from '../primitives/button'
import { FilterBar } from './filter-bar'

export type AdaptiveFilterControl = {
  id: string
  label: string
  node: ReactNode
  placement?: 'search' | 'field' | 'action'
}
export type ActiveFilter = { id: string; label: string; onRemove: () => void }

function actionItems(nodes: ReactNode): ReactNode[] {
  return Children.toArray(nodes).flatMap((node) => isValidElement<{ children?: ReactNode }>(node) && node.type === Fragment
    ? actionItems(node.props.children) : [node])
}

/** The container owns presentation only; values and requests remain caller-owned. */
export function AdaptiveFilterBar({ controls, activeFilters = [], onReset, endActions, leading, maxColumns = 6, 'aria-label': ariaLabel }: {
  controls: readonly AdaptiveFilterControl[]
  activeFilters?: readonly ActiveFilter[]
  onReset?: () => void
  leading?: ReactNode
  endActions?: ReactNode
  maxColumns?: 6 | 7
  'aria-label'?: string
}) {
  const fields = controls.filter((control) => control.placement !== 'action')
  const actions = controls.filter((control) => control.placement === 'action')
  const endItems = actionItems(endActions)
  const slotCount = fields.length
  const count = activeFilters.length
  const columns = (limit: number) => {
    const size = Math.max(slotCount, 1)
    return `repeat(${Math.ceil(size / Math.ceil(size / limit))}, minmax(0, 1fr))`
  }
  return (
    <div data-slot="adaptive-filter-bar" className="@container grid min-w-0 gap-2">
      <FilterBar
        aria-label={ariaLabel}
        className="sm:flex-col sm:items-stretch"
        controls={(
          <div className="flex w-full min-w-0 flex-wrap items-start gap-3">
          {leading ? <div data-slot="filter-leading-controls" className="flex max-w-full shrink-0 items-center gap-2">{leading}</div> : null}
          <div className={cn('@container min-w-0 flex-1', leading && fields.length === 1 ? 'basis-56 max-w-64' : 'basis-96')}>
          <div data-slot="filter-field-grid" style={{ '--filter-grid-narrow': columns(2), '--filter-grid-medium': columns(3), '--filter-grid-large': columns(4), '--filter-grid-wide': columns(6), '--filter-grid-expanded': columns(maxColumns) } as CSSProperties}
            className="grid w-full min-w-0 grid-cols-1 gap-2 @min-[24rem]:grid-cols-(--filter-grid-narrow) @min-[40rem]:grid-cols-(--filter-grid-medium) @min-[52rem]:grid-cols-(--filter-grid-large) @min-[78rem]:grid-cols-(--filter-grid-wide) @min-[84rem]:grid-cols-(--filter-grid-expanded)">
            {fields.map((control) => <div key={control.id} className={control.placement === 'search' && slotCount > 1
              ? 'min-w-0 @min-[24rem]:col-span-2 @min-[40rem]:col-span-1' : 'min-w-0'}>{control.node}</div>)}
          </div>
          </div>
          {endItems.length > 0 ? <div data-slot="filter-end-actions" className="ml-auto flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">{endItems}</div> : null}
          </div>
        )}
        actions={actions.length > 0 || (count > 0 && onReset) ? (
          <>
            {actions.map((control) => <div key={control.id}>{control.node}</div>)}
            {count > 0 && onReset ? <Button variant="ghost" size="sm" onClick={onReset}>{copy.ui.filter.reset}</Button> : null}
          </>
        ) : undefined}
      />
      {count > 0 ? <div className="flex min-w-0 flex-wrap gap-1.5" aria-label={copy.ui.filter.active}>
        {activeFilters.map((filter) => <Button key={filter.id} variant="outline" size="sm"
          className="h-7 max-w-full gap-2 text-xs" onClick={filter.onRemove}
          aria-label={copy.ui.filter.remove(filter.label)}>
          <span className="truncate">{filter.label}</span><PhX aria-hidden="true" className="size-3 shrink-0" />
        </Button>)}
      </div> : null}

    </div>
  )
}
