import type { ReactNode } from 'react'
import { createPortal } from 'react-dom'

import { Button } from '../primitives/button'

/** Presentation only: callers own selection scope, permissions and mutations. */
export function BulkActionBar({
  selectedLabel,
  selectAllLabel,
  invertSelectionLabel,
  onSelectAll,
  onInvertSelection,
  controls,
  actions,
  children,
  'data-testid': testId,
}: {
  selectedLabel: string
  selectAllLabel: string
  invertSelectionLabel: string
  onSelectAll: () => void
  onInvertSelection: () => void
  controls?: ReactNode
  actions: ReactNode
  children?: ReactNode
  'data-testid'?: string
}) {
  const bar = (
    <section
      data-testid={testId}
      data-slot="bulk-action-bar"
      className="fixed inset-x-4 bottom-4 z-50 mx-auto flex w-fit max-w-[calc(100vw-2rem)] min-w-0 flex-wrap items-center gap-4 rounded-lg border border-border bg-card px-4 py-3 text-card-foreground shadow-[var(--design-shadow-popover)] max-[720px]:inset-x-3 max-[720px]:bottom-3 max-[720px]:w-[calc(100vw-1.5rem)] max-[720px]:gap-2"
      aria-label={selectedLabel}
    >
      <div className="flex shrink-0 items-center gap-1" data-slot="bulk-selection-actions">
        <Button type="button" size="sm" variant="link" className="px-1.5" onClick={onSelectAll}>
          {selectAllLabel}
        </Button>
        <Button type="button" size="sm" variant="link" className="px-1.5" onClick={onInvertSelection}>
          {invertSelectionLabel}
        </Button>
      </div>
      <div className="flex min-w-0 items-center gap-2 max-[720px]:order-3 max-[720px]:w-full" data-slot="bulk-status-control">
        <span className="shrink-0 text-sm">{selectedLabel}</span>
        {controls}
      </div>
      <div className="ml-auto flex shrink-0 items-center gap-2" data-slot="bulk-mutation-actions">
        {actions}
      </div>
      {children}
    </section>
  )
  return typeof document === 'undefined' ? bar : createPortal(bar, document.body)
}
